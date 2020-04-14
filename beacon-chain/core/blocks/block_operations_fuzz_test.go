package blocks

import (
	"context"
	"testing"

	eth "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	stateTrie "github.com/prysmaticlabs/prysm/beacon-chain/state"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"

	fuzz "github.com/google/gofuzz"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"

	//"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	beaconstate "github.com/prysmaticlabs/prysm/beacon-chain/state"
	ethereum_beacon_p2p_v1 "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
)

func TestFuzzProcessAttestationNoVerify_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	ctx := context.Background()
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	att := &eth.Attestation{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(att)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		if _, err = ProcessAttestationNoVerify(ctx, s, att); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzProcessBlockHeader_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	block := &eth.SignedBeaconBlock{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(block)

		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		if _, err = ProcessBlockHeader(s, block); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzverifySigningRoot_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	pubkey := [48]byte{}
	sig := [96]byte{}
	domain := [4]byte{}
	p := []byte{}
	s := []byte{}
	d := uint64(0)
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(&pubkey)
		fuzzer.Fuzz(&sig)
		fuzzer.Fuzz(&domain)
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(&p)
		fuzzer.Fuzz(&s)
		fuzzer.Fuzz(&d)
		domain := bytesutil.FromBytes4(domain[:])
		if err := verifySigningRoot(state, pubkey[:], sig[:], domain); err != nil {
			t.Log(err)
		}
		if err := verifySigningRoot(state, p, s, d); err != nil {
			t.Log(err)
		}

	}
}

func TestFuzzverifyDepositDataSigningRoot_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	ba := []byte{}
	pubkey := [48]byte{}
	sig := [96]byte{}
	domain := [4]byte{}
	p := []byte{}
	s := []byte{}
	d := uint64(0)
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(&ba)
		fuzzer.Fuzz(&pubkey)
		fuzzer.Fuzz(&sig)
		fuzzer.Fuzz(&domain)
		fuzzer.Fuzz(&p)
		fuzzer.Fuzz(&s)
		fuzzer.Fuzz(&d)
		domain := bytesutil.FromBytes4(domain[:])
		if err := verifySignature(ba, pubkey[:], sig[:], domain); err != nil {
			t.Log(err)
		}
		if err := verifySignature(ba, p, s, d); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzProcessEth1DataInBlock_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	block := &eth.BeaconBlock{}
	state := &stateTrie.BeaconState{}
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(block)
		s, err := ProcessEth1DataInBlock(state, block)
		if err != nil && s != nil {
			t.Fatalf("state should be nil on err. found: %v on error: %v for state: %v and block: %v", s, err, state, block)
		}
	}
}

func TestFuzzareEth1DataEqual_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	eth1data := &eth.Eth1Data{}
	eth1data2 := &eth.Eth1Data{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(eth1data)
		fuzzer.Fuzz(eth1data2)
		areEth1DataEqual(eth1data, eth1data2)
		areEth1DataEqual(eth1data, eth1data)
	}
}

func TestFuzzEth1DataHasEnoughSupport_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	eth1data := &eth.Eth1Data{}
	stateVotes := []*eth.Eth1Data{}
	for i := 0; i < 100000; i++ {
		fuzzer.Fuzz(eth1data)
		fuzzer.Fuzz(&stateVotes)
		s, err := beaconstate.InitializeFromProto(&ethereum_beacon_p2p_v1.BeaconState{
			Eth1DataVotes: stateVotes,
		})
		if err != nil {
			t.Log(err)
		}
		if _, err := Eth1DataHasEnoughSupport(s, eth1data); err != nil {
			t.Log(err)
		}
	}

}

func TestFuzzProcessBlockHeaderNoVerify_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	block := &eth.BeaconBlock{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(block)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		if _, err = ProcessBlockHeaderNoVerify(s, block); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzProcessRandao_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessRandao(s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzProcessRandaoNoVerify_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessRandaoNoVerify(s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzProcessProposerSlashings_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessProposerSlashings(ctx, s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzVerifyProposerSlashing_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	proposerSlashing := &eth.ProposerSlashing{}
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(proposerSlashing)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		if err := VerifyProposerSlashing(s, proposerSlashing); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzProcessAttesterSlashings_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessAttesterSlashings(ctx, s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzVerifyAttesterSlashing_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	attesterSlashing := &eth.AttesterSlashing{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(attesterSlashing)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		if err := VerifyAttesterSlashing(ctx, s, attesterSlashing); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzIsSlashableAttestationData_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	attestationData := &eth.AttestationData{}
	attestationData2 := &eth.AttestationData{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(attestationData)
		fuzzer.Fuzz(attestationData2)
		IsSlashableAttestationData(attestationData, attestationData2)
	}
}

func TestFuzzslashableAttesterIndices_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	attesterSlashing := &eth.AttesterSlashing{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(attesterSlashing)
		slashableAttesterIndices(attesterSlashing)
	}
}

func TestFuzzProcessAttestations_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessAttestations(ctx, s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzProcessAttestationsNoVerify_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessAttestationsNoVerify(ctx, s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzProcessAttestation_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	attestation := &eth.Attestation{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(attestation)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessAttestation(ctx, s, attestation)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, attestation)
		}
	}
}

func TestFuzzVerifyIndexedAttestationn_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	idxAttestation := &eth.IndexedAttestation{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(idxAttestation)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		if err := VerifyIndexedAttestation(ctx, s, idxAttestation); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzVerifyAttestation_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	attestation := &eth.Attestation{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(attestation)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		if err := VerifyAttestation(ctx, s, attestation); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzProcessDeposits_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessDeposits(ctx, s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzProcessPreGenesisDeposit_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	deposit := &eth.Deposit{}
	ctx := context.Background()

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(deposit)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessPreGenesisDeposit(ctx, s, deposit)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, deposit)
		}
	}
}

func TestFuzzProcessDeposit_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	deposit := &eth.Deposit{}

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(deposit)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessDeposit(s, deposit)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, deposit)
		}
	}
}

func TestFuzzverifyDeposit_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	deposit := &eth.Deposit{}
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(deposit)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		if err := verifyDeposit(s, deposit); err != nil {
			t.Log(err)
		}
	}
}

func TestFuzzProcessVoluntaryExits_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}
	ctx := context.Background()
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessVoluntaryExits(ctx, s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzProcessVoluntaryExitsNoVerify_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	state := &ethereum_beacon_p2p_v1.BeaconState{}
	blockBody := &eth.BeaconBlockBody{}
	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(state)
		fuzzer.Fuzz(blockBody)
		s, err := beaconstate.InitializeFromProtoUnsafe(state)
		if err != nil {
			t.Log(err)
		}
		r, err := ProcessVoluntaryExitsNoVerify(s, blockBody)
		if err != nil && r != nil {
			t.Fatalf("return value should be nil on err. found: %v on error: %v for state: %v and block: %v", r, err, state, blockBody)
		}
	}
}

func TestFuzzVerifyExit_10000(t *testing.T) {
	fuzzer := fuzz.NewWithSeed(0)
	ve := &eth.SignedVoluntaryExit{}
	val := &eth.Validator{}
	fork := &pb.Fork{}
	var slot uint64

	for i := 0; i < 10000; i++ {
		fuzzer.Fuzz(ve)
		fuzzer.Fuzz(val)
		fuzzer.Fuzz(fork)
		fuzzer.Fuzz(&slot)
		if err := VerifyExit(val, slot, fork, ve); err != nil {
			t.Log(err)
		}
	}
}
