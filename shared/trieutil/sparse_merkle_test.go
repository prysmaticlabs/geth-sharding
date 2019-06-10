package trieutil

import (
	"math/big"
	"strconv"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	contracts "github.com/prysmaticlabs/prysm/contracts/deposit-contract"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestMerkleTrie_BranchIndices(t *testing.T) {
	indices := BranchIndices(1024, 3 /* depth */)
	expected := []int{1024, 512, 256}
	for i := 0; i < len(indices); i++ {
		if expected[i] != indices[i] {
			t.Errorf("Expected %d, received %d", expected[i], indices[i])
		}
	}
}

func TestMerkleTrie_MerkleProofOutOfRange(t *testing.T) {
	h := hashutil.Hash([]byte("hi"))
	m := &MerkleTrie{
		branches: [][][]byte{
			{
				h[:],
			},
			{
				h[:],
			},
			{
				[]byte{},
			},
		},
	}
	if _, err := m.MerkleProof(-1); err == nil {
		t.Error("Expected out of range failure, received nil", err)
	}
	if _, err := m.MerkleProof(2); err == nil {
		t.Error("Expected out of range failure, received nil", err)
	}
	if _, err := m.MerkleProof(0); err == nil {
		t.Error("Expected out of range failure, received nil", err)
	}
}

func TestMerkleTrieRoot_EmptyTrie(t *testing.T) {
	trie, err := NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatalf("Could not create empty trie %v", err)
	}
	testAccount, err := contracts.Setup()
	if err != nil {
		t.Fatal(err)
	}

	depRoot, err := testAccount.Contract.GetDepositRoot(&bind.CallOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if depRoot != trie.Root() {
		t.Errorf("Trie root for an empty trie isn't as expected. Expected: %#x but got %#x", depRoot, trie.Root())
	}
}

func TestGenerateTrieFromItems_NoItemsProvided(t *testing.T) {
	if _, err := GenerateTrieFromItems(nil, 32); err == nil {
		t.Error("Expected error when providing nil items received nil")
	}
}

func TestMerkleTrie_VerifyMerkleProof(t *testing.T) {
	items := [][]byte{
		[]byte("fe"),
		[]byte("fi"),
		[]byte("fo"),
		[]byte("fum"),
		[]byte("foo"),
		[]byte("bar"),
		[]byte("baz"),
	}
	m, err := GenerateTrieFromItems(items, 32)
	if err != nil {
		t.Fatalf("Could not generate Merkle trie from items: %v", err)
	}
	proof, err := m.MerkleProof(2)
	if err != nil {
		t.Fatalf("Could not generate Merkle proof: %v", err)
	}
	root := m.Root()
	if ok := VerifyMerkleProof(root[:], items[2], 2, proof); !ok {
		t.Error("Merkle proof did not verify")
	}
	proof, err = m.MerkleProof(3)
	if err != nil {
		t.Fatalf("Could not generate Merkle proof: %v", err)
	}
	if ok := VerifyMerkleProof(root[:], items[3], 3, proof); !ok {
		t.Error("Merkle proof did not verify")
	}
	if ok := VerifyMerkleProof(root[:], []byte("buzz"), 3, proof); ok {
		t.Error("Item not in tree should fail to verify")
	}
}

func BenchmarkGenerateTrieFromItems(b *testing.B) {
	items := [][]byte{
		[]byte("fe"),
		[]byte("fi"),
		[]byte("fo"),
		[]byte("fum"),
		[]byte("foo"),
		[]byte("bar"),
		[]byte("baz"),
	}
	for i := 0; i < b.N; i++ {
		if _, err := GenerateTrieFromItems(items, 32); err != nil {
			b.Fatalf("Could not generate Merkle trie from items: %v", err)
		}
	}
}

func BenchmarkVerifyMerkleBranch(b *testing.B) {
	items := [][]byte{
		[]byte("fe"),
		[]byte("fi"),
		[]byte("fo"),
		[]byte("fum"),
		[]byte("foo"),
		[]byte("bar"),
		[]byte("baz"),
	}
	m, err := GenerateTrieFromItems(items, 32)
	if err != nil {
		b.Fatalf("Could not generate Merkle trie from items: %v", err)
	}
	proof, err := m.MerkleProof(2)
	if err != nil {
		b.Fatalf("Could not generate Merkle proof: %v", err)
	}
	for i := 0; i < b.N; i++ {
		if ok := VerifyMerkleProof(m.branches[0][0], items[2], 2, proof); !ok {
			b.Error("Merkle proof did not verify")
		}
	}
}

func TestDepositTrieRoot_OK(t *testing.T) {
	testAcc, err := contracts.Setup()
	if err != nil {
		t.Fatal(err)
	}

	localTrie, err := NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(err)
	}

	depRoot, err := testAcc.Contract.GetDepositRoot(&bind.CallOpts{})
	if err != nil {
		t.Fatal(err)
	}

	if depRoot != localTrie.Root() {
		t.Errorf("Local deposit trie root and contract deposit trie root are not equal. Expected %#x , Got %#x", depRoot, localTrie.Root())
	}

	var pubkey [48]byte
	var withdrawalCreds [32]byte
	var sig [96]byte

	data := &pb.DepositData{
		Pubkey:                pubkey[:],
		Signature:             sig[:],
		WithdrawalCredentials: withdrawalCreds[:],
		Amount:                big.NewInt(0).Div(contracts.Amount32Eth(), big.NewInt(1e9)).Uint64(), // In Gwei
	}

	testAcc.TxOpts.Value = contracts.Amount32Eth()
	testAcc.TxOpts.GasLimit = 1000000

	for i := 0; i < 100; i++ {
		copy(data.Pubkey, []byte(strconv.Itoa(i)))
		copy(data.WithdrawalCredentials, []byte(strconv.Itoa(i)))
		copy(data.Signature, []byte(strconv.Itoa(i)))

		if _, err := testAcc.Contract.Deposit(testAcc.TxOpts, data.Pubkey, data.WithdrawalCredentials, data.Signature); err != nil {
			t.Fatalf("Could not deposit to deposit contract %v", err)
		}

		testAcc.Backend.Commit()
		item, err := hashutil.DepositHash(data)
		if err != nil {
			t.Fatal(err)
		}

		err = localTrie.InsertIntoTrie(item[:], i)
		if err != nil {
			t.Error(err)
		}

		depRoot, err = testAcc.Contract.GetDepositRoot(&bind.CallOpts{})
		if err != nil {
			t.Fatal(err)
		}

		if depRoot != localTrie.Root() {
			t.Errorf("Local deposit trie root and contract deposit trie root are not equal for index %d. Expected %#x , Got %#x", i, depRoot, localTrie.Root())
		}
	}

}

