package params

import (
	"io/ioutil"
	"path"
	"strings"
	"testing"

	"github.com/bazelbuild/rules_go/go/tools/bazel"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
)

func TestLoadConfigFileMainnet(t *testing.T) {
	// See https://media.githubusercontent.com/media/ethereum/eth2.0-spec-tests/master/tests/minimal/config/phase0.yaml
	assertVals := func(name string, c1, c2 *BeaconChainConfig) {
		//  Misc params.
		assert.Equal(t, c1.MaxCommitteesPerSlot, c2.MaxCommitteesPerSlot, "%s: MaxCommitteesPerSlot", name)
		assert.Equal(t, c1.TargetCommitteeSize, c2.TargetCommitteeSize, "%s: TargetCommitteeSize", name)
		assert.Equal(t, c1.MaxValidatorsPerCommittee, c2.MaxValidatorsPerCommittee, "%s: MaxValidatorsPerCommittee", name)
		assert.Equal(t, c1.MinPerEpochChurnLimit, c2.MinPerEpochChurnLimit, "%s: MinPerEpochChurnLimit", name)
		assert.Equal(t, c1.ChurnLimitQuotient, c2.ChurnLimitQuotient, "%s: ChurnLimitQuotient", name)
		assert.Equal(t, c1.ShuffleRoundCount, c2.ShuffleRoundCount, "%s: ShuffleRoundCount", name)
		assert.Equal(t, c1.MinGenesisActiveValidatorCount, c2.MinGenesisActiveValidatorCount, "%s: MinGenesisActiveValidatorCount", name)
		assert.Equal(t, c1.MinGenesisTime, c2.MinGenesisTime, "%s: MinGenesisTime", name)
		assert.Equal(t, c1.HysteresisQuotient, c2.HysteresisQuotient, "%s: HysteresisQuotient", name)
		assert.Equal(t, c1.HysteresisDownwardMultiplier, c2.HysteresisDownwardMultiplier, "%s: HysteresisDownwardMultiplier", name)
		assert.Equal(t, c1.HysteresisUpwardMultiplier, c2.HysteresisUpwardMultiplier, "%s: HysteresisUpwardMultiplier", name)

		// Fork Choice params.
		assert.Equal(t, c1.SafeSlotsToUpdateJustified, c2.SafeSlotsToUpdateJustified, "%s: SafeSlotsToUpdateJustified", name)

		// Validator params.
		assert.Equal(t, c1.Eth1FollowDistance, c2.Eth1FollowDistance, "%s: Eth1FollowDistance", name)
		assert.Equal(t, c1.TargetAggregatorsPerCommittee, c2.TargetAggregatorsPerCommittee, "%s: TargetAggregatorsPerCommittee", name)
		assert.Equal(t, c1.RandomSubnetsPerValidator, c2.RandomSubnetsPerValidator, "%s: RandomSubnetsPerValidator", name)
		assert.Equal(t, c1.EpochsPerRandomSubnetSubscription, c2.EpochsPerRandomSubnetSubscription, "%s: EpochsPerRandomSubnetSubscription", name)
		assert.Equal(t, c1.SecondsPerETH1Block, c2.SecondsPerETH1Block, "%s: SecondsPerETH1Block", name)

		// Deposit contract.
		assert.Equal(t, c1.DepositChainID, c2.DepositChainID, "%s: DepositChainID", name)
		assert.Equal(t, c1.DepositNetworkID, c2.DepositNetworkID, "%s: DepositNetworkID", name)
		assert.Equal(t, c1.DepositContractAddress, c2.DepositContractAddress, "%s: DepositContractAddress", name)

		// Gwei values.
		assert.Equal(t, c1.MinDepositAmount, c2.MinDepositAmount, "%s: MinDepositAmount", name)
		assert.Equal(t, c1.MaxEffectiveBalance, c2.MaxEffectiveBalance, "%s: MaxEffectiveBalance", name)
		assert.Equal(t, c1.EjectionBalance, c2.EjectionBalance, "%s: EjectionBalance", name)
		assert.Equal(t, c1.EffectiveBalanceIncrement, c2.EffectiveBalanceIncrement, "%s: EffectiveBalanceIncrement", name)

		// Initial values.
		assert.DeepEqual(t, c1.GenesisForkVersion, c2.GenesisForkVersion, "%s: GenesisForkVersion", name)
		assert.DeepEqual(t, c1.BLSWithdrawalPrefixByte, c2.BLSWithdrawalPrefixByte, "%s: BLSWithdrawalPrefixByte", name)

		// Time parameters.
		assert.Equal(t, c1.GenesisDelay, c2.GenesisDelay, "%s: GenesisDelay", name)
		assert.Equal(t, c1.SecondsPerSlot, c2.SecondsPerSlot, "%s: SecondsPerSlot", name)
		assert.Equal(t, c1.MinAttestationInclusionDelay, c2.MinAttestationInclusionDelay, "%s: MinAttestationInclusionDelay", name)
		assert.Equal(t, c1.SlotsPerEpoch, c2.SlotsPerEpoch, "%s: SlotsPerEpoch", name)
		assert.Equal(t, c1.MinSeedLookahead, c2.MinSeedLookahead, "%s: MinSeedLookahead", name)
		assert.Equal(t, c1.MaxSeedLookahead, c2.MaxSeedLookahead, "%s: MaxSeedLookahead", name)
		assert.Equal(t, c1.EpochsPerEth1VotingPeriod, c2.EpochsPerEth1VotingPeriod, "%s: EpochsPerEth1VotingPeriod", name)
		assert.Equal(t, c1.SlotsPerHistoricalRoot, c2.SlotsPerHistoricalRoot, "%s: SlotsPerHistoricalRoot", name)
		assert.Equal(t, c1.MinValidatorWithdrawabilityDelay, c2.MinValidatorWithdrawabilityDelay, "%s: MinValidatorWithdrawabilityDelay", name)
		assert.Equal(t, c1.ShardCommitteePeriod, c2.ShardCommitteePeriod, "%s: ShardCommitteePeriod", name)
		assert.Equal(t, c1.MinEpochsToInactivityPenalty, c2.MinEpochsToInactivityPenalty, "%s: MinEpochsToInactivityPenalty", name)

		// State vector lengths.
		assert.Equal(t, c1.EpochsPerHistoricalVector, c2.EpochsPerHistoricalVector, "%s: EpochsPerHistoricalVector", name)
		assert.Equal(t, c1.EpochsPerSlashingsVector, c2.EpochsPerSlashingsVector, "%s: EpochsPerSlashingsVector", name)
		assert.Equal(t, c1.HistoricalRootsLimit, c2.HistoricalRootsLimit, "%s: HistoricalRootsLimit", name)
		assert.Equal(t, c1.ValidatorRegistryLimit, c2.ValidatorRegistryLimit, "%s: ValidatorRegistryLimit", name)

		// Reward and penalty quotients.
		assert.Equal(t, c1.BaseRewardFactor, c2.BaseRewardFactor, "%s: BaseRewardFactor", name)
		assert.Equal(t, c1.WhistleBlowerRewardQuotient, c2.WhistleBlowerRewardQuotient, "%s: WhistleBlowerRewardQuotient", name)
		assert.Equal(t, c1.ProposerRewardQuotient, c2.ProposerRewardQuotient, "%s: ProposerRewardQuotient", name)
		assert.Equal(t, c1.InactivityPenaltyQuotient, c2.InactivityPenaltyQuotient, "%s: InactivityPenaltyQuotient", name)
		assert.Equal(t, c1.InactivityPenaltyQuotientAltair, c2.InactivityPenaltyQuotientAltair, "%s: InactivityPenaltyQuotientAltair", name)
		assert.Equal(t, c1.MinSlashingPenaltyQuotient, c2.MinSlashingPenaltyQuotient, "%s: MinSlashingPenaltyQuotient", name)
		assert.Equal(t, c1.MinSlashingPenaltyQuotientAltair, c2.MinSlashingPenaltyQuotientAltair, "%s: MinSlashingPenaltyQuotientAltair", name)
		assert.Equal(t, c1.ProportionalSlashingMultiplier, c2.ProportionalSlashingMultiplier, "%s: ProportionalSlashingMultiplier", name)
		assert.Equal(t, c1.ProportionalSlashingMultiplierAltair, c2.ProportionalSlashingMultiplierAltair, "%s: ProportionalSlashingMultiplierAltair", name)

		// Max operations per block.
		assert.Equal(t, c1.MaxProposerSlashings, c2.MaxProposerSlashings, "%s: MaxProposerSlashings", name)
		assert.Equal(t, c1.MaxAttesterSlashings, c2.MaxAttesterSlashings, "%s: MaxAttesterSlashings", name)
		assert.Equal(t, c1.MaxAttestations, c2.MaxAttestations, "%s: MaxAttestations", name)
		assert.Equal(t, c1.MaxDeposits, c2.MaxDeposits, "%s: MaxDeposits", name)
		assert.Equal(t, c1.MaxVoluntaryExits, c2.MaxVoluntaryExits, "%s: MaxVoluntaryExits", name)

		// Signature domains.
		assert.Equal(t, c1.DomainBeaconProposer, c2.DomainBeaconProposer, "%s: DomainBeaconProposer", name)
		assert.Equal(t, c1.DomainBeaconAttester, c2.DomainBeaconAttester, "%s: DomainBeaconAttester", name)
		assert.Equal(t, c1.DomainRandao, c2.DomainRandao, "%s: DomainRandao", name)
		assert.Equal(t, c1.DomainDeposit, c2.DomainDeposit, "%s: DomainDeposit", name)
		assert.Equal(t, c1.DomainVoluntaryExit, c2.DomainVoluntaryExit, "%s: DomainVoluntaryExit", name)
		assert.Equal(t, c1.DomainSelectionProof, c2.DomainSelectionProof, "%s: DomainSelectionProof", name)
		assert.Equal(t, c1.DomainAggregateAndProof, c2.DomainAggregateAndProof, "%s: DomainAggregateAndProof", name)
	}

	t.Run("mainnet", func(t *testing.T) {
		mainnetConfigFile := configFilePath(t, "mainnet")
		LoadChainConfigFile(mainnetConfigFile)
		mainnetPresetFile := configFilePath(t, "mainnet")
		LoadChainConfigFile(mainnetPresetFile)
		assertVals("mainnet", MainnetConfig(), BeaconConfig())
	})

	t.Run("minimal", func(t *testing.T) {
		minimalConfigFile := configFilePath(t, "minimal")
		LoadChainConfigFile(minimalConfigFile)
		minimalPresetFile := presetsFilePath(t, "minimal")
		LoadChainConfigFile(minimalPresetFile)
		assertVals("minimal", MinimalSpecConfig(), BeaconConfig())
	})
}

