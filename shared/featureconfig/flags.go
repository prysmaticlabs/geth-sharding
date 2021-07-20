package featureconfig

import (
	"time"

	"github.com/urfave/cli/v2"
)

var (
	// ToledoTestnet flag for the multiclient Ethereum consensus testnet.
	ToledoTestnet = &cli.BoolFlag{
		Name:  "toledo",
		Usage: "This defines the flag through which we can run on the Toledo Multiclient Testnet",
	}
	// PyrmontTestnet flag for the multiclient Ethereum consensus testnet.
	PyrmontTestnet = &cli.BoolFlag{
		Name:  "pyrmont",
		Usage: "This defines the flag through which we can run on the Pyrmont Multiclient Testnet",
	}
	// PraterTestnet flag for the multiclient Ethereum consensus testnet.
	PraterTestnet = &cli.BoolFlag{
		Name:  "prater",
		Usage: "Run Prysm configured for the Prater test network",
	}
	// Mainnet flag for easier tooling, no-op
	Mainnet = &cli.BoolFlag{
		Value: true,
		Name:  "mainnet",
		Usage: "Run on Ethereum Beacon Chain Main Net. This is the default and can be omitted.",
	}
	devModeFlag = &cli.BoolFlag{
		Name:  "dev",
		Usage: "Enable experimental features still in development. These features may not be stable.",
	}
	writeSSZStateTransitionsFlag = &cli.BoolFlag{
		Name:  "interop-write-ssz-state-transitions",
		Usage: "Write ssz states to disk after attempted state transition",
	}
	kafkaBootstrapServersFlag = &cli.StringFlag{
		Name:  "kafka-url",
		Usage: "Stream attestations and blocks to specified kafka servers. This field is used for bootstrap.servers kafka config field.",
	}
	enableExternalSlasherProtectionFlag = &cli.BoolFlag{
		Name: "enable-external-slasher-protection",
		Usage: "Enables the validator to connect to external slasher to prevent it from " +
			"transmitting a slashable offence over the network.",
	}
	disableLookbackFlag = &cli.BoolFlag{
		Name:  "disable-lookback",
		Usage: "Disables use of the lookback feature and updates attestation history for validators from head to epoch 0",
	}
	disableGRPCConnectionLogging = &cli.BoolFlag{
		Name:  "disable-grpc-connection-logging",
		Usage: "Disables displaying logs for newly connected grpc clients",
	}
	attestationAggregationStrategy = &cli.StringFlag{
		Name:  "attestation-aggregation-strategy",
		Usage: "Which strategy to use when aggregating attestations, one of: naive, max_cover, opt_max_cover.",
		Value: "max_cover",
	}
	forceOptMaxCoverAggregationStategy = &cli.BoolFlag{
		Name:  "attestation-aggregation-force-opt-maxcover",
		Usage: "When enabled, forces --attestation-aggregation-strategy=opt_max_cover setting.",
	}
	disableAccountsV2 = &cli.BoolFlag{
		Name:  "disable-accounts-v2",
		Usage: "Disables usage of v2 for Prysm validator accounts",
	}
	enablePeerScorer = &cli.BoolFlag{
		Name:  "enable-peer-scorer",
		Usage: "Enable experimental P2P peer scorer",
	}
	checkPtInfoCache = &cli.BoolFlag{
		Name:  "use-check-point-cache",
		Usage: "Enables check point info caching",
	}
	enableLargerGossipHistory = &cli.BoolFlag{
		Name:  "enable-larger-gossip-history",
		Usage: "Enables the node to store a larger amount of gossip messages in its cache.",
	}
	writeWalletPasswordOnWebOnboarding = &cli.BoolFlag{
		Name: "write-wallet-password-on-web-onboarding",
		Usage: "(Danger): Writes the wallet password to the wallet directory on completing Prysm web onboarding. " +
			"We recommend against this flag unless you are an advanced user.",
	}
	disableAttestingHistoryDBCache = &cli.BoolFlag{
		Name: "disable-attesting-history-db-cache",
		Usage: "(Danger): Disables the cache for attesting history in the validator DB, greatly increasing " +
			"disk reads and writes as well as increasing time required for attestations to be produced",
	}
	dynamicKeyReloadDebounceInterval = &cli.DurationFlag{
		Name: "dynamic-key-reload-debounce-interval",
		Usage: "(Advanced): Specifies the time duration the validator waits to reload new keys if they have " +
			"changed on disk. Default 1s, can be any type of duration such as 1.5s, 1000ms, 1m.",
		Value: time.Second,
	}
	disableBroadcastSlashingFlag = &cli.BoolFlag{
		Name:  "disable-broadcast-slashings",
		Usage: "Disables broadcasting slashings submitted to the beacon node.",
	}
	attestTimely = &cli.BoolFlag{
		Name:  "attest-timely",
		Usage: "Fixes validator can attest timely after current block processes. See #8185 for more details",
	}
	enableNextSlotStateCache = &cli.BoolFlag{
		Name:  "enable-next-slot-state-cache",
		Usage: "Improves attesting and proposing efficiency by caching the next slot state at the end of the current slot",
	}
	disableUpdateHeadTimely = &cli.BoolFlag{
		Name:  "disable-update-head-timely",
		Usage: "Disables updating head right after state transition",
	}
	disableProposerAttsSelectionUsingMaxCover = &cli.BoolFlag{
		Name:  "disable-proposer-atts-selection-using-max-cover",
		Usage: "Disable max-cover algorithm when selecting attestations for proposer",
	}
	enableSlashingProtectionPruning = &cli.BoolFlag{
		Name:  "enable-slashing-protection-pruning",
		Usage: "Enables the pruning of the validator client's slashing protectin database",
	}
	disableOptimizedBalanceUpdate = &cli.BoolFlag{
		Name:  "disable-optimized-balance-update",
		Usage: "Disable the optimized method of updating validator balances.",
	}
	enableDoppelGangerProtection = &cli.BoolFlag{
		Name: "enable-doppelganger",
		Usage: "Enables the validator to perform a doppelganger check. (Warning): This is not " +
			"a foolproof method to find duplicate instances in the network. Your validator will still be" +
			" vulnerable if it is being run in unsafe configurations.",
	}
	enableHistoricalSpaceRepresentation = &cli.BoolFlag{
		Name: "enable-historical-state-representation",
		Usage: "Enables the beacon chain to save historical states in a space efficient manner." +
			" (Warning): Once enabled, this feature migrates your database in to a new schema and " +
			"there is no going back. At worst, your entire database might get corrupted.",
	}
)

