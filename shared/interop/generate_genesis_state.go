// Package interop contains deterministic utilities for generating
// genesis states and keys.
package interop

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	coreState "github.com/prysmaticlabs/prysm/beacon-chain/core/state"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/mputil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/timeutils"
	"github.com/prysmaticlabs/prysm/shared/trieutil"
)

var (
	// This is the recommended mock eth1 block hash according to the Ethereum consensus interop guidelines.
	// https://github.com/ethereum/eth2.0-pm/blob/a085c9870f3956d6228ed2a40cd37f0c6580ecd7/interop/mocked_start/README.md
	mockEth1BlockHash = []byte{66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66, 66}
)

// GenerateGenesisState deterministically given a genesis time and number of validators.
// If a genesis time of 0 is supplied it is set to the current time.
func GenerateGenesisState(ctx context.Context, genesisTime, numValidators uint64) (*pb.BeaconState, []*ethpb.Deposit, error) {
	privKeys, pubKeys, err := DeterministicallyGenerateKeys(0 /*startIndex*/, numValidators)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "could not deterministically generate keys for %d validators", numValidators)
	}
	depositDataItems, depositDataRoots, err := DepositDataFromKeys(privKeys, pubKeys)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not generate deposit data from keys")
	}
	return GenerateGenesisStateFromDepositData(ctx, genesisTime, depositDataItems, depositDataRoots)
}

// GenerateGenesisStateFromDepositData creates a genesis state given a list of
// deposit data items and their corresponding roots.
func GenerateGenesisStateFromDepositData(
	ctx context.Context, genesisTime uint64, depositData []*ethpb.Deposit_Data, depositDataRoots [][]byte,
) (*pb.BeaconState, []*ethpb.Deposit, error) {
	trie, err := trieutil.GenerateTrieFromItems(depositDataRoots, params.BeaconConfig().DepositContractTreeDepth)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not generate Merkle trie for deposit proofs")
	}
	deposits, err := GenerateDepositsFromData(depositData, trie)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not generate deposits from the deposit data provided")
	}
	root := trie.Root()
	if genesisTime == 0 {
		genesisTime = uint64(timeutils.Now().Unix())
	}
	beaconState, err := coreState.GenesisBeaconState(ctx, deposits, genesisTime, &ethpb.Eth1Data{
		DepositRoot:  root[:],
		DepositCount: uint64(len(deposits)),
		BlockHash:    mockEth1BlockHash,
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not generate genesis state")
	}

	pbState, err := v1.ProtobufBeaconState(beaconState.CloneInnerState())
	if err != nil {
		return nil, nil, err
	}
	return pbState, deposits, nil
}

