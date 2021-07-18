package ssz_static

import (
	"context"
	"encoding/hex"
	"errors"
	"path"
	"testing"

	fssz "github.com/ferranbt/fastssz"
	"github.com/golang/snappy"
	stateAltair "github.com/prysmaticlabs/prysm/beacon-chain/state/v2"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/testutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/spectest/utils"
)

// SSZRoots --
type SSZRoots struct {
	Root        string `json:"root"`
	SigningRoot string `json:"signing_root"`
}

// RunSSZStaticTests executes "ssz_static" tests.
func RunSSZStaticTests(t *testing.T, config string) {
	require.NoError(t, utils.SetConfig(t, config))

	testFolders, _ := utils.TestFolders(t, config, "altair", "ssz_static")
	for _, folder := range testFolders {
		innerPath := path.Join("ssz_static", folder.Name(), "ssz_random")
		innerTestFolders, innerTestsFolderPath := utils.TestFolders(t, config, "altair", innerPath)

		for _, innerFolder := range innerTestFolders {
			t.Run(path.Join(folder.Name(), innerFolder.Name()), func(t *testing.T) {
				serializedBytes, err := testutil.BazelFileBytes(innerTestsFolderPath, innerFolder.Name(), "serialized.ssz_snappy")
				require.NoError(t, err)
				serializedSSZ, err := snappy.Decode(nil /* dst */, serializedBytes)
				require.NoError(t, err, "Failed to decompress")
				object, err := UnmarshalledSSZ(t, serializedSSZ, folder.Name())
				require.NoError(t, err, "Could not unmarshall serialized SSZ")

				rootsYamlFile, err := testutil.BazelFileBytes(innerTestsFolderPath, innerFolder.Name(), "roots.yaml")
				require.NoError(t, err)
				rootsYaml := &SSZRoots{}
				require.NoError(t, utils.UnmarshalYaml(rootsYamlFile, rootsYaml), "Failed to Unmarshal")

				// Custom hash tree root for beacon state.
				var htr func(interface{}) ([32]byte, error)
				if _, ok := object.(*pb.BeaconStateAltair); ok {
					htr = func(s interface{}) ([32]byte, error) {
						beaconState, err := stateAltair.InitializeFromProto(s.(*pb.BeaconStateAltair))
						require.NoError(t, err)
						return beaconState.HashTreeRoot(context.Background())
					}
				} else {
					htr = func(s interface{}) ([32]byte, error) {
						sszObj, ok := s.(fssz.HashRoot)
						if !ok {
							return [32]byte{}, errors.New("could not get hash root, not compatible object")
						}
						return sszObj.HashTreeRoot()
					}
				}

				root, err := htr(object)
				require.NoError(t, err)
				rootBytes, err := hex.DecodeString(rootsYaml.Root[2:])
				require.NoError(t, err)
				require.DeepEqual(t, rootBytes, root[:], "Did not receive expected hash tree root")

				if rootsYaml.SigningRoot == "" {
					return
				}

				var signingRoot [32]byte
				if v, ok := object.(fssz.HashRoot); ok {
					signingRoot, err = v.HashTreeRoot()
				} else {
					t.Fatal("object does not meet fssz.HashRoot")
				}

				require.NoError(t, err)
				signingRootBytes, err := hex.DecodeString(rootsYaml.SigningRoot[2:])
				require.NoError(t, err)
				require.DeepEqual(t, signingRootBytes, signingRoot[:], "Did not receive expected signing root")
			})
		}
	}
}

// UnmarshalledSSZ unmarshalls serialized input.
func UnmarshalledSSZ(t *testing.T, serializedBytes []byte, folderName string) (interface{}, error) {
	var obj interface{}
	switch folderName {
	case "Attestation":
		obj = &ethpb.Attestation{}
	case "AttestationData":
		obj = &ethpb.AttestationData{}
	case "AttesterSlashing":
		obj = &ethpb.AttesterSlashing{}
	case "AggregateAndProof":
		obj = &ethpb.AggregateAttestationAndProof{}
	case "BeaconBlock":
		obj = &prysmv2.BeaconBlock{}
	case "BeaconBlockBody":
		obj = &prysmv2.BeaconBlockBody{}
	case "BeaconBlockHeader":
		obj = &ethpb.BeaconBlockHeader{}
	case "BeaconState":
		obj = &pb.BeaconStateAltair{}
	case "Checkpoint":
		obj = &ethpb.Checkpoint{}
	case "Deposit":
		obj = &ethpb.Deposit{}
	case "DepositMessage":
		obj = &pb.DepositMessage{}
	case "DepositData":
		obj = &ethpb.Deposit_Data{}
	case "Eth1Data":
		obj = &ethpb.Eth1Data{}
	case "Eth1Block":
		t.Skip("Unused type")
		return nil, nil
	case "Fork":
		obj = &pb.Fork{}
	case "ForkData":
		obj = &pb.ForkData{}
	case "HistoricalBatch":
		obj = &pb.HistoricalBatch{}
	case "IndexedAttestation":
		obj = &ethpb.IndexedAttestation{}
	case "PendingAttestation":
		obj = &pb.PendingAttestation{}
	case "ProposerSlashing":
		obj = &ethpb.ProposerSlashing{}
	case "SignedAggregateAndProof":
		obj = &ethpb.SignedAggregateAttestationAndProof{}
	case "SignedBeaconBlock":
		obj = &prysmv2.SignedBeaconBlock{}
	case "SignedBeaconBlockHeader":
		obj = &ethpb.SignedBeaconBlockHeader{}
	case "SignedVoluntaryExit":
		obj = &ethpb.SignedVoluntaryExit{}
	case "SigningData":
		obj = &pb.SigningData{}
	case "Validator":
		obj = &ethpb.Validator{}
	case "VoluntaryExit":
		obj = &ethpb.VoluntaryExit{}
	case "SyncCommitteeMessage":
		obj = &prysmv2.SyncCommitteeMessage{}
	case "SyncCommitteeContribution":
		obj = &prysmv2.SyncCommitteeContribution{}
	case "ContributionAndProof":
		obj = &prysmv2.ContributionAndProof{}
	case "SignedContributionAndProof":
		obj = &prysmv2.SignedContributionAndProof{}
	case "SyncAggregate":
		obj = &prysmv2.SyncAggregate{}
	case "SyncAggregatorSelectionData":
		obj = &pb.SyncAggregatorSelectionData{}
	case "SyncCommittee":
		t.Skip("TODO(8638): fssz bug, using custom HTR so state works")
		return nil, nil
	case "LightClientSnapshot":
		t.Skip("not a beacon node type, this is a light node type")
		return nil, nil
	case "LightClientUpdate":
		t.Skip("not a beacon node type, this is a light node type")
		return nil, nil
	default:
		return nil, errors.New("type not found")
	}
	var err error
	if o, ok := obj.(fssz.Unmarshaler); ok {
		err = o.UnmarshalSSZ(serializedBytes)
	} else {
		err = errors.New("could not unmarshal object, not a fastssz compatible object")
	}
	return obj, err
}
