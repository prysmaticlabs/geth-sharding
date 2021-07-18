// Package peers provides information about peers at the Ethereum consensus protocol level.
//
// "Protocol level" is the level above the network level, so this layer never sees or interacts with
// (for example) hosts that are uncontactable due to being down, firewalled, etc. Instead, this works
// with peers that are contactable but may or may not be of the correct fork version, not currently
// required due to the number of current connections, etc.
//
// A peer can have one of a number of states:
//
// - connected if we are able to talk to the remote peer
// - connecting if we are attempting to be able to talk to the remote peer
// - disconnecting if we are attempting to stop being able to talk to the remote peer
// - disconnected if we are not able to talk to the remote peer
//
// For convenience, there are two aggregate states expressed in functions:
//
// - active if we are connecting or connected
// - inactive if we are disconnecting or disconnected
//
// Peer information is persistent for the run of the service. This allows for collection of useful
// long-term statistics such as number of bad responses obtained from the peer, giving the basis for
// decisions to not talk to known-bad peers (by de-scoring them).
package peers

import (
	"context"
	"math"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers/peerdata"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p/peers/scorers"
	"github.com/prysmaticlabs/prysm/proto/beacon/p2p"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/rand"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
)

const (
	// PeerDisconnected means there is no connection to the peer.
	PeerDisconnected peerdata.PeerConnectionState = iota
	// PeerDisconnecting means there is an on-going attempt to disconnect from the peer.
	PeerDisconnecting
	// PeerConnected means the peer has an active connection.
	PeerConnected
	// PeerConnecting means there is an on-going attempt to connect to the peer.
	PeerConnecting
)

const (
	// ColocationLimit restricts how many peer identities we can see from a single ip or ipv6 subnet.
	ColocationLimit = 5

	// Additional buffer beyond current peer limit, from which we can store the relevant peer statuses.
	maxLimitBuffer = 150

	// InboundRatio is the proportion of our connected peer limit at which we will allow inbound peers.
	InboundRatio = float64(0.8)

	// MinBackOffDuration minimum amount (in milliseconds) to wait before peer is re-dialed.
	// When node and peer are dialing each other simultaneously connection may fail. In order, to break
	// of constant dialing, peer is assigned some backoff period, and only dialed again once that backoff is up.
	MinBackOffDuration = 100
	// MaxBackOffDuration maximum amount (in milliseconds) to wait before peer is re-dialed.
	MaxBackOffDuration = 5000
)

// Status is the structure holding the peer status information.
type Status struct {
	ctx       context.Context
	scorers   *scorers.Service
	store     *peerdata.Store
	ipTracker map[string]uint64
	rand      *rand.Rand
}

// StatusConfig represents peer status service params.
type StatusConfig struct {
	// PeerLimit specifies maximum amount of concurrent peers that are expected to be connect to the node.
	PeerLimit int
	// ScorerParams holds peer scorer configuration params.
	ScorerParams *scorers.Config
}

// NewStatus creates a new status entity.
func NewStatus(ctx context.Context, config *StatusConfig) *Status {
	store := peerdata.NewStore(ctx, &peerdata.StoreConfig{
		MaxPeers: maxLimitBuffer + config.PeerLimit,
	})
	return &Status{
		ctx:       ctx,
		store:     store,
		scorers:   scorers.NewService(ctx, store, config.ScorerParams),
		ipTracker: map[string]uint64{},
		// Random generator used to calculate dial backoff period.
		// It is ok to use deterministic generator, no need for true entropy.
		rand: rand.NewDeterministicGenerator(),
	}
}

// Scorers exposes peer scoring management service.
func (p *Status) Scorers() *scorers.Service {
	return p.scorers
}

// MaxPeerLimit returns the max peer limit stored in the current peer store.
func (p *Status) MaxPeerLimit() int {
	return p.store.Config().MaxPeers
}

