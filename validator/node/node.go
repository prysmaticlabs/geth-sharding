// Package node is the main process which handles the lifecycle of
// the runtime services in a validator client process, gracefully shutting
// everything down upon close.
package node

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	gwruntime "github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/prysm/cmd/validator/flags"
	pb "github.com/prysmaticlabs/prysm/proto/validator/accounts/v2"
	"github.com/prysmaticlabs/prysm/shared"
	"github.com/prysmaticlabs/prysm/shared/backuputil"
	"github.com/prysmaticlabs/prysm/shared/cmd"
	"github.com/prysmaticlabs/prysm/shared/debug"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/fileutil"
	"github.com/prysmaticlabs/prysm/shared/gateway"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/prereq"
	"github.com/prysmaticlabs/prysm/shared/prometheus"
	"github.com/prysmaticlabs/prysm/shared/tracing"
	"github.com/prysmaticlabs/prysm/shared/version"
	accountsiface "github.com/prysmaticlabs/prysm/validator/accounts/iface"
	"github.com/prysmaticlabs/prysm/validator/accounts/wallet"
	"github.com/prysmaticlabs/prysm/validator/client"
	"github.com/prysmaticlabs/prysm/validator/db/kv"
	g "github.com/prysmaticlabs/prysm/validator/graffiti"
	"github.com/prysmaticlabs/prysm/validator/keymanager"
	"github.com/prysmaticlabs/prysm/validator/keymanager/imported"
	"github.com/prysmaticlabs/prysm/validator/rpc"
	slashingprotection "github.com/prysmaticlabs/prysm/validator/slashing-protection"
	"github.com/prysmaticlabs/prysm/validator/slashing-protection/iface"
	"github.com/prysmaticlabs/prysm/validator/web"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli/v2"
	"google.golang.org/protobuf/encoding/protojson"
)

// ValidatorClient defines an instance of an Ethereum validator that manages
// the entire lifecycle of services attached to it participating in proof of stake.
type ValidatorClient struct {
	cliCtx            *cli.Context
	ctx               context.Context
	cancel            context.CancelFunc
	db                *kv.Store
	services          *shared.ServiceRegistry // Lifecycle and service store.
	lock              sync.RWMutex
	wallet            *wallet.Wallet
	walletInitialized *event.Feed
	stop              chan struct{} // Channel to wait for termination notifications.
}

// NewValidatorClient creates a new instance of the Prysm validator client.
func NewValidatorClient(cliCtx *cli.Context) (*ValidatorClient, error) {
	if err := tracing.Setup(
		"validator", // service name
		cliCtx.String(cmd.TracingProcessNameFlag.Name),
		cliCtx.String(cmd.TracingEndpointFlag.Name),
		cliCtx.Float64(cmd.TraceSampleFractionFlag.Name),
		cliCtx.Bool(cmd.EnableTracingFlag.Name),
	); err != nil {
		return nil, err
	}

	verbosity := cliCtx.String(cmd.VerbosityFlag.Name)
	level, err := logrus.ParseLevel(verbosity)
	if err != nil {
		return nil, err
	}
	logrus.SetLevel(level)

	// Warn if user's platform is not supported
	prereq.WarnIfPlatformNotSupported(cliCtx.Context)

	registry := shared.NewServiceRegistry()
	ctx, cancel := context.WithCancel(cliCtx.Context)
	validatorClient := &ValidatorClient{
		cliCtx:            cliCtx,
		ctx:               ctx,
		cancel:            cancel,
		services:          registry,
		walletInitialized: new(event.Feed),
		stop:              make(chan struct{}),
	}

	featureconfig.ConfigureValidator(cliCtx)
	cmd.ConfigureValidator(cliCtx)

	if cliCtx.IsSet(cmd.ChainConfigFileFlag.Name) {
		chainConfigFileName := cliCtx.String(cmd.ChainConfigFileFlag.Name)
		params.LoadChainConfigFile(chainConfigFileName)
	}

	// If the --web flag is enabled to administer the validator
	// client via a web portal, we start the validator client in a different way.
	if cliCtx.IsSet(flags.EnableWebFlag.Name) {
		log.Info("Enabling web portal to manage the validator client")
		if err := validatorClient.initializeForWeb(cliCtx); err != nil {
			return nil, err
		}
		return validatorClient, nil
	}

	if cliCtx.IsSet(cmd.ChainConfigFileFlag.Name) {
		chainConfigFileName := cliCtx.String(cmd.ChainConfigFileFlag.Name)
		params.LoadChainConfigFile(chainConfigFileName)
	}

	// Initializes any forks here.
	params.BeaconConfig().InitializeForkSchedule()

	if err := validatorClient.initializeFromCLI(cliCtx); err != nil {
		return nil, err
	}

	return validatorClient, nil
}

