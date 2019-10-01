package initialsync

import (
	"context"
	"fmt"
	"io"
	"sort"
	"time"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/paulbellamy/ratecounter"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	prysmsync "github.com/prysmaticlabs/prysm/beacon-chain/sync"
	"github.com/prysmaticlabs/prysm/beacon-chain/sync/peerstatus"
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/mathutil"
)

const blockBatchSize = 64
const counterSeconds = 20

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
func (s *InitialSync) roundRobinSync(genesis time.Time) error {
	ctx := context.Background()

	counter := ratecounter.NewRateCounter(counterSeconds * time.Second)

	var lastEmptyRequests int
	// Step 1 - Sync to end of finalized epoch.
	for s.chain.HeadSlot() < helpers.StartSlot(highestFinalizedEpoch()+1) {
		root, finalizedEpoch, peers := bestFinalized()

		var blocks []*eth.BeaconBlock

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

			// Handle block large block ranges of skipped slots.
			start += count * uint64(lastEmptyRequests*len(peers))

			for i, pid := range peers {
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
						return nil, errors.WithStack(errors.New("no peers left to request blocks"))
					}
					_, err = request(start, step, count/uint64(len(ps)) /*count*/, ps, int(count)%len(ps) /*remainder*/)
					if err != nil {
						return nil, err
					}
				}
				blocks = append(blocks, resp...)
			}

			return blocks, nil
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
		var checkParentExists func(blk *eth.BeaconBlock, parentBlocks []*eth.BeaconBlock) ([]*eth.BeaconBlock, error)
		checkParentExists = func(blk *eth.BeaconBlock, parentBlocks []*eth.BeaconBlock) ([]*eth.BeaconBlock, error) {
			ok, err := s.chain.ParentExists(ctx, blk)
			if err != nil {
				return nil, err
			}
			if !ok {
				bl, err := s.requestBlocksByRoot(ctx, [][32]byte{bytesutil.ToBytes32(blk.ParentRoot)}, peers[0])
				if err != nil {
					return nil, err
				}
				parentBlocks = append(bl, parentBlocks...)

			}
			return parentBlocks, nil
		}

		var receiveBlocks func(blocks []*eth.BeaconBlock) error
		receiveBlocks = func(blocks []*eth.BeaconBlock) error {
			for _, blk := range blocks {
				logSyncStatus(genesis, blk, peers, counter)
				emptyBlk := make([]*eth.BeaconBlock, 0)

				prBlocks, err := checkParentExists(blk, emptyBlk)
				if err != nil {
					return err
				}
				if len(prBlocks) > 0 {
					if err := receiveBlocks(prBlocks); err != nil {
						return err
					}
				}
				if featureconfig.FeatureConfig().InitSyncNoVerify {
					if err := s.chain.ReceiveBlockNoVerify(ctx, blk); err != nil {
						return err
					}
				} else {
					if err := s.chain.ReceiveBlockNoPubsubForkchoice(ctx, blk); err != nil {
						return err
					}
				}
			}
			return nil
		}
		if err := receiveBlocks(blocks); err != nil {
			return err
		}

		// If there were no blocks in the last request range, increment the counter so the same
		// range isn't requested again on the next loop as the headSlot didn't change.
		if len(blocks) == 0 {
			lastEmptyRequests++
		} else {
			lastEmptyRequests = 0
		}
	}

	log.Debug("Synced to finalized epoch. Syncing blocks to head slot now.")

	if s.chain.HeadSlot() == slotsSinceGenesis(genesis) {
		return nil
	}

	// Step 2 - sync to head from any single peer.
	// This step might need to be improved for cases where there has been a long period since
	// finality. This step is less important than syncing to finality in terms of threat
	// mitigation. We are already convinced that we are on the correct finalized chain. Any blocks
	// we receive there after must build on the finalized chain or be considered invalid during
	// fork choice resolution / block processing.
	best := bestPeer()
	root, _, _ := bestFinalized()
	req := &p2ppb.BeaconBlocksByRangeRequest{
		HeadBlockRoot: root,
		StartSlot:     s.chain.HeadSlot() + 1,
		Count:         slotsSinceGenesis(genesis) - s.chain.HeadSlot() + 1,
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
		logSyncStatus(genesis, blk, []peer.ID{best}, counter)
		if err := s.chain.ReceiveBlockNoPubsubForkchoice(ctx, blk); err != nil {
			return err
		}
	}

	return nil
}

