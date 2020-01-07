package initialsync

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"sort"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/paulbellamy/ratecounter"
	"github.com/pkg/errors"
	eth "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	prysmsync "github.com/prysmaticlabs/prysm/beacon-chain/sync"
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/slotutil"
	"github.com/sirupsen/logrus"
)

const blockBatchSize = 64
const counterSeconds = 20
const refreshTime = 6 * time.Second

// Round Robin sync looks at the latest peer statuses and syncs with the highest
// finalized peer.
//
// Step 1 - Sync to finalized epoch.
// Sync with peers of lowest finalized root with epoch greater than head state.
//
// Step 2 - Sync to head from finalized epoch.
// Using the finalized root as the head_block_root and the epoch start slot
// after the finalized epoch, request blocks to head from some subset of peers
// where step = 1.
func (s *Service) roundRobinSync(genesis time.Time) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	counter := ratecounter.NewRateCounter(counterSeconds * time.Second)
	randGenerator := rand.New(rand.NewSource(time.Now().Unix()))
	var lastEmptyRequests int
	// Step 1 - Sync to end of finalized epoch.
	for s.chain.HeadSlot() < helpers.StartSlot(s.highestFinalizedEpoch()+1) {
		root, finalizedEpoch, peers := s.p2p.Peers().BestFinalized(params.BeaconConfig().MaxPeersToSync)
		if len(peers) == 0 {
			log.Warn("No peers; waiting for reconnect")
			time.Sleep(refreshTime)
			continue
		}

		// shuffle peers to prevent a bad peer from
		// stalling sync with invalid blocks
		randGenerator.Shuffle(len(peers), func(i, j int) {
			peers[i], peers[j] = peers[j], peers[i]
		})

		// request a range of blocks to be requested from multiple peers.
		// Example:
		//   - number of peers = 4
		//   - range of block slots is 64...128
		//   Four requests will be spread across the peers using step argument to distribute the load
		//   i.e. the first peer is asked for block 64, 68, 72... while the second peer is asked for
		//   65, 69, 73... and so on for other peers.
		var request func(start uint64, step uint64, count uint64, peers []peer.ID, remainder int) ([]*eth.BeaconBlock, error)
		request = func(start uint64, step uint64, count uint64, peers []peer.ID, remainder int) ([]*eth.BeaconBlock, error) {
			if len(peers) == 0 {
				return nil, errors.WithStack(errors.New("no peers left to request blocks"))
			}
			var p2pRequestCount int32
			errChan := make(chan error)
			blocksChan := make(chan []*eth.BeaconBlock)

			// Handle block large block ranges of skipped slots.
			start += count * uint64(lastEmptyRequests*len(peers))
			if count <= 1 {
				step = 1
			}

			// Short circuit start far exceeding the highest finalized epoch in some infinite loop.
			if start > helpers.StartSlot(s.highestFinalizedEpoch()+1) {
				return nil, errors.Errorf("attempted to ask for a start slot of %d which is greater than the next highest epoch of %d", start, s.highestFinalizedEpoch()+1)
			}

			atomic.AddInt32(&p2pRequestCount, int32(len(peers)))
			for i, pid := range peers {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				start := start + uint64(i)*step
				step := step * uint64(len(peers))
				count := mathutil.Min(count, (helpers.StartSlot(finalizedEpoch+1)-start)/step)
				// If the count was divided by an odd number of peers, there will be some blocks
				// missing from the first requests so we accommodate that scenario.
				if i < remainder {
					count++
				}
				// asking for no blocks may cause the client to hang. This should never happen and
				// the peer may return an error anyway, but we'll ask for at least one block.
				if count == 0 {
					count = 1
				}
				req := &p2ppb.BeaconBlocksByRangeRequest{
					HeadBlockRoot: root,
					StartSlot:     start,
					Count:         count,
					Step:          step,
				}

				go func(i int, pid peer.ID) {
					defer func() {
						zeroIfIAmTheLast := atomic.AddInt32(&p2pRequestCount, -1)
						if zeroIfIAmTheLast == 0 {
							close(blocksChan)
						}
					}()

					resp, err := s.requestBlocks(ctx, req, pid)
					log.WithField("peer", pid.Pretty()).Debugf("Received %d blocks", len(resp))
					if err != nil {
						// fail over to other peers by splitting this requests evenly across them.
						ps := append(peers[:i], peers[i+1:]...)
						log.WithError(err).WithField(
							"remaining peers",
							len(ps),
						).WithField(
							"peer",
							pid.Pretty(),
						).Debug("Request failed, trying to round robin with other peers")
						if len(ps) == 0 {
							errChan <- errors.WithStack(errors.New("no peers left to request blocks"))
							return
						}
						resp, err = request(start, step, count/uint64(len(ps)) /*count*/, ps, int(count)%len(ps) /*remainder*/)
						if err != nil {
							errChan <- err
							return
						}
					}
					blocksChan <- resp
				}(i, pid)
			}

			var unionRespBlocks []*eth.BeaconBlock
			for {
				select {
				case err := <-errChan:
					return nil, err
				case resp, ok := <-blocksChan:
					if ok {
						//  if this synchronization becomes a bottleneck:
						//    think about immediately allocating space for all peers in unionRespBlocks,
						//    and write without synchronization
						unionRespBlocks = append(unionRespBlocks, resp...)
					} else {
						return unionRespBlocks, nil
					}
				}
			}
		}
		startBlock := s.chain.HeadSlot() + 1
		skippedBlocks := blockBatchSize * uint64(lastEmptyRequests*len(peers))
		if startBlock+skippedBlocks > helpers.StartSlot(finalizedEpoch+1) {
			log.WithField("finalizedEpoch", finalizedEpoch).Debug("Requested block range is greater than the finalized epoch")
			break
		}

		blocks, err := request(
			s.chain.HeadSlot()+1, // start
			1,                    // step
			blockBatchSize,       // count
			peers,                // peers
			0,                    // remainder
		)
		if err != nil {
			return err
		}

		// Since the block responses were appended to the list, we must sort them in order to
		// process sequentially. This method doesn't make much wall time compared to block
		// processing.
		sort.Slice(blocks, func(i, j int) bool {
			return blocks[i].Slot < blocks[j].Slot
		})

		for _, blk := range blocks {
			s.logSyncStatus(genesis, blk, peers, counter)
			if !s.db.HasBlock(ctx, bytesutil.ToBytes32(blk.ParentRoot)) {
				log.Debugf("Beacon node doesn't have a block in db with root %#x", blk.ParentRoot)
				continue
			}
			if featureconfig.Get().InitSyncNoVerify {
				if err := s.chain.ReceiveBlockNoVerify(ctx, blk); err != nil {
					return err
				}
			} else {
				if err := s.chain.ReceiveBlockNoPubsubForkchoice(ctx, blk); err != nil {
					return err
				}
			}
		}
		// If there were no blocks in the last request range, increment the counter so the same
		// range isn't requested again on the next loop as the headSlot didn't change.
		if len(blocks) == 0 {
			lastEmptyRequests++
		} else {
			lastEmptyRequests = 0
		}
	}

	log.Debug("Synced to finalized epoch - now syncing blocks up to current head")

	if s.chain.HeadSlot() == slotutil.SlotsSinceGenesis(genesis) {
		return nil
	}

	// Step 2 - sync to head from any single peer.
	// This step might need to be improved for cases where there has been a long period since
	// finality. This step is less important than syncing to finality in terms of threat
	// mitigation. We are already convinced that we are on the correct finalized chain. Any blocks
	// we receive there after must build on the finalized chain or be considered invalid during
	// fork choice resolution / block processing.
	best := s.bestPeer()
	root, _, _ := s.p2p.Peers().BestFinalized(params.BeaconConfig().MaxPeersToSync)

	// if no best peer exists, retry until a new best peer is found.
	for len(best) == 0 {
		time.Sleep(refreshTime)
		best = s.bestPeer()
		root, _, _ = s.p2p.Peers().BestFinalized(params.BeaconConfig().MaxPeersToSync)
	}
	for head := slotutil.SlotsSinceGenesis(genesis); s.chain.HeadSlot() < head; {
		req := &p2ppb.BeaconBlocksByRangeRequest{
			HeadBlockRoot: root,
			StartSlot:     s.chain.HeadSlot() + 1,
			Count:         mathutil.Min(slotutil.SlotsSinceGenesis(genesis)-s.chain.HeadSlot()+1, 256),
			Step:          1,
		}

		log.WithField("req", req).WithField("peer", best.Pretty()).Debug(
			"Sending batch block request",
		)

		resp, err := s.requestBlocks(ctx, req, best)
		if err != nil {
			return err
		}

		for _, blk := range resp {
			s.logSyncStatus(genesis, blk, []peer.ID{best}, counter)
			if err := s.chain.ReceiveBlockNoPubsubForkchoice(ctx, blk); err != nil {
				return err
			}
		}
		if len(resp) == 0 {
			break
		}
	}

	return nil
}

