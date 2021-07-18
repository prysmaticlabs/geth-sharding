package params

// AltairE2EForkEpoch is the pre-determined altair fork epoch in our
// E2E test.
const AltairE2EForkEpoch = 6

// UseE2EConfig for beacon chain services.
func UseE2EConfig() {
	beaconConfig = E2ETestConfig()

	cfg := BeaconNetworkConfig().Copy()
	// Due to the small number of peers in the e2e test network
	// setting this to 0, prevents the node from being overwhelmed
	// from discovery lookups.
	cfg.MinimumPeersInSubnet = 0
	OverrideBeaconNetworkConfig(cfg)
}

// E2ETestConfig retrieves the configurations made specifically for E2E testing.
// Warning: This config is only for testing, it is not meant for use outside of E2E.
func E2ETestConfig() *BeaconChainConfig {
	e2eConfig := MinimalSpecConfig()

	// Misc.
	e2eConfig.MinGenesisActiveValidatorCount = 256
	e2eConfig.GenesisDelay = 10 // 10 seconds so E2E has enough time to process deposits and get started.

	// Time parameters.
	e2eConfig.SecondsPerSlot = 10
	e2eConfig.SlotsPerEpoch = 6
	e2eConfig.SecondsPerETH1Block = 2
	e2eConfig.Eth1FollowDistance = 4
	e2eConfig.EpochsPerEth1VotingPeriod = 2
	e2eConfig.ShardCommitteePeriod = 4
	e2eConfig.MaxSeedLookahead = 1

	// PoW parameters.
	e2eConfig.DepositChainID = 1337   // Chain ID of eth1 dev net.
	e2eConfig.DepositNetworkID = 1337 // Network ID of eth1 dev net.

	// Altair Fork Parameters.
	e2eConfig.AltairForkEpoch = AltairE2EForkEpoch

	// Prysm constants.
	e2eConfig.ConfigName = ConfigNames[EndToEnd]

	return e2eConfig
}
