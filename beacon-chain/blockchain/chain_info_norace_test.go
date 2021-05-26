package blockchain

import (
	"context"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	testDB "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	pcl := protocol.ID(p2p.RPCBlocksByRootTopicV1)"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestHeadSlot_DataRace(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	s := &Service{
		cfg: &Config{BeaconDB: beaconDB},
	}
	go func() {
		require.NoError(t, s.saveHead(context.Background(), [32]byte{}))
	}()
	s.HeadSlot()
}

func TestHeadRoot_DataRace(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	s := &Service{
		cfg:  &Config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB)},
		head: &head{root: [32]byte{'A'}},
	}
	go func() {
		require.NoError(t, s.saveHead(context.Background(), [32]byte{}))
	}()
	_, err := s.HeadRoot(context.Background())
	require.NoError(t, err)
}

func TestHeadBlock_DataRace(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	s := &Service{
		cfg:  &Config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB)},
		head: &head{block: interfaces.NewWrappedSignedBeaconBlock(&ethpb.SignedBeaconBlock{})},
	}
	go func() {
		require.NoError(t, s.saveHead(context.Background(), [32]byte{}))
	}()
	_, err := s.HeadBlock(context.Background())
	require.NoError(t, err)
}

func TestHeadState_DataRace(t *testing.T) {
	beaconDB := testDB.SetupDB(t)
	s := &Service{
		cfg: &Config{BeaconDB: beaconDB, StateGen: stategen.New(beaconDB)},
	}
	go func() {
		require.NoError(t, s.saveHead(context.Background(), [32]byte{}))
	}()
	_, err := s.HeadState(context.Background())
	require.NoError(t, err)
}