// Add adds a peer.
// If a peer already exists with this ID its address and direction are updated with the supplied data.
func (p *Status) Add(record *enr.Record, pid peer.ID, address ma.Multiaddr, direction network.Direction) {
	p.store.Lock()
	defer p.store.Unlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		// Peer already exists, just update its address info.
		prevAddress := peerData.Address
		peerData.Address = address
		peerData.Direction = direction
		if record != nil {
			peerData.Enr = record
		}
		if !sameIP(prevAddress, address) {
			p.addIpToTracker(pid)
		}
		return
	}
	peerData := &peerdata.PeerData{
		Address:   address,
		Direction: direction,
		// Peers start disconnected; state will be updated when the handshake process begins.
		ConnState: PeerDisconnected,
	}
	if record != nil {
		peerData.Enr = record
	}
	p.store.SetPeerData(pid, peerData)
	p.addIpToTracker(pid)
}

// Address returns the multiaddress of the given remote peer.
// This will error if the peer does not exist.
func (p *Status) Address(pid peer.ID) (ma.Multiaddr, error) {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		return peerData.Address, nil
	}
	return nil, peerdata.ErrPeerUnknown
}

// Direction returns the direction of the given remote peer.
// This will error if the peer does not exist.
func (p *Status) Direction(pid peer.ID) (network.Direction, error) {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		return peerData.Direction, nil
	}
	return network.DirUnknown, peerdata.ErrPeerUnknown
}

// ENR returns the enr for the corresponding peer id.
func (p *Status) ENR(pid peer.ID) (*enr.Record, error) {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		return peerData.Enr, nil
	}
	return nil, peerdata.ErrPeerUnknown
}

// SetChainState sets the chain state of the given remote peer.
func (p *Status) SetChainState(pid peer.ID, chainState *pb.Status) {
	p.scorers.PeerStatusScorer().SetPeerStatus(pid, chainState, nil)
}

// ChainState gets the chain state of the given remote peer.
// This can return nil if there is no known chain state for the peer.
// This will error if the peer does not exist.
func (p *Status) ChainState(pid peer.ID) (*pb.Status, error) {
	return p.scorers.PeerStatusScorer().PeerStatus(pid)
}

// IsActive checks if a peers is active and returns the result appropriately.
func (p *Status) IsActive(pid peer.ID) bool {
	p.store.RLock()
	defer p.store.RUnlock()

	peerData, ok := p.store.PeerData(pid)
	return ok && (peerData.ConnState == PeerConnected || peerData.ConnState == PeerConnecting)
}

// IsAboveInboundLimit checks if we are above our current inbound
// peer limit.
func (p *Status) IsAboveInboundLimit() bool {
	p.store.RLock()
	defer p.store.RUnlock()
	totalInbound := 0
	for _, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerConnected &&
			peerData.Direction == network.DirInbound {
			totalInbound += 1
		}
	}
	inboundLimit := int(float64(p.ConnectedPeerLimit()) * InboundRatio)
	return totalInbound > inboundLimit
}

// InboundLimit returns the current inbound
// peer limit.
func (p *Status) InboundLimit() int {
	p.store.RLock()
	defer p.store.RUnlock()
	return int(float64(p.ConnectedPeerLimit()) * InboundRatio)
}

// SetMetadata sets the metadata of the given remote peer.
func (p *Status) SetMetadata(pid peer.ID, metaData p2p.Metadata) {
	p.store.Lock()
	defer p.store.Unlock()

	peerData := p.store.PeerDataGetOrCreate(pid)
	peerData.MetaData = metaData.Copy()
}

// Metadata returns a copy of the metadata corresponding to the provided
// peer id.
func (p *Status) Metadata(pid peer.ID) (p2p.Metadata, error) {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		if peerData.MetaData == nil || peerData.MetaData.IsNil() {
			return nil, nil
		}
		return peerData.MetaData.Copy(), nil
	}
	return nil, peerdata.ErrPeerUnknown
}

// CommitteeIndices retrieves the committee subnets the peer is subscribed to.
func (p *Status) CommitteeIndices(pid peer.ID) ([]uint64, error) {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		if peerData.Enr == nil || peerData.MetaData == nil || peerData.MetaData.IsNil() {
			return []uint64{}, nil
		}
		return indicesFromBitfield(peerData.MetaData.AttnetsBitfield()), nil
	}
	return nil, peerdata.ErrPeerUnknown
}

