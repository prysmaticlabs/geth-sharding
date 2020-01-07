package powchain

import (
	"context"
	"strings"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/db/kv"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
)

const pubKeyErr = "could not deserialize validator public key"

func TestProcessDeposit_OK(t *testing.T) {
	web3Service, err := NewService(context.Background(), &Web3ServiceConfig{
		ETH1Endpoint: endpoint,
		BeaconDB:     &kv.Store{},
	})
	if err != nil {
		t.Fatalf("Unable to setup web3 ETH1.0 chain service: %v", err)
	}
	web3Service = setDefaultMocks(web3Service)

	deposits, _, _ := testutil.DeterministicDepositsAndKeys(1)

	eth1Data, err := testutil.DeterministicEth1Data(len(deposits))
	if err != nil {
		t.Fatal(err)
	}

	if err := web3Service.processDeposit(eth1Data, deposits[0]); err != nil {
		t.Fatalf("Could not process deposit %v", err)
	}

	if web3Service.activeValidatorCount != 1 {
		t.Errorf("Did not get correct active validator count received %d, but wanted %d", web3Service.activeValidatorCount, 1)
	}
}

func TestProcessDeposit_InvalidMerkleBranch(t *testing.T) {
	web3Service, err := NewService(context.Background(), &Web3ServiceConfig{
		ETH1Endpoint: endpoint,
		BeaconDB:     &kv.Store{},
	})
	if err != nil {
		t.Fatalf("Unable to setup web3 ETH1.0 chain service: %v", err)
	}
	web3Service = setDefaultMocks(web3Service)

	deposits, _, _ := testutil.DeterministicDepositsAndKeys(1)

	eth1Data, err := testutil.DeterministicEth1Data(len(deposits))
	if err != nil {
		t.Fatal(err)
	}

	deposits[0].Proof = [][]byte{{'f', 'a', 'k', 'e'}}

	err = web3Service.processDeposit(eth1Data, deposits[0])
	if err == nil {
		t.Fatal("No errors, when an error was expected")
	}

	want := "deposit merkle branch of deposit root did not verify for root"

	if !strings.Contains(err.Error(), want) {
		t.Errorf("Did not get expected error. Wanted: '%s' but got '%s'", want, err.Error())
	}

}

func TestProcessDeposit_InvalidPublicKey(t *testing.T) {
	web3Service, err := NewService(context.Background(), &Web3ServiceConfig{
		ETH1Endpoint: endpoint,
		BeaconDB:     &kv.Store{},
	})
	if err != nil {
		t.Fatalf("Unable to setup web3 ETH1.0 chain service: %v", err)
	}
	web3Service = setDefaultMocks(web3Service)

	deposits, _, _ := testutil.DeterministicDepositsAndKeys(1)
	deposits[0].Data.PublicKey = []byte("junk")

	leaf, err := ssz.HashTreeRoot(deposits[0].Data)
	if err != nil {
		t.Fatalf("Could not hash deposit %v", err)
	}
	trie, err := trieutil.GenerateTrieFromItems([][]byte{leaf[:]}, int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		log.Error(err)
	}
	deposits[0].Proof, err = trie.MerkleProof((0))
	if err != nil {
		t.Fatal(err)
	}

	root := trie.Root()

	eth1Data := &ethpb.Eth1Data{
		DepositCount: 1,
		DepositRoot:  root[:],
	}

	err = web3Service.processDeposit(eth1Data, deposits[0])
	if err == nil {
		t.Fatal("No errors, when an error was expected")
	}

	if !strings.Contains(err.Error(), pubKeyErr) {
		t.Errorf("Did not get expected error. Wanted: '%s' but got '%s'", pubKeyErr, err.Error())
	}
}

func TestProcessDeposit_InvalidSignature(t *testing.T) {
	web3Service, err := NewService(context.Background(), &Web3ServiceConfig{
		ETH1Endpoint: endpoint,
		BeaconDB:     &kv.Store{},
	})
	if err != nil {
		t.Fatalf("Unable to setup web3 ETH1.0 chain service: %v", err)
	}
	web3Service = setDefaultMocks(web3Service)

	deposits, _, _ := testutil.DeterministicDepositsAndKeys(1)
	var fakeSig [96]byte
	copy(fakeSig[:], []byte{'F', 'A', 'K', 'E'})
	deposits[0].Data.Signature = fakeSig[:]

	leaf, err := ssz.HashTreeRoot(deposits[0].Data)
	if err != nil {
		t.Fatalf("Could not hash deposit %v", err)
	}

	trie, err := trieutil.GenerateTrieFromItems([][]byte{leaf[:]}, int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		log.Error(err)
	}

	root := trie.Root()

	eth1Data := &ethpb.Eth1Data{
		DepositCount: 1,
		DepositRoot:  root[:],
	}

	err = web3Service.processDeposit(eth1Data, deposits[0])
	if err == nil {
		t.Fatal("No errors, when an error was expected")
	}

	if !strings.Contains(err.Error(), pubKeyErr) {
		t.Errorf("Did not get expected error. Wanted: '%s' but got '%s'", pubKeyErr, err.Error())
	}

}