func TestDepositTrieRoot_Fail(t *testing.T) {
	testAcc, err := contracts.Setup()
	if err != nil {
		t.Fatal(err)
	}

	localTrie, err := NewTrie(int(params.BeaconConfig().DepositContractTreeDepth))
	if err != nil {
		t.Fatal(err)
	}

	depRoot, err := testAcc.Contract.GetDepositRoot(&bind.CallOpts{})
	if err != nil {
		t.Fatal(err)
	}

	if depRoot != localTrie.Root() {
		t.Errorf("Local deposit trie root and contract deposit trie root are not equal. Expected %#x , Got %#x", depRoot, localTrie.Root())
	}

	var pubkey [48]byte
	var withdrawalCreds [32]byte
	var sig [96]byte

	data := &pb.DepositData{
		Pubkey:                pubkey[:],
		Signature:             sig[:],
		WithdrawalCredentials: withdrawalCreds[:],
		Amount:                big.NewInt(0).Div(contracts.Amount32Eth(), big.NewInt(1e9)).Uint64(), // In Gwei
	}

	testAcc.TxOpts.Value = contracts.Amount32Eth()
	testAcc.TxOpts.GasLimit = 1000000

	for i := 0; i < 100; i++ {
		copy(data.Pubkey, []byte(strconv.Itoa(i)))
		copy(data.WithdrawalCredentials, []byte(strconv.Itoa(i)))
		copy(data.Signature, []byte(strconv.Itoa(i)))

		if _, err := testAcc.Contract.Deposit(testAcc.TxOpts, data.Pubkey, data.WithdrawalCredentials, data.Signature); err != nil {
			t.Fatalf("Could not deposit to deposit contract %v", err)
		}

		copy(data.Pubkey, []byte(strconv.Itoa(i+10)))
		copy(data.WithdrawalCredentials, []byte(strconv.Itoa(i+10)))
		copy(data.Signature, []byte(strconv.Itoa(i+10)))

		testAcc.Backend.Commit()
		item, err := hashutil.DepositHash(data)
		if err != nil {
			t.Fatal(err)
		}

		err = localTrie.InsertIntoTrie(item[:], i)
		if err != nil {
			t.Error(err)
		}

		depRoot, err = testAcc.Contract.GetDepositRoot(&bind.CallOpts{})
		if err != nil {
			t.Fatal(err)
		}

		if depRoot == localTrie.Root() {
			t.Errorf("Local deposit trie root and contract deposit trie root are equal for index %d when they were expected to be not equal", i)
		}
	}

}