// devModeFlags holds list of flags that are set when development mode is on.
var devModeFlags = []cli.Flag{
	enableLargerGossipHistory,
	enableNextSlotStateCache,
	forceOptMaxCoverAggregationStategy,
	enableHistoricalSpaceRepresentation,
}

// ValidatorFlags contains a list of all the feature flags that apply to the validator client.
var ValidatorFlags = append(deprecatedFlags, []cli.Flag{
	writeWalletPasswordOnWebOnboarding,
	enableExternalSlasherProtectionFlag,
	disableAttestingHistoryDBCache,
	ToledoTestnet,
	PyrmontTestnet,
	PraterTestnet,
	Mainnet,
	disableAccountsV2,
	dynamicKeyReloadDebounceInterval,
	attestTimely,
	enableSlashingProtectionPruning,
	enableDoppelGangerProtection,
}...)

// SlasherFlags contains a list of all the feature flags that apply to the slasher client.
var SlasherFlags = append(deprecatedFlags, []cli.Flag{
	disableLookbackFlag,
	ToledoTestnet,
	PyrmontTestnet,
	PraterTestnet,
	Mainnet,
}...)

// E2EValidatorFlags contains a list of the validator feature flags to be tested in E2E.
var E2EValidatorFlags = []string{
	"--enable-doppelganger",
}

// BeaconChainFlags contains a list of all the feature flags that apply to the beacon-chain client.
var BeaconChainFlags = append(deprecatedFlags, []cli.Flag{
	devModeFlag,
	writeSSZStateTransitionsFlag,
	kafkaBootstrapServersFlag,
	disableGRPCConnectionLogging,
	attestationAggregationStrategy,
	ToledoTestnet,
	PyrmontTestnet,
	PraterTestnet,
	Mainnet,
	enablePeerScorer,
	enableLargerGossipHistory,
	checkPtInfoCache,
	disableBroadcastSlashingFlag,
	enableNextSlotStateCache,
	forceOptMaxCoverAggregationStategy,
	disableUpdateHeadTimely,
	disableProposerAttsSelectionUsingMaxCover,
	disableOptimizedBalanceUpdate,
	enableHistoricalSpaceRepresentation,
}...)

// E2EBeaconChainFlags contains a list of the beacon chain feature flags to be tested in E2E.
var E2EBeaconChainFlags = []string{
	"--attestation-aggregation-strategy=opt_max_cover",
	"--dev",
	"--use-check-point-cache",
}
