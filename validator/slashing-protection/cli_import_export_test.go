package slashingprotection

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/fileutil"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	dbTest "github.com/prysmaticlabs/prysm/validator/db/testing"
	"github.com/prysmaticlabs/prysm/validator/flags"
	protectionFormat "github.com/prysmaticlabs/prysm/validator/slashing-protection/local/standard-protection-format"
	mocks "github.com/prysmaticlabs/prysm/validator/testing"
	"github.com/urfave/cli/v2"
)

func setupCliCtx(
	tb testing.TB,
	dbPath,
	protectionFilePath,
	outputDir string,
) *cli.Context {
	app := cli.App{}
	set := flag.NewFlagSet("test", 0)
	set.String(cmd.DataDirFlag.Name, dbPath, "")
	set.String(flags.SlashingProtectionJSONFileFlag.Name, protectionFilePath, "")
	set.String(flags.SlashingProtectionExportDirFlag.Name, outputDir, "")
	require.NoError(tb, set.Set(flags.SlashingProtectionJSONFileFlag.Name, protectionFilePath))
	assert.NoError(tb, set.Set(cmd.DataDirFlag.Name, dbPath))
	assert.NoError(tb, set.Set(flags.SlashingProtectionExportDirFlag.Name, outputDir))
	return cli.NewContext(&app, set, nil)
}

func TestImportExportSlashingProtectionCli_RoundTrip(t *testing.T) {
	numValidators := 10
	numEpochs := 20
	outputPath := filepath.Join(os.TempDir(), "slashing-exports")
	err := fileutil.MkdirAll(outputPath)
	require.NoError(t, err)
	protectionFileName := "slashing_history_import.json"

	// Create some mock slashing protection history. and JSON file
	pubKeys, err := mocks.CreateRandomPubKeys(numValidators)
	require.NoError(t, err)
	attestingHistory, proposalHistory, err := mocks.MockAttestingAndProposalHistories(pubKeys, numEpochs)
	require.NoError(t, err)
	mockJSON, err := mocks.MockSlashingProtectionJSON(pubKeys, attestingHistory, proposalHistory)
	require.NoError(t, err)

	// We JSON encode the protection file and save it to disk as a JSON file.
	encoded, err := json.Marshal(mockJSON)
	require.NoError(t, err)

	protectionFilePath := filepath.Join(outputPath, protectionFileName)
	err = fileutil.WriteFile(protectionFilePath, encoded)
	require.NoError(t, err)

	// We create a CLI context with the required values, such as the database datadir and output directory.
	validatorDB := dbTest.SetupDB(t, pubKeys)
	dbPath := validatorDB.DatabasePath()
	require.NoError(t, validatorDB.Close())
	cliCtx := setupCliCtx(t, dbPath, protectionFilePath, outputPath)

	// We import the slashing protection history file via CLI.
	err = ImportSlashingProtectionCLI(cliCtx)
	require.NoError(t, err)

	// We export the slashing protection history file via CLI.
	err = ExportSlashingProtectionJSONCli(cliCtx)
	require.NoError(t, err)

	// Attempt to read the exported file from the output directory.
	enc, err := fileutil.ReadFileAsBytes(filepath.Join(outputPath, jsonExportFileName))
	require.NoError(t, err)

	receivedJSON := &protectionFormat.EIPSlashingProtectionFormat{}
	err = json.Unmarshal(enc, receivedJSON)
	require.NoError(t, err)

	// We verify the parsed JSON file matches. Given there is no guarantee of order,
	// we will have to carefully compare and sort values as needed.
	//
	// First, we compare basic data such as the Metadata value in the JSON file.
	require.DeepEqual(t, mockJSON.Metadata, receivedJSON.Metadata)
	wantedHistoryByPublicKey := make(map[string]*protectionFormat.ProtectionData)
	for _, item := range mockJSON.Data {
		wantedHistoryByPublicKey[item.Pubkey] = item
	}

	// Next, we compare all the data for each validator public key.
	for _, item := range receivedJSON.Data {
		wanted, ok := wantedHistoryByPublicKey[item.Pubkey]
		require.Equal(t, true, ok)
		require.Equal(t, len(wanted.SignedBlocks), len(item.SignedBlocks))
		require.Equal(t, len(wanted.SignedAttestations), len(item.SignedAttestations))
		require.DeepEqual(t, wanted.SignedBlocks, item.SignedBlocks)
		require.DeepEqual(t, wanted.SignedAttestations, item.SignedAttestations)
	}
}