// requestBlocks by range to a specific peer.
func (s *InitialSync) requestBlocks(ctx context.Context, req *p2ppb.BeaconBlocksByRangeRequest, pid peer.ID) ([]*eth.BeaconBlock, error) {
	log.WithField("peer", pid.Pretty()).WithField("req", req).Debug("requesting blocks")
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

// requestBlock sends a beacon blocks request to a peer to get
// those corresponding blocks from that peer.
func (r *InitialSync) requestBlocksByRoot(ctx context.Context, blockRoots [][32]byte, id peer.ID) ([]*eth.BeaconBlock, error) {
	log.WithField("peer", id.Pretty()).WithField("roots", blockRoots).Debug("requesting blocks")
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	stream, err := r.p2p.Send(ctx, blockRoots, id)
	if err != nil {
		return nil, err
	}
	resp := make([]*eth.BeaconBlock, 0, len(blockRoots))
	for i := 0; i < len(blockRoots); i++ {
		blk, err := prysmsync.ReadChunkedBlock(stream, r.p2p)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.WithError(err).Error("Unable to retrieve block from stream")
			return nil, err
		}
		resp = append(resp, blk)

	}
	return resp, nil
}

// highestFinalizedEpoch as reported by peers. This is the absolute highest finalized epoch as
// reported by peers.
func highestFinalizedEpoch() uint64 {
	_, epoch, _ := bestFinalized()
	return epoch
}

// bestFinalized returns the highest finalized epoch that is agreed upon by the majority of
// peers. This method may not return the absolute highest finalized, but the finalized epoch in
// which most peers can serve blocks. Ideally, all peers would be reporting the same finalized
// epoch.
// Returns the best finalized root, epoch number, and peers that agree.
func bestFinalized() ([]byte, uint64, []peer.ID) {
	finalized := make(map[[32]byte]uint64)
	rootToEpoch := make(map[[32]byte]uint64)
	for _, k := range peerstatus.Keys() {
		s := peerstatus.Get(k)
		r := bytesutil.ToBytes32(s.FinalizedRoot)
		finalized[r]++
		rootToEpoch[r] = s.FinalizedEpoch
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
	for _, k := range peerstatus.Keys() {
		s := peerstatus.Get(k)
		if s.FinalizedEpoch >= rootToEpoch[mostVotedFinalizedRoot] {
			pids = append(pids, k)
		}
	}

	return mostVotedFinalizedRoot[:], rootToEpoch[mostVotedFinalizedRoot], pids
}

// bestPeer returns the peer ID of the peer reporting the highest head slot.
func bestPeer() peer.ID {
	var best peer.ID
	var bestSlot uint64
	for _, k := range peerstatus.Keys() {
		s := peerstatus.Get(k)
		if s.HeadSlot >= bestSlot {
			bestSlot = s.HeadSlot
			best = k
		}
	}
	return best
}

// logSyncStatus and increment block processing counter.
func logSyncStatus(genesis time.Time, blk *eth.BeaconBlock, peers []peer.ID, counter *ratecounter.RateCounter) {
	counter.Incr(1)
	rate := float64(counter.Rate()) / counterSeconds
	if rate == 0 {
		rate = 1
	}
	timeRemaining := time.Duration(float64(slotsSinceGenesis(genesis)-blk.Slot)/rate) * time.Second
	log.WithField(
		"peers",
		fmt.Sprintf("%d/%d", len(peers), len(peerstatus.Keys())),
	).WithField(
		"blocks per second",
		fmt.Sprintf("%.1f", rate),
	).Infof(
		"Processing block %d/%d. Estimated %s remaining.",
		blk.Slot,
		slotsSinceGenesis(genesis),
		timeRemaining,
	)
}