// SubscribedToSubnet retrieves the peers subscribed to the given
// committee subnet.
func (p *Status) SubscribedToSubnet(index uint64) []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()

	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		// look at active peers
		connectedStatus := peerData.ConnState == PeerConnecting || peerData.ConnState == PeerConnected
		if connectedStatus && peerData.MetaData != nil && !peerData.MetaData.IsNil() && peerData.MetaData.AttnetsBitfield() != nil {
			indices := indicesFromBitfield(peerData.MetaData.AttnetsBitfield())
			for _, idx := range indices {
				if idx == index {
					peers = append(peers, pid)
					break
				}
			}
		}
	}
	return peers
}

// SetConnectionState sets the connection state of the given remote peer.
func (p *Status) SetConnectionState(pid peer.ID, state peerdata.PeerConnectionState) {
	p.store.Lock()
	defer p.store.Unlock()

	peerData := p.store.PeerDataGetOrCreate(pid)
	peerData.ConnState = state
}

// ConnectionState gets the connection state of the given remote peer.
// This will error if the peer does not exist.
func (p *Status) ConnectionState(pid peer.ID) (peerdata.PeerConnectionState, error) {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		return peerData.ConnState, nil
	}
	return PeerDisconnected, peerdata.ErrPeerUnknown
}

// ChainStateLastUpdated gets the last time the chain state of the given remote peer was updated.
// This will error if the peer does not exist.
func (p *Status) ChainStateLastUpdated(pid peer.ID) (time.Time, error) {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		return peerData.ChainStateLastUpdated, nil
	}
	return timeutils.Now(), peerdata.ErrPeerUnknown
}

// IsBad states if the peer is to be considered bad (by *any* of the registered scorers).
// If the peer is unknown this will return `false`, which makes using this function easier than returning an error.
func (p *Status) IsBad(pid peer.ID) bool {
	return p.isfromBadIP(pid) || p.scorers.IsBadPeer(pid)
}

// NextValidTime gets the earliest possible time it is to contact/dial
// a peer again. This is used to back-off from peers in the event
// they are 'full' or have banned us.
func (p *Status) NextValidTime(pid peer.ID) (time.Time, error) {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		return peerData.NextValidTime, nil
	}
	return timeutils.Now(), peerdata.ErrPeerUnknown
}

// SetNextValidTime sets the earliest possible time we are
// able to contact this peer again.
func (p *Status) SetNextValidTime(pid peer.ID, nextTime time.Time) {
	p.store.Lock()
	defer p.store.Unlock()

	peerData := p.store.PeerDataGetOrCreate(pid)
	peerData.NextValidTime = nextTime
}

// RandomizeBackOff adds extra backoff period during which peer will not be dialed.
func (p *Status) RandomizeBackOff(pid peer.ID) {
	p.store.Lock()
	defer p.store.Unlock()

	peerData := p.store.PeerDataGetOrCreate(pid)

	// No need to add backoff period, if the previous one hasn't expired yet.
	if !time.Now().After(peerData.NextValidTime) {
		return
	}

	duration := time.Duration(math.Max(MinBackOffDuration, float64(p.rand.Intn(MaxBackOffDuration)))) * time.Millisecond
	peerData.NextValidTime = time.Now().Add(duration)
}

// IsReadyToDial checks where the given peer is ready to be
// dialed again.
func (p *Status) IsReadyToDial(pid peer.ID) bool {
	p.store.RLock()
	defer p.store.RUnlock()

	if peerData, ok := p.store.PeerData(pid); ok {
		timeIsZero := peerData.NextValidTime.IsZero()
		isInvalidTime := peerData.NextValidTime.After(time.Now())
		return timeIsZero || !isInvalidTime
	}
	// If no record exists, we don't restrict dials to the
	// peer.
	return true
}

// Connecting returns the peers that are connecting.
func (p *Status) Connecting() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerConnecting {
			peers = append(peers, pid)
		}
	}
	return peers
}

// Connected returns the peers that are connected.
func (p *Status) Connected() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerConnected {
			peers = append(peers, pid)
		}
	}
	return peers
}

// Inbound returns the current batch of inbound peers.
func (p *Status) Inbound() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.Direction == network.DirInbound {
			peers = append(peers, pid)
		}
	}
	return peers
}

// InboundConnected returns the current batch of inbound peers that are connected.
func (p *Status) InboundConnected() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerConnected && peerData.Direction == network.DirInbound {
			peers = append(peers, pid)
		}
	}
	return peers
}

