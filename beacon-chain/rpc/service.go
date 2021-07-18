// Package rpc defines a gRPC server implementing the Ethereum consensus API as needed
// by validator clients and consumers of chain data.
package rpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache/depositcache"
	blockfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/block"
	opfeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/operation"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/slashings"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/synccommittee"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/voluntaryexits"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/eth/v1/beacon"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/eth/v1/debug"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/eth/v1/events"
	node "github.com/prysmaticlabs/prysm/beacon-chain/rpc/eth/v1/node"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/eth/v1/validator"
	beaconv1alpha1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/beacon"
	debugv1alpha1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/debug"
	nodev1alpha1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/node"
	validatorv1alpha1 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v1alpha1/validator"
	beaconv2 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v2/beacon"
	validatorv2 "github.com/prysmaticlabs/prysm/beacon-chain/rpc/prysm/v2/validator"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/statefetcher"
	"github.com/prysmaticlabs/prysm/beacon-chain/state/stategen"
	chainSync "github.com/prysmaticlabs/prysm/beacon-chain/sync"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pbrpc "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	ethpbv1 "github.com/prysmaticlabs/prysm/proto/eth/v1"
	ethpbv1alpha1 "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/featureconfig"
	"github.com/prysmaticlabs/prysm/shared/logutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/reflection"
)

const attestationBufferSize = 100

// Service defining an RPC server for a beacon node.
type Service struct {
	cfg                  *Config
	ctx                  context.Context
	cancel               context.CancelFunc
	listener             net.Listener
	grpcServer           *grpc.Server
	canonicalStateChan   chan *pbp2p.BeaconState
	incomingAttestation  chan *ethpbv1alpha1.Attestation
	credentialError      error
	connectedRPCClients  map[net.Addr]bool
	clientConnectionLock sync.Mutex
}

// Config options for the beacon node RPC server.
type Config struct {
	Host                    string
	Port                    string
	CertFlag                string
	KeyFlag                 string
	BeaconMonitoringHost    string
	BeaconMonitoringPort    int
	BeaconDB                db.HeadAccessDatabase
	ChainInfoFetcher        blockchain.ChainInfoFetcher
	HeadFetcher             blockchain.HeadFetcher
	CanonicalFetcher        blockchain.CanonicalFetcher
	ForkFetcher             blockchain.ForkFetcher
	FinalizationFetcher     blockchain.FinalizationFetcher
	AttestationReceiver     blockchain.AttestationReceiver
	BlockReceiver           blockchain.BlockReceiver
	POWChainService         powchain.Chain
	ChainStartFetcher       powchain.ChainStartFetcher
	GenesisTimeFetcher      blockchain.TimeFetcher
	GenesisFetcher          blockchain.GenesisFetcher
	EnableDebugRPCEndpoints bool
	MockEth1Votes           bool
	AttestationsPool        attestations.Pool
	ExitPool                voluntaryexits.PoolManager
	SlashingsPool           slashings.PoolManager
	SyncCommitteeObjectPool synccommittee.Pool
	SyncService             chainSync.Checker
	Broadcaster             p2p.Broadcaster
	PeersFetcher            p2p.PeersProvider
	PeerManager             p2p.PeerManager
	MetadataProvider        p2p.MetadataProvider
	DepositFetcher          depositcache.DepositFetcher
	PendingDepositFetcher   depositcache.PendingDepositsFetcher
	StateNotifier           statefeed.Notifier
	BlockNotifier           blockfeed.Notifier
	OperationNotifier       opfeed.Notifier
	StateGen                *stategen.State
	MaxMsgSize              int
}

// NewService instantiates a new RPC service instance that will
// be registered into a running beacon node.
func NewService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		cfg:                 cfg,
		ctx:                 ctx,
		cancel:              cancel,
		canonicalStateChan:  make(chan *pbp2p.BeaconState, params.BeaconConfig().DefaultBufferSize),
		incomingAttestation: make(chan *ethpbv1alpha1.Attestation, params.BeaconConfig().DefaultBufferSize),
		connectedRPCClients: make(map[net.Addr]bool),
	}
}

