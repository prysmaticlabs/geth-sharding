package testutil

import (
	"encoding/binary"
	"sync"
	"testing"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-ssz"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/interop"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
)

var lock sync.Mutex

// Caches
var deposits []*ethpb.Deposit
var depositDataRoots [][32]byte
var privKeys []*bls.SecretKey
var trie *trieutil.MerkleTrie

// SetupInitialDeposits prepares the entered amount of deposits,
// deposit data roots, and secret keys.
// The deposits are configured such that for deposit n the validator
// account is key n and the withdrawal account is key n+1.  As such,
// if all secret keys for n validators are required then numDeposits
// should be n+1
func SetupInitialDeposits(t testing.TB, numDeposits uint64) ([]*ethpb.Deposit, [][32]byte, []*bls.SecretKey) {
	lock.Lock()
	defer lock.Unlock()

	var err error

	// Populate trie cache, if not initialized yet.
	if trie == nil {
		trie, err = trieutil.NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
		if err != nil {
			t.Fatal(err)
		}
	}

	if numDeposits > uint64(len(deposits)) {
		// Additional deposits required
		numExisting := uint64(len(deposits))
		numRequired := numDeposits - uint64(len(deposits))
		// Fetch the required number of keys
		secretKeys, publicKeys, err := interop.DeterministicallyGenerateKeys(numExisting, numRequired+1)
		if err != nil {
			t.Fatalf("could not create deterministic keys: %v", err)
		}
		privKeys = append(privKeys, secretKeys[:len(secretKeys)-1]...)

		// Create the deposits
		for i := uint64(0); i < numRequired; i++ {
			withdrawalCreds := hashutil.Hash(publicKeys[i+1].Marshal())
			withdrawalCreds[0] = params.BeaconConfig().BLSWithdrawalPrefixByte

			depositData := &ethpb.Deposit_Data{
				PublicKey:             publicKeys[i].Marshal()[:],
				Amount:                params.BeaconConfig().MaxEffectiveBalance,
				WithdrawalCredentials: withdrawalCreds[:],
			}

			domain := bls.ComputeDomain(params.BeaconConfig().DomainDeposit)
			root, err := ssz.SigningRoot(depositData)
			if err != nil {
				t.Fatalf("could not get signing root of deposit data %v", err)
			}
			depositData.Signature = secretKeys[i].Sign(root[:], domain).Marshal()

			depositDataRoot, err := ssz.HashTreeRoot(depositData)
			if err != nil {
				t.Fatalf("could not get hash tree root of deposit data %v", err)
			}
			depositDataRoots = append(depositDataRoots, depositDataRoot)

			deposit := &ethpb.Deposit{
				Data: depositData,
			}

			deposits = append(deposits, deposit)
		}
	}

	d, _ := GenerateDepositProof(t, deposits[0:numDeposits])
	return d, depositDataRoots[0:numDeposits], privKeys[0:numDeposits]
}

// GenerateDepositProof takes an array of deposits and generates the deposit trie for them and proofs.
func GenerateDepositProof(t testing.TB, deposits []*ethpb.Deposit) ([]*ethpb.Deposit, [32]byte) {
	encodedDeposits := make([][]byte, len(deposits))
	for i := 0; i < len(encodedDeposits); i++ {
		hashedDeposit, err := ssz.HashTreeRoot(deposits[i].Data)
		if err != nil {
			t.Fatalf("could not tree hash deposit data: %v", err)
		}
		encodedDeposits[i] = hashedDeposit[:]
	}

	depositTrie, err := trieutil.GenerateTrieFromItems(encodedDeposits, int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("Could not generate deposit trie: %v", err)
	}

	for i := range deposits {
		proof, err := depositTrie.MerkleProof(int(i))
		if err != nil {
			t.Fatalf("Could not generate proof: %v", err)
		}
		deposits[i].Proof = proof
	}
	root := depositTrie.Root()
	return deposits, root
}

// GenerateEth1Data takes an array of deposits and generates the deposit trie for them.
func GenerateEth1Data(t testing.TB, deposits []*ethpb.Deposit) *ethpb.Eth1Data {
	_, root := GenerateDepositProof(t, deposits)
	eth1Data := &ethpb.Eth1Data{
		BlockHash:    root[:],
		DepositRoot:  root[:],
		DepositCount: uint64(len(deposits)),
	}

	return eth1Data
}

// SignBlock generates a signed block using the block slot and the beacon proposer priv key.
func SignBlock(beaconState *pb.BeaconState, block *ethpb.BeaconBlock, privKeys []*bls.SecretKey) (*ethpb.BeaconBlock, error) {
	slot := beaconState.Slot
	beaconState.Slot = block.Slot
	proposerIdx, err := helpers.BeaconProposerIndex(beaconState)
	if err != nil {
		return nil, err
	}
	beaconState.Slot = slot
	signingRoot, err := ssz.SigningRoot(block)
	if err != nil {
		return nil, err
	}
	epoch := helpers.SlotToEpoch(block.Slot)
	domain := helpers.Domain(beaconState.Fork, epoch, params.BeaconConfig().DomainBeaconProposer)
	blockSig := privKeys[proposerIdx].Sign(signingRoot[:], domain).Marshal()
	block.Signature = blockSig[:]
	return block, nil
}

// CreateRandaoReveal generates a epoch signature using the beacon proposer priv key.
func CreateRandaoReveal(beaconState *pb.BeaconState, epoch uint64, privKeys []*bls.SecretKey) ([]byte, error) {
	// We fetch the proposer's index as that is whom the RANDAO will be verified against.
	proposerIdx, err := helpers.BeaconProposerIndex(beaconState)
	if err != nil {
		return []byte{}, errors.Wrap(err, "could not get beacon proposer index")
	}
	buf := make([]byte, 32)
	binary.LittleEndian.PutUint64(buf, epoch)
	domain := helpers.Domain(beaconState.Fork, epoch, params.BeaconConfig().DomainRandao)
	// We make the previous validator's index sign the message instead of the proposer.
	epochSignature := privKeys[proposerIdx].Sign(buf, domain)
	return epochSignature.Marshal(), nil
}

// ResetCache clears out the old trie, private keys and deposits.
func ResetCache() {
	trie = nil
	privKeys = []*bls.SecretKey{}
	deposits = []*ethpb.Deposit{}
	depositDataRoots = [][32]byte{}
}