// Outbound returns the current batch of outbound peers.
func (p *Status) Outbound() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.Direction == network.DirOutbound {
			peers = append(peers, pid)
		}
	}
	return peers
}

// OutboundConnected returns the current batch of outbound peers that are connected.
func (p *Status) OutboundConnected() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerConnected && peerData.Direction == network.DirOutbound {
			peers = append(peers, pid)
		}
	}
	return peers
}

// Active returns the peers that are connecting or connected.
func (p *Status) Active() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerConnecting || peerData.ConnState == PeerConnected {
			peers = append(peers, pid)
		}
	}
	return peers
}

// Disconnecting returns the peers that are disconnecting.
func (p *Status) Disconnecting() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerDisconnecting {
			peers = append(peers, pid)
		}
	}
	return peers
}

// Disconnected returns the peers that are disconnected.
func (p *Status) Disconnected() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerDisconnected {
			peers = append(peers, pid)
		}
	}
	return peers
}

// Inactive returns the peers that are disconnecting or disconnected.
func (p *Status) Inactive() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	peers := make([]peer.ID, 0)
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerDisconnecting || peerData.ConnState == PeerDisconnected {
			peers = append(peers, pid)
		}
	}
	return peers
}

// Bad returns the peers that are bad.
func (p *Status) Bad() []peer.ID {
	return p.scorers.BadResponsesScorer().BadPeers()
}

// All returns all the peers regardless of state.
func (p *Status) All() []peer.ID {
	p.store.RLock()
	defer p.store.RUnlock()
	pids := make([]peer.ID, 0, len(p.store.Peers()))
	for pid := range p.store.Peers() {
		pids = append(pids, pid)
	}
	return pids
}

// Prune clears out and removes outdated and disconnected peers.
func (p *Status) Prune() {
	p.store.Lock()
	defer p.store.Unlock()

	// Exit early if there is nothing to prune.
	if len(p.store.Peers()) <= p.store.Config().MaxPeers {
		return
	}

	notBadPeer := func(peerData *peerdata.PeerData) bool {
		return peerData.BadResponses < p.scorers.BadResponsesScorer().Params().Threshold
	}
	type peerResp struct {
		pid     peer.ID
		badResp int
	}
	peersToPrune := make([]*peerResp, 0)
	// Select disconnected peers with a smaller bad response count.
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerDisconnected && notBadPeer(peerData) {
			peersToPrune = append(peersToPrune, &peerResp{
				pid:     pid,
				badResp: peerData.BadResponses,
			})
		}
	}

	// Sort peers in ascending order, so the peers with the
	// least amount of bad responses are pruned first. This
	// is to protect the node from malicious/lousy peers so
	// that their memory is still kept.
	sort.Slice(peersToPrune, func(i, j int) bool {
		return peersToPrune[i].badResp < peersToPrune[j].badResp
	})

	limitDiff := len(p.store.Peers()) - p.store.Config().MaxPeers
	if limitDiff > len(peersToPrune) {
		limitDiff = len(peersToPrune)
	}

	peersToPrune = peersToPrune[:limitDiff]

	// Delete peers from map.
	for _, peerData := range peersToPrune {
		p.store.DeletePeerData(peerData.pid)
	}
	p.tallyIPTracker()
}

