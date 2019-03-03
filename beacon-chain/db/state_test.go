package db

import (
	"bytes"
	"context"
	"crypto/rand"
	"testing"
	"time"

	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"

	"github.com/gogo/protobuf/proto"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func setupInitialDeposits(t testing.TB, numDeposits int) ([]*pb.Deposit, []*bls.SecretKey) {
	privKeys := make([]*bls.SecretKey, numDeposits)
	deposits := make([]*pb.Deposit, numDeposits)
	for i := 0; i < len(deposits); i++ {
		priv, err := bls.RandKey(rand.Reader)
		if err != nil {
			t.Fatal(err)
		}
		depositInput := &pb.DepositInput{
			Pubkey: priv.PublicKey().Marshal(),
		}
		balance := params.BeaconConfig().MaxDepositAmount
		depositData, err := helpers.EncodeDepositData(depositInput, balance, time.Now().Unix())
		if err != nil {
			t.Fatalf("Cannot encode data: %v", err)
		}
		deposits[i] = &pb.Deposit{DepositData: depositData}
		privKeys[i] = priv
	}
	return deposits, privKeys
}

func TestInitializeState_OK(t *testing.T) {
	db := setupDB(t)
	defer teardownDB(t, db)
	ctx := context.Background()

	genesisTime := uint64(time.Now().Unix())
	deposits, _ := setupInitialDeposits(t, 10)
	if err := db.InitializeState(genesisTime, deposits, &pb.Eth1Data{}); err != nil {
		t.Fatalf("Failed to initialize state: %v", err)
	}
	b, err := db.ChainHead()
	if err != nil {
		t.Fatalf("Failed to get chain head: %v", err)
	}
	if b.GetSlot() != params.BeaconConfig().GenesisSlot {
		t.Fatalf("Expected block height to equal 1. Got %d", b.GetSlot())
	}

	beaconState, err := db.State(ctx)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	if beaconState == nil {
		t.Fatalf("Failed to retrieve state: %v", beaconState)
	}
	beaconStateEnc, err := proto.Marshal(beaconState)
	if err != nil {
		t.Fatalf("Failed to encode state: %v", err)
	}

	statePrime, err := db.State(ctx)
	if err != nil {
		t.Fatalf("Failed to get state: %v", err)
	}
	statePrimeEnc, err := proto.Marshal(statePrime)
	if err != nil {
		t.Fatalf("Failed to encode state: %v", err)
	}

	if !bytes.Equal(beaconStateEnc, statePrimeEnc) {
		t.Fatalf("Expected %#x and %#x to be equal", beaconStateEnc, statePrimeEnc)
	}
}

func TestGenesisTime_OK(t *testing.T) {
	db := setupDB(t)
	defer teardownDB(t, db)
	ctx := context.Background()

	genesisTime, err := db.GenesisTime(ctx)
	if err == nil {
		t.Fatal("expected GenesisTime to fail")
	}

	deposits, _ := setupInitialDeposits(t, 10)
	if err := db.InitializeState(uint64(genesisTime.Unix()), deposits, &pb.Eth1Data{}); err != nil {
		t.Fatalf("failed to initialize state: %v", err)
	}

	time1, err := db.GenesisTime(ctx)
	if err != nil {
		t.Fatalf("GenesisTime failed on second attempt: %v", err)
	}
	time2, err := db.GenesisTime(ctx)
	if err != nil {
		t.Fatalf("GenesisTime failed on second attempt: %v", err)
	}

	if time1 != time2 {
		t.Fatalf("Expected %v and %v to be equal", time1, time2)
	}
}

func BenchmarkState_ReadingFromCache(b *testing.B) {
	db := setupDB(b)
	defer teardownDB(b, db)
	ctx := context.Background()

	genesisTime := uint64(time.Now().Unix())
	deposits, _ := setupInitialDeposits(b, 10)
	if err := db.InitializeState(genesisTime, deposits, &pb.Eth1Data{}); err != nil {
		b.Fatalf("Failed to initialize state: %v", err)
	}

	// Initial read should not be from DB
	if db.currentState != nil {
		b.Fatal("cache should not be prepared on newly initialized state")
	}

	state, err := db.State(ctx)
	if err != nil {
		b.Fatalf("Could not read DV beacon state from DB: %v", err)
	}
	state.Slot++
	err = db.SaveState(state)
	if err != nil {
		b.Fatalf("Could not save beacon state to cache from DB: %v", err)
	}

	if db.currentState.Slot != params.BeaconConfig().GenesisSlot+1 {
		b.Fatal("cache should be prepared on state after saving to DB")
	}

	b.N = 20
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := db.State(ctx)
		if err != nil {
			b.Fatalf("Could not read beacon state from cache: %v", err)
		}
	}
}

func BenchmarkState_ReadingFromDB(b *testing.B) {
	db := setupDB(b)
	defer teardownDB(b, db)
	ctx := context.Background()

	genesisTime := uint64(time.Now().Unix())
	deposits, _ := setupInitialDeposits(b, 10)
	if err := db.InitializeState(genesisTime, deposits, &pb.Eth1Data{}); err != nil {
		b.Fatalf("Failed to initialize state: %v", err)
	}

	// Initial read should not be from DB
	if db.currentState != nil {
		b.Fatal("cache should not be prepared on newly initialized state")
	}

	b.N = 20
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := db.State(ctx)
		if err != nil {
			b.Fatalf("Could not read beacon state from DB: %v", err)
		}
	}
}
