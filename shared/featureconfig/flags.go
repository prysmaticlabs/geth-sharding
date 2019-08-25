package featureconfig

import (
	"github.com/urfave/cli"
)

var (
	// EnableCanonicalAttestationFilter filters and sends canonical attestation to RPC requests.
	EnableCanonicalAttestationFilter = cli.BoolFlag{
		Name:  "enable-canonical-attestation-filter",
		Usage: "Enable filtering and sending canonical attestations to RPC request, default is disabled.",
	}
	// DisableHistoricalStatePruningFlag allows the database to keep old historical states.
	DisableHistoricalStatePruningFlag = cli.BoolFlag{
		Name:  "disable-historical-state-pruning",
		Usage: "Disable database pruning of historical states after finalized epochs.",
	}
	// DisableGossipSubFlag uses floodsub in place of gossipsub.
	DisableGossipSubFlag = cli.BoolFlag{
		Name:  "disable-gossip-sub",
		Usage: "Disable gossip sub messaging and use floodsub messaging",
	}
	// EnableExcessDepositsFlag enables a validator to have total amount deposited as more than the
	// max deposit amount.
	EnableExcessDepositsFlag = cli.BoolFlag{
		Name:  "enables-excess-deposit",
		Usage: "Enables balances more than max deposit amount for a validator",
	}
	// NoGenesisDelayFlag disables the standard genesis delay.
	NoGenesisDelayFlag = cli.BoolFlag{
		Name:  "no-genesis-delay",
		Usage: "Process genesis event 30s after the ETH1 block time, rather than wait to midnight of the next day.",
	}
	// UseNewP2PFlag to start the beacon chain with the new p2p library.
	UseNewP2PFlag = cli.BoolFlag{
		Name:  "experimental-p2p",
		Usage: "Use the new experimental p2p library. See issue #3147.",
	}
	// UseNewSyncFlag to start the beacon chain using the new sync library.
	UseNewSyncFlag = cli.BoolFlag{
		Name:  "experimental-sync",
		Usage: "Use the new experimental sync libraries. See issue #3147.",
	}
	// UseNewDatabaseFlag to start the beacon chain using new database library.
	UseNewDatabaseFlag = cli.BoolFlag{
		Name:  "experimental-db",
		Usage: "Use the new experimental database library.",
	}
	// UseNewBlockChainFlag to start the beacon chain using new blockchain library.
	UseNewBlockChainFlag = cli.BoolFlag{
		Name:  "experimental-blockchain",
		Usage: "Use the new experimental blockchain library.",
	}
	// NextFlag to enable all experimental features.
	NextFlag = cli.BoolFlag{
		Name:  "next",
		Usage: "Use next version experimental features.",
	}
	// EnableActiveBalanceCacheFlag see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableActiveBalanceCacheFlag = cli.BoolFlag{
		Name:  "enable-active-balance-cache",
		Usage: "Enable unsafe cache mechanism. See https://github.com/prysmaticlabs/prysm/issues/3106",
	}
	// EnableAttestationCacheFlag see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableAttestationCacheFlag = cli.BoolFlag{
		Name:  "enable-attestation-cache",
		Usage: "Enable unsafe cache mechanism. See https://github.com/prysmaticlabs/prysm/issues/3106",
	}
	// EnableAncestorBlockCacheFlag see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableAncestorBlockCacheFlag = cli.BoolFlag{
		Name:  "enable-ancestor-block-cache",
		Usage: "Enable unsafe cache mechanism. See https://github.com/prysmaticlabs/prysm/issues/3106",
	}
	// EnableEth1DataVoteCacheFlag see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableEth1DataVoteCacheFlag = cli.BoolFlag{
		Name:  "enable-eth1-data-vote-cache",
		Usage: "Enable unsafe cache mechanism. See https://github.com/prysmaticlabs/prysm/issues/3106",
	}
	// EnableSeedCacheFlag see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableSeedCacheFlag = cli.BoolFlag{
		Name:  "enable-seed-cache",
		Usage: "Enable unsafe cache mechanism. See https://github.com/prysmaticlabs/prysm/issues/3106",
	}
	// EnableStartShardCacheFlag see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableStartShardCacheFlag = cli.BoolFlag{
		Name:  "enable-start-shard-cache",
		Usage: "Enable unsafe cache mechanism. See https://github.com/prysmaticlabs/prysm/issues/3106",
	}
	// EnableTotalBalanceCacheFlag see https://github.com/prysmaticlabs/prysm/issues/3106.
	EnableTotalBalanceCacheFlag = cli.BoolFlag{
		Name:  "enable-total-balance-cache",
		Usage: "Enable unsafe cache mechanism. See https://github.com/prysmaticlabs/prysm/issues/3106",
	}
	// HashSlingingSlasherFlag run a slashing proof server instance.
	HashSlingingSlasherFlag = cli.BoolFlag{
		Name:  "hash-slinging-slasher",
		Usage: "Store 54,000 epochs of attestations and block headers to serve as a slashing watchtower and generate slashing proofs during the weak subjectivity period.",
	}
)

// ValidatorFlags contains a list of all the feature flags that apply to the validator client.
var ValidatorFlags = []cli.Flag{}

// BeaconChainFlags contains a list of all the feature flags that apply to the beacon-chain client.
var BeaconChainFlags = []cli.Flag{
	EnableCanonicalAttestationFilter,
	DisableHistoricalStatePruningFlag,
	DisableGossipSubFlag,
	EnableExcessDepositsFlag,
	NoGenesisDelayFlag,
	UseNewP2PFlag,
	UseNewSyncFlag,
	UseNewDatabaseFlag,
	NextFlag,
	EnableActiveBalanceCacheFlag,
	EnableAttestationCacheFlag,
	EnableAncestorBlockCacheFlag,
	EnableEth1DataVoteCacheFlag,
	EnableSeedCacheFlag,
	EnableStartShardCacheFlag,
	EnableTotalBalanceCacheFlag,
	HashSlingingSlasherFlag,
}
