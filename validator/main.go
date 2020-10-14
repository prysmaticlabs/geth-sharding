// Package main defines a validator client, a critical actor in eth2 which manages
// a keystore of private keys, connects to a beacon node to receive assignments,
// and submits blocks/attestations as needed.
package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	runtimeDebug "runtime/debug"
	"strings"
	"time"

	joonix "github.com/joonix/log"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/debug"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/journald"
	"github.com/prysmaticlabs/prysm/shared/logutil"
	_ "github.com/prysmaticlabs/prysm/shared/maxprocs"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/tos"
	"github.com/prysmaticlabs/prysm/shared/version"
	v1 "github.com/prysmaticlabs/prysm/validator/accounts/v1"
	v2 "github.com/prysmaticlabs/prysm/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/validator/client"
	"github.com/prysmaticlabs/prysm/validator/flags"
	"github.com/prysmaticlabs/prysm/validator/node"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	prefixed "github.com/x-cray/logrus-prefixed-formatter"
	"google.golang.org/grpc"
)

// connTimeout defines a period after which connection to beacon node is cancelled.
const connTimeout = 10 * time.Second

var log = logrus.WithField("prefix", "main")

func startNode(ctx *cli.Context) error {
	validatorClient, err := node.NewValidatorClient(ctx)
	if err != nil {
		return err
	}
	validatorClient.Start()
	return nil
}

var appFlags = []cli.Flag{
	flags.BeaconRPCProviderFlag,
	flags.BeaconRPCGatewayProviderFlag,
	flags.CertFlag,
	flags.GraffitiFlag,
	flags.KeystorePathFlag,
	flags.SourceDirectories,
	flags.SourceDirectory,
	flags.TargetDirectory,
	flags.PasswordFlag,
	flags.DisablePenaltyRewardLogFlag,
	flags.UnencryptedKeysFlag,
	flags.InteropStartIndex,
	flags.InteropNumValidators,
	flags.EnableRPCFlag,
	flags.RPCHost,
	flags.RPCPort,
	flags.GRPCGatewayPort,
	flags.GRPCGatewayHost,
	flags.GrpcRetriesFlag,
	flags.GrpcRetryDelayFlag,
	flags.GrpcHeadersFlag,
	flags.GPRCGatewayCorsDomain,
	flags.KeyManager,
	flags.KeyManagerOpts,
	flags.DisableAccountMetricsFlag,
	cmd.MonitoringHostFlag,
	flags.MonitoringPortFlag,
	cmd.DisableMonitoringFlag,
	flags.SlasherRPCProviderFlag,
	flags.SlasherCertFlag,
	flags.DeprecatedPasswordsDirFlag,
	flags.WalletPasswordFileFlag,
	flags.WalletDirFlag,
	flags.EnableWebFlag,
	flags.WebHostFlag,
	flags.WebPortFlag,
	cmd.MinimalConfigFlag,
	cmd.E2EConfigFlag,
	cmd.VerbosityFlag,
	cmd.DataDirFlag,
	cmd.ClearDB,
	cmd.ForceClearDB,
	cmd.EnableTracingFlag,
	cmd.TracingProcessNameFlag,
	cmd.TracingEndpointFlag,
	cmd.TraceSampleFractionFlag,
	cmd.LogFormat,
	cmd.LogFileName,
	cmd.ConfigFileFlag,
	cmd.ChainConfigFileFlag,
	cmd.GrpcMaxCallRecvMsgSizeFlag,
	debug.PProfFlag,
	debug.PProfAddrFlag,
	debug.PProfPortFlag,
	debug.MemProfileRateFlag,
	debug.CPUProfileFlag,
	debug.TraceFlag,
}

func init() {
	appFlags = cmd.WrapFlags(append(appFlags, featureconfig.ValidatorFlags...))
}