// Start every service in the validator client.
func (c *ValidatorClient) Start() {
	c.lock.Lock()

	log.WithFields(logrus.Fields{
		"version": version.Version(),
	}).Info("Starting validator node")

	c.services.StartAll()

	stop := c.stop
	c.lock.Unlock()

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got interrupt, shutting down...")
		debug.Exit(c.cliCtx) // Ensure trace and CPU profile data are flushed.
		go c.Close()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.WithField("times", i-1).Info("Already shutting down, interrupt more to panic.")
			}
		}
		panic("Panic closing the validator client")
	}()

	// Wait for stop channel to be closed.
	<-stop
}

// Close handles graceful shutdown of the system.
func (c *ValidatorClient) Close() {
	c.lock.Lock()
	defer c.lock.Unlock()

	c.services.StopAll()
	log.Info("Stopping Prysm validator")
	c.cancel()
	close(c.stop)
}

func (c *ValidatorClient) initializeFromCLI(cliCtx *cli.Context) error {
	var keyManager keymanager.IKeymanager
	var err error
	if cliCtx.IsSet(flags.InteropNumValidators.Name) {
		numValidatorKeys := cliCtx.Uint64(flags.InteropNumValidators.Name)
		offset := cliCtx.Uint64(flags.InteropStartIndex.Name)
		keyManager, err = imported.NewInteropKeymanager(cliCtx.Context, offset, numValidatorKeys)
		if err != nil {
			return errors.Wrap(err, "could not generate interop keys")
		}
	} else {
		// Read the wallet from the specified path.
		w, err := wallet.OpenWalletOrElseCli(cliCtx, func(cliCtx *cli.Context) (*wallet.Wallet, error) {
			return nil, wallet.ErrNoWalletFound
		})
		if err != nil {
			return errors.Wrap(err, "could not open wallet")
		}
		c.wallet = w
		log.WithFields(logrus.Fields{
			"wallet":          w.AccountsDir(),
			"keymanager-kind": w.KeymanagerKind().String(),
		}).Info("Opened validator wallet")
		keyManager, err = w.InitializeKeymanager(cliCtx.Context, accountsiface.InitKeymanagerConfig{ListenForChanges: true})
		if err != nil {
			return errors.Wrap(err, "could not read keymanager for wallet")
		}
	}
	dataDir := cliCtx.String(flags.WalletDirFlag.Name)
	if c.wallet != nil {
		dataDir = c.wallet.AccountsDir()
	}
	if cliCtx.String(cmd.DataDirFlag.Name) != cmd.DefaultDataDir() {
		dataDir = cliCtx.String(cmd.DataDirFlag.Name)
	}
	clearFlag := cliCtx.Bool(cmd.ClearDB.Name)
	forceClearFlag := cliCtx.Bool(cmd.ForceClearDB.Name)
	if clearFlag || forceClearFlag {
		if dataDir == "" && c.wallet != nil {
			dataDir = c.wallet.AccountsDir()
			if dataDir == "" {
				log.Fatal(
					"Could not determine your system'c HOME path, please specify a --datadir you wish " +
						"to use for your validator data",
				)
			}

		}
		if err := clearDB(cliCtx.Context, dataDir, forceClearFlag); err != nil {
			return err
		}
	} else {
		dataFile := filepath.Join(dataDir, kv.ProtectionDbFileName)
		if !fileutil.FileExists(dataFile) {
			log.Warnf("Slashing protection file %s is missing.\n"+
				"If you changed your --wallet-dir or --datadir, please copy your previous \"validator.db\" file into your current --datadir.\n"+
				"Disregard this warning if this is the first time you are running this set of keys.", dataFile)
		}
	}
	log.WithField("databasePath", dataDir).Info("Checking DB")

	valDB, err := kv.NewKVStore(cliCtx.Context, dataDir, &kv.Config{
		PubKeys:         nil,
		InitialMMapSize: cliCtx.Int(cmd.BoltMMapInitialSizeFlag.Name),
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize db")
	}
	c.db = valDB
	if err := valDB.RunUpMigrations(cliCtx.Context); err != nil {
		return errors.Wrap(err, "could not run database migration")
	}

	if !cliCtx.Bool(cmd.DisableMonitoringFlag.Name) {
		if err := c.registerPrometheusService(cliCtx); err != nil {
			return err
		}
	}
	if featureconfig.Get().SlasherProtection {
		if err := c.registerSlasherService(); err != nil {
			return err
		}
	}
	if err := c.registerValidatorService(keyManager); err != nil {
		return err
	}
	if cliCtx.Bool(flags.EnableRPCFlag.Name) {
		if err := c.registerRPCService(cliCtx, keyManager); err != nil {
			return err
		}
		if err := c.registerRPCGatewayService(cliCtx); err != nil {
			return err
		}
	}
	return nil
}

func (c *ValidatorClient) initializeForWeb(cliCtx *cli.Context) error {
	var keyManager keymanager.IKeymanager
	var err error

	// Read the wallet password file from the cli context.
	if err = setWalletPasswordFilePath(cliCtx); err != nil {
		return errors.Wrap(err, "could not read wallet password file")
	}

	// Read the wallet from the specified path.
	w, err := wallet.OpenWalletOrElseCli(cliCtx, func(cliCtx *cli.Context) (*wallet.Wallet, error) {
		return nil, nil
	})
	if err != nil {
		return errors.Wrap(err, "could not open wallet")
	}
	if w != nil {
		c.wallet = w
		log.WithFields(logrus.Fields{
			"wallet":          w.AccountsDir(),
			"keymanager-kind": w.KeymanagerKind().String(),
		}).Info("Opened validator wallet")
		keyManager, err = w.InitializeKeymanager(cliCtx.Context, accountsiface.InitKeymanagerConfig{ListenForChanges: true})
		if err != nil {
			return errors.Wrap(err, "could not read keymanager for wallet")
		}
	}
	dataDir := cliCtx.String(flags.WalletDirFlag.Name)
	if c.wallet != nil {
		dataDir = c.wallet.AccountsDir()
	}
	if cliCtx.String(cmd.DataDirFlag.Name) != cmd.DefaultDataDir() {
		dataDir = cliCtx.String(cmd.DataDirFlag.Name)
	}
	clearFlag := cliCtx.Bool(cmd.ClearDB.Name)
	forceClearFlag := cliCtx.Bool(cmd.ForceClearDB.Name)

	if clearFlag || forceClearFlag {
		if dataDir == "" {
			dataDir = cmd.DefaultDataDir()
			if dataDir == "" {
				log.Fatal(
					"Could not determine your system'c HOME path, please specify a --datadir you wish " +
						"to use for your validator data",
				)
			}

		}
		if err := clearDB(cliCtx.Context, dataDir, forceClearFlag); err != nil {
			return err
		}
	}
	log.WithField("databasePath", dataDir).Info("Checking DB")
	valDB, err := kv.NewKVStore(cliCtx.Context, dataDir, &kv.Config{
		PubKeys:         nil,
		InitialMMapSize: cliCtx.Int(cmd.BoltMMapInitialSizeFlag.Name),
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize db")
	}
	c.db = valDB
	if err := valDB.RunUpMigrations(cliCtx.Context); err != nil {
		return errors.Wrap(err, "could not run database migration")
	}

	if !cliCtx.Bool(cmd.DisableMonitoringFlag.Name) {
		if err := c.registerPrometheusService(cliCtx); err != nil {
			return err
		}
	}
	if featureconfig.Get().SlasherProtection {
		if err := c.registerSlasherService(); err != nil {
			return err
		}
	}
	if err := c.registerValidatorService(keyManager); err != nil {
		return err
	}
	if err := c.registerRPCService(cliCtx, keyManager); err != nil {
		return err
	}
	if err := c.registerRPCGatewayService(cliCtx); err != nil {
		return err
	}
	gatewayHost := cliCtx.String(flags.GRPCGatewayHost.Name)
	gatewayPort := cliCtx.Int(flags.GRPCGatewayPort.Name)
	webAddress := fmt.Sprintf("http://%s:%d", gatewayHost, gatewayPort)
	log.WithField("address", webAddress).Info(
		"Starting Prysm web UI on address, open in browser to access",
	)
	return nil
}

func (c *ValidatorClient) registerPrometheusService(cliCtx *cli.Context) error {
	var additionalHandlers []prometheus.Handler
	if cliCtx.IsSet(cmd.EnableBackupWebhookFlag.Name) {
		additionalHandlers = append(
			additionalHandlers,
			prometheus.Handler{
				Path:    "/db/backup",
				Handler: backuputil.BackupHandler(c.db, cliCtx.String(cmd.BackupWebhookOutputDir.Name)),
			},
		)
	}
	service := prometheus.NewService(
		fmt.Sprintf("%s:%d", c.cliCtx.String(cmd.MonitoringHostFlag.Name), c.cliCtx.Int(flags.MonitoringPortFlag.Name)),
		c.services,
		additionalHandlers...,
	)
	logrus.AddHook(prometheus.NewLogrusCollector())
	return c.services.RegisterService(service)
}

func (c *ValidatorClient) registerValidatorService(
	keyManager keymanager.IKeymanager,
) error {
	endpoint := c.cliCtx.String(flags.BeaconRPCProviderFlag.Name)
	dataDir := c.cliCtx.String(cmd.DataDirFlag.Name)
	logValidatorBalances := !c.cliCtx.Bool(flags.DisablePenaltyRewardLogFlag.Name)
	emitAccountMetrics := !c.cliCtx.Bool(flags.DisableAccountMetricsFlag.Name)
	cert := c.cliCtx.String(flags.CertFlag.Name)
	graffiti := c.cliCtx.String(flags.GraffitiFlag.Name)
	maxCallRecvMsgSize := c.cliCtx.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)
	grpcRetries := c.cliCtx.Uint(flags.GrpcRetriesFlag.Name)
	grpcRetryDelay := c.cliCtx.Duration(flags.GrpcRetryDelayFlag.Name)
	var sp *slashingprotection.Service
	var protector iface.Protector
	if err := c.services.FetchService(&sp); err == nil {
		protector = sp
	}

	gStruct := &g.Graffiti{}
	var err error
	if c.cliCtx.IsSet(flags.GraffitiFileFlag.Name) {
		n := c.cliCtx.String(flags.GraffitiFileFlag.Name)
		gStruct, err = g.ParseGraffitiFile(n)
		if err != nil {
			log.WithError(err).Warn("Could not parse graffiti file")
		}
	}

	v, err := client.NewValidatorService(c.cliCtx.Context, &client.Config{
		Endpoint:                   endpoint,
		DataDir:                    dataDir,
		KeyManager:                 keyManager,
		LogValidatorBalances:       logValidatorBalances,
		EmitAccountMetrics:         emitAccountMetrics,
		CertFlag:                   cert,
		GraffitiFlag:               g.ParseHexGraffiti(graffiti),
		GrpcMaxCallRecvMsgSizeFlag: maxCallRecvMsgSize,
		GrpcRetriesFlag:            grpcRetries,
		GrpcRetryDelay:             grpcRetryDelay,
		GrpcHeadersFlag:            c.cliCtx.String(flags.GrpcHeadersFlag.Name),
		Protector:                  protector,
		ValDB:                      c.db,
		UseWeb:                     c.cliCtx.Bool(flags.EnableWebFlag.Name),
		WalletInitializedFeed:      c.walletInitialized,
		GraffitiStruct:             gStruct,
		LogDutyCountDown:           c.cliCtx.Bool(flags.EnableDutyCountDown.Name),
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize validator service")
	}

	return c.services.RegisterService(v)
}
func (c *ValidatorClient) registerSlasherService() error {
	endpoint := c.cliCtx.String(flags.SlasherRPCProviderFlag.Name)
	if endpoint == "" {
		return errors.New("external slasher feature flag is set but no slasher endpoint is configured")

	}
	cert := c.cliCtx.String(flags.SlasherCertFlag.Name)
	maxCallRecvMsgSize := c.cliCtx.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)
	grpcRetries := c.cliCtx.Uint(flags.GrpcRetriesFlag.Name)
	grpcRetryDelay := c.cliCtx.Duration(flags.GrpcRetryDelayFlag.Name)
	sp, err := slashingprotection.NewService(c.cliCtx.Context, &slashingprotection.Config{
		Endpoint:                   endpoint,
		CertFlag:                   cert,
		GrpcMaxCallRecvMsgSizeFlag: maxCallRecvMsgSize,
		GrpcRetriesFlag:            grpcRetries,
		GrpcRetryDelay:             grpcRetryDelay,
		GrpcHeadersFlag:            c.cliCtx.String(flags.GrpcHeadersFlag.Name),
	})
	if err != nil {
		return errors.Wrap(err, "could not initialize slasher service")
	}
	return c.services.RegisterService(sp)
}

func (c *ValidatorClient) registerRPCService(cliCtx *cli.Context, km keymanager.IKeymanager) error {
	var vs *client.ValidatorService
	if err := c.services.FetchService(&vs); err != nil {
		return err
	}
	validatorGatewayHost := cliCtx.String(flags.GRPCGatewayHost.Name)
	validatorGatewayPort := cliCtx.Int(flags.GRPCGatewayPort.Name)
	validatorMonitoringHost := cliCtx.String(cmd.MonitoringHostFlag.Name)
	validatorMonitoringPort := cliCtx.Int(flags.MonitoringPortFlag.Name)
	rpcHost := cliCtx.String(flags.RPCHost.Name)
	rpcPort := cliCtx.Int(flags.RPCPort.Name)
	nodeGatewayEndpoint := cliCtx.String(flags.BeaconRPCGatewayProviderFlag.Name)
	beaconClientEndpoint := cliCtx.String(flags.BeaconRPCProviderFlag.Name)
	maxCallRecvMsgSize := c.cliCtx.Int(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)
	grpcRetries := c.cliCtx.Uint(flags.GrpcRetriesFlag.Name)
	grpcRetryDelay := c.cliCtx.Duration(flags.GrpcRetryDelayFlag.Name)
	walletDir := cliCtx.String(flags.WalletDirFlag.Name)
	grpcHeaders := c.cliCtx.String(flags.GrpcHeadersFlag.Name)
	clientCert := c.cliCtx.String(flags.CertFlag.Name)
	server := rpc.NewServer(cliCtx.Context, &rpc.Config{
		ValDB:                    c.db,
		Host:                     rpcHost,
		Port:                     fmt.Sprintf("%d", rpcPort),
		WalletInitializedFeed:    c.walletInitialized,
		ValidatorService:         vs,
		SyncChecker:              vs,
		GenesisFetcher:           vs,
		NodeGatewayEndpoint:      nodeGatewayEndpoint,
		WalletDir:                walletDir,
		Wallet:                   c.wallet,
		Keymanager:               km,
		ValidatorGatewayHost:     validatorGatewayHost,
		ValidatorGatewayPort:     validatorGatewayPort,
		ValidatorMonitoringHost:  validatorMonitoringHost,
		ValidatorMonitoringPort:  validatorMonitoringPort,
		BeaconClientEndpoint:     beaconClientEndpoint,
		ClientMaxCallRecvMsgSize: maxCallRecvMsgSize,
		ClientGrpcRetries:        grpcRetries,
		ClientGrpcRetryDelay:     grpcRetryDelay,
		ClientGrpcHeaders:        strings.Split(grpcHeaders, ","),
		ClientWithCert:           clientCert,
	})
	return c.services.RegisterService(server)
}

func (c *ValidatorClient) registerRPCGatewayService(cliCtx *cli.Context) error {
	gatewayHost := cliCtx.String(flags.GRPCGatewayHost.Name)
	if gatewayHost != flags.DefaultGatewayHost {
		log.WithField("web-host", gatewayHost).Warn(
			"You are using a non-default web host. Web traffic is served by HTTP, so be wary of " +
				"changing this parameter if you are exposing this host to the Internet!",
		)
	}
	gatewayPort := cliCtx.Int(flags.GRPCGatewayPort.Name)
	rpcHost := cliCtx.String(flags.RPCHost.Name)
	rpcPort := cliCtx.Int(flags.RPCPort.Name)
	rpcAddr := fmt.Sprintf("%s:%d", rpcHost, rpcPort)
	gatewayAddress := fmt.Sprintf("%s:%d", gatewayHost, gatewayPort)
	allowedOrigins := strings.Split(cliCtx.String(flags.GPRCGatewayCorsDomain.Name), ",")
	maxCallSize := cliCtx.Uint64(cmd.GrpcMaxCallRecvMsgSizeFlag.Name)

	registrations := []gateway.PbHandlerRegistration{
		pb.RegisterAuthHandler,
		pb.RegisterWalletHandler,
		pb.RegisterHealthHandler,
		pb.RegisterAccountsHandler,
		pb.RegisterBeaconHandler,
		pb.RegisterSlashingProtectionHandler,
	}
	mux := gwruntime.NewServeMux(
		gwruntime.WithMarshalerOption(gwruntime.MIMEWildcard, &gwruntime.HTTPBodyMarshaler{
			Marshaler: &gwruntime.JSONPb{
				MarshalOptions: protojson.MarshalOptions{
					EmitUnpopulated: true,
				},
				UnmarshalOptions: protojson.UnmarshalOptions{
					DiscardUnknown: true,
				},
			},
		}),
		gwruntime.WithMarshalerOption(
			"text/event-stream", &gwruntime.EventSourceJSONPb{},
		),
	)
	muxHandler := func(h http.Handler, w http.ResponseWriter, req *http.Request) {
		if strings.HasPrefix(req.URL.Path, "/api") {
			http.StripPrefix("/api", h).ServeHTTP(w, req)
		} else {
			web.Handler(w, req)
		}
	}

	pbHandler := gateway.PbMux{
		Registrations: registrations,
		Patterns:      []string{"/accounts/", "/v2/"},
		Mux:           mux,
	}

	gw := gateway.New(
		cliCtx.Context,
		[]gateway.PbMux{pbHandler},
		muxHandler,
		rpcAddr,
		gatewayAddress,
	).WithAllowedOrigins(allowedOrigins).WithMaxCallRecvMsgSize(maxCallSize)

	return c.services.RegisterService(gw)
}

func setWalletPasswordFilePath(cliCtx *cli.Context) error {
	walletDir := cliCtx.String(flags.WalletDirFlag.Name)
	defaultWalletPasswordFilePath := filepath.Join(walletDir, wallet.DefaultWalletPasswordFile)
	if fileutil.FileExists(defaultWalletPasswordFilePath) {
		// Ensure file has proper permissions.
		hasPerms, err := fileutil.HasReadWritePermissions(defaultWalletPasswordFilePath)
		if err != nil {
			return err
		}
		if !hasPerms {
			return fmt.Errorf(
				"wallet password file %s does not have proper 0600 permissions",
				defaultWalletPasswordFilePath,
			)
		}

		// Set the filepath into the cli context.
		if err := cliCtx.Set(flags.WalletPasswordFileFlag.Name, defaultWalletPasswordFilePath); err != nil {
			return errors.Wrap(err, "could not set default wallet password file path")
		}
	}
	return nil
}

func clearDB(ctx context.Context, dataDir string, force bool) error {
	var err error
	clearDBConfirmed := force

	if !force {
		actionText := "This will delete your validator's historical actions database stored in your data directory. " +
			"This may lead to potential slashing - do you want to proceed? (Y/N)"
		deniedText := "The historical actions database will not be deleted. No changes have been made."
		clearDBConfirmed, err = cmd.ConfirmAction(actionText, deniedText)
		if err != nil {
			return errors.Wrapf(err, "Could not clear DB in dir %s", dataDir)
		}
	}

	if clearDBConfirmed {
		valDB, err := kv.NewKVStore(ctx, dataDir, &kv.Config{})
		if err != nil {
			return errors.Wrapf(err, "Could not create DB in dir %s", dataDir)
		}
		if err := valDB.Close(); err != nil {
			return errors.Wrapf(err, "could not close DB in dir %s", dataDir)
		}

		log.Warning("Removing database")
		if err := valDB.ClearDB(); err != nil {
			return errors.Wrapf(err, "Could not clear DB in dir %s", dataDir)
		}
	}

	return nil
}
