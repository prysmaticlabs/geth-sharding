package beacon

import (
	"context"
	"encoding/hex"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
	"github.com/prysmaticlabs/prysm/shared/testutil/require"
	"google.golang.org/protobuf/types/known/emptypb"
)

func TestGetSpec(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	config := params.BeaconConfig()

	config.ConfigName = "ConfigName"
	config.MaxCommitteesPerSlot = 1
	config.TargetCommitteeSize = 2
	config.MaxValidatorsPerCommittee = 3
	config.MinPerEpochChurnLimit = 4
	config.ChurnLimitQuotient = 5
	config.ShuffleRoundCount = 6
	config.MinGenesisActiveValidatorCount = 7
	config.MinGenesisTime = 8
	config.HysteresisQuotient = 9
	config.HysteresisDownwardMultiplier = 10
	config.HysteresisUpwardMultiplier = 11
	config.SafeSlotsToUpdateJustified = 12
	config.Eth1FollowDistance = 13
	config.TargetAggregatorsPerCommittee = 14
	config.RandomSubnetsPerValidator = 15
	config.EpochsPerRandomSubnetSubscription = 16
	config.SecondsPerETH1Block = 17
	config.DepositChainID = 18
	config.DepositNetworkID = 19
	config.DepositContractAddress = "DepositContractAddress"
	config.MinDepositAmount = 20
	config.MaxEffectiveBalance = 21
	config.EjectionBalance = 22
	config.EffectiveBalanceIncrement = 23
	config.GenesisForkVersion = []byte("GenesisForkVersion")
	config.AltairForkVersion = []byte("AltairForkVersion")
	config.AltairForkEpoch = 100
	config.BLSWithdrawalPrefixByte = byte('b')
	config.GenesisDelay = 24
	config.SecondsPerSlot = 25
	config.MinAttestationInclusionDelay = 26
	config.SlotsPerEpoch = 27
	config.MinSeedLookahead = 28
	config.MaxSeedLookahead = 29
	config.EpochsPerEth1VotingPeriod = 30
	config.SlotsPerHistoricalRoot = 31
	config.MinValidatorWithdrawabilityDelay = 32
	config.ShardCommitteePeriod = 33
	config.MinEpochsToInactivityPenalty = 34
	config.EpochsPerHistoricalVector = 35
	config.EpochsPerSlashingsVector = 36
	config.HistoricalRootsLimit = 37
	config.ValidatorRegistryLimit = 38
	config.BaseRewardFactor = 39
	config.WhistleBlowerRewardQuotient = 40
	config.ProposerRewardQuotient = 41
	config.InactivityPenaltyQuotient = 42
	config.MinSlashingPenaltyQuotient = 44
	config.ProportionalSlashingMultiplier = 46
	config.MaxProposerSlashings = 48
	config.MaxAttesterSlashings = 49
	config.MaxAttestations = 50
	config.MaxDeposits = 51
	config.MaxVoluntaryExits = 52
	config.TimelyHeadFlagIndex = 53
	config.TimelySourceFlagIndex = 54
	config.TimelyTargetFlagIndex = 55
	config.TimelyHeadWeight = 56
	config.TimelySourceWeight = 57
	config.TimelyTargetWeight = 58
	config.SyncRewardWeight = 59
	config.WeightDenominator = 60
	config.TargetAggregatorsPerSyncSubcommittee = 61
	config.SyncCommitteeSubnetCount = 62
	config.SyncCommitteeSize = 63
	config.InactivityScoreBias = 65
	config.EpochsPerSyncCommitteePeriod = 66
	config.InactivityPenaltyQuotientAltair = 67
	config.MinSlashingPenaltyQuotientAltair = 68
	config.ProportionalSlashingMultiplierAltair = 69
	config.InactivityScoreRecoveryRate = 70

	var dbp [4]byte
	copy(dbp[:], []byte{'0', '0', '0', '1'})
	config.DomainBeaconProposer = dbp
	var dba [4]byte
	copy(dba[:], []byte{'0', '0', '0', '2'})
	config.DomainBeaconAttester = dba
	var dr [4]byte
	copy(dr[:], []byte{'0', '0', '0', '3'})
	config.DomainRandao = dr
	var dd [4]byte
	copy(dd[:], []byte{'0', '0', '0', '4'})
	config.DomainDeposit = dd
	var dve [4]byte
	copy(dve[:], []byte{'0', '0', '0', '5'})
	config.DomainVoluntaryExit = dve
	var dsp [4]byte
	copy(dsp[:], []byte{'0', '0', '0', '6'})
	config.DomainSelectionProof = dsp
	var daap [4]byte
	copy(daap[:], []byte{'0', '0', '0', '7'})
	config.DomainAggregateAndProof = daap

	params.OverrideBeaconConfig(config)

	server := &Server{}
	resp, err := server.GetSpec(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)

	assert.Equal(t, 83, len(resp.Data))
	for k, v := range resp.Data {
		switch k {
		case "CONFIG_NAME":
			assert.Equal(t, "ConfigName", v)
		case "MAX_COMMITTEES_PER_SLOT":
			assert.Equal(t, "1", v)
		case "TARGET_COMMITTEE_SIZE":
			assert.Equal(t, "2", v)
		case "MAX_VALIDATORS_PER_COMMITTEE":
			assert.Equal(t, "3", v)
		case "MIN_PER_EPOCH_CHURN_LIMIT":
			assert.Equal(t, "4", v)
		case "CHURN_LIMIT_QUOTIENT":
			assert.Equal(t, "5", v)
		case "SHUFFLE_ROUND_COUNT":
			assert.Equal(t, "6", v)
		case "MIN_GENESIS_ACTIVE_VALIDATOR_COUNT":
			assert.Equal(t, "7", v)
		case "MIN_GENESIS_TIME":
			assert.Equal(t, "8", v)
		case "HYSTERESIS_QUOTIENT":
			assert.Equal(t, "9", v)
		case "HYSTERESIS_DOWNWARD_MULTIPLIER":
			assert.Equal(t, "10", v)
		case "HYSTERESIS_UPWARD_MULTIPLIER":
			assert.Equal(t, "11", v)
		case "SAFE_SLOTS_TO_UPDATE_JUSTIFIED":
			assert.Equal(t, "12", v)
		case "ETH1_FOLLOW_DISTANCE":
			assert.Equal(t, "13", v)
		case "TARGET_AGGREGATORS_PER_COMMITTEE":
			assert.Equal(t, "14", v)
		case "RANDOM_SUBNETS_PER_VALIDATOR":
			assert.Equal(t, "15", v)
		case "EPOCHS_PER_RANDOM_SUBNET_SUBSCRIPTION":
			assert.Equal(t, "16", v)
		case "SECONDS_PER_ETH1_BLOCK":
			assert.Equal(t, "17", v)
		case "DEPOSIT_CHAIN_ID":
			assert.Equal(t, "18", v)
		case "DEPOSIT_NETWORK_ID":
			assert.Equal(t, "19", v)
		case "DEPOSIT_CONTRACT_ADDRESS":
			assert.Equal(t, "DepositContractAddress", v)
		case "MIN_DEPOSIT_AMOUNT":
			assert.Equal(t, "20", v)
		case "MAX_EFFECTIVE_BALANCE":
			assert.Equal(t, "21", v)
		case "EJECTION_BALANCE":
			assert.Equal(t, "22", v)
		case "EFFECTIVE_BALANCE_INCREMENT":
			assert.Equal(t, "23", v)
		case "GENESIS_FORK_VERSION":
			assert.Equal(t, "0x"+hex.EncodeToString([]byte("GenesisForkVersion")), v)
		case "ALTAIR_FORK_VERSION":
			assert.Equal(t, "0x"+hex.EncodeToString([]byte("AltairForkVersion")), v)
		case "ALTAIR_FORK_EPOCH":
			assert.Equal(t, "100", v)
		case "BLS_WITHDRAWAL_PREFIX":
			assert.Equal(t, "0x62", v)
		case "GENESIS_DELAY":
			assert.Equal(t, "24", v)
		case "SECONDS_PER_SLOT":
			assert.Equal(t, "25", v)
		case "MIN_ATTESTATION_INCLUSION_DELAY":
			assert.Equal(t, "26", v)
		case "SLOTS_PER_EPOCH":
			assert.Equal(t, "27", v)
		case "MIN_SEED_LOOKAHEAD":
			assert.Equal(t, "28", v)
		case "MAX_SEED_LOOKAHEAD":
			assert.Equal(t, "29", v)
		case "EPOCHS_PER_ETH1_VOTING_PERIOD":
			assert.Equal(t, "30", v)
		case "SLOTS_PER_HISTORICAL_ROOT":
			assert.Equal(t, "31", v)
		case "MIN_VALIDATOR_WITHDRAWABILITY_DELAY":
			assert.Equal(t, "32", v)
		case "SHARD_COMMITTEE_PERIOD":
			assert.Equal(t, "33", v)
		case "MIN_EPOCHS_TO_INACTIVITY_PENALTY":
			assert.Equal(t, "34", v)
		case "EPOCHS_PER_HISTORICAL_VECTOR":
			assert.Equal(t, "35", v)
		case "EPOCHS_PER_SLASHINGS_VECTOR":
			assert.Equal(t, "36", v)
		case "HISTORICAL_ROOTS_LIMIT":
			assert.Equal(t, "37", v)
		case "VALIDATOR_REGISTRY_LIMIT":
			assert.Equal(t, "38", v)
		case "BASE_REWARD_FACTOR":
			assert.Equal(t, "39", v)
		case "WHISTLEBLOWER_REWARD_QUOTIENT":
			assert.Equal(t, "40", v)
		case "PROPOSER_REWARD_QUOTIENT":
			assert.Equal(t, "41", v)
		case "INACTIVITY_PENALTY_QUOTIENT":
			assert.Equal(t, "42", v)
		case "HF1_INACTIVITY_PENALTY_QUOTIENT":
			assert.Equal(t, "43", v)
		case "MIN_SLASHING_PENALTY_QUOTIENT":
			assert.Equal(t, "44", v)
		case "HF1_MIN_SLASHING_PENALTY_QUOTIENT":
			assert.Equal(t, "45", v)
		case "PROPORTIONAL_SLASHING_MULTIPLIER":
			assert.Equal(t, "46", v)
		case "HF1_PROPORTIONAL_SLASHING_MULTIPLIER":
			assert.Equal(t, "47", v)
		case "MAX_PROPOSER_SLASHINGS":
			assert.Equal(t, "48", v)
		case "MAX_ATTESTER_SLASHINGS":
			assert.Equal(t, "49", v)
		case "MAX_ATTESTATIONS":
			assert.Equal(t, "50", v)
		case "MAX_DEPOSITS":
			assert.Equal(t, "51", v)
		case "MAX_VOLUNTARY_EXITS":
			assert.Equal(t, "52", v)
		case "TIMELY_HEAD_FLAG_INDEX":
			assert.Equal(t, "0x35", v)
		case "TIMELY_SOURCE_FLAG_INDEX":
			assert.Equal(t, "0x36", v)
		case "TIMELY_TARGET_FLAG_INDEX":
			assert.Equal(t, "0x37", v)
		case "TIMELY_HEAD_WEIGHT":
			assert.Equal(t, "56", v)
		case "TIMELY_SOURCE_WEIGHT":
			assert.Equal(t, "57", v)
		case "TIMELY_TARGET_WEIGHT":
			assert.Equal(t, "58", v)
		case "SYNC_REWARD_WEIGHT":
			assert.Equal(t, "59", v)
		case "WEIGHT_DENOMINATOR":
			assert.Equal(t, "60", v)
		case "TARGET_AGGREGATORS_PER_SYNC_SUBCOMMITTEE":
			assert.Equal(t, "61", v)
		case "SYNC_COMMITTEE_SUBNET_COUNT":
			assert.Equal(t, "62", v)
		case "SYNC_COMMITTEE_SIZE":
			assert.Equal(t, "63", v)
		case "SYNC_PUBKEYS_PER_AGGREGATE":
			assert.Equal(t, "64", v)
		case "INACTIVITY_SCORE_BIAS":
			assert.Equal(t, "65", v)
		case "EPOCHS_PER_SYNC_COMMITTEE_PERIOD":
			assert.Equal(t, "66", v)
		case "INACTIVITY_PENALTY_QUOTIENT_ALTAIR":
			assert.Equal(t, "67", v)
		case "MIN_SLASHING_PENALTY_QUOTIENT_ALTAIR":
			assert.Equal(t, "68", v)
		case "PROPORTIONAL_SLASHING_MULTIPLIER_ALTAIR":
			assert.Equal(t, "69", v)
		case "INACTIVITY_SCORE_RECOVERY_RATE":
			assert.Equal(t, "70", v)
		case "PROPOSER_WEIGHT":
			assert.Equal(t, "8", v)
		case "DOMAIN_BEACON_PROPOSER":
			assert.Equal(t, "0x30303031", v)
		case "DOMAIN_BEACON_ATTESTER":
			assert.Equal(t, "0x30303032", v)
		case "DOMAIN_RANDAO":
			assert.Equal(t, "0x30303033", v)
		case "DOMAIN_DEPOSIT":
			assert.Equal(t, "0x30303034", v)
		case "DOMAIN_VOLUNTARY_EXIT":
			assert.Equal(t, "0x30303035", v)
		case "DOMAIN_SELECTION_PROOF":
			assert.Equal(t, "0x30303036", v)
		case "DOMAIN_AGGREGATE_AND_PROOF":
			assert.Equal(t, "0x30303037", v)
		case "DOMAIN_SYNC_COMMITTEE":
			assert.Equal(t, "0x07000000", v)
		case "DOMAIN_SYNC_COMMITTEE_SELECTION_PROOF":
			assert.Equal(t, "0x08000000", v)
		case "DOMAIN_CONTRIBUTION_AND_PROOF":
			assert.Equal(t, "0x09000000", v)
		default:
			t.Errorf("Incorrect key: %s", k)
		}
	}
}

