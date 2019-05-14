// Package beacon-chain defines all the utilities needed for a beacon chain node.
package main

import (
	"fmt"
	"os"
	"runtime"

	joonix "github.com/joonix/log"
	"github.com/prysmaticlabs/prysm/beacon-chain/node"
	"github.com/prysmaticlabs/prysm/beacon-chain/utils"
	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/debug"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/version"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/x-cray/logrus-prefixed-formatter"
)

func startNode(ctx *cli.Context) error {
	verbosity := ctx.GlobalString(cmd.VerbosityFlag.Name)
	level, err := logrus.ParseLevel(verbosity)
	if err != nil {
		return err
	}
	logrus.SetLevel(level)

	beacon, err := node.NewBeaconNode(ctx)
	if err != nil {
		return err
	}
	beacon.Start()
	return nil
}

func main() {
	log := logrus.WithField("prefix", "main")
	app := cli.NewApp()
	app.Name = "beacon-chain"
	app.Usage = "this is a beacon chain implementation for Ethereum 2.0"
	app.Action = startNode
	app.Version = version.GetVersion()

	app.Flags = []cli.Flag{
		utils.NoCustomConfigFlag,
		utils.DepositContractFlag,
		utils.Web3ProviderFlag,
		utils.HTTPWeb3ProviderFlag,
		utils.RPCPort,
		utils.CertFlag,
		utils.KeyFlag,
		utils.EnableDBCleanup,
		cmd.BootstrapNode,
		cmd.NoDiscovery,
		cmd.StaticPeers,
		cmd.RelayNode,
		cmd.P2PPort,
		cmd.P2PHost,
		cmd.P2PMaxPeers,
		cmd.DataDirFlag,
		cmd.VerbosityFlag,
		cmd.EnableTracingFlag,
		cmd.TracingProcessNameFlag,
		cmd.TracingEndpointFlag,
		cmd.TraceSampleFractionFlag,
		cmd.MonitoringPortFlag,
		cmd.DisableMonitoringFlag,
		cmd.ClearDB,
		cmd.LogFormat,
		debug.PProfFlag,
		debug.PProfAddrFlag,
		debug.PProfPortFlag,
		debug.MemProfileRateFlag,
		debug.CPUProfileFlag,
		debug.TraceFlag,
	}

	app.Flags = append(app.Flags, featureconfig.BeaconChainFlags...)

	app.Before = func(ctx *cli.Context) error {
		format := ctx.GlobalString(cmd.LogFormat.Name)
		switch format {
		case "text":
			formatter := new(prefixed.TextFormatter)
			formatter.TimestampFormat = "2006-01-02 15:04:05"
			formatter.FullTimestamp = true
			logrus.SetFormatter(formatter)
			break
		case "fluentd":
			logrus.SetFormatter(&joonix.FluentdFormatter{})
			break
		case "json":
			logrus.SetFormatter(&logrus.JSONFormatter{})
			break
		default:
			return fmt.Errorf("unknown log format %s", format)
		}

		runtime.GOMAXPROCS(runtime.NumCPU())
		return debug.Setup(ctx)
	}

	if err := app.Run(os.Args); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
