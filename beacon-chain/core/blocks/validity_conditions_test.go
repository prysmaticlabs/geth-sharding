package blocks

import (
	"context"
	"testing"
	"time"

	"github.com/prysmaticlabs/prysm/shared/params"

	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

type mockDB struct {
	hasBlock bool
}

func (f *mockDB) HasBlock(h [32]byte) bool {
	return f.hasBlock
}

type mockPOWClient struct {
	blockExists bool
}

func (m *mockPOWClient) BlockByHash(ctx context.Context, hash common.Hash) (*gethTypes.Block, error) {
	if m.blockExists {
		return &gethTypes.Block{}, nil
	}
	return nil, nil
}

func TestBadBlock(t *testing.T) {
	beaconState := &pb.BeaconState{}

	ctx := context.Background()

	db := &mockDB{}
	powClient := &mockPOWClient{}

	beaconState.Slot = params.BeaconConfig().GenesisSlot + 3

	block := &pb.BeaconBlock{
		Slot: params.BeaconConfig().GenesisSlot + 4,
	}

	genesisTime := time.Unix(0, 0)

	db.hasBlock = false

	if err := IsValidBlock(ctx, beaconState, block, true,
		db.HasBlock, powClient.BlockByHash, genesisTime); err == nil {
		t.Fatal("block is valid despite not having a parent")
	}

	block.Slot = params.BeaconConfig().GenesisSlot + 3
	db.hasBlock = true

	beaconState.LatestEth1Data = &pb.Eth1Data{
		DepositRootHash32: []byte{2},
		BlockHash32:       []byte{3},
	}
	if err := IsValidBlock(ctx, beaconState, block, true,
		db.HasBlock, powClient.BlockByHash, genesisTime); err == nil {
		t.Fatalf("block is valid despite having an invalid slot %d", block.Slot)
	}

	block.Slot = params.BeaconConfig().GenesisSlot + 4
	powClient.blockExists = false
	beaconState.LatestEth1Data = &pb.Eth1Data{
		DepositRootHash32: []byte{2},
		BlockHash32:       []byte{3},
	}

	if err := IsValidBlock(ctx, beaconState, block, true,
		db.HasBlock, powClient.BlockByHash, genesisTime); err == nil {
		t.Fatalf("block is valid despite having an invalid pow reference block")
	}

	invalidTime := time.Now().AddDate(1, 2, 3)
	powClient.blockExists = false

	if err := IsValidBlock(ctx, beaconState, block, true,
		db.HasBlock, powClient.BlockByHash, genesisTime); err == nil {
		t.Fatalf("block is valid despite having an invalid genesis time %v", invalidTime)
	}

}

func TestValidBlock(t *testing.T) {
	beaconState := &pb.BeaconState{}
	ctx := context.Background()

	db := &mockDB{}
	powClient := &mockPOWClient{}

	beaconState.Slot = params.BeaconConfig().GenesisSlot + 3
	db.hasBlock = true

	block := &pb.BeaconBlock{
		Slot: params.BeaconConfig().GenesisSlot + 4,
	}

	genesisTime := time.Unix(0, 0)
	powClient.blockExists = true

	beaconState.LatestEth1Data = &pb.Eth1Data{
		DepositRootHash32: []byte{2},
		BlockHash32:       []byte{3},
	}

	if err := IsValidBlock(ctx, beaconState, block, true,
		db.HasBlock, powClient.BlockByHash, genesisTime); err != nil {
		t.Fatal(err)
	}
}
