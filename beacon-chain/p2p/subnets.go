package p2p

import (
	"context"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1/wrapper"
	"go.opencensus.io/trace"

	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var attestationSubnetCount = params.BeaconNetworkConfig().AttestationSubnetCount
var syncCommsSubnetCount = params.BeaconConfig().SyncCommitteeSubnetCount

var attSubnetEnrKey = params.BeaconNetworkConfig().AttSubnetKey
var syncCommsSubnetEnrKey = params.BeaconNetworkConfig().SyncCommsSubnetKey

// FindPeersWithSubnet performs a network search for peers
// subscribed to a particular subnet. Then we try to connect
// with those peers. This method will block until the required amount of
// peers are found, the method only exits in the event of context timeouts.
func (s *Service) FindPeersWithSubnet(ctx context.Context, topic string,
	index, threshold uint64) (bool, error) {
	ctx, span := trace.StartSpan(ctx, "p2p.FindPeersWithSubnet")
	defer span.End()

	span.AddAttributes(trace.Int64Attribute("index", int64(index)))

	if s.dv5Listener == nil {
		// return if discovery isn't set
		return false, nil
	}

	topic += s.Encoding().ProtocolSuffix()
	iterator := s.dv5Listener.RandomNodes()
	switch {
	case strings.Contains(topic, GossipAttestationMessage):
		iterator = filterNodes(ctx, iterator, s.filterPeerForAttSubnet(index))
	case strings.Contains(topic, GossipSyncCommitteeMessage):
		iterator = filterNodes(ctx, iterator, s.filterPeerForSyncSubnet(index))
	default:
		return false, errors.New("no subnet exists for provided topic")
	}

	currNum := uint64(len(s.pubsub.ListPeers(topic)))
	wg := new(sync.WaitGroup)
	for {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		if currNum >= threshold {
			break
		}
		nodes := enode.ReadNodes(iterator, int(params.BeaconNetworkConfig().MinimumPeersInSubnetSearch))
		for _, node := range nodes {
			info, _, err := convertToAddrInfo(node)
			if err != nil {
				continue
			}
			wg.Add(1)
			go func() {
				if err := s.connectWithPeer(ctx, *info); err != nil {
					log.WithError(err).Tracef("Could not connect with peer %s", info.String())
				}
				wg.Done()
			}()
		}
		// Wait for all dials to be completed.
		wg.Wait()
		currNum = uint64(len(s.pubsub.ListPeers(topic)))
	}
	return true, nil
}

// returns a method with filters peers specifically for a particular attestation subnet.
func (s *Service) filterPeerForAttSubnet(index uint64) func(node *enode.Node) bool {
	return func(node *enode.Node) bool {
		if !s.filterPeer(node) {
			return false
		}
		subnets, err := attSubnets(node.Record())
		if err != nil {
			return false
		}
		indExists := false
		for _, comIdx := range subnets {
			if comIdx == index {
				indExists = true
				break
			}
		}
		return indExists
	}
}

// returns a method with filters peers specifically for a particular sync subnet.
func (s *Service) filterPeerForSyncSubnet(index uint64) func(node *enode.Node) bool {
	return func(node *enode.Node) bool {
		if !s.filterPeer(node) {
			return false
		}
		subnets, err := syncSubnets(node.Record())
		if err != nil {
			return false
		}
		indExists := false
		for _, comIdx := range subnets {
			if comIdx == index {
				indExists = true
				break
			}
		}
		return indExists
	}
}

// lower threshold to broadcast object compared to searching
// for a subnet. So that even in the event of poor peer
// connectivity, we can still broadcast an attestation.
func (s *Service) hasPeerWithSubnet(topic string) bool {
	return len(s.pubsub.ListPeers(topic+s.Encoding().ProtocolSuffix())) >= 1
}

// Updates the service's discv5 listener record's attestation subnet
// with a new value for a bitfield of subnets tracked. It also updates
// the node's metadata by increasing the sequence number and the
// subnets tracked by the node.
func (s *Service) updateSubnetRecordWithMetadata(bitV bitfield.Bitvector64) {
	entry := enr.WithEntry(attSubnetEnrKey, &bitV)
	s.dv5Listener.LocalNode().Set(entry)
	s.metaData = wrapper.WrappedMetadataV0(&pb.MetaDataV0{
		SeqNumber: s.metaData.SequenceNumber() + 1,
		Attnets:   bitV,
	})
}

// Updates the service's discv5 listener record's attestation subnet
// with a new value for a bitfield of subnets tracked. It also record's
// the sync committee subnet in the enr. It also updates the node's
// metadata by increasing the sequence number and the subnets tracked by the node.
func (s *Service) updateSubnetRecordWithMetadataV2(bitV bitfield.Bitvector64, bitS bitfield.Bitvector4) {
	entry := enr.WithEntry(attSubnetEnrKey, &bitV)
	subEntry := enr.WithEntry(syncCommsSubnetEnrKey, &bitS)
	s.dv5Listener.LocalNode().Set(entry)
	s.dv5Listener.LocalNode().Set(subEntry)
	s.metaData = wrapper.WrappedMetadataV1(&pb.MetaDataV1{
		SeqNumber: s.metaData.SequenceNumber() + 1,
		Attnets:   bitV,
		Syncnets:  bitS,
	})
}

// Initializes a bitvector of attestation subnets beacon nodes is subscribed to
// and creates a new ENR entry with its default value.
func initializeAttSubnets(node *enode.LocalNode) *enode.LocalNode {
	bitV := bitfield.NewBitvector64()
	entry := enr.WithEntry(attSubnetEnrKey, bitV.Bytes())
	node.Set(entry)
	return node
}

// Initializes a bitvector of sync committees subnets beacon nodes is subscribed to
// and creates a new ENR entry with its default value.
func initializeSyncCommSubnets(node *enode.LocalNode) *enode.LocalNode {
	bitV := bitfield.Bitvector4{byte(0x00)}
	entry := enr.WithEntry(syncCommsSubnetEnrKey, bitV.Bytes())
	node.Set(entry)
	return node
}

// Reads the attestation subnets entry from a node's ENR and determines
// the committee indices of the attestation subnets the node is subscribed to.
func attSubnets(record *enr.Record) ([]uint64, error) {
	bitV, err := attBitvector(record)
	if err != nil {
		return nil, err
	}
	if len(bitV) != determineSize(int(attestationSubnetCount)) {
		return []uint64{}, errors.Errorf("invalid bitvector provided, it has a size of %d", len(bitV))
	}
	var committeeIdxs []uint64
	for i := uint64(0); i < attestationSubnetCount; i++ {
		if bitV.BitAt(i) {
			committeeIdxs = append(committeeIdxs, i)
		}
	}
	return committeeIdxs, nil
}

// Reads the sync subnets entry from a node's ENR and determines
// the committee indices of the sync subnets the node is subscribed to.
func syncSubnets(record *enr.Record) ([]uint64, error) {
	bitV, err := syncBitvector(record)
	if err != nil {
		return nil, err
	}
	if len(bitV) != determineSize(int(syncCommsSubnetCount)) {
		return []uint64{}, errors.Errorf("invalid bitvector provided, it has a size of %d", len(bitV))
	}
	var committeeIdxs []uint64
	for i := uint64(0); i < syncCommsSubnetCount; i++ {
		if bitV.BitAt(i) {
			committeeIdxs = append(committeeIdxs, i)
		}
	}
	return committeeIdxs, nil
}

// Parses the attestation subnets ENR entry in a node and extracts its value
// as a bitvector for further manipulation.
func attBitvector(record *enr.Record) (bitfield.Bitvector64, error) {
	bitV := bitfield.NewBitvector64()
	entry := enr.WithEntry(attSubnetEnrKey, &bitV)
	err := record.Load(entry)
	if err != nil {
		return nil, err
	}
	return bitV, nil
}

// Parses the attestation subnets ENR entry in a node and extracts its value
// as a bitvector for further manipulation.
func syncBitvector(record *enr.Record) (bitfield.Bitvector4, error) {
	bitV := bitfield.Bitvector4{byte(0x00)}
	entry := enr.WithEntry(syncCommsSubnetEnrKey, &bitV)
	err := record.Load(entry)
	if err != nil {
		return nil, err
	}
	return bitV, nil
}

func (s *Service) subnetLocker(i uint64) *sync.RWMutex {
	s.subnetsLockLock.Lock()
	defer s.subnetsLockLock.Unlock()
	l, ok := s.subnetsLock[i]
	if !ok {
		l = &sync.RWMutex{}
		s.subnetsLock[i] = l
	}
	return l
}

func determineSize(bitCount int) int {
	numOfBytes := bitCount / 8
	if bitCount%8 != 0 {
		numOfBytes++
	}
	return numOfBytes
}