// requestBlocks by range to a specific peer.
func (s *Service) requestBlocks(ctx context.Context, req *p2ppb.BeaconBlocksByRangeRequest, pid peer.ID) ([]*eth.BeaconBlock, error) {
	log.WithFields(logrus.Fields{
		"peer":  pid,
		"start": req.StartSlot,
		"count": req.Count,
		"step":  req.Step,
		"head":  fmt.Sprintf("%#x", req.HeadBlockRoot),
	}).Debug("Requesting blocks")
	stream, err := s.p2p.Send(ctx, req, pid)
	if err != nil {
		return nil, errors.Wrap(err, "failed to send request to peer")
	}
	defer stream.Close()

	resp := make([]*eth.BeaconBlock, 0, req.Count)
	for {
		blk, err := prysmsync.ReadChunkedBlock(stream, s.p2p)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, errors.Wrap(err, "failed to read chunked block")
		}
		resp = append(resp, blk)
	}

	return resp, nil
}

// highestFinalizedEpoch as reported by peers. This is the absolute highest finalized epoch as
// reported by peers.
func (s *Service) highestFinalizedEpoch() uint64 {
	_, epoch, _ := s.p2p.Peers().BestFinalized(params.BeaconConfig().MaxPeersToSync)
	return epoch
}

// bestFinalized returns the highest finalized epoch that is agreed upon by the majority of
// peers. This method may not return the absolute highest finalized, but the finalized epoch in
// which most peers can serve blocks. Ideally, all peers would be reporting the same finalized
// epoch.
// Returns the best finalized root, epoch number, and peers that agree.
func (s *Service) bestFinalized() ([]byte, uint64, []peer.ID) {
	finalized := make(map[[32]byte]uint64)
	rootToEpoch := make(map[[32]byte]uint64)
	for _, pid := range s.p2p.Peers().Connected() {
		peerChainState, err := s.p2p.Peers().ChainState(pid)
		if err == nil && peerChainState != nil {
			r := bytesutil.ToBytes32(peerChainState.FinalizedRoot)
			finalized[r]++
			rootToEpoch[r] = peerChainState.FinalizedEpoch
		}
	}

	var mostVotedFinalizedRoot [32]byte
	var mostVotes uint64
	for root, count := range finalized {
		if count > mostVotes {
			mostVotes = count
			mostVotedFinalizedRoot = root
		}
	}

	var pids []peer.ID
	for _, pid := range s.p2p.Peers().Connected() {
		peerChainState, err := s.p2p.Peers().ChainState(pid)
		if err == nil && peerChainState != nil && peerChainState.FinalizedEpoch >= rootToEpoch[mostVotedFinalizedRoot] {
			pids = append(pids, pid)
			if len(pids) >= params.BeaconConfig().MaxPeersToSync {
				break
			}
		}
	}

	return mostVotedFinalizedRoot[:], rootToEpoch[mostVotedFinalizedRoot], pids
}