func TestProcessDeposit_UnableToVerify(t *testing.T) {
	web3Service, err := NewService(context.Background(), &Web3ServiceConfig{
		ETH1Endpoint: endpoint,
		BeaconDB:     &kv.Store{},
	})
	if err != nil {
		t.Fatalf("Unable to setup web3 ETH1.0 chain service: %v", err)
	}
	web3Service = setDefaultMocks(web3Service)
	testutil.ResetCache()

	deposits, keys, _ := testutil.DeterministicDepositsAndKeys(1)
	sig := keys[0].Sign([]byte{'F', 'A', 'K', 'E'}, bls.ComputeDomain(params.BeaconConfig().DomainDeposit))
	deposits[0].Data.Signature = sig.Marshal()[:]

	trie, _, err := testutil.DepositTrieFromDeposits(deposits)
	if err != nil {
		t.Fatal(err)
	}
	root := trie.Root()
	eth1Data := &ethpb.Eth1Data{
		DepositCount: 1,
		DepositRoot:  root[:],
	}
	proof, err := trie.MerkleProof(0)
	if err != nil {
		t.Fatal(err)
	}
	deposits[0].Proof = proof
	err = web3Service.processDeposit(eth1Data, deposits[0])
	if err == nil {
		t.Fatal("No errors, when an error was expected")
	}

	want := "deposit signature did not verify"

	if !strings.Contains(err.Error(), want) {
		t.Errorf("Did not get expected error. Wanted: '%s' but got '%s'", want, err.Error())
	}

}

func TestProcessDeposit_IncompleteDeposit(t *testing.T) {
	web3Service, err := NewService(context.Background(), &Web3ServiceConfig{
		ETH1Endpoint: endpoint,
		BeaconDB:     &kv.Store{},
	})
	if err != nil {
		t.Fatalf("Unable to setup web3 ETH1.0 chain service: %v", err)
	}
	web3Service = setDefaultMocks(web3Service)

	deposit := &ethpb.Deposit{
		Data: &ethpb.Deposit_Data{
			Amount:                params.BeaconConfig().EffectiveBalanceIncrement, // incomplete deposit
			WithdrawalCredentials: []byte("testing"),
		},
	}

	sk := bls.RandKey()
	deposit.Data.PublicKey = sk.PublicKey().Marshal()
	signedRoot, err := ssz.SigningRoot(deposit.Data)
	if err != nil {
		t.Fatal(err)
	}

	sig := sk.Sign(signedRoot[:], bls.ComputeDomain(params.BeaconConfig().DomainDeposit))
	deposit.Data.Signature = sig.Marshal()

	trie, _, err := testutil.DepositTrieFromDeposits([]*ethpb.Deposit{deposit})
	if err != nil {
		t.Fatal(err)
	}
	root := trie.Root()
	eth1Data := &ethpb.Eth1Data{
		DepositCount: 1,
		DepositRoot:  root[:],
	}
	proof, err := trie.MerkleProof(0)
	if err != nil {
		t.Fatal(err)
	}
	deposit.Proof = proof

	factor := params.BeaconConfig().MaxEffectiveBalance / params.BeaconConfig().EffectiveBalanceIncrement
	// deposit till 31e9
	for i := 0; i < int(factor-1); i++ {
		if err := web3Service.processDeposit(eth1Data, deposit); err != nil {
			t.Fatalf("Could not process deposit %v", err)
		}

		if web3Service.activeValidatorCount == 1 {
			t.Errorf("Did not get correct active validator count received %d, but wanted %d", web3Service.activeValidatorCount, 0)
		}
	}
}

func TestProcessDeposit_AllDepositedSuccessfully(t *testing.T) {
	web3Service, err := NewService(context.Background(), &Web3ServiceConfig{
		ETH1Endpoint: endpoint,
		BeaconDB:     &kv.Store{},
	})
	if err != nil {
		t.Fatalf("Unable to setup web3 ETH1.0 chain service: %v", err)
	}
	web3Service = setDefaultMocks(web3Service)
	testutil.ResetCache()

	deposits, keys, _ := testutil.DeterministicDepositsAndKeys(10)
	eth1Data, err := testutil.DeterministicEth1Data(len(deposits))
	if err != nil {
		t.Fatal(err)
	}

	for i, k := range keys {
		eth1Data.DepositCount = uint64(i + 1)
		if err := web3Service.processDeposit(eth1Data, deposits[i]); err != nil {
			t.Fatalf("Could not process deposit %v", err)
		}

		if web3Service.activeValidatorCount != uint64(i+1) {
			t.Errorf("Did not get correct active validator count received %d, but wanted %d", web3Service.activeValidatorCount, uint64(i+1))
		}
		pubkey := bytesutil.ToBytes48(k.PublicKey().Marshal())
		if web3Service.depositedPubkeys[pubkey] != params.BeaconConfig().MaxEffectiveBalance {
			t.Errorf("Wanted a full deposit of %d but got %d", params.BeaconConfig().MaxEffectiveBalance, web3Service.depositedPubkeys[pubkey])
		}
	}
}
