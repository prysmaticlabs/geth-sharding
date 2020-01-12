/*
Package featureconfig defines which features are enabled for runtime
in order to selctively enable certain features to maintain a stable runtime.

The process for implementing new features using this package is as follows:
	1. Add a new CMD flag in flags.go, and place it in the proper list(s) var for its client.
	2. Add a condition for the flag in the proper Configure function(s) below.
	3. Place any "new" behavior in the `if flagEnabled` statement.
	4. Place any "previous" behavior in the `else` statement.
	5. Ensure any tests using the new feature fail if the flag isn't enabled.
	5a. Use the following to enable your flag for tests:
	cfg := &featureconfig.Flags{
		VerifyAttestationSigs: true,
	}
	featureconfig.Init(cfg)
*/
package featureconfig

import (
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var log = logrus.WithField("prefix", "flags")

// Flags is a struct to represent which features the client will perform on runtime.
type Flags struct {
	NoGenesisDelay            bool   // NoGenesisDelay signals to start the chain as quickly as possible.
	MinimalConfig             bool   // MinimalConfig as defined in the spec.
	WriteSSZStateTransitions  bool   // WriteSSZStateTransitions to tmp directory.
	InitSyncNoVerify          bool   // InitSyncNoVerify when initial syncing w/o verifying block's contents.
	SkipBLSVerify             bool   // Skips BLS verification across the runtime.
	EnableBackupWebhook       bool   // EnableBackupWebhook to allow database backups to trigger from monitoring port /db/backup.
	PruneEpochBoundaryStates  bool   // PruneEpochBoundaryStates prunes the epoch boundary state before last finalized check point.
	EnableSnappyDBCompression bool   // EnableSnappyDBCompression in the database.
	InitSyncCacheState        bool   // InitSyncCacheState caches state during initial sync.
	KafkaBootstrapServers     string // KafkaBootstrapServers to find kafka servers to stream blocks, attestations, etc.
	EnableSavingOfDepositData bool   // EnableSavingOfDepositData allows the saving of eth1 related data such as deposits,chain data to be saved.

	// Cache toggles.
	EnableAttestationCache   bool // EnableAttestationCache; see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableEth1DataVoteCache  bool // EnableEth1DataVoteCache; see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableSkipSlotsCache     bool // EnableSkipSlotsCache caches the state in skipped slots.
	EnableSlasherConnection  bool // EnableSlasher enable retrieval of slashing events from a slasher instance.
	EnableBlockTreeCache     bool // EnableBlockTreeCache enable fork choice service to maintain latest filtered block tree.
}

var featureConfig *Flags

// Get retrieves feature config.
func Get() *Flags {
	if featureConfig == nil {
		return &Flags{}
	}
	return featureConfig
}

// Init sets the global config equal to the config that is passed in.
func Init(c *Flags) {
	featureConfig = c
}

// ConfigureBeaconChain sets the global config based
// on what flags are enabled for the beacon-chain client.
func ConfigureBeaconChain(ctx *cli.Context) {
	complainOnDeprecatedFlags(ctx)
	cfg := &Flags{}
	if ctx.GlobalBool(noGenesisDelayFlag.Name) {
		log.Warn("Starting ETH2 with no genesis delay")
		cfg.NoGenesisDelay = true
	}
	if ctx.GlobalBool(MinimalConfigFlag.Name) {
		log.Warn("Using minimal config")
		cfg.MinimalConfig = true
	}
	if ctx.GlobalBool(writeSSZStateTransitionsFlag.Name) {
		log.Warn("Writing SSZ states and blocks after state transitions")
		cfg.WriteSSZStateTransitions = true
	}
	if ctx.GlobalBool(EnableAttestationCacheFlag.Name) {
		log.Warn("Enabled unsafe attestation cache")
		cfg.EnableAttestationCache = true
	}
	if ctx.GlobalBool(EnableEth1DataVoteCacheFlag.Name) {
		log.Warn("Enabled unsafe eth1 data vote cache")
		cfg.EnableEth1DataVoteCache = true
	}
	if ctx.GlobalBool(initSyncVerifyEverythingFlag.Name) {
		log.Warn("Initial syncing with verifying all block's content signatures.")
		cfg.InitSyncNoVerify = false
	} else {
		cfg.InitSyncNoVerify = true
	}
	if ctx.GlobalBool(SkipBLSVerifyFlag.Name) {
		log.Warn("UNSAFE: Skipping BLS verification at runtime")
		cfg.SkipBLSVerify = true
	}
	if ctx.GlobalBool(enableBackupWebhookFlag.Name) {
		log.Warn("Allowing database backups to be triggered from HTTP webhook.")
		cfg.EnableBackupWebhook = true
	}
	if ctx.GlobalBool(enableSkipSlotsCache.Name) {
		log.Warn("Enabled skip slots cache.")
		cfg.EnableSkipSlotsCache = true
	}
	if ctx.GlobalString(kafkaBootstrapServersFlag.Name) != "" {
		log.Warn("Enabling experimental kafka streaming.")
		cfg.KafkaBootstrapServers = ctx.GlobalString(kafkaBootstrapServersFlag.Name)
	}
	if ctx.GlobalBool(initSyncCacheState.Name) {
		log.Warn("Enabled initial sync cache state mode.")
		cfg.InitSyncCacheState = true
	}
	if ctx.GlobalBool(saveDepositData.Name) {
		log.Warn("Enabled saving of eth1 related chain/deposit data.")
		cfg.EnableSavingOfDepositData = true
	}
	if ctx.GlobalBool(enableSlasherFlag.Name) {
		log.Warn("Enable slasher connection.")
		cfg.EnableSlasherConnection = true
	}
	if ctx.GlobalBool(cacheFilteredBlockTree.Name) {
		log.Warn("Enabled filtered block tree cache for fork choice.")
		cfg.EnableBlockTreeCache = true
	}
	Init(cfg)
}

// ConfigureValidator sets the global config based
// on what flags are enabled for the validator client.
func ConfigureValidator(ctx *cli.Context) {
	complainOnDeprecatedFlags(ctx)
	cfg := &Flags{}
	if ctx.GlobalBool(MinimalConfigFlag.Name) {
		log.Warn("Using minimal config")
		cfg.MinimalConfig = true
	}
	Init(cfg)
}

func complainOnDeprecatedFlags(ctx *cli.Context) {
	for _, f := range deprecatedFlags {
		if ctx.IsSet(f.GetName()) {
			log.Errorf("%s is deprecated and has no effect. Do not use this flag, it will be deleted soon.", f.GetName())
		}
	}
}