func TestGetDepositContract(t *testing.T) {
	const chainId = 99
	const address = "0x0000000000000000000000000000000000000009"
	params.SetupTestConfigCleanup(t)
	config := params.BeaconConfig()
	config.DepositChainID = chainId
	config.DepositContractAddress = address
	params.OverrideBeaconConfig(config)

	s := Server{}
	resp, err := s.GetDepositContract(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	assert.Equal(t, uint64(chainId), resp.Data.ChainId)
	assert.Equal(t, address, resp.Data.Address)
}

func TestForkSchedule_Ok(t *testing.T) {
	genesisForkVersion := []byte("Genesis")
	firstForkVersion, firstForkEpoch := []byte("Firs"), types.Epoch(100)
	secondForkVersion, secondForkEpoch := []byte("Seco"), types.Epoch(200)
	thirdForkVersion, thirdForkEpoch := []byte("Thir"), types.Epoch(300)

	params.SetupTestConfigCleanup(t)
	config := params.BeaconConfig()
	config.GenesisForkVersion = genesisForkVersion
	// Create fork schedule adding keys in non-sorted order.
	schedule := make(map[[4]byte]types.Epoch, 3)
	schedule[bytesutil.ToBytes4(secondForkVersion)] = secondForkEpoch
	schedule[bytesutil.ToBytes4(firstForkVersion)] = firstForkEpoch
	schedule[bytesutil.ToBytes4(thirdForkVersion)] = thirdForkEpoch
	config.ForkVersionSchedule = schedule
	params.OverrideBeaconConfig(config)

	s := &Server{}
	resp, err := s.GetForkSchedule(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	require.Equal(t, 3, len(resp.Data))
	fork := resp.Data[0]
	assert.DeepEqual(t, genesisForkVersion, fork.PreviousVersion)
	assert.DeepEqual(t, string(firstForkVersion), string(fork.CurrentVersion))
	assert.Equal(t, firstForkEpoch, fork.Epoch)
	fork = resp.Data[1]
	assert.DeepEqual(t, firstForkVersion, fork.PreviousVersion)
	assert.DeepEqual(t, secondForkVersion, fork.CurrentVersion)
	assert.Equal(t, secondForkEpoch, fork.Epoch)
	fork = resp.Data[2]
	assert.DeepEqual(t, secondForkVersion, fork.PreviousVersion)
	assert.DeepEqual(t, thirdForkVersion, fork.CurrentVersion)
	assert.Equal(t, thirdForkEpoch, fork.Epoch)
}

func TestForkSchedule_CorrectNumberOfForks(t *testing.T) {
	s := &Server{}
	resp, err := s.GetForkSchedule(context.Background(), &emptypb.Empty{})
	require.NoError(t, err)
	// Genesis and Altair.
	assert.Equal(t, 2, len(resp.Data))
}