// Start the gRPC server.
func (s *Service) Start() {
	address := fmt.Sprintf("%s:%s", s.cfg.Host, s.cfg.Port)
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Errorf("Could not listen to port in Start() %s: %v", address, err)
	}
	s.listener = lis
	log.WithField("address", address).Info("gRPC server listening on port")

	opts := []grpc.ServerOption{
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
		grpc.StreamInterceptor(middleware.ChainStreamServer(
			recovery.StreamServerInterceptor(
				recovery.WithRecoveryHandlerContext(traceutil.RecoveryHandlerFunc),
			),
			grpc_prometheus.StreamServerInterceptor,
			grpc_opentracing.StreamServerInterceptor(),
			s.validatorStreamConnectionInterceptor,
		)),
		grpc.UnaryInterceptor(middleware.ChainUnaryServer(
			recovery.UnaryServerInterceptor(
				recovery.WithRecoveryHandlerContext(traceutil.RecoveryHandlerFunc),
			),
			grpc_prometheus.UnaryServerInterceptor,
			grpc_opentracing.UnaryServerInterceptor(),
			s.validatorUnaryConnectionInterceptor,
		)),
		grpc.MaxRecvMsgSize(s.cfg.MaxMsgSize),
	}
	grpc_prometheus.EnableHandlingTimeHistogram()
	if s.cfg.CertFlag != "" && s.cfg.KeyFlag != "" {
		creds, err := credentials.NewServerTLSFromFile(s.cfg.CertFlag, s.cfg.KeyFlag)
		if err != nil {
			log.WithError(err).Fatal("Could not load TLS keys")
		}
		opts = append(opts, grpc.Creds(creds))
	} else {
		log.Warn("You are using an insecure gRPC server. If you are running your beacon node and " +
			"validator on the same machines, you can ignore this message. If you want to know " +
			"how to enable secure connections, see: https://docs.prylabs.network/docs/prysm-usage/secure-grpc")
	}
	s.grpcServer = grpc.NewServer(opts...)

	validatorServer := &validatorv1alpha1.Server{
		Ctx:                    s.ctx,
		AttestationCache:       cache.NewAttestationCache(),
		AttPool:                s.cfg.AttestationsPool,
		ExitPool:               s.cfg.ExitPool,
		SyncCommitteePool:      s.cfg.SyncCommitteeObjectPool,
		HeadFetcher:            s.cfg.HeadFetcher,
		ForkFetcher:            s.cfg.ForkFetcher,
		FinalizationFetcher:    s.cfg.FinalizationFetcher,
		TimeFetcher:            s.cfg.GenesisTimeFetcher,
		CanonicalStateChan:     s.canonicalStateChan,
		BlockFetcher:           s.cfg.POWChainService,
		DepositFetcher:         s.cfg.DepositFetcher,
		ChainStartFetcher:      s.cfg.ChainStartFetcher,
		Eth1InfoFetcher:        s.cfg.POWChainService,
		SyncChecker:            s.cfg.SyncService,
		StateNotifier:          s.cfg.StateNotifier,
		BlockNotifier:          s.cfg.BlockNotifier,
		OperationNotifier:      s.cfg.OperationNotifier,
		P2P:                    s.cfg.Broadcaster,
		BlockReceiver:          s.cfg.BlockReceiver,
		MockEth1Votes:          s.cfg.MockEth1Votes,
		Eth1BlockFetcher:       s.cfg.POWChainService,
		PendingDepositsFetcher: s.cfg.PendingDepositFetcher,
		SlashingsPool:          s.cfg.SlashingsPool,
		StateGen:               s.cfg.StateGen,
	}

	validatorServerV2 := &validatorv2.Server{
		V1Server:          validatorServer,
		Ctx:               s.ctx,
		HeadFetcher:       s.cfg.HeadFetcher,
		P2P:               s.cfg.Broadcaster,
		SyncCommitteePool: s.cfg.SyncCommitteeObjectPool,
	}

	validatorServerV1 := &validator.Server{
		HeadFetcher: s.cfg.HeadFetcher,
		TimeFetcher: s.cfg.GenesisTimeFetcher,
		SyncChecker: s.cfg.SyncService,
	}

	nodeServer := &nodev1alpha1.Server{
		LogsStreamer:         logutil.NewStreamServer(),
		StreamLogsBufferSize: 1000, // Enough to handle bursts of beacon node logs for gRPC streaming.
		BeaconDB:             s.cfg.BeaconDB,
		Server:               s.grpcServer,
		SyncChecker:          s.cfg.SyncService,
		GenesisTimeFetcher:   s.cfg.GenesisTimeFetcher,
		PeersFetcher:         s.cfg.PeersFetcher,
		PeerManager:          s.cfg.PeerManager,
		GenesisFetcher:       s.cfg.GenesisFetcher,
		BeaconMonitoringHost: s.cfg.BeaconMonitoringHost,
		BeaconMonitoringPort: s.cfg.BeaconMonitoringPort,
	}
	nodeServerV1 := &node.Server{
		BeaconDB:           s.cfg.BeaconDB,
		Server:             s.grpcServer,
		SyncChecker:        s.cfg.SyncService,
		GenesisTimeFetcher: s.cfg.GenesisTimeFetcher,
		PeersFetcher:       s.cfg.PeersFetcher,
		PeerManager:        s.cfg.PeerManager,
		MetadataProvider:   s.cfg.MetadataProvider,
		HeadFetcher:        s.cfg.HeadFetcher,
	}

	beaconChainServer := &beaconv1alpha1.Server{
		Ctx:                         s.ctx,
		BeaconDB:                    s.cfg.BeaconDB,
		AttestationsPool:            s.cfg.AttestationsPool,
		SlashingsPool:               s.cfg.SlashingsPool,
		HeadFetcher:                 s.cfg.HeadFetcher,
		FinalizationFetcher:         s.cfg.FinalizationFetcher,
		CanonicalFetcher:            s.cfg.CanonicalFetcher,
		ChainStartFetcher:           s.cfg.ChainStartFetcher,
		DepositFetcher:              s.cfg.DepositFetcher,
		BlockFetcher:                s.cfg.POWChainService,
		CanonicalStateChan:          s.canonicalStateChan,
		GenesisTimeFetcher:          s.cfg.GenesisTimeFetcher,
		StateNotifier:               s.cfg.StateNotifier,
		BlockNotifier:               s.cfg.BlockNotifier,
		AttestationNotifier:         s.cfg.OperationNotifier,
		Broadcaster:                 s.cfg.Broadcaster,
		StateGen:                    s.cfg.StateGen,
		SyncChecker:                 s.cfg.SyncService,
		ReceivedAttestationsBuffer:  make(chan *ethpbv1alpha1.Attestation, attestationBufferSize),
		CollectedAttestationsBuffer: make(chan []*ethpbv1alpha1.Attestation, attestationBufferSize),
	}
	beaconChainServerV2 := &beaconv2.Server{
		Ctx:      s.ctx,
		V1Server: beaconChainServer,
	}
	beaconChainServerV1 := &beacon.Server{
		BeaconDB:           s.cfg.BeaconDB,
		AttestationsPool:   s.cfg.AttestationsPool,
		SlashingsPool:      s.cfg.SlashingsPool,
		ChainInfoFetcher:   s.cfg.ChainInfoFetcher,
		GenesisTimeFetcher: s.cfg.GenesisTimeFetcher,
		BlockNotifier:      s.cfg.BlockNotifier,
		Broadcaster:        s.cfg.Broadcaster,
		BlockReceiver:      s.cfg.BlockReceiver,
		StateGenService:    s.cfg.StateGen,
		StateFetcher: &statefetcher.StateProvider{
			BeaconDB:           s.cfg.BeaconDB,
			ChainInfoFetcher:   s.cfg.ChainInfoFetcher,
			GenesisTimeFetcher: s.cfg.GenesisTimeFetcher,
			StateGenService:    s.cfg.StateGen,
		},
		VoluntaryExitsPool: s.cfg.ExitPool,
	}
	ethpbv1alpha1.RegisterNodeServer(s.grpcServer, nodeServer)
	ethpbv1.RegisterBeaconNodeServer(s.grpcServer, nodeServerV1)
	pbrpc.RegisterHealthServer(s.grpcServer, nodeServer)
	ethpbv1alpha1.RegisterBeaconChainServer(s.grpcServer, beaconChainServer)
	ethpbv1.RegisterBeaconChainServer(s.grpcServer, beaconChainServerV1)
	ethpbv1.RegisterEventsServer(s.grpcServer, &events.Server{
		Ctx:               s.ctx,
		StateNotifier:     s.cfg.StateNotifier,
		BlockNotifier:     s.cfg.BlockNotifier,
		OperationNotifier: s.cfg.OperationNotifier,
	})
	if s.cfg.EnableDebugRPCEndpoints {
		log.Info("Enabled debug gRPC endpoints")

		debugServer := &debugv1alpha1.Server{
			GenesisTimeFetcher: s.cfg.GenesisTimeFetcher,
			BeaconDB:           s.cfg.BeaconDB,
			StateGen:           s.cfg.StateGen,
			HeadFetcher:        s.cfg.HeadFetcher,
			PeerManager:        s.cfg.PeerManager,
			PeersFetcher:       s.cfg.PeersFetcher,
		}
		debugServerV1 := &debug.Server{
			BeaconDB:    s.cfg.BeaconDB,
			HeadFetcher: s.cfg.HeadFetcher,
			StateFetcher: &statefetcher.StateProvider{
				BeaconDB:           s.cfg.BeaconDB,
				ChainInfoFetcher:   s.cfg.ChainInfoFetcher,
				GenesisTimeFetcher: s.cfg.GenesisTimeFetcher,
				StateGenService:    s.cfg.StateGen,
			},
		}
		pbrpc.RegisterDebugServer(s.grpcServer, debugServer)
		ethpbv1.RegisterBeaconDebugServer(s.grpcServer, debugServerV1)
	}

	ethpbv1alpha1.RegisterBeaconNodeValidatorServer(s.grpcServer, validatorServer)
	prysmv2.RegisterBeaconChainAltairServer(s.grpcServer, beaconChainServerV2)
	prysmv2.RegisterBeaconNodeValidatorAltairServer(s.grpcServer, validatorServerV2)
	ethpbv1.RegisterBeaconValidatorServer(s.grpcServer, validatorServerV1)
	// Register reflection service on gRPC server.
	reflection.Register(s.grpcServer)

	go func() {
		if s.listener != nil {
			if err := s.grpcServer.Serve(s.listener); err != nil {
				log.Errorf("Could not serve gRPC: %v", err)
			}
		}
	}()
}

