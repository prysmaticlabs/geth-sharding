package simulator

import (
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	mock "github.com/prysmaticlabs/prysm/beacon-chain/blockchain/testing"
	dbtest "github.com/prysmaticlabs/prysm/beacon-chain/db/testing"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func setupService(t *testing.T, params *Parameters) *Simulator {
	slasherDB := dbtest.SetupSlasherDB(t)
	beaconState, err := testutil.NewBeaconState()
	require.NoError(t, err)

	// We setup validators in the beacon state along with their
	// private keys used to generate valid signatures in generated objects.
	validators := make([]*ethpb.Validator, params.NumValidators)
	privKeys := make(map[types.ValidatorIndex]bls.SecretKey)
	for valIdx := range validators {
		privKey, err := bls.RandKey()
		require.NoError(t, err)
		privKeys[types.ValidatorIndex(valIdx)] = privKey
		validators[valIdx] = &ethpb.Validator{
			PublicKey:             privKey.PublicKey().Marshal(),
			WithdrawalCredentials: make([]byte, 32),
		}
	}
	err = beaconState.SetValidators(validators)
	require.NoError(t, err)

	gen := stategen.NewMockService()
	gen.AddStateForRoot(beaconState, [32]byte{})
	return &Simulator{
		srvConfig: &ServiceConfig{
			Params:                      params,
			Database:                    slasherDB,
			AttestationStateFetcher:     &mock.ChainService{State: beaconState},
			PrivateKeysByValidatorIndex: privKeys,
			StateGen:                    gen,
		},
	}
}
