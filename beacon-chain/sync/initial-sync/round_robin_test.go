package initialsync

import (
	"context"
	"testing"

	eth "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/flags"
	p2pt "github.com/prysmaticlabs/prysm/beacon-chain/p2p/testing"
	stateTrie "github.com/prysmaticlabs/prysm/beacon-chain/state"
	p2ppb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/sliceutil"
)

func TestConstants(t *testing.T) {
	if params.BeaconConfig().MaxPeersToSync*flags.Get().BlockBatchLimit > 1000 {
		t.Fatal("rpc rejects requests over 1000 range slots")
	}
}

func TestRoundRobinSync(t *testing.T) {
	tests := []struct {
		name               string
		currentSlot        uint64
		expectedBlockSlots []uint64
		peers              []*peerData
	}{
		{
			name:               "Single peer with all blocks",
			currentSlot:        131,
			expectedBlockSlots: makeSequence(1, 131),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 131),
					finalizedEpoch: 1,
					headSlot:       131,
				},
			},
		},
		{
			name:               "Multiple peers with all blocks",
			currentSlot:        131,
			expectedBlockSlots: makeSequence(1, 131),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 131),
					finalizedEpoch: 1,
					headSlot:       131,
				},
				{
					blocks:         makeSequence(1, 131),
					finalizedEpoch: 1,
					headSlot:       131,
				},
				{
					blocks:         makeSequence(1, 131),
					finalizedEpoch: 1,
					headSlot:       131,
				},
				{
					blocks:         makeSequence(1, 131),
					finalizedEpoch: 1,
					headSlot:       131,
				},
			},
		},
		{
			name:               "Multiple peers with failures",
			currentSlot:        320, // 10 epochs
			expectedBlockSlots: makeSequence(1, 320),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 8,
					headSlot:       320,
				},
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 8,
					headSlot:       320,
					failureSlots:   makeSequence(1, 32), // first epoch
				},
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 8,
					headSlot:       320,
				},
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 8,
					headSlot:       320,
				},
			},
		},
		{
			name:               "Multiple peers with many skipped slots",
			currentSlot:        640, // 10 epochs
			expectedBlockSlots: append(makeSequence(1, 64), makeSequence(500, 640)...),
			peers: []*peerData{
				{
					blocks:         append(makeSequence(1, 64), makeSequence(500, 640)...),
					finalizedEpoch: 18,
					headSlot:       640,
				},
				{
					blocks:         append(makeSequence(1, 64), makeSequence(500, 640)...),
					finalizedEpoch: 18,
					headSlot:       640,
				},
				{
					blocks:         append(makeSequence(1, 64), makeSequence(500, 640)...),
					finalizedEpoch: 18,
					headSlot:       640,
				},
			},
		},

		// TODO(3147): Handle multiple failures.
		//{
		//	name:               "Multiple peers with multiple failures",
		//	currentSlot:        320, // 10 epochs
		//	expectedBlockSlots: makeSequence(1, 320),
		//	peers: []*peerData{
		//		{
		//			blocks:         makeSequence(1, 320),
		//			finalizedEpoch: 4,
		//			headSlot:       320,
		//		},
		//		{
		//			blocks:         makeSequence(1, 320),
		//			finalizedEpoch: 4,
		//			headSlot:       320,
		//			failureSlots:   makeSequence(1, 320),
		//		},
		//		{
		//			blocks:         makeSequence(1, 320),
		//			finalizedEpoch: 4,
		//			headSlot:       320,
		//			failureSlots:   makeSequence(1, 320),
		//		},
		//		{
		//			blocks:         makeSequence(1, 320),
		//			finalizedEpoch: 4,
		//			headSlot:       320,
		//			failureSlots:   makeSequence(1, 320),
		//		},
		//	},
		//},
		{
			name:               "Multiple peers with different finalized epoch",
			currentSlot:        320, // 10 epochs
			expectedBlockSlots: makeSequence(1, 320),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 320),
					finalizedEpoch: 4,
					headSlot:       320,
				},
				{
					blocks:         makeSequence(1, 256),
					finalizedEpoch: 3,
					headSlot:       256,
				},
				{
					blocks:         makeSequence(1, 256),
					finalizedEpoch: 3,
					headSlot:       256,
				},
				{
					blocks:         makeSequence(1, 192),
					finalizedEpoch: 2,
					headSlot:       192,
				},
			},
		},
		{
			name:               "Multiple peers with missing parent blocks",
			currentSlot:        160, // 5 epochs
			expectedBlockSlots: makeSequence(1, 160),
			peers: []*peerData{
				{
					blocks:         makeSequence(1, 160),
					finalizedEpoch: 4,
					headSlot:       160,
				},
				{
					blocks:         append(makeSequence(1, 6), makeSequence(161, 165)...),
					finalizedEpoch: 4,
					headSlot:       160,
					forkedPeer:     true,
				},
				{
					blocks:         makeSequence(1, 160),
					finalizedEpoch: 4,
					headSlot:       160,
				},
				{
					blocks:         makeSequence(1, 160),
					finalizedEpoch: 4,
					headSlot:       160,
				},
				{
					blocks:         makeSequence(1, 160),
					finalizedEpoch: 4,
					headSlot:       160,
				},
				{
					blocks:         makeSequence(1, 160),
					finalizedEpoch: 4,
					headSlot:       160,
				},
				{
					blocks:         makeSequence(1, 160),
					finalizedEpoch: 4,
					headSlot:       160,
				},
				{
					blocks:         makeSequence(1, 160),
					finalizedEpoch: 4,
					headSlot:       160,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache.initializeRootCache(tt.expectedBlockSlots, t)

			p := p2pt.NewTestP2P(t)
			beaconDB := dbtest.SetupDB(t)

			connectPeers(t, p, tt.peers, p.Peers())
			cache.RLock()
			genesisRoot := cache.rootCache[0]
			cache.RUnlock()

			err := beaconDB.SaveBlock(context.Background(), &eth.SignedBeaconBlock{
				Block: &eth.BeaconBlock{
					Slot: 0,
				}})
			if err != nil {
				t.Fatal(err)
			}

			st, err := stateTrie.InitializeFromProto(&p2ppb.BeaconState{})
			if err != nil {
				t.Fatal(err)
			}
			mc := &mock.ChainService{
				State: st,
				Root:  genesisRoot[:],
				DB:    beaconDB,
			} // no-op mock
			s := &Service{
				chain:         mc,
				blockNotifier: mc.BlockNotifier(),
				p2p:           p,
				db:            beaconDB,
				synced:        false,
				chainStarted:  true,
			}
			if err := s.roundRobinSync(makeGenesisTime(tt.currentSlot)); err != nil {
				t.Error(err)
			}
			if s.chain.HeadSlot() != tt.currentSlot {
				t.Errorf("Head slot (%d) is not currentSlot (%d)", s.chain.HeadSlot(), tt.currentSlot)
			}
			if len(mc.BlocksReceived) != len(tt.expectedBlockSlots) {
				t.Errorf("Processes wrong number of blocks. Wanted %d got %d", len(tt.expectedBlockSlots), len(mc.BlocksReceived))
			}
			var receivedBlockSlots []uint64
			for _, blk := range mc.BlocksReceived {
				receivedBlockSlots = append(receivedBlockSlots, blk.Block.Slot)
			}
			if missing := sliceutil.NotUint64(sliceutil.IntersectionUint64(tt.expectedBlockSlots, receivedBlockSlots), tt.expectedBlockSlots); len(missing) > 0 {
				t.Errorf("Missing blocks at slots %v", missing)
			}
		})
	}
}
