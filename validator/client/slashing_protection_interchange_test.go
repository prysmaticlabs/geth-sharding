package client

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/fileutil"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	interchangeformat "github.com/prysmaticlabs/prysm/validator/slashing-protection/local/standard-protection-format"
)

type eip3076TestCase struct {
	Name                  string `json:"name"`
	GenesisValidatorsRoot string `json:"genesis_validators_root"`
	Steps                 []struct {
		ShouldSucceed      bool `json:"should_succeed"`
		AllowPartialImport bool `json:"allow_partial_import"`
		Interchange        struct {
			Metadata struct {
				InterchangeFormatVersion string `json:"interchange_format_version"`
				GenesisValidatorsRoot    string `json:"genesis_validators_root"`
			} `json:"metadata"`
			Data []struct {
				Pubkey       string `json:"pubkey"`
				SignedBlocks []struct {
					Slot        string `json:"slot"`
					SigningRoot string `json:"signing_root"`
				} `json:"signed_blocks"`
				SignedAttestations []struct {
					SourceEpoch string `json:"source_epoch"`
					TargetEpoch string `json:"target_epoch"`
					SigningRoot string `json:"signing_root"`
				} `json:"signed_attestations"`
			} `json:"data"`
		} `json:"interchange"`
		Blocks []struct {
			Pubkey        string `json:"pubkey"`
			Slot          string `json:"slot"`
			SigningRoot   string `json:"signing_root"`
			ShouldSucceed bool   `json:"should_succeed"`
		} `json:"blocks"`
		Attestations []struct {
			Pubkey        string `json:"pubkey"`
			SourceEpoch   string `json:"source_epoch"`
			TargetEpoch   string `json:"target_epoch"`
			SigningRoot   string `json:"signing_root"`
			ShouldSucceed bool   `json:"should_succeed"`
		} `json:"attestations"`
	} `json:"steps"`
}

func setupEIP3076SpecTests(t *testing.T) []*eip3076TestCase {
	testFolders, err := bazel.ListRunfiles()
	require.NoError(t, err)
	testCases := make([]*eip3076TestCase, 0)
	for _, ff := range testFolders {
		if strings.Contains(ff.ShortPath, "eip3076_spec_tests") &&
			strings.Contains(ff.ShortPath, "generated/") {
			enc, err := fileutil.ReadFileAsBytes(ff.Path)
			require.NoError(t, err)
			testCase := &eip3076TestCase{}
			require.NoError(t, json.Unmarshal(enc, testCase))
			testCases = append(testCases, testCase)
		}
	}
	return testCases
}

func TestEIP3076SpecTests(t *testing.T) {
	config := &featureconfig.Flags{
		SlasherProtection: true,
	}
	reset := featureconfig.InitWithReset(config)
	defer reset()

	testCases := setupEIP3076SpecTests(t)
	for _, tt := range testCases {
		t.Run(tt.Name, func(t *testing.T) {
			if tt.Name == "" {
				t.Skip("Skipping eip3076TestCase with empty name")
			}
			for _, step := range tt.Steps {
				// Set up validator client, one new validator client per eip3076TestCase.
				// This ensures we initialize a new (empty) slashing protection database.
				validator, _, _, _ := setup(t)

				if tt.GenesisValidatorsRoot != "" {
					r, err := interchangeformat.RootFromHex(tt.GenesisValidatorsRoot)
					require.NoError(t, validator.db.SaveGenesisValidatorsRoot(context.Background(), r[:]))
					require.NoError(t, err)
				}

				// The eip3076TestCase config contains the interchange config in json.
				// This loads the interchange data via ImportStandardProtectionJSON.
				interchangeBytes, err := json.Marshal(step.Interchange)
				if err != nil {
					t.Fatal(err)
				}
				b := bytes.NewBuffer(interchangeBytes)
				if err := interchangeformat.ImportStandardProtectionJSON(context.Background(), validator.db, b); err != nil {
					if step.ShouldSucceed {
						t.Fatal(err)
					}
				} else if !step.ShouldSucceed {
					require.NotNil(t, err, "import standard protection json should have failed")
				}

				// This loops through a list of block signings to attempt after importing the interchange data above.
				for _, sb := range step.Blocks {
					bSlot, err := interchangeformat.SlotFromString(sb.Slot)
					require.NoError(t, err)
					pk, err := interchangeformat.PubKeyFromHex(sb.Pubkey)
					require.NoError(t, err)
					b := testutil.NewBeaconBlock()
					b.Block.Slot = bSlot

					var signingRoot [32]byte
					if sb.SigningRoot != "" {
						signingRootBytes, err := hex.DecodeString(strings.TrimPrefix(sb.SigningRoot, "0x"))
						require.NoError(t, err)
						copy(signingRoot[:], signingRootBytes)
					}

					err = validator.preBlockSignValidations(context.Background(), pk, wrapper.WrappedPhase0BeaconBlock(b.Block), signingRoot)
					if sb.ShouldSucceed {
						require.NoError(t, err)
					} else {
						require.NotEqual(t, nil, err, "pre validation should have failed for block")
					}

					// Only proceed post update if pre validation did not error.
					if err == nil {
						err = validator.postBlockSignUpdate(context.Background(), pk, wrapper.WrappedPhase0SignedBeaconBlock(b), signingRoot)
						if sb.ShouldSucceed {
							require.NoError(t, err)
						} else {
							require.NotEqual(t, nil, err, "post validation should have failed for block")
						}
					}
				}

				// This loops through a list of attestation signings to attempt after importing the interchange data above.
				for _, sa := range step.Attestations {
					target, err := interchangeformat.EpochFromString(sa.TargetEpoch)
					require.NoError(t, err)
					source, err := interchangeformat.EpochFromString(sa.SourceEpoch)
					require.NoError(t, err)
					pk, err := interchangeformat.PubKeyFromHex(sa.Pubkey)
					require.NoError(t, err)
					ia := &ethpb.IndexedAttestation{
						Data: &ethpb.AttestationData{
							BeaconBlockRoot: make([]byte, 32),
							Target:          &ethpb.Checkpoint{Epoch: target, Root: make([]byte, 32)},
							Source:          &ethpb.Checkpoint{Epoch: source, Root: make([]byte, 32)},
						},
						Signature: make([]byte, 96),
					}

					var signingRoot [32]byte
					if sa.SigningRoot != "" {
						signingRootBytes, err := hex.DecodeString(strings.TrimPrefix(sa.SigningRoot, "0x"))
						require.NoError(t, err)
						copy(signingRoot[:], signingRootBytes)
					}

					err = validator.slashableAttestationCheck(context.Background(), ia, pk, signingRoot)
					if sa.ShouldSucceed {
						require.NoError(t, err)
					} else {
						require.NotNil(t, err, "pre validation should have failed for attestation")
					}
				}
				require.NoError(t, err, validator.db.Close())
			}
		})
	}
}
