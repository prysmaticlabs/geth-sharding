// This code was adapted from https://github.com/ethereum/go-ethereum/blob/master/cmd/geth/usage.go
package main

import (
	"io"
	"sort"

	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/debug"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/validator/flags"
	"github.com/urfave/cli/v2"
)

var appHelpTemplate = `NAME:
   {{.App.Name}} - {{.App.Usage}}
USAGE:
   {{.App.HelpName}} [options]{{if .App.Commands}} command [command options]{{end}} {{if .App.ArgsUsage}}{{.App.ArgsUsage}}{{else}}[arguments...]{{end}}
   {{if .App.Version}}
AUTHOR:
   {{range .App.Authors}}{{ . }}{{end}}
   {{end}}{{if .App.Commands}}
GLOBAL OPTIONS:
   {{range .App.Commands}}{{join .Names ", "}}{{ "\t" }}{{.Usage}}
   {{end}}{{end}}{{if .FlagGroups}}
{{range .FlagGroups}}{{.Name}} OPTIONS:
  {{range .Flags}}{{.}}
  {{end}}
{{end}}{{end}}{{if .App.Copyright }}
COPYRIGHT:
   {{.App.Copyright}}
VERSION:
   {{.App.Version}}
   {{end}}{{if len .App.Authors}}
   {{end}}
`

type flagGroup struct {
	Name  string
	Flags []cli.Flag
}

var appHelpFlagGroups = []flagGroup{
	{
		Name: "cmd",
		Flags: []cli.Flag{
			cmd.MinimalConfigFlag,
			cmd.E2EConfigFlag,
			cmd.VerbosityFlag,
			cmd.DataDirFlag,
			cmd.ClearDB,
			cmd.ForceClearDB,
			cmd.EnableBackupWebhookFlag,
			cmd.BackupWebhookOutputDir,
			cmd.EnableTracingFlag,
			cmd.TracingProcessNameFlag,
			cmd.TracingEndpointFlag,
			cmd.TraceSampleFractionFlag,
			cmd.MonitoringHostFlag,
			flags.MonitoringPortFlag,
			cmd.DisableMonitoringFlag,
			cmd.LogFormat,
			cmd.LogFileName,
			cmd.ConfigFileFlag,
			cmd.ChainConfigFileFlag,
			cmd.GrpcMaxCallRecvMsgSizeFlag,
			cmd.AcceptTosFlag,
		},
	},
	{
		Name: "debug",
		Flags: []cli.Flag{
			debug.PProfFlag,
			debug.PProfAddrFlag,
			debug.PProfPortFlag,
			debug.MemProfileRateFlag,
			debug.CPUProfileFlag,
			debug.TraceFlag,
			debug.BlockProfileRateFlag,
			debug.MutexProfileFractionFlag,
		},
	},
	{
		Name: "validator",
		Flags: []cli.Flag{
			flags.BeaconRPCProviderFlag,
			flags.BeaconRPCGatewayProviderFlag,
			flags.CertFlag,
			flags.EnableWebFlag,
			flags.DisablePenaltyRewardLogFlag,
			flags.GraffitiFlag,
			flags.EnableRPCFlag,
			flags.RPCHost,
			flags.RPCPort,
			flags.GRPCGatewayPort,
			flags.GRPCGatewayHost,
			flags.GrpcRetriesFlag,
			flags.GrpcRetryDelayFlag,
			flags.GPRCGatewayCorsDomain,
			flags.GrpcHeadersFlag,
			flags.SlasherRPCProviderFlag,
			flags.SlasherCertFlag,
			flags.DisableAccountMetricsFlag,
			flags.WalletDirFlag,
			flags.WalletPasswordFileFlag,
			flags.GraffitiFileFlag,
			flags.EnableDutyCountDown,
		},
	},
	{
		Name:  "features",
		Flags: featureconfig.ActiveFlags(featureconfig.ValidatorFlags),
	},
	{
		Name: "interop",
		Flags: []cli.Flag{
			flags.InteropNumValidators,
			flags.InteropStartIndex,
		},
	},
}

func init() {
	cli.AppHelpTemplate = appHelpTemplate

	type helpData struct {
		App        interface{}
		FlagGroups []flagGroup
	}

	originalHelpPrinter := cli.HelpPrinter
	cli.HelpPrinter = func(w io.Writer, tmpl string, data interface{}) {
		if tmpl == appHelpTemplate {
			for _, group := range appHelpFlagGroups {
				sort.Sort(cli.FlagsByName(group.Flags))
			}
			originalHelpPrinter(w, tmpl, helpData{data, appHelpFlagGroups})
		} else {
			originalHelpPrinter(w, tmpl, data)
		}
	}
}