func main() {
	app := cli.App{}
	app.Name = "validator"
	app.Usage = `launches an Ethereum 2.0 validator client that interacts with a beacon chain,
				 starts proposer and attester services, p2p connections, and more`
	app.Version = version.GetVersion()
	app.Action = startNode
	app.Commands = []*cli.Command{
		v2.WalletCommands,
		v2.AccountCommands,
		{
			Name:     "accounts",
			Category: "accounts",
			Usage:    "defines useful functions for interacting with the validator client's account",
			Subcommands: []*cli.Command{
				{
					Name: "create",
					Description: `creates a new validator account keystore containing private keys for Ethereum 2.0 -
this command outputs a deposit data string which can be used to deposit Ether into the ETH1.0 deposit
contract in order to activate the validator client`,
					Flags: cmd.WrapFlags(append(featureconfig.ActiveFlags(featureconfig.ValidatorFlags),
						[]cli.Flag{
							flags.KeystorePathFlag,
							flags.PasswordFlag,
							cmd.ChainConfigFileFlag,
						}...)),
					Before: func(cliCtx *cli.Context) error {
						return cmd.LoadFlagsFromConfig(cliCtx, cliCtx.Command.Flags)
					},
					Action: func(cliCtx *cli.Context) error {
						featureconfig.ConfigureValidator(cliCtx)

						if cliCtx.IsSet(cmd.ChainConfigFileFlag.Name) {
							chainConfigFileName := cliCtx.String(cmd.ChainConfigFileFlag.Name)
							params.LoadChainConfigFile(chainConfigFileName)
						}

						keystorePath, passphrase, err := v1.HandleEmptyKeystoreFlags(cliCtx, true /*confirmPassword*/)
						if err != nil {
							log.WithError(err).Error("Could not list keys")
							return nil
						}
						if _, _, err := v1.CreateValidatorAccount(keystorePath, passphrase); err != nil {
							log.WithField("err", err.Error()).Fatalf("Could not create validator at path: %s", keystorePath)
						}
						return nil
					},
				},
				{
					Name:        "keys",
					Description: `lists the private keys for 'keystore' keymanager keys`,
					Flags: cmd.WrapFlags([]cli.Flag{
						flags.KeystorePathFlag,
						flags.PasswordFlag,
					}),
					Before: func(cliCtx *cli.Context) error {
						return cmd.LoadFlagsFromConfig(cliCtx, cliCtx.Command.Flags)
					},
					Action: func(cliCtx *cli.Context) error {
						keystorePath, passphrase, err := v1.HandleEmptyKeystoreFlags(cliCtx, false /*confirmPassword*/)
						if err != nil {
							log.WithError(err).Error("Could not list keys")
						}
						if err := v1.PrintPublicAndPrivateKeys(keystorePath, passphrase); err != nil {
							log.WithError(err).Errorf("Could not list private and public keys in path %s", keystorePath)
						}
						return nil
					},
				},
				{
					Name:        "status",
					Description: `list the validator status for existing validator keys`,
					Flags: cmd.WrapFlags([]cli.Flag{
						cmd.GrpcMaxCallRecvMsgSizeFlag,
						flags.BeaconRPCProviderFlag,
						flags.CertFlag,
						flags.GrpcHeadersFlag,
						flags.GrpcRetriesFlag,
						flags.GrpcRetryDelayFlag,
						flags.KeyManager,
						flags.KeyManagerOpts,
					}),
					Before: func(cliCtx *cli.Context) error {
						return cmd.LoadFlagsFromConfig(cliCtx, cliCtx.Command.Flags)
					},
					Action: func(cliCtx *cli.Context) error {
						var err error
						var pubKeys [][]byte
						if cliCtx.String(flags.KeyManager.Name) == "" {
							keystorePath, passphrase, err := v1.HandleEmptyKeystoreFlags(cliCtx, false /*confirmPassword*/)
							if err != nil {
								return err
							}
							pubKeys, err = v1.ExtractPublicKeysFromKeyStore(keystorePath, passphrase)
							if err != nil {
								return err
							}
						}
						ctx, cancel := context.WithTimeout(cliCtx.Context, connTimeout)
						defer cancel()
						dialOpts := client.ConstructDialOptions(
							cliCtx.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name),
							cliCtx.String(flags.CertFlag.Name),
							strings.Split(cliCtx.String(flags.GrpcHeadersFlag.Name), ","),
							cliCtx.Uint(flags.GrpcRetriesFlag.Name),
							cliCtx.Duration(flags.GrpcRetryDelayFlag.Name),
							grpc.WithBlock())
						endpoint := cliCtx.String(flags.BeaconRPCProviderFlag.Name)
						conn, err := grpc.DialContext(ctx, endpoint, dialOpts...)
						if err != nil {
							log.WithError(err).Errorf("Failed to dial beacon node endpoint at %s", endpoint)
							return err
						}
						err = v1.RunStatusCommand(ctx, pubKeys, ethpb.NewBeaconNodeValidatorClient(conn))
						if closed := conn.Close(); closed != nil {
							log.WithError(closed).Error("Could not close connection to beacon node")
						}
						return err
					},
				},
				{
					Name:        "change-password",
					Description: "changes password for all keys located in a keystore",
					Flags: cmd.WrapFlags([]cli.Flag{
						flags.KeystorePathFlag,
						flags.PasswordFlag,
					}),
					Before: func(cliCtx *cli.Context) error {
						return cmd.LoadFlagsFromConfig(cliCtx, cliCtx.Command.Flags)
					},
					Action: func(cliCtx *cli.Context) error {
						keystorePath, oldPassword, err := v1.HandleEmptyKeystoreFlags(cliCtx, false /*confirmPassword*/)
						if err != nil {
							log.WithError(err).Error("Could not read keystore path and/or the old password")
						}

						log.Info("Please enter the new password")
						newPassword, err := cmd.EnterPassword(true, cmd.StdInPasswordReader{})
						if err != nil {
							log.WithError(err).Error("Could not read the new password")
						}

						err = v1.ChangePassword(keystorePath, oldPassword, newPassword)
						if err != nil {
							log.WithError(err).Error("Changing password failed")
						} else {
							log.Info("Password changed successfully")
						}

						return nil
					},
				},
				{
					Name:        "merge",
					Description: "merges data from several validator databases into a new validator database",
					Flags: cmd.WrapFlags([]cli.Flag{
						flags.SourceDirectories,
						flags.TargetDirectory,
					}),
					Before: func(cliCtx *cli.Context) error {
						return cmd.LoadFlagsFromConfig(cliCtx, cliCtx.Command.Flags)
					},
					Action: func(cliCtx *cli.Context) error {
						passedSources := cliCtx.String(flags.SourceDirectories.Name)
						sources := strings.Split(passedSources, ",")
						target := cliCtx.String(flags.TargetDirectory.Name)

						if err := v1.Merge(cliCtx.Context, sources, target); err != nil {
							log.WithError(err).Error("Merging validator data failed")
						} else {
							log.Info("Merge completed successfully")
						}

						return nil
					},
				},
				{
					Name:        "split",
					Description: "splits one validator database into several databases - one for each public key",
					Flags: cmd.WrapFlags([]cli.Flag{
						flags.SourceDirectory,
						flags.TargetDirectory,
					}),
					Before: func(cliCtx *cli.Context) error {
						return cmd.LoadFlagsFromConfig(cliCtx, cliCtx.Command.Flags)
					},
					Action: func(cliCtx *cli.Context) error {
						source := cliCtx.String(flags.SourceDirectory.Name)
						target := cliCtx.String(flags.TargetDirectory.Name)

						if err := v1.Split(cliCtx.Context, source, target); err != nil {
							log.WithError(err).Error("Splitting validator data failed")
						} else {
							log.Info("Split completed successfully")
						}

						return nil
					},
				},
			},
		},
	}

	app.Flags = appFlags

	app.Before = func(ctx *cli.Context) error {
		// verify if ToS accepted
		if err := tos.VerifyTosAcceptedOrPrompt(ctx); err != nil {
			return err
		}

		// Load flags from config file, if specified.
		if err := cmd.LoadFlagsFromConfig(ctx, app.Flags); err != nil {
			return err
		}

		flags.ComplainOnDeprecatedFlags(ctx)

		format := ctx.String(cmd.LogFormat.Name)
		switch format {
		case "text":
			formatter := new(prefixed.TextFormatter)
			formatter.TimestampFormat = "2006-01-02 15:04:05"
			formatter.FullTimestamp = true
			// If persistent log files are written - we disable the log messages coloring because
			// the colors are ANSI codes and seen as Gibberish in the log files.
			formatter.DisableColors = ctx.String(cmd.LogFileName.Name) != ""
			logrus.SetFormatter(formatter)
		case "fluentd":
			f := joonix.NewFormatter()
			if err := joonix.DisableTimestampFormat(f); err != nil {
				panic(err)
			}
			logrus.SetFormatter(f)
		case "json":
			logrus.SetFormatter(&logrus.JSONFormatter{})
		case "journald":
			if err := journald.Enable(); err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown log format %s", format)
		}

		logFileName := ctx.String(cmd.LogFileName.Name)
		if logFileName != "" {
			if err := logutil.ConfigurePersistentLogging(logFileName); err != nil {
				log.WithError(err).Error("Failed to configuring logging to disk.")
			}
		}

		runtime.GOMAXPROCS(runtime.NumCPU())
		return debug.Setup(ctx)
	}

	app.After = func(ctx *cli.Context) error {
		debug.Exit(ctx)
		return nil
	}

	defer func() {
		if x := recover(); x != nil {
			log.Errorf("Runtime panic: %v\n%v", x, string(runtimeDebug.Stack()))
			panic(x)
		}
	}()

	if err := app.Run(os.Args); err != nil {
		log.Error(err.Error())
		os.Exit(1)
	}
}