// BestFinalized returns the highest finalized epoch equal to or higher than ours that is agreed
// upon by the majority of peers. This method may not return the absolute highest finalized, but
// the finalized epoch in which most peers can serve blocks (plurality voting).
// Ideally, all peers would be reporting the same finalized epoch but some may be behind due to their
// own latency, or because of their finalized epoch at the time we queried them.
// Returns epoch number and list of peers that are at or beyond that epoch.
func (p *Status) BestFinalized(maxPeers int, ourFinalizedEpoch types.Epoch) (types.Epoch, []peer.ID) {
	connected := p.Connected()
	finalizedEpochVotes := make(map[types.Epoch]uint64)
	pidEpoch := make(map[peer.ID]types.Epoch, len(connected))
	pidHead := make(map[peer.ID]types.Slot, len(connected))
	potentialPIDs := make([]peer.ID, 0, len(connected))
	for _, pid := range connected {
		peerChainState, err := p.ChainState(pid)
		if err == nil && peerChainState != nil && peerChainState.FinalizedEpoch >= ourFinalizedEpoch {
			finalizedEpochVotes[peerChainState.FinalizedEpoch]++
			pidEpoch[pid] = peerChainState.FinalizedEpoch
			potentialPIDs = append(potentialPIDs, pid)
			pidHead[pid] = peerChainState.HeadSlot
		}
	}

	// Select the target epoch, which is the epoch most peers agree upon.
	var targetEpoch types.Epoch
	var mostVotes uint64
	for epoch, count := range finalizedEpochVotes {
		if count > mostVotes || (count == mostVotes && epoch > targetEpoch) {
			mostVotes = count
			targetEpoch = epoch
		}
	}

	// Sort PIDs by finalized epoch, in decreasing order.
	sort.Slice(potentialPIDs, func(i, j int) bool {
		if pidEpoch[potentialPIDs[i]] == pidEpoch[potentialPIDs[j]] {
			return pidHead[potentialPIDs[i]] > pidHead[potentialPIDs[j]]
		}
		return pidEpoch[potentialPIDs[i]] > pidEpoch[potentialPIDs[j]]
	})

	// Trim potential peers to those on or after target epoch.
	for i, pid := range potentialPIDs {
		if pidEpoch[pid] < targetEpoch {
			potentialPIDs = potentialPIDs[:i]
			break
		}
	}

	// Trim potential peers to at most maxPeers.
	if len(potentialPIDs) > maxPeers {
		potentialPIDs = potentialPIDs[:maxPeers]
	}

	return targetEpoch, potentialPIDs
}

// BestNonFinalized returns the highest known epoch, higher than ours,
// and is shared by at least minPeers.
func (p *Status) BestNonFinalized(minPeers int, ourHeadEpoch types.Epoch) (types.Epoch, []peer.ID) {
	connected := p.Connected()
	epochVotes := make(map[types.Epoch]uint64)
	pidEpoch := make(map[peer.ID]types.Epoch, len(connected))
	pidHead := make(map[peer.ID]types.Slot, len(connected))
	potentialPIDs := make([]peer.ID, 0, len(connected))

	ourHeadSlot := params.BeaconConfig().SlotsPerEpoch.Mul(uint64(ourHeadEpoch))
	for _, pid := range connected {
		peerChainState, err := p.ChainState(pid)
		if err == nil && peerChainState != nil && peerChainState.HeadSlot > ourHeadSlot {
			epoch := helpers.SlotToEpoch(peerChainState.HeadSlot)
			epochVotes[epoch]++
			pidEpoch[pid] = epoch
			pidHead[pid] = peerChainState.HeadSlot
			potentialPIDs = append(potentialPIDs, pid)
		}
	}

	// Select the target epoch, which has enough peers' votes (>= minPeers).
	var targetEpoch types.Epoch
	for epoch, votes := range epochVotes {
		if votes >= uint64(minPeers) && targetEpoch < epoch {
			targetEpoch = epoch
		}
	}

	// Sort PIDs by head slot, in decreasing order.
	sort.Slice(potentialPIDs, func(i, j int) bool {
		return pidHead[potentialPIDs[i]] > pidHead[potentialPIDs[j]]
	})

	// Trim potential peers to those on or after target epoch.
	for i, pid := range potentialPIDs {
		if pidEpoch[pid] < targetEpoch {
			potentialPIDs = potentialPIDs[:i]
			break
		}
	}

	return targetEpoch, potentialPIDs
}

