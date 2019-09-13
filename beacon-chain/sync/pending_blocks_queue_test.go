package sync

import (
	"context"
	"testing"

	"github.com/prysmaticlabs/go-ssz"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/sirupsen/logrus"
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
}

//    /- b1 - b2
// b0
//    \- b3
// Test b1 was missing then received and we can process b0 -> b1 -> b2
func TestRegularSyncBeaconBlockSubscriber_ProcessPendingBlocks1(t *testing.T) {
	db := dbtest.SetupDB(t)
	defer dbtest.TeardownDB(t, db)

	r := &RegularSync{
		db:                  db,
		chain:               &mock.ChainService{},
		slotToPendingBlocks: make(map[uint64]*ethpb.BeaconBlock),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}

	b0 := &ethpb.BeaconBlock{}
	if err := r.db.SaveBlock(context.Background(), b0); err != nil {
		t.Fatal(err)
	}
	b0Root, _ := ssz.SigningRoot(b0)
	b3 := &ethpb.BeaconBlock{Slot: 3, ParentRoot: b0Root[:]}
	if err := r.db.SaveBlock(context.Background(), b3); err != nil {
		t.Fatal(err)
	}
	// Incomplete block link
	b1 := &ethpb.BeaconBlock{Slot: 1, ParentRoot: b0Root[:]}
	b1Root, _ := ssz.SigningRoot(b1)
	b2 := &ethpb.BeaconBlock{Slot: 2, ParentRoot: b1Root[:]}
	b2Root, _ := ssz.SigningRoot(b1)

	// Add b2 to the cache
	r.slotToPendingBlocks[b2.Slot] = b2
	r.seenPendingBlocks[b2Root] = true

	if err := r.processPendingBlocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.slotToPendingBlocks) != 1 {
		t.Errorf("Incorrect size for slot to pending blocks cache: got %d", len(r.slotToPendingBlocks))
	}
	if len(r.seenPendingBlocks) != 1 {
		t.Errorf("Incorrect size for seen pending block: got %d", len(r.seenPendingBlocks))
	}

	// Add b1 to the cache
	r.slotToPendingBlocks[b1.Slot] = b1
	r.seenPendingBlocks[b1Root] = true
	if err := r.db.SaveBlock(context.Background(), b1); err != nil {
		t.Fatal(err)
	}
	if err := r.processPendingBlocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.slotToPendingBlocks) != 0 {
		t.Errorf("Incorrect size for slot to pending blocks cache: got %d", len(r.slotToPendingBlocks))
	}
	if len(r.seenPendingBlocks) != 0 {
		t.Errorf("Incorrect size for seen pending block: got %d", len(r.seenPendingBlocks))
	}
}

//    /- b1 - b2 - b5
// b0
//    \- b3 - b4
// Test b2 and b3 were missed, after receiving them we can process 2 chains.
func TestRegularSyncBeaconBlockSubscriber_ProcessPendingBlocks2(t *testing.T) {
	db := dbtest.SetupDB(t)
	defer dbtest.TeardownDB(t, db)

	r := &RegularSync{
		db:                  db,
		chain:               &mock.ChainService{},
		slotToPendingBlocks: make(map[uint64]*ethpb.BeaconBlock),
		seenPendingBlocks:   make(map[[32]byte]bool),
	}

	b0 := &ethpb.BeaconBlock{}
	if err := r.db.SaveBlock(context.Background(), b0); err != nil {
		t.Fatal(err)
	}
	b0Root, _ := ssz.SigningRoot(b0)
	b1 := &ethpb.BeaconBlock{Slot: 1, ParentRoot: b0Root[:]}
	if err := r.db.SaveBlock(context.Background(), b1); err != nil {
		t.Fatal(err)
	}
	b1Root, _ := ssz.SigningRoot(b1)

	// Incomplete block links
	b2 := &ethpb.BeaconBlock{Slot: 2, ParentRoot: b1Root[:]}
	b2Root, _ := ssz.SigningRoot(b2)
	b5 := &ethpb.BeaconBlock{Slot: 5, ParentRoot: b2Root[:]}
	b5Root, _ := ssz.SigningRoot(b5)
	b3 := &ethpb.BeaconBlock{Slot: 3, ParentRoot: b0Root[:]}
	b3Root, _ := ssz.SigningRoot(b3)
	b4 := &ethpb.BeaconBlock{Slot: 4, ParentRoot: b3Root[:]}
	b4Root, _ := ssz.SigningRoot(b4)

	r.slotToPendingBlocks[b4.Slot] = b4
	r.seenPendingBlocks[b4Root] = true
	r.slotToPendingBlocks[b5.Slot] = b5
	r.seenPendingBlocks[b5Root] = true

	if err := r.processPendingBlocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(r.slotToPendingBlocks) != 2 {
		t.Errorf("Incorrect size for slot to pending blocks cache: got %d", len(r.slotToPendingBlocks))
	}
	if len(r.seenPendingBlocks) != 2 {
		t.Errorf("Incorrect size for seen pending block: got %d", len(r.seenPendingBlocks))
	}

	// Add b3 to the cache
	r.slotToPendingBlocks[b3.Slot] = b3
	r.seenPendingBlocks[b3Root] = true
	if err := r.db.SaveBlock(context.Background(), b3); err != nil {
		t.Fatal(err)
	}
	if err := r.processPendingBlocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	for s := range r.seenPendingBlocks {
		t.Log(s)
	}
	if len(r.slotToPendingBlocks) != 1 {
		t.Errorf("Incorrect size for slot to pending blocks cache: got %d", len(r.slotToPendingBlocks))
	}
	if len(r.seenPendingBlocks) != 1 {
		t.Errorf("Incorrect size for seen pending block: got %d", len(r.seenPendingBlocks))
	}

	// Add b2 to the cache
	r.slotToPendingBlocks[b2.Slot] = b2
	r.seenPendingBlocks[b2Root] = true

	if err := r.db.SaveBlock(context.Background(), b2); err != nil {
		t.Fatal(err)
	}
	if err := r.processPendingBlocks(context.Background()); err != nil {
		t.Fatal(err)
	}
	for s := range r.seenPendingBlocks {
		t.Log(s)
	}
	if len(r.slotToPendingBlocks) != 0 {
		t.Errorf("Incorrect size for slot to pending blocks cache: got %d", len(r.slotToPendingBlocks))
	}
	if len(r.seenPendingBlocks) != 0 {
		t.Errorf("Incorrect size for seen pending block: got %d", len(r.seenPendingBlocks))
	}
}
