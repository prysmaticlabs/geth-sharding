package client

import (
	"context"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	mockSlasher "github.com/prysmaticlabs/prysm/validator/testing"
)

func TestPreBlockSignLocalValidation_PreventsLowerThanMinProposal(t *testing.T) {
	ctx := context.Background()
	validator, _, validatorKey, finish := setup(t)
	defer finish()
	lowestSignedSlot := types.Slot(10)
	pubKeyBytes := [48]byte{}
	copy(pubKeyBytes[:], validatorKey.PublicKey().Marshal())

	// We save a proposal at the lowest signed slot in the DB.
	err := validator.db.SaveProposalHistoryForSlot(ctx, pubKeyBytes, lowestSignedSlot, []byte{1})
	require.NoError(t, err)
	require.NoError(t, err)

	// We expect the same block with a slot lower than the lowest
	// signed slot to fail validation.
	block := &ethpb.BeaconBlock{
		Slot:          lowestSignedSlot - 1,
		ProposerIndex: 0,
	}
	err = validator.preBlockSignValidations(context.Background(), pubKeyBytes, wrapper.WrappedPhase0BeaconBlock(block), [32]byte{4})
	require.ErrorContains(t, "could not sign block with slot <= lowest signed", err)

	// We expect the same block with a slot equal to the lowest
	// signed slot to pass validation if signing roots are equal.
	block = &ethpb.BeaconBlock{
		Slot:          lowestSignedSlot,
		ProposerIndex: 0,
	}
	err = validator.preBlockSignValidations(context.Background(), pubKeyBytes, wrapper.WrappedPhase0BeaconBlock(block), [32]byte{1})
	require.NoError(t, err)

	// We expect the same block with a slot equal to the lowest
	// signed slot to fail validation if signing roots are different.
	block = &ethpb.BeaconBlock{
		Slot:          lowestSignedSlot,
		ProposerIndex: 0,
	}
	err = validator.preBlockSignValidations(context.Background(), pubKeyBytes, wrapper.WrappedPhase0BeaconBlock(block), [32]byte{4})
	require.ErrorContains(t, failedPreBlockSignLocalErr, err)

	// We expect the same block with a slot > than the lowest
	// signed slot to pass validation.
	block = &ethpb.BeaconBlock{
		Slot:          lowestSignedSlot + 1,
		ProposerIndex: 0,
	}
	err = validator.preBlockSignValidations(context.Background(), pubKeyBytes, wrapper.WrappedPhase0BeaconBlock(block), [32]byte{3})
	require.NoError(t, err)
}

func TestPreBlockSignLocalValidation(t *testing.T) {
	ctx := context.Background()
	config := &featureconfig.Flags{
		SlasherProtection: false,
	}
	reset := featureconfig.InitWithReset(config)
	defer reset()
	validator, _, validatorKey, finish := setup(t)
	defer finish()

	block := &ethpb.BeaconBlock{
		Slot:          10,
		ProposerIndex: 0,
	}
	pubKeyBytes := [48]byte{}
	copy(pubKeyBytes[:], validatorKey.PublicKey().Marshal())

	// We save a proposal at slot 1 as our lowest proposal.
	err := validator.db.SaveProposalHistoryForSlot(ctx, pubKeyBytes, 1, []byte{1})
	require.NoError(t, err)

	// We save a proposal at slot 10 with a dummy signing root.
	dummySigningRoot := [32]byte{1}
	err = validator.db.SaveProposalHistoryForSlot(ctx, pubKeyBytes, 10, dummySigningRoot[:])
	require.NoError(t, err)
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())

	// We expect the same block sent out with the same root should not be slasahble.
	err = validator.preBlockSignValidations(context.Background(), pubKey, wrapper.WrappedPhase0BeaconBlock(block), dummySigningRoot)
	require.NoError(t, err)

	// We expect the same block sent out with a different signing root should be slasahble.
	err = validator.preBlockSignValidations(context.Background(), pubKey, wrapper.WrappedPhase0BeaconBlock(block), [32]byte{2})
	require.ErrorContains(t, failedPreBlockSignLocalErr, err)

	// We save a proposal at slot 11 with a nil signing root.
	block.Slot = 11
	err = validator.db.SaveProposalHistoryForSlot(ctx, pubKeyBytes, block.Slot, nil)
	require.NoError(t, err)

	// We expect the same block sent out should return slashable error even
	// if we had a nil signing root stored in the database.
	err = validator.preBlockSignValidations(context.Background(), pubKey, wrapper.WrappedPhase0BeaconBlock(block), [32]byte{2})
	require.ErrorContains(t, failedPreBlockSignLocalErr, err)

	// A block with a different slot for which we do not have a proposing history
	// should not be failing validation.
	block.Slot = 9
	err = validator.preBlockSignValidations(context.Background(), pubKey, wrapper.WrappedPhase0BeaconBlock(block), [32]byte{3})
	require.NoError(t, err, "Expected allowed block not to throw error")
}

func TestPreBlockSignValidation(t *testing.T) {
	config := &featureconfig.Flags{
		SlasherProtection: true,
	}
	reset := featureconfig.InitWithReset(config)
	defer reset()
	validator, _, validatorKey, finish := setup(t)
	defer finish()
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())

	block := testutil.NewBeaconBlock()
	block.Block.Slot = 10
	mockProtector := &mockSlasher.MockProtector{AllowBlock: false}
	validator.protector = mockProtector
	err := validator.preBlockSignValidations(context.Background(), pubKey, wrapper.WrappedPhase0BeaconBlock(block.Block), [32]byte{2})
	require.ErrorContains(t, failedPreBlockSignExternalErr, err)
	mockProtector.AllowBlock = true
	err = validator.preBlockSignValidations(context.Background(), pubKey, wrapper.WrappedPhase0BeaconBlock(block.Block), [32]byte{2})
	require.NoError(t, err, "Expected allowed block not to throw error")
}

func TestPostBlockSignUpdate(t *testing.T) {
	config := &featureconfig.Flags{
		SlasherProtection: true,
	}
	reset := featureconfig.InitWithReset(config)
	defer reset()
	validator, _, validatorKey, finish := setup(t)
	defer finish()
	pubKey := [48]byte{}
	copy(pubKey[:], validatorKey.PublicKey().Marshal())
	emptyBlock := testutil.NewBeaconBlock()
	emptyBlock.Block.Slot = 10
	emptyBlock.Block.ProposerIndex = 0
	mockProtector := &mockSlasher.MockProtector{AllowBlock: false}
	validator.protector = mockProtector
	err := validator.postBlockSignUpdate(context.Background(), pubKey, wrapper.WrappedPhase0SignedBeaconBlock(emptyBlock), [32]byte{})
	require.ErrorContains(t, failedPostBlockSignErr, err, "Expected error when post signature update is detected as slashable")
	mockProtector.AllowBlock = true
	err = validator.postBlockSignUpdate(context.Background(), pubKey, wrapper.WrappedPhase0SignedBeaconBlock(emptyBlock), [32]byte{})
	require.NoError(t, err, "Expected allowed block not to throw error")
}
