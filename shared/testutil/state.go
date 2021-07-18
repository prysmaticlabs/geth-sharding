package testutil

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common/hexutil"
	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

// FillRootsNaturalOpt is meant to be used as an option when calling NewBeaconState.
// It fills state and block roots with hex representations of natural numbers starting with 0.
// Example: 16 becomes 0x00...0f.
func FillRootsNaturalOpt(state *pb.BeaconState) error {
	rootsLen := params.MainnetConfig().SlotsPerHistoricalRoot
	roots := make([][]byte, rootsLen)
	for i := types.Slot(0); i < rootsLen; i++ {
		roots[i] = make([]byte, 32)
	}
	for j := 0; j < len(roots); j++ {
		// Remove '0x' prefix and left-pad '0' to have 64 chars in total.
		s := fmt.Sprintf("%064s", hexutil.EncodeUint64(uint64(j))[2:])
		h, err := hexutil.Decode("0x" + s)
		if err != nil {
			return err
		}
		roots[j] = h
	}
	state.StateRoots = roots
	state.BlockRoots = roots
	return nil
}

// NewBeaconState creates a beacon state with minimum marshalable fields.
func NewBeaconState(options ...func(state *pb.BeaconState) error) (*v1.BeaconState, error) {
	seed := &pb.BeaconState{
		BlockRoots:                 filledByteSlice2D(uint64(params.MainnetConfig().SlotsPerHistoricalRoot), 32),
		StateRoots:                 filledByteSlice2D(uint64(params.MainnetConfig().SlotsPerHistoricalRoot), 32),
		Slashings:                  make([]uint64, params.MainnetConfig().EpochsPerSlashingsVector),
		RandaoMixes:                filledByteSlice2D(uint64(params.MainnetConfig().EpochsPerHistoricalVector), 32),
		Validators:                 make([]*ethpb.Validator, 0),
		CurrentJustifiedCheckpoint: &ethpb.Checkpoint{Root: make([]byte, 32)},
		Eth1Data: &ethpb.Eth1Data{
			DepositRoot: make([]byte, 32),
			BlockHash:   make([]byte, 32),
		},
		Fork: &pb.Fork{
			PreviousVersion: make([]byte, 4),
			CurrentVersion:  make([]byte, 4),
		},
		Eth1DataVotes:               make([]*ethpb.Eth1Data, 0),
		HistoricalRoots:             make([][]byte, 0),
		JustificationBits:           bitfield.Bitvector4{0x0},
		FinalizedCheckpoint:         &ethpb.Checkpoint{Root: make([]byte, 32)},
		LatestBlockHeader:           HydrateBeaconHeader(&ethpb.BeaconBlockHeader{}),
		PreviousEpochAttestations:   make([]*pb.PendingAttestation, 0),
		CurrentEpochAttestations:    make([]*pb.PendingAttestation, 0),
		PreviousJustifiedCheckpoint: &ethpb.Checkpoint{Root: make([]byte, 32)},
	}

	for _, opt := range options {
		err := opt(seed)
		if err != nil {
			return nil, err
		}
	}

	var st, err = v1.InitializeFromProtoUnsafe(seed)
	if err != nil {
		return nil, err
	}

	return st.Copy().(*v1.BeaconState), nil
}

// SSZ will fill 2D byte slices with their respective values, so we must fill these in too for round
// trip testing.
func filledByteSlice2D(length, innerLen uint64) [][]byte {
	b := make([][]byte, length)
	for i := uint64(0); i < length; i++ {
		b[i] = make([]byte, innerLen)
	}
	return b
}
