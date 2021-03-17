package params

import (
	eth1Params "github.com/ethereum/go-ethereum/params"
)

// UsePraterNetworkConfig uses the Prater specific
// network config.
func UsePraterNetworkConfig() {
	cfg := BeaconNetworkConfig().Copy()
	cfg.ContractDeploymentBlock = 4367322
	cfg.BootstrapNodes = []string{
		// Prysm's bootnode
		"enr:-Ku4QFmUkNp0g9bsLX2PfVeIyT-9WO-PZlrqZBNtEyofOOfLMScDjaTzGxIb1Ns9Wo5Pm_8nlq-SZwcQfTH2cgO-s88Bh2F0dG5ldHOIAAAAAAAAAACEZXRoMpDkvpOTAAAQIP__________gmlkgnY0gmlwhBLf22SJc2VjcDI1NmsxoQLV_jMOIxKbjHFKgrkFvwDvpexo6Nd58TK5k7ss4Vt0IoN1ZHCCG1g"
		// Lighthouse's bootnode
		"enr:-LK4QH1xnjotgXwg25IDPjrqRGFnH1ScgNHA3dv1Z8xHCp4uP3N3Jjl_aYv_WIxQRdwZvSukzbwspXZ7JjpldyeVDzMCh2F0dG5ldHOIAAAAAAAAAACEZXRoMpB53wQoAAAQIP__________gmlkgnY0gmlwhIe1te-Jc2VjcDI1NmsxoQOkcGXqbCJYbcClZ3z5f6NWhX_1YPFRYRRWQpJjwSHpVIN0Y3CCIyiDdWRwgiMo",
		// Teku's bootnode
		"enr:-KG4QCIzJZTY_fs_2vqWEatJL9RrtnPwDCv-jRBuO5FQ2qBrfJubWOWazri6s9HsyZdu-fRUfEzkebhf1nvO42_FVzwDhGV0aDKQed8EKAAAECD__________4JpZIJ2NIJpcISHtbYziXNlY3AyNTZrMaED4m9AqVs6F32rSCGsjtYcsyfQE2K8nDiGmocUY_iq-TSDdGNwgiMog3VkcIIjKA,"
	}
	OverrideBeaconNetworkConfig(cfg)
}

// UsePraterConfig sets the main beacon chain
// config for Prater.
func UsePraterConfig() {
	beaconConfig = PraterConfig()
}

// PraterConfig defines the config for the
// Prater testnet.
func PraterConfig() *BeaconChainConfig {
	cfg := MainnetConfig().Copy()
	cfg.MinGenesisTime = 1614588812
	cfg.GenesisDelay = 1919188
	cfg.ConfigName = ConfigNames[Prater]
	cfg.GenesisForkVersion = []byte{0x00, 0x00, 0x10, 0x20}
	cfg.SecondsPerETH1Block = 14
	cfg.DepositChainID = eth1Params.GoerliChainConfig.ChainID.Uint64()
	cfg.DepositNetworkID = eth1Params.GoerliChainConfig.ChainID.Uint64()
	cfg.DepositContractAddress = "0xff50ed3d0ec03ac01d4c79aad74928bff48a7b2b"
	return cfg
}