func TestLoadConfigFile_OverwriteCorrectly(t *testing.T) {
	file, err := ioutil.TempFile("", "")
	require.NoError(t, err)
	// Set current config to minimal config
	OverrideBeaconConfig(MinimalSpecConfig())

	// load empty config file, so that it defaults to mainnet values
	LoadChainConfigFile(file.Name())
	if BeaconConfig().MinGenesisTime != MainnetConfig().MinGenesisTime {
		t.Errorf("Expected MinGenesisTime to be set to mainnet value: %d found: %d",
			MainnetConfig().MinGenesisTime,
			BeaconConfig().MinGenesisTime)
	}
	if BeaconConfig().SlotsPerEpoch != MainnetConfig().SlotsPerEpoch {
		t.Errorf("Expected SlotsPerEpoch to be set to mainnet value: %d found: %d",
			MainnetConfig().SlotsPerEpoch,
			BeaconConfig().SlotsPerEpoch)
	}
}

func Test_replaceHexStringWithYAMLFormat(t *testing.T) {

	testLines := []struct {
		line   string
		wanted string
	}{
		{
			line:   "ONE_BYTE: 0x41",
			wanted: "ONE_BYTE: 65\n",
		},
		{
			line:   "FOUR_BYTES: 0x41414141",
			wanted: "FOUR_BYTES: \n- 65\n- 65\n- 65\n- 65\n",
		},
		{
			line:   "THREE_BYTES: 0x414141",
			wanted: "THREE_BYTES: \n- 65\n- 65\n- 65\n- 0\n",
		},
		{
			line:   "EIGHT_BYTES: 0x4141414141414141",
			wanted: "EIGHT_BYTES: \n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n",
		},
		{
			line: "SIXTEEN_BYTES: 0x41414141414141414141414141414141",
			wanted: "SIXTEEN_BYTES: \n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n",
		},
		{
			line: "TWENTY_BYTES: 0x4141414141414141414141414141414141414141",
			wanted: "TWENTY_BYTES: \n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n",
		},
		{
			line: "THIRTY_TWO_BYTES: 0x4141414141414141414141414141414141414141414141414141414141414141",
			wanted: "THIRTY_TWO_BYTES: \n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n",
		},
		{
			line: "FORTY_EIGHT_BYTES: 0x41414141414141414141414141414141414141414141414141414141414141414141" +
				"4141414141414141414141414141",
			wanted: "FORTY_EIGHT_BYTES: \n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n",
		},
		{
			line: "NINETY_SIX_BYTES: 0x414141414141414141414141414141414141414141414141414141414141414141414141" +
				"4141414141414141414141414141414141414141414141414141414141414141414141414141414141414141414141" +
				"41414141414141414141414141",
			wanted: "NINETY_SIX_BYTES: \n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n" +
				"- 65\n- 65\n- 65\n- 65\n- 65\n- 65\n",
		},
	}
	for _, line := range testLines {
		parts := replaceHexStringWithYAMLFormat(line.line)
		res := strings.Join(parts, "\n")

		if res != line.wanted {
			t.Errorf("expected conversion to be: %v got: %v", line.wanted, res)
		}
	}
}

// configFilePath sets the proper config and returns the relevant
// config file path from eth2-spec-tests directory.
func configFilePath(t *testing.T, config string) string {
	filepath, err := bazel.Runfile("external/eth2_spec")
	require.NoError(t, err)
	configFilePath := path.Join(filepath, "configs", config+".yaml")
	return configFilePath
}

// presetsFilePath sets the proper preset and returns the relevant
// preset file path from eth2-spec-tests directory.
func presetsFilePath(t *testing.T, config string) string {
	filepath, err := bazel.Runfile("external/eth2_spec")
	require.NoError(t, err)
	configFilePath := path.Join(filepath, "presets", config, "phase0.yaml")
	return configFilePath
}
