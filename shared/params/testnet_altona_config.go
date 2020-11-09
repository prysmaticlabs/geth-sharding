package params

// UseAltonaNetworkConfig uses the Altona specific
// network config.
func UseAltonaNetworkConfig() {
	cfg := BeaconNetworkConfig().Copy()
	cfg.ContractDeploymentBlock = 2917810
	cfg.DepositContractAddress = "0x16e82D77882A663454Ef92806b7DeCa1D394810f"
	cfg.ChainID = 5   // Chain ID of eth1 goerli testnet.
	cfg.NetworkID = 5 // Network ID of eth1 goerli testnet.
	cfg.BootstrapNodes = []string{
		"enr:-LK4QFtV7Pz4reD5a7cpfi1z6yPrZ2I9eMMU5mGQpFXLnLoKZW8TXvVubShzLLpsEj6aayvVO1vFx-MApijD3HLPhlECh2F0dG5ldHOIAAAAAAAAAACEZXRoMpD6etXjAAABIf__________gmlkgnY0gmlwhDMPYfCJc2VjcDI1NmsxoQIerw_qBc9apYfZqo2awiwS930_vvmGnW2psuHsTzrJ8YN0Y3CCIyiDdWRwgiMo",
		"enr:-LK4QPVkFd_MKzdW0219doTZryq40tTe8rwWYO75KDmeZM78fBskGsfCuAww9t8y3u0Q0FlhXOhjE1CWpx3SGbUaU80Ch2F0dG5ldHOIAAAAAAAAAACEZXRoMpD6etXjAAABIf__________gmlkgnY0gmlwhDMPRgeJc2VjcDI1NmsxoQNHu-QfNgzl8VxbMiPgv6wgAljojnqAOrN18tzJMuN8oYN0Y3CCIyiDdWRwgiMo",
		"enr:-LK4QHe52XPPrcv6-MvcmN5GqDe_sgCwo24n_2hedlfwD_oxNt7cXL3tXJ7h9aYv6CTS1C_H2G2_dkeqm_LBO9nrpiYBh2F0dG5ldHOIAAAAAAAAAACEZXRoMpD9yjmwAAABIf__________gmlkgnY0gmlwhANzD9uJc2VjcDI1NmsxoQJX7zMnRU3szfGfS8MAIfPaQKOBpu3sBVTXf4Qq0b_m-4N0Y3CCIyiDdWRwgiMo",
		"enr:-LK4QLkbbq7xuRa_EnWd_kc0TkQk0pd0B0cZYR5LvBsncFQBDyPbGdy8d24TzRVeK7ZWwM5_2EcSJK223f8TYUOQYfwBh2F0dG5ldHOIAAAAAAAAAACEZXRoMpD9yjmwAAABIf__________gmlkgnY0gmlwhAPsjtOJc2VjcDI1NmsxoQJNw_aZgWXl2SstD--WAjooGudjWLjEbbCIddJuEPxzWYN0Y3CCIyiDdWRwgiMo",
		"enr:-LK4QHy-glnxN1WTk5f6d7-xXwy_UKJLs5k7p_S4KRY9I925KTzW_kQLjfFriIpH0de7kygBwrSl726ukq9_OG_sgKMCh2F0dG5ldHOIUjEAIQEAFMiEZXRoMpD9yjmwAAABIf__________gmlkgnY0gmlwhBLmhrCJc2VjcDI1NmsxoQNlU7gT0HUvpLA41n-P5GrCgjwMwtG02YsRRO0lAmpmBYN0Y3CCIyiDdWRwgiMo",
		"enr:-LK4QDz0n0vpyOpuStB8e22h9ayHVcvmN7o0trC7eC0DnZV9GYGzK5uKv7WlzpMQM2nDTG43DWvF_DZYwJOZCbF4iCQBh2F0dG5ldHOI__________-EZXRoMpD9yjmwAAABIf__________gmlkgnY0gmlwhBKN136Jc2VjcDI1NmsxoQP5gcOUcaruHuMuTv8ht7ZEawp3iih7CmeLqcoY1hxOnoN0Y3CCIyiDdWRwgiMo",
		"enr:-LK4QOScOZ35sOXEH6CEW15lfv7I3DhqQAzCPQ_nRav95otuSh4yi9ol0AruKDiIk9qqGXyD-wQDaBAPLhwl4t-rUSQBh2F0dG5ldHOI__________-EZXRoMpD9yjmwAAABIf__________gmlkgnY0gmlwhCL68KuJc2VjcDI1NmsxoQK5fYR3Ipoc01dz0d2-EcL7m26zKQSkAbf4rwcMMM09CoN0Y3CCIyiDdWRwgiMo",
		"enr:-Ku4QMqmWPFkgM58F16wxB50cqWDaWaIsyANHL8wUNSB4Cy1TP9__uJQNRODvx_dvO6rY-BT3psrYTMAaxnMGXb6DuoBh2F0dG5ldHOIAAAAAAAAAACEZXRoMpD1pf1CAAAAAP__________gmlkgnY0gmlwhBLf22SJc2VjcDI1NmsxoQNoed9JnQh7ltcAacHEGOjwocL1BhMQbYTgaPX0kFuXtIN1ZHCCE4g",
	}

	OverrideBeaconNetworkConfig(cfg)
}

// UseAltonaConfig sets the main beacon chain
// config for altona.
func UseAltonaConfig() {
	beaconConfig = AltonaConfig()
}

// AltonaConfig defines the config for the
// altona testnet.
func AltonaConfig() *BeaconChainConfig {
	altCfg := MainnetConfig().Copy()
	altCfg.MinGenesisActiveValidatorCount = 640
	altCfg.MinGenesisTime = 1593433800
	altCfg.GenesisForkVersion = []byte{0x00, 0x00, 0x01, 0x21}
	altCfg.NetworkName = "Altona"
	altCfg.SecondsPerETH1Block = 14
	return altCfg
}
