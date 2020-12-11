package interchangeformat

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/hashutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"github.com/prysmaticlabs/prysm/validator/db/kv"
	dbtest "github.com/prysmaticlabs/prysm/validator/db/testing"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

func TestStore_ImportInterchangeData_BadJSON(t *testing.T) {
	ctx := context.Background()
	validatorDB := dbtest.SetupDB(t, nil)

	buf := bytes.NewBuffer([]byte("helloworld"))
	err := ImportStandardProtectionJSON(ctx, validatorDB, buf)
	require.ErrorContains(t, "could not unmarshal slashing protection JSON file", err)
}

func TestStore_ImportInterchangeData_NilData_FailsSilently(t *testing.T) {
	hook := logTest.NewGlobal()
	ctx := context.Background()
	validatorDB := dbtest.SetupDB(t, nil)

	interchangeJSON := &EIPSlashingProtectionFormat{}
	encoded, err := json.Marshal(interchangeJSON)
	require.NoError(t, err)

	buf := bytes.NewBuffer(encoded)
	err = ImportStandardProtectionJSON(ctx, validatorDB, buf)
	require.NoError(t, err)
	require.LogsContain(t, hook, "No slashing protection data to import")
}

func Test_validateMetadata(t *testing.T) {
	goodRoot := [32]byte{1}
	goodStr := make([]byte, hex.EncodedLen(len(goodRoot)))
	hex.Encode(goodStr, goodRoot[:])
	tests := []struct {
		name                   string
		interchangeJSON        *EIPSlashingProtectionFormat
		dbGenesisValidatorRoot []byte
		wantErr                bool
		wantFatal              string
	}{
		{
			name: "Incorrect version for EIP format should fail",
			interchangeJSON: &EIPSlashingProtectionFormat{
				Metadata: struct {
					InterchangeFormatVersion string `json:"interchange_format_version"`
					GenesisValidatorsRoot    string `json:"genesis_validators_root"`
				}{
					InterchangeFormatVersion: "1",
					GenesisValidatorsRoot:    string(goodStr),
				},
			},
			wantErr: true,
		},
		{
			name: "Junk data for version should fail",
			interchangeJSON: &EIPSlashingProtectionFormat{
				Metadata: struct {
					InterchangeFormatVersion string `json:"interchange_format_version"`
					GenesisValidatorsRoot    string `json:"genesis_validators_root"`
				}{
					InterchangeFormatVersion: "asdljas$d",
					GenesisValidatorsRoot:    string(goodStr),
				},
			},
			wantErr: true,
		},
		{
			name: "Proper version field should pass",
			interchangeJSON: &EIPSlashingProtectionFormat{
				Metadata: struct {
					InterchangeFormatVersion string `json:"interchange_format_version"`
					GenesisValidatorsRoot    string `json:"genesis_validators_root"`
				}{
					InterchangeFormatVersion: INTERCHANGE_FORMAT_VERSION,
					GenesisValidatorsRoot:    string(goodStr),
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validatorDB := dbtest.SetupDB(t, nil)
			ctx := context.Background()
			if err := validateMetadata(ctx, validatorDB, tt.interchangeJSON); (err != nil) != tt.wantErr {
				t.Errorf("validateMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}

		})
	}
}

func Test_validateMetadataGenesisValidatorRoot(t *testing.T) {
	goodRoot := [32]byte{1}
	goodStr := make([]byte, hex.EncodedLen(len(goodRoot)))
	hex.Encode(goodStr, goodRoot[:])
	secondRoot := [32]byte{2}
	secondStr := make([]byte, hex.EncodedLen(len(secondRoot)))
	hex.Encode(secondStr, secondRoot[:])

	tests := []struct {
		name                   string
		interchangeJSON        *EIPSlashingProtectionFormat
		dbGenesisValidatorRoot []byte
		wantErr                bool
	}{
		{
			name: "Same genesis roots should not fail",
			interchangeJSON: &EIPSlashingProtectionFormat{
				Metadata: struct {
					InterchangeFormatVersion string `json:"interchange_format_version"`
					GenesisValidatorsRoot    string `json:"genesis_validators_root"`
				}{
					InterchangeFormatVersion: INTERCHANGE_FORMAT_VERSION,
					GenesisValidatorsRoot:    string(goodStr),
				},
			},
			dbGenesisValidatorRoot: goodRoot[:],
			wantErr:                false,
		},
		{
			name: "Different genesis roots should not fail",
			interchangeJSON: &EIPSlashingProtectionFormat{
				Metadata: struct {
					InterchangeFormatVersion string `json:"interchange_format_version"`
					GenesisValidatorsRoot    string `json:"genesis_validators_root"`
				}{
					InterchangeFormatVersion: INTERCHANGE_FORMAT_VERSION,
					GenesisValidatorsRoot:    string(secondStr),
				},
			},
			dbGenesisValidatorRoot: goodRoot[:],
			wantErr:                true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validatorDB := dbtest.SetupDB(t, nil)
			ctx := context.Background()
			require.NoError(t, validatorDB.SaveGenesisValidatorsRoot(ctx, tt.dbGenesisValidatorRoot))
			err := validateMetadata(ctx, validatorDB, tt.interchangeJSON)
			if tt.wantErr {
				require.ErrorContains(t, "genesis validator root doesnt match the one that is stored", err)
			} else {
				require.NoError(t, err)
			}

		})
	}
}

func Test_parseUniqueSignedBlocksByPubKey(t *testing.T) {
	numValidators := 4
	pubKeys := createRandomPubKeys(t, numValidators)
	roots := createRandomRoots(t, numValidators)
	tests := []struct {
		name    string
		data    []*ProtectionData
		want    map[[48]byte][]*SignedBlock
		wantErr bool
	}{
		{
			name: "nil values are skipped",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						nil,
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "3",
							SigningRoot: fmt.Sprintf("%x", roots[2]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedBlock{
				pubKeys[0]: {
					{
						Slot:        "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						Slot:        "3",
						SigningRoot: fmt.Sprintf("%x", roots[2]),
					},
				},
			},
		},
		{
			name: "same blocks but different public keys are parsed correctly",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						{
							Slot:        "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[1]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						{
							Slot:        "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedBlock{
				pubKeys[0]: {
					{
						Slot:        "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						Slot:        "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
				},
				pubKeys[1]: {
					{
						Slot:        "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						Slot:        "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
				},
			},
		},
		{
			name: "disjoint sets of signed blocks by the same public key are parsed correctly",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						{
							Slot:        "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "3",
							SigningRoot: fmt.Sprintf("%x", roots[2]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedBlock{
				pubKeys[0]: {
					{
						Slot:        "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						Slot:        "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
					{
						Slot:        "3",
						SigningRoot: fmt.Sprintf("%x", roots[2]),
					},
				},
			},
		},
		{
			name: "full duplicate entries are uniquely parsed",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedBlock{
				pubKeys[0]: {
					{
						Slot:        "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						Slot:        "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
				},
			},
		},
		{
			name: "intersecting duplicate public key entries are handled properly",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						{
							Slot:        "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedBlocks: []*SignedBlock{
						{
							Slot:        "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
						{
							Slot:        "3",
							SigningRoot: fmt.Sprintf("%x", roots[2]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedBlock{
				pubKeys[0]: {
					{
						Slot:        "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						Slot:        "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
					{
						Slot:        "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
					{
						Slot:        "3",
						SigningRoot: fmt.Sprintf("%x", roots[2]),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBlocksForUniquePublicKeys(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBlocksForUniquePublicKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseBlocksForUniquePublicKeys() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseUniqueSignedAttestationsByPubKey(t *testing.T) {
	numValidators := 4
	pubKeys := createRandomPubKeys(t, numValidators)
	roots := createRandomRoots(t, numValidators)
	tests := []struct {
		name    string
		data    []*ProtectionData
		want    map[[48]byte][]*SignedAttestation
		wantErr bool
	}{
		{
			name: "nil values are skipped",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "1",
							TargetEpoch: "3",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						nil,
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "3",
							TargetEpoch: "5",
							SigningRoot: fmt.Sprintf("%x", roots[2]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedAttestation{
				pubKeys[0]: {
					{
						SourceEpoch: "1",
						TargetEpoch: "3",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						SourceEpoch: "3",
						TargetEpoch: "5",
						SigningRoot: fmt.Sprintf("%x", roots[2]),
					},
				},
			},
		},
		{
			name: "same attestations but different public keys are parsed correctly",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						{
							SourceEpoch: "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[1]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						{
							SourceEpoch: "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedAttestation{
				pubKeys[0]: {
					{
						SourceEpoch: "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						SourceEpoch: "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
				},
				pubKeys[1]: {
					{
						SourceEpoch: "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						SourceEpoch: "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
				},
			},
		},
		{
			name: "disjoint sets of signed attestations by the same public key are parsed correctly",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "1",
							TargetEpoch: "3",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						{
							SourceEpoch: "2",
							TargetEpoch: "4",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "3",
							TargetEpoch: "5",
							SigningRoot: fmt.Sprintf("%x", roots[2]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedAttestation{
				pubKeys[0]: {
					{
						SourceEpoch: "1",
						TargetEpoch: "3",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
					{
						SourceEpoch: "3",
						TargetEpoch: "5",
						SigningRoot: fmt.Sprintf("%x", roots[2]),
					},
				},
			},
		},
		{
			name: "full duplicate entries are uniquely parsed",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedAttestation{
				pubKeys[0]: {
					{
						SourceEpoch: "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						SourceEpoch: "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
				},
			},
		},
		{
			name: "intersecting duplicate public key entries are handled properly",
			data: []*ProtectionData{
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "1",
							SigningRoot: fmt.Sprintf("%x", roots[0]),
						},
						{
							SourceEpoch: "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
					},
				},
				{
					Pubkey: fmt.Sprintf("%x", pubKeys[0]),
					SignedAttestations: []*SignedAttestation{
						{
							SourceEpoch: "2",
							SigningRoot: fmt.Sprintf("%x", roots[1]),
						},
						{
							SourceEpoch: "3",
							SigningRoot: fmt.Sprintf("%x", roots[2]),
						},
					},
				},
			},
			want: map[[48]byte][]*SignedAttestation{
				pubKeys[0]: {
					{
						SourceEpoch: "1",
						SigningRoot: fmt.Sprintf("%x", roots[0]),
					},
					{
						SourceEpoch: "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
					{
						SourceEpoch: "2",
						SigningRoot: fmt.Sprintf("%x", roots[1]),
					},
					{
						SourceEpoch: "3",
						SigningRoot: fmt.Sprintf("%x", roots[2]),
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAttestationsForUniquePublicKeys(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseAttestationsForUniquePublicKeys() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseAttestationsForUniquePublicKeys() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_saveLowestSourceTargetToDBt_Ok(t *testing.T) {
	ctx := context.Background()
	numValidators := 2
	publicKeys := createRandomPubKeys(t, numValidators)
	validatorDB := dbtest.SetupDB(t, publicKeys)

	m := make(map[[48]byte][]*SignedAttestation)
	m[publicKeys[0]] = []*SignedAttestation{{SourceEpoch: "1", TargetEpoch: "2"}, {SourceEpoch: "3", TargetEpoch: "4"}}
	m[publicKeys[1]] = []*SignedAttestation{{SourceEpoch: "8", TargetEpoch: "7"}, {SourceEpoch: "6", TargetEpoch: "5"}}
	require.NoError(t, saveLowestSourceTargetToDB(ctx, validatorDB, m))

	got, e, err := validatorDB.LowestSignedTargetEpoch(ctx, publicKeys[0])
	require.NoError(t, err)
	require.Equal(t, true, e)
	require.Equal(t, uint64(2), got)
	got, e, err = validatorDB.LowestSignedTargetEpoch(ctx, publicKeys[1])
	require.NoError(t, err)
	require.Equal(t, true, e)
	require.Equal(t, uint64(5), got)
	got, e, err = validatorDB.LowestSignedSourceEpoch(ctx, publicKeys[0])
	require.NoError(t, err)
	require.Equal(t, true, e)
	require.Equal(t, uint64(1), got)
	got, e, err = validatorDB.LowestSignedSourceEpoch(ctx, publicKeys[1])
	require.NoError(t, err)
	require.Equal(t, true, e)
	require.Equal(t, uint64(6), got)
}

func Test_filterSlashablePubKeysFromBlocks(t *testing.T) {
	var tests = []struct {
		name     string
		expected [][48]byte
		given    map[[48]byte][]*SignedBlock
	}{
		{
			name:     "No slashable keys returns empty",
			expected: make([][48]byte, 0),
			given: map[[48]byte][]*SignedBlock{
				{1}: {
					{
						Slot: "1",
					},
					{
						Slot: "2",
					},
				},
				{2}: {
					{
						Slot: "2",
					},
					{
						Slot: "3",
					},
				},
			},
		},
		{
			name:     "Empty data returns empty",
			expected: make([][48]byte, 0),
			given:    make(map[[48]byte][]*SignedBlock),
		},
		{
			name: "Properly finds public keys with slashable data",
			expected: [][48]byte{
				{1}, {3},
			},
			given: map[[48]byte][]*SignedBlock{
				{1}: {
					{
						Slot: "1",
					},
					{
						Slot: "1",
					},
					{
						Slot: "2",
					},
				},
				{2}: {
					{
						Slot: "2",
					},
					{
						Slot: "3",
					},
				},
				{3}: {
					{
						Slot: "3",
					},
					{
						Slot: "3",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			historyByPubKey := make(map[[48]byte]kv.ProposalHistoryForPubkey)
			for pubKey, signedBlocks := range tt.given {
				proposalHistory, err := transformSignedBlocks(ctx, signedBlocks)
				require.NoError(t, err)
				historyByPubKey[pubKey] = *proposalHistory
			}
			slashablePubKeys := filterSlashablePubKeysFromBlocks(context.Background(), historyByPubKey)
			wantedPubKeys := make(map[[48]byte]bool)
			for _, pk := range tt.expected {
				wantedPubKeys[pk] = true
			}
			for _, pk := range slashablePubKeys {
				ok := wantedPubKeys[pk]
				require.Equal(t, true, ok)
			}
		})
	}
}

func Test_filterSlashablePubKeysFromAttestations(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name                 string
		previousAttsByPubKey map[[48]byte][]*SignedAttestation
		incomingAttsByPubKey map[[48]byte][]*SignedAttestation
		want                 map[[48]byte]bool
		wantErr              bool
	}{
		{
			name: "Properly filters out double voting attester keys",
			previousAttsByPubKey: map[[48]byte][]*SignedAttestation{
				{1}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
				},
				{2}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
					{
						SourceEpoch: "2",
						TargetEpoch: "5",
					},
				},
				{3}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
				},
			},
			want: map[[48]byte]bool{
				{1}: true,
				{3}: true,
			},
		},
		{
			name: "Returns empty if no keys are slashable",
			previousAttsByPubKey: map[[48]byte][]*SignedAttestation{
				{1}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
				},
				{2}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
					{
						SourceEpoch: "2",
						TargetEpoch: "5",
					},
				},
				{3}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
					{
						SourceEpoch: "3",
						TargetEpoch: "6",
					},
				},
			},
			want: map[[48]byte]bool{},
		},
		{
			name: "Properly filters out surround voting attester keys",
			previousAttsByPubKey: map[[48]byte][]*SignedAttestation{
				{1}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
					{
						SourceEpoch: "1",
						TargetEpoch: "5",
					},
				},
				{2}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "4",
					},
					{
						SourceEpoch: "2",
						TargetEpoch: "5",
					},
				},
				{3}: {
					{
						SourceEpoch: "2",
						TargetEpoch: "5",
					},
					{
						SourceEpoch: "3",
						TargetEpoch: "4",
					},
				},
			},
			want: map[[48]byte]bool{
				{1}: true,
				{3}: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			attestingHistoryByPubKey := make(map[[48]byte]kv.EncHistoryData)
			for pubKey, signedAtts := range tt.incomingAttsByPubKey {
				attestingHistory, err := transformSignedAttestations(ctx, signedAtts)
				require.NoError(t, err)
				attestingHistoryByPubKey[pubKey] = *attestingHistory
			}
			got, err := filterSlashablePubKeysFromAttestations(ctx, attestingHistoryByPubKey, tt.incomingAttsByPubKey)
			if (err != nil) != tt.wantErr {
				t.Errorf("filterSlashablePubKeysFromAttestations() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for _, pubKey := range got {
				ok := tt.want[pubKey]
				assert.Equal(t, true, ok)
			}
		})
	}
}

func createRandomPubKeys(t *testing.T, numValidators int) [][48]byte {
	pubKeys := make([][48]byte, numValidators)
	for i := 0; i < numValidators; i++ {
		randKey, err := bls.RandKey()
		require.NoError(t, err)
		copy(pubKeys[i][:], randKey.PublicKey().Marshal())
	}
	return pubKeys
}

func createRandomRoots(t *testing.T, numRoots int) [][32]byte {
	roots := make([][32]byte, numRoots)
	for i := 0; i < numRoots; i++ {
		roots[i] = hashutil.Hash([]byte(fmt.Sprintf("%d", i)))
	}
	return roots
}