// Stop the service.
func (s *Service) Stop() error {
	s.cancel()
	if s.listener != nil {
		s.grpcServer.GracefulStop()
		log.Debug("Initiated graceful stop of gRPC server")
	}
	return nil
}

// Status returns nil or credentialError
func (s *Service) Status() error {
	if s.cfg.SyncService.Syncing() {
		return errors.New("syncing")
	}
	if s.credentialError != nil {
		return s.credentialError
	}
	return nil
}

// Stream interceptor for new validator client connections to the beacon node.
func (s *Service) validatorStreamConnectionInterceptor(
	srv interface{},
	ss grpc.ServerStream,
	_ *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error {
	s.logNewClientConnection(ss.Context())
	return handler(srv, ss)
}

// Unary interceptor for new validator client connections to the beacon node.
func (s *Service) validatorUnaryConnectionInterceptor(
	ctx context.Context,
	req interface{},
	_ *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	s.logNewClientConnection(ctx)
	return handler(ctx, req)
}

func (s *Service) logNewClientConnection(ctx context.Context) {
	if featureconfig.Get().DisableGRPCConnectionLogs {
		return
	}
	if clientInfo, ok := peer.FromContext(ctx); ok {
		// Check if we have not yet observed this grpc client connection
		// in the running beacon node.
		s.clientConnectionLock.Lock()
		defer s.clientConnectionLock.Unlock()
		if !s.connectedRPCClients[clientInfo.Addr] {
			log.WithFields(logrus.Fields{
				"addr": clientInfo.Addr.String(),
			}).Infof("New gRPC client connected to beacon node")
			s.connectedRPCClients[clientInfo.Addr] = true
		}
	}
}