// PeersToPrune selects the most sutiable inbound peers
// to disconnect the host peer from. As of this moment
// the pruning relies on simple heuristics such as
// bad response count. In the future scoring will be used
// to determine the most suitable peers to take out.
func (p *Status) PeersToPrune() []peer.ID {
	connLimit := p.ConnectedPeerLimit()
	inBoundLimit := p.InboundLimit()
	activePeers := p.Active()
	numInboundPeers := len(p.InboundConnected())
	// Exit early if we are still below our max
	// limit.
	if len(activePeers) <= int(connLimit) {
		return []peer.ID{}
	}
	p.store.Lock()
	defer p.store.Unlock()

	type peerResp struct {
		pid     peer.ID
		badResp int
	}
	peersToPrune := make([]*peerResp, 0)
	// Select connected and inbound peers to prune.
	for pid, peerData := range p.store.Peers() {
		if peerData.ConnState == PeerConnected &&
			peerData.Direction == network.DirInbound {
			peersToPrune = append(peersToPrune, &peerResp{
				pid:     pid,
				badResp: peerData.BadResponses,
			})
		}
	}

	// Sort in descending order to favour pruning peers with a
	// higher bad response count.
	sort.Slice(peersToPrune, func(i, j int) bool {
		return peersToPrune[i].badResp > peersToPrune[j].badResp
	})

	// Determine amount of peers to prune using our
	// max connection limit.
	amountToPrune := len(activePeers) - int(connLimit)

	// Also check for inbound peers above our limit.
	excessInbound := 0
	if numInboundPeers > inBoundLimit {
		excessInbound = numInboundPeers - inBoundLimit
	}
	// Prune the largest amount between excess peers and
	// excess inbound peers.
	if excessInbound > amountToPrune {
		amountToPrune = excessInbound
	}
	if amountToPrune < len(peersToPrune) {
		peersToPrune = peersToPrune[:amountToPrune]
	}
	ids := make([]peer.ID, 0, len(peersToPrune))
	for _, pr := range peersToPrune {
		ids = append(ids, pr.pid)
	}
	return ids
}

// HighestEpoch returns the highest epoch reported epoch amongst peers.
func (p *Status) HighestEpoch() types.Epoch {
	p.store.RLock()
	defer p.store.RUnlock()
	var highestSlot types.Slot
	for _, peerData := range p.store.Peers() {
		if peerData != nil && peerData.ChainState != nil && peerData.ChainState.HeadSlot > highestSlot {
			highestSlot = peerData.ChainState.HeadSlot
		}
	}
	return helpers.SlotToEpoch(highestSlot)
}

// ConnectedPeerLimit returns the peer limit of
// concurrent peers connected to the beacon-node.
func (p *Status) ConnectedPeerLimit() uint64 {
	maxLim := p.MaxPeerLimit()
	if maxLim <= maxLimitBuffer {
		return 0
	}
	return uint64(maxLim) - maxLimitBuffer
}

func (p *Status) isfromBadIP(pid peer.ID) bool {
	p.store.RLock()
	defer p.store.RUnlock()

	peerData, ok := p.store.PeerData(pid)
	if !ok {
		return false
	}
	if peerData.Address == nil {
		return false
	}
	ip, err := manet.ToIP(peerData.Address)
	if err != nil {
		return true
	}
	if val, ok := p.ipTracker[ip.String()]; ok {
		if val > ColocationLimit {
			return true
		}
	}
	return false
}

func (p *Status) addIpToTracker(pid peer.ID) {
	data, ok := p.store.PeerData(pid)
	if !ok {
		return
	}
	if data.Address == nil {
		return
	}
	ip, err := manet.ToIP(data.Address)
	if err != nil {
		// Should never happen, it is
		// assumed every IP coming in
		// is a valid ip.
		return
	}
	// Ignore loopback addresses.
	if ip.IsLoopback() {
		return
	}
	stringIP := ip.String()
	p.ipTracker[stringIP] += 1
}

func (p *Status) tallyIPTracker() {
	tracker := map[string]uint64{}
	// Iterate through all peers.
	for _, peerData := range p.store.Peers() {
		if peerData.Address == nil {
			continue
		}
		ip, err := manet.ToIP(peerData.Address)
		if err != nil {
			// Should never happen, it is
			// assumed every IP coming in
			// is a valid ip.
			continue
		}
		stringIP := ip.String()
		tracker[stringIP] += 1
	}
	p.ipTracker = tracker
}

func sameIP(firstAddr, secondAddr ma.Multiaddr) bool {
	// Exit early if we do get nil multiaddresses
	if firstAddr == nil || secondAddr == nil {
		return false
	}
	firstIP, err := manet.ToIP(firstAddr)
	if err != nil {
		return false
	}
	secondIP, err := manet.ToIP(secondAddr)
	if err != nil {
		return false
	}
	return firstIP.Equal(secondIP)
}

func indicesFromBitfield(bitV bitfield.Bitvector64) []uint64 {
	committeeIdxs := make([]uint64, 0, bitV.Count())
	for i := uint64(0); i < 64; i++ {
		if bitV.BitAt(i) {
			committeeIdxs = append(committeeIdxs, i)
		}
	}
	return committeeIdxs
}