// bestPeer returns the peer ID of the peer reporting the highest head slot.
func (s *Service) bestPeer() peer.ID {
	var best peer.ID
	var bestSlot uint64
	for _, k := range s.p2p.Peers().Connected() {
		peerChainState, err := s.p2p.Peers().ChainState(k)
		if err == nil && peerChainState != nil && peerChainState.HeadSlot >= bestSlot {
			bestSlot = peerChainState.HeadSlot
			best = k
		}
	}
	return best
}

// logSyncStatus and increment block processing counter.
func (s *Service) logSyncStatus(genesis time.Time, blk *eth.BeaconBlock, syncingPeers []peer.ID, counter *ratecounter.RateCounter) {
	counter.Incr(1)
	rate := float64(counter.Rate()) / counterSeconds
	if rate == 0 {
		rate = 1
	}
	timeRemaining := time.Duration(float64(slotutil.SlotsSinceGenesis(genesis)-blk.Slot)/rate) * time.Second
	log.WithField(
		"peers",
		fmt.Sprintf("%d/%d", len(syncingPeers), len(s.p2p.Peers().Connected())),
	).WithField(
		"blocksPerSecond",
		fmt.Sprintf("%.1f", rate),
	).Infof(
		"Processing block %d/%d - estimated time remaining %s",
		blk.Slot,
		slotutil.SlotsSinceGenesis(genesis),
		timeRemaining,
	)
}
