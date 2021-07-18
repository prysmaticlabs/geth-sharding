package v1_test

import (
	"bytes"
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/go-bitfield"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	v1 "github.com/prysmaticlabs/prysm/beacon-chain/state/v1"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestInitializeFromProto(t *testing.T) {
	testState, _ := testutil.DeterministicGenesisState(t, 64)
	pbState, err := v1.ProtobufBeaconState(testState.InnerStateUnsafe())
	require.NoError(t, err)
	type test struct {
		name  string
		state *pbp2p.BeaconState
		error string
	}
	initTests := []test{
		{
			name:  "nil state",
			state: nil,
			error: "received nil state",
		},
		{
			name: "nil validators",
			state: &pbp2p.BeaconState{
				Slot:       4,
				Validators: nil,
			},
		},
		{
			name:  "empty state",
			state: &pbp2p.BeaconState{},
		},
		{
			name:  "full state",
			state: pbState,
		},
	}
	for _, tt := range initTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v1.InitializeFromProto(tt.state)
			if tt.error != "" {
				assert.ErrorContains(t, tt.error, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestInitializeFromProtoUnsafe(t *testing.T) {
	testState, _ := testutil.DeterministicGenesisState(t, 64)
	pbState, err := v1.ProtobufBeaconState(testState.InnerStateUnsafe())
	require.NoError(t, err)
	type test struct {
		name  string
		state *pbp2p.BeaconState
		error string
	}
	initTests := []test{
		{
			name:  "nil state",
			state: nil,
			error: "received nil state",
		},
		{
			name: "nil validators",
			state: &pbp2p.BeaconState{
				Slot:       4,
				Validators: nil,
			},
		},
		{
			name:  "empty state",
			state: &pbp2p.BeaconState{},
		},
		{
			name:  "full state",
			state: pbState,
		},
	}
	for _, tt := range initTests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := v1.InitializeFromProtoUnsafe(tt.state)
			if tt.error != "" {
				assert.ErrorContains(t, tt.error, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBeaconState_HashTreeRoot(t *testing.T) {
	testState, _ := testutil.DeterministicGenesisState(t, 64)

	type test struct {
		name        string
		stateModify func(beaconState iface.BeaconState) (iface.BeaconState, error)
		error       string
	}
	initTests := []test{
		{
			name: "unchanged state",
			stateModify: func(beaconState iface.BeaconState) (iface.BeaconState, error) {
				return beaconState, nil
			},
			error: "",
		},
		{
			name: "different slot",
			stateModify: func(beaconState iface.BeaconState) (iface.BeaconState, error) {
				if err := beaconState.SetSlot(5); err != nil {
					return nil, err
				}
				return beaconState, nil
			},
			error: "",
		},
		{
			name: "different validator balance",
			stateModify: func(beaconState iface.BeaconState) (iface.BeaconState, error) {
				val, err := beaconState.ValidatorAtIndex(5)
				if err != nil {
					return nil, err
				}
				val.EffectiveBalance = params.BeaconConfig().MaxEffectiveBalance - params.BeaconConfig().EffectiveBalanceIncrement
				if err := beaconState.UpdateValidatorAtIndex(5, val); err != nil {
					return nil, err
				}
				return beaconState, nil
			},
			error: "",
		},
	}

	var err error
	var oldHTR []byte
	for _, tt := range initTests {
		t.Run(tt.name, func(t *testing.T) {
			testState, err = tt.stateModify(testState)
			assert.NoError(t, err)
			root, err := testState.HashTreeRoot(context.Background())
			if err == nil && tt.error != "" {
				t.Errorf("Expected error, expected %v, recevied %v", tt.error, err)
			}
			pbState, err := v1.ProtobufBeaconState(testState.InnerStateUnsafe())
			require.NoError(t, err)
			genericHTR, err := pbState.HashTreeRoot()
			if err == nil && tt.error != "" {
				t.Errorf("Expected error, expected %v, recevied %v", tt.error, err)
			}
			assert.DeepNotEqual(t, []byte{}, root[:], "Received empty hash tree root")
			assert.DeepEqual(t, genericHTR[:], root[:], "Expected hash tree root to match generic")
			if len(oldHTR) != 0 && bytes.Equal(root[:], oldHTR) {
				t.Errorf("Expected HTR to change, received %#x == old %#x", root, oldHTR)
			}
			oldHTR = root[:]
		})
	}
}

func TestBeaconState_HashTreeRoot_FieldTrie(t *testing.T) {
	testState, _ := testutil.DeterministicGenesisState(t, 64)

	type test struct {
		name        string
		stateModify func(iface.BeaconState) (iface.BeaconState, error)
		error       string
	}
	initTests := []test{
		{
			name: "unchanged state",
			stateModify: func(beaconState iface.BeaconState) (iface.BeaconState, error) {
				return beaconState, nil
			},
			error: "",
		},
		{
			name: "different slot",
			stateModify: func(beaconState iface.BeaconState) (iface.BeaconState, error) {
				if err := beaconState.SetSlot(5); err != nil {
					return nil, err
				}
				return beaconState, nil
			},
			error: "",
		},
		{
			name: "different validator balance",
			stateModify: func(beaconState iface.BeaconState) (iface.BeaconState, error) {
				val, err := beaconState.ValidatorAtIndex(5)
				if err != nil {
					return nil, err
				}
				val.EffectiveBalance = params.BeaconConfig().MaxEffectiveBalance - params.BeaconConfig().EffectiveBalanceIncrement
				if err := beaconState.UpdateValidatorAtIndex(5, val); err != nil {
					return nil, err
				}
				return beaconState, nil
			},
			error: "",
		},
	}

	var err error
	var oldHTR []byte
	for _, tt := range initTests {
		t.Run(tt.name, func(t *testing.T) {
			testState, err = tt.stateModify(testState)
			assert.NoError(t, err)
			root, err := testState.HashTreeRoot(context.Background())
			if err == nil && tt.error != "" {
				t.Errorf("Expected error, expected %v, recevied %v", tt.error, err)
			}
			pbState, err := v1.ProtobufBeaconState(testState.InnerStateUnsafe())
			require.NoError(t, err)
			genericHTR, err := pbState.HashTreeRoot()
			if err == nil && tt.error != "" {
				t.Errorf("Expected error, expected %v, recevied %v", tt.error, err)
			}
			assert.DeepNotEqual(t, []byte{}, root[:], "Received empty hash tree root")
			assert.DeepEqual(t, genericHTR[:], root[:], "Expected hash tree root to match generic")
			if len(oldHTR) != 0 && bytes.Equal(root[:], oldHTR) {
				t.Errorf("Expected HTR to change, received %#x == old %#x", root, oldHTR)
			}
			oldHTR = root[:]
		})
	}
}

func TestBeaconState_AppendValidator_DoesntMutateCopy(t *testing.T) {
	st0, err := testutil.NewBeaconState()
	require.NoError(t, err)
	st1 := st0.Copy()
	originalCount := st1.NumValidators()

	val := &eth.Validator{Slashed: true}
	assert.NoError(t, st0.AppendValidator(val))
	assert.Equal(t, originalCount, st1.NumValidators(), "st1 NumValidators mutated")
	_, ok := st1.ValidatorIndexByPubkey(bytesutil.ToBytes48(val.PublicKey))
	assert.Equal(t, false, ok, "Expected no validator index to be present in st1 for the newly inserted pubkey")
}

func TestBeaconState_ToProto(t *testing.T) {
	source, err := testutil.NewBeaconState(testutil.FillRootsNaturalOpt, func(state *pbp2p.BeaconState) error {
		state.GenesisTime = 1
		state.GenesisValidatorsRoot = bytesutil.PadTo([]byte("genesisvalidatorroot"), 32)
		state.Slot = 2
		state.Fork = &pbp2p.Fork{
			PreviousVersion: bytesutil.PadTo([]byte("123"), 4),
			CurrentVersion:  bytesutil.PadTo([]byte("456"), 4),
			Epoch:           3,
		}
		state.LatestBlockHeader = &eth.BeaconBlockHeader{
			Slot:          4,
			ProposerIndex: 5,
			ParentRoot:    bytesutil.PadTo([]byte("lbhparentroot"), 32),
			StateRoot:     bytesutil.PadTo([]byte("lbhstateroot"), 32),
			BodyRoot:      bytesutil.PadTo([]byte("lbhbodyroot"), 32),
		}
		state.BlockRoots = [][]byte{bytesutil.PadTo([]byte("blockroots"), 32)}
		state.StateRoots = [][]byte{bytesutil.PadTo([]byte("stateroots"), 32)}
		state.HistoricalRoots = [][]byte{bytesutil.PadTo([]byte("historicalroots"), 32)}
		state.Eth1Data = &eth.Eth1Data{
			DepositRoot:  bytesutil.PadTo([]byte("e1ddepositroot"), 32),
			DepositCount: 6,
			BlockHash:    bytesutil.PadTo([]byte("e1dblockhash"), 32),
		}
		state.Eth1DataVotes = []*eth.Eth1Data{{
			DepositRoot:  bytesutil.PadTo([]byte("e1dvdepositroot"), 32),
			DepositCount: 7,
			BlockHash:    bytesutil.PadTo([]byte("e1dvblockhash"), 32),
		}}
		state.Eth1DepositIndex = 8
		state.Validators = []*eth.Validator{{
			PublicKey:                  bytesutil.PadTo([]byte("publickey"), 48),
			WithdrawalCredentials:      bytesutil.PadTo([]byte("withdrawalcredentials"), 32),
			EffectiveBalance:           9,
			Slashed:                    true,
			ActivationEligibilityEpoch: 10,
			ActivationEpoch:            11,
			ExitEpoch:                  12,
			WithdrawableEpoch:          13,
		}}
		state.Balances = []uint64{14}
		state.RandaoMixes = [][]byte{bytesutil.PadTo([]byte("randaomixes"), 32)}
		state.Slashings = []uint64{15}
		state.PreviousEpochAttestations = []*pbp2p.PendingAttestation{{
			AggregationBits: bitfield.Bitlist{16},
			Data: &eth.AttestationData{
				Slot:            17,
				CommitteeIndex:  18,
				BeaconBlockRoot: bytesutil.PadTo([]byte("peabeaconblockroot"), 32),
				Source: &eth.Checkpoint{
					Epoch: 19,
					Root:  bytesutil.PadTo([]byte("peasroot"), 32),
				},
				Target: &eth.Checkpoint{
					Epoch: 20,
					Root:  bytesutil.PadTo([]byte("peatroot"), 32),
				},
			},
			InclusionDelay: 21,
			ProposerIndex:  22,
		}}
		state.CurrentEpochAttestations = []*pbp2p.PendingAttestation{{
			AggregationBits: bitfield.Bitlist{23},
			Data: &eth.AttestationData{
				Slot:            24,
				CommitteeIndex:  25,
				BeaconBlockRoot: bytesutil.PadTo([]byte("ceabeaconblockroot"), 32),
				Source: &eth.Checkpoint{
					Epoch: 26,
					Root:  bytesutil.PadTo([]byte("ceasroot"), 32),
				},
				Target: &eth.Checkpoint{
					Epoch: 27,
					Root:  bytesutil.PadTo([]byte("ceatroot"), 32),
				},
			},
			InclusionDelay: 28,
			ProposerIndex:  29,
		}}
		state.JustificationBits = bitfield.Bitvector4{1}
		state.PreviousJustifiedCheckpoint = &eth.Checkpoint{
			Epoch: 30,
			Root:  bytesutil.PadTo([]byte("pjcroot"), 32),
		}
		state.CurrentJustifiedCheckpoint = &eth.Checkpoint{
			Epoch: 31,
			Root:  bytesutil.PadTo([]byte("cjcroot"), 32),
		}
		state.FinalizedCheckpoint = &eth.Checkpoint{
			Epoch: 32,
			Root:  bytesutil.PadTo([]byte("fcroot"), 32),
		}
		return nil
	})
	require.NoError(t, err)

	result, err := source.ToProto()
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, uint64(1), result.GenesisTime)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("genesisvalidatorroot"), 32), result.GenesisValidatorsRoot)
	assert.Equal(t, types.Slot(2), result.Slot)
	resultFork := result.Fork
	require.NotNil(t, resultFork)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("123"), 4), resultFork.PreviousVersion)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("456"), 4), resultFork.CurrentVersion)
	assert.Equal(t, types.Epoch(3), resultFork.Epoch)
	resultLatestBlockHeader := result.LatestBlockHeader
	require.NotNil(t, resultLatestBlockHeader)
	assert.Equal(t, types.Slot(4), resultLatestBlockHeader.Slot)
	assert.Equal(t, types.ValidatorIndex(5), resultLatestBlockHeader.ProposerIndex)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("lbhparentroot"), 32), resultLatestBlockHeader.ParentRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("lbhstateroot"), 32), resultLatestBlockHeader.StateRoot)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("lbhbodyroot"), 32), resultLatestBlockHeader.BodyRoot)
	assert.DeepEqual(t, [][]byte{bytesutil.PadTo([]byte("blockroots"), 32)}, result.BlockRoots)
	assert.DeepEqual(t, [][]byte{bytesutil.PadTo([]byte("stateroots"), 32)}, result.StateRoots)
	assert.DeepEqual(t, [][]byte{bytesutil.PadTo([]byte("historicalroots"), 32)}, result.HistoricalRoots)
	resultEth1Data := result.Eth1Data
	require.NotNil(t, resultEth1Data)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("e1ddepositroot"), 32), resultEth1Data.DepositRoot)
	assert.Equal(t, uint64(6), resultEth1Data.DepositCount)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("e1dblockhash"), 32), resultEth1Data.BlockHash)
	require.Equal(t, 1, len(result.Eth1DataVotes))
	resultEth1DataVote := result.Eth1DataVotes[0]
	require.NotNil(t, resultEth1DataVote)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("e1dvdepositroot"), 32), resultEth1DataVote.DepositRoot)
	assert.Equal(t, uint64(7), resultEth1DataVote.DepositCount)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("e1dvblockhash"), 32), resultEth1DataVote.BlockHash)
	assert.Equal(t, uint64(8), result.Eth1DepositIndex)
	require.Equal(t, 1, len(result.Validators))
	resultValidator := result.Validators[0]
	require.NotNil(t, resultValidator)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("publickey"), 48), resultValidator.Pubkey)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("withdrawalcredentials"), 32), resultValidator.WithdrawalCredentials)
	assert.Equal(t, uint64(9), resultValidator.EffectiveBalance)
	assert.Equal(t, true, resultValidator.Slashed)
	assert.Equal(t, types.Epoch(10), resultValidator.ActivationEligibilityEpoch)
	assert.Equal(t, types.Epoch(11), resultValidator.ActivationEpoch)
	assert.Equal(t, types.Epoch(12), resultValidator.ExitEpoch)
	assert.Equal(t, types.Epoch(13), resultValidator.WithdrawableEpoch)
	assert.DeepEqual(t, []uint64{14}, result.Balances)
	assert.DeepEqual(t, [][]byte{bytesutil.PadTo([]byte("randaomixes"), 32)}, result.RandaoMixes)
	assert.DeepEqual(t, []uint64{15}, result.Slashings)
	require.Equal(t, 1, len(result.PreviousEpochAttestations))
	resultPrevEpochAtt := result.PreviousEpochAttestations[0]
	require.NotNil(t, resultPrevEpochAtt)
	assert.DeepEqual(t, bitfield.Bitlist{16}, resultPrevEpochAtt.AggregationBits)
	resultPrevEpochAttData := resultPrevEpochAtt.Data
	require.NotNil(t, resultPrevEpochAttData)
	assert.Equal(t, types.Slot(17), resultPrevEpochAttData.Slot)
	assert.Equal(t, types.CommitteeIndex(18), resultPrevEpochAttData.Index)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("peabeaconblockroot"), 32), resultPrevEpochAttData.BeaconBlockRoot)
	resultPrevEpochAttSource := resultPrevEpochAttData.Source
	require.NotNil(t, resultPrevEpochAttSource)
	assert.Equal(t, types.Epoch(19), resultPrevEpochAttSource.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("peasroot"), 32), resultPrevEpochAttSource.Root)
	resultPrevEpochAttTarget := resultPrevEpochAttData.Target
	require.NotNil(t, resultPrevEpochAttTarget)
	assert.Equal(t, types.Epoch(20), resultPrevEpochAttTarget.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("peatroot"), 32), resultPrevEpochAttTarget.Root)
	assert.Equal(t, types.Slot(21), resultPrevEpochAtt.InclusionDelay)
	assert.Equal(t, types.ValidatorIndex(22), resultPrevEpochAtt.ProposerIndex)
	resultCurrEpochAtt := result.CurrentEpochAttestations[0]
	require.NotNil(t, resultCurrEpochAtt)
	assert.DeepEqual(t, bitfield.Bitlist{23}, resultCurrEpochAtt.AggregationBits)
	resultCurrEpochAttData := resultCurrEpochAtt.Data
	require.NotNil(t, resultCurrEpochAttData)
	assert.Equal(t, types.Slot(24), resultCurrEpochAttData.Slot)
	assert.Equal(t, types.CommitteeIndex(25), resultCurrEpochAttData.Index)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("ceabeaconblockroot"), 32), resultCurrEpochAttData.BeaconBlockRoot)
	resultCurrEpochAttSource := resultCurrEpochAttData.Source
	require.NotNil(t, resultCurrEpochAttSource)
	assert.Equal(t, types.Epoch(26), resultCurrEpochAttSource.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("ceasroot"), 32), resultCurrEpochAttSource.Root)
	resultCurrEpochAttTarget := resultCurrEpochAttData.Target
	require.NotNil(t, resultCurrEpochAttTarget)
	assert.Equal(t, types.Epoch(27), resultCurrEpochAttTarget.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("ceatroot"), 32), resultCurrEpochAttTarget.Root)
	assert.Equal(t, types.Slot(28), resultCurrEpochAtt.InclusionDelay)
	assert.Equal(t, types.ValidatorIndex(29), resultCurrEpochAtt.ProposerIndex)
	assert.DeepEqual(t, bitfield.Bitvector4{1}, result.JustificationBits)
	resultPrevJustifiedCheckpoint := result.PreviousJustifiedCheckpoint
	require.NotNil(t, resultPrevJustifiedCheckpoint)
	assert.Equal(t, types.Epoch(30), resultPrevJustifiedCheckpoint.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("pjcroot"), 32), resultPrevJustifiedCheckpoint.Root)
	resultCurrJustifiedCheckpoint := result.CurrentJustifiedCheckpoint
	require.NotNil(t, resultCurrJustifiedCheckpoint)
	assert.Equal(t, types.Epoch(31), resultCurrJustifiedCheckpoint.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("cjcroot"), 32), resultCurrJustifiedCheckpoint.Root)
	resultFinalizedCheckpoint := result.FinalizedCheckpoint
	require.NotNil(t, resultFinalizedCheckpoint)
	assert.Equal(t, types.Epoch(32), resultFinalizedCheckpoint.Epoch)
	assert.DeepEqual(t, bytesutil.PadTo([]byte("fcroot"), 32), resultFinalizedCheckpoint.Root)
}