// GenerateDepositsFromData a list of deposit items by creating proofs for each of them from a sparse Merkle trie.
func GenerateDepositsFromData(depositDataItems []*ethpb.Deposit_Data, trie *trieutil.SparseMerkleTrie) ([]*ethpb.Deposit, error) {
	deposits := make([]*ethpb.Deposit, len(depositDataItems))
	results, err := mputil.Scatter(len(depositDataItems), func(offset int, entries int, _ *sync.RWMutex) (interface{}, error) {
		return generateDepositsFromData(depositDataItems[offset:offset+entries], offset, trie)
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate deposits from data")
	}
	for _, result := range results {
		if depositExtent, ok := result.Extent.([]*ethpb.Deposit); ok {
			copy(deposits[result.Offset:], depositExtent)
		} else {
			return nil, errors.New("extent not of expected type")
		}
	}
	return deposits, nil
}

// generateDepositsFromData a list of deposit items by creating proofs for each of them from a sparse Merkle trie.
func generateDepositsFromData(depositDataItems []*ethpb.Deposit_Data, offset int, trie *trieutil.SparseMerkleTrie) ([]*ethpb.Deposit, error) {
	deposits := make([]*ethpb.Deposit, len(depositDataItems))
	for i, item := range depositDataItems {
		proof, err := trie.MerkleProof(i + offset)
		if err != nil {
			return nil, errors.Wrapf(err, "could not generate proof for deposit %d", i+offset)
		}
		deposits[i] = &ethpb.Deposit{
			Proof: proof,
			Data:  item,
		}
	}
	return deposits, nil
}

// DepositDataFromKeys generates a list of deposit data items from a set of BLS validator keys.
func DepositDataFromKeys(privKeys []bls.SecretKey, pubKeys []bls.PublicKey) ([]*ethpb.Deposit_Data, [][]byte, error) {
	type depositData struct {
		items []*ethpb.Deposit_Data
		roots [][]byte
	}
	depositDataItems := make([]*ethpb.Deposit_Data, len(privKeys))
	depositDataRoots := make([][]byte, len(privKeys))
	results, err := mputil.Scatter(len(privKeys), func(offset int, entries int, _ *sync.RWMutex) (interface{}, error) {
		items, roots, err := depositDataFromKeys(privKeys[offset:offset+entries], pubKeys[offset:offset+entries])
		return &depositData{items: items, roots: roots}, err
	})
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate deposit data from keys")
	}
	for _, result := range results {
		if depositDataExtent, ok := result.Extent.(*depositData); ok {
			copy(depositDataItems[result.Offset:], depositDataExtent.items)
			copy(depositDataRoots[result.Offset:], depositDataExtent.roots)
		} else {
			return nil, nil, errors.New("extent not of expected type")
		}
	}
	return depositDataItems, depositDataRoots, nil
}

func depositDataFromKeys(privKeys []bls.SecretKey, pubKeys []bls.PublicKey) ([]*ethpb.Deposit_Data, [][]byte, error) {
	dataRoots := make([][]byte, len(privKeys))
	depositDataItems := make([]*ethpb.Deposit_Data, len(privKeys))
	for i := 0; i < len(privKeys); i++ {
		data, err := createDepositData(privKeys[i], pubKeys[i])
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not create deposit data for key: %#x", privKeys[i].Marshal())
		}
		hash, err := data.HashTreeRoot()
		if err != nil {
			return nil, nil, errors.Wrap(err, "could not hash tree root deposit data item")
		}
		dataRoots[i] = hash[:]
		depositDataItems[i] = data
	}
	return depositDataItems, dataRoots, nil
}

// Generates a deposit data item from BLS keys and signs the hash tree root of the data.
func createDepositData(privKey bls.SecretKey, pubKey bls.PublicKey) (*ethpb.Deposit_Data, error) {
	depositMessage := &pb.DepositMessage{
		PublicKey:             pubKey.Marshal(),
		WithdrawalCredentials: withdrawalCredentialsHash(pubKey.Marshal()),
		Amount:                params.BeaconConfig().MaxEffectiveBalance,
	}
	sr, err := depositMessage.HashTreeRoot()
	if err != nil {
		return nil, err
	}
	domain, err := helpers.ComputeDomain(params.BeaconConfig().DomainDeposit, nil, nil)
	if err != nil {
		return nil, err
	}
	root, err := (&pb.SigningData{ObjectRoot: sr[:], Domain: domain}).HashTreeRoot()
	if err != nil {
		return nil, err
	}
	di := &ethpb.Deposit_Data{
		PublicKey:             depositMessage.PublicKey,
		WithdrawalCredentials: depositMessage.WithdrawalCredentials,
		Amount:                depositMessage.Amount,
		Signature:             privKey.Sign(root[:]).Marshal(),
	}
	return di, nil
}

// withdrawalCredentialsHash forms a 32 byte hash of the withdrawal public
// address.
//
// The specification is as follows:
//   withdrawal_credentials[:1] == BLS_WITHDRAWAL_PREFIX_BYTE
//   withdrawal_credentials[1:] == hash(withdrawal_pubkey)[1:]
// where withdrawal_credentials is of type bytes32.
func withdrawalCredentialsHash(pubKey []byte) []byte {
	h := hashutil.Hash(pubKey)
	return append([]byte{blsWithdrawalPrefixByte}, h[1:]...)[:32]
}
