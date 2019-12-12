// Package rpc defines the services that the beacon-chain uses to communicate via gRPC.
package rpc

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"os"

	middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	recovery "github.com/grpc-ecosystem/go-grpc-middleware/recovery"
	grpc_opentracing "github.com/grpc-ecosystem/go-grpc-middleware/tracing/opentracing"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/blockchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	"github.com/prysmaticlabs/prysm/beacon-chain/cache/depositcache"
	statefeed "github.com/prysmaticlabs/prysm/beacon-chain/core/feed/state"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	"github.com/prysmaticlabs/prysm/beacon-chain/operations/attestations"
	"github.com/prysmaticlabs/prysm/beacon-chain/p2p"
	"github.com/prysmaticlabs/prysm/beacon-chain/powchain"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/aggregator"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/beacon"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/node"
	"github.com/prysmaticlabs/prysm/beacon-chain/rpc/validator"
	"github.com/prysmaticlabs/prysm/beacon-chain/sync"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/traceutil"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/plugin/ocgrpc"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

var log logrus.FieldLogger

func init() {
	log = logrus.WithField("prefix", "rpc")
	rand.Seed(int64(os.Getpid()))
}

// Service defining an RPC server for a beacon node.
type Service struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	beaconDB              db.Database
	headFetcher           blockchain.HeadFetcher
	forkFetcher           blockchain.ForkFetcher
	finalizationFetcher   blockchain.FinalizationFetcher
	genesisTimeFetcher    blockchain.GenesisTimeFetcher
	attestationReceiver   blockchain.AttestationReceiver
	blockReceiver         blockchain.BlockReceiver
	powChainService       powchain.Chain
	chainStartFetcher     powchain.ChainStartFetcher
	mockEth1Votes         bool
	attestationsPool      attestations.Pool
	syncService           sync.Checker
	port                  string
	listener              net.Listener
	withCert              string
	withKey               string
	grpcServer            *grpc.Server
	canonicalStateChan    chan *pbp2p.BeaconState
	incomingAttestation   chan *ethpb.Attestation
	credentialError       error
	p2p                   p2p.Broadcaster
	peersFetcher          p2p.PeersProvider
	depositFetcher        depositcache.DepositFetcher
	pendingDepositFetcher depositcache.PendingDepositsFetcher
	stateNotifier         statefeed.Notifier
}

// Config options for the beacon node RPC server.
type Config struct {
	Port                  string
	CertFlag              string
	KeyFlag               string
	BeaconDB              db.Database
	HeadFetcher           blockchain.HeadFetcher
	ForkFetcher           blockchain.ForkFetcher
	FinalizationFetcher   blockchain.FinalizationFetcher
	AttestationReceiver   blockchain.AttestationReceiver
	BlockReceiver         blockchain.BlockReceiver
	POWChainService       powchain.Chain
	ChainStartFetcher     powchain.ChainStartFetcher
	GenesisTimeFetcher    blockchain.GenesisTimeFetcher
	MockEth1Votes         bool
	AttestationsPool      attestations.Pool
	SyncService           sync.Checker
	Broadcaster           p2p.Broadcaster
	PeersFetcher          p2p.PeersProvider
	DepositFetcher        depositcache.DepositFetcher
	PendingDepositFetcher depositcache.PendingDepositsFetcher
	StateNotifier         statefeed.Notifier
}

// NewService instantiates a new RPC service instance that will
// be registered into a running beacon node.
func NewService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		ctx:                   ctx,
		cancel:                cancel,
		beaconDB:              cfg.BeaconDB,
		headFetcher:           cfg.HeadFetcher,
		forkFetcher:           cfg.ForkFetcher,
		finalizationFetcher:   cfg.FinalizationFetcher,
		genesisTimeFetcher:    cfg.GenesisTimeFetcher,
		attestationReceiver:   cfg.AttestationReceiver,
		blockReceiver:         cfg.BlockReceiver,
		p2p:                   cfg.Broadcaster,
		peersFetcher:          cfg.PeersFetcher,
		powChainService:       cfg.POWChainService,
		chainStartFetcher:     cfg.ChainStartFetcher,
		mockEth1Votes:         cfg.MockEth1Votes,
		attestationsPool:      cfg.AttestationsPool,
		syncService:           cfg.SyncService,
		port:                  cfg.Port,
		withCert:              cfg.CertFlag,
		withKey:               cfg.KeyFlag,
		depositFetcher:        cfg.DepositFetcher,
		pendingDepositFetcher: cfg.PendingDepositFetcher,
		canonicalStateChan:    make(chan *pbp2p.BeaconState, params.BeaconConfig().DefaultBufferSize),
		incomingAttestation:   make(chan *ethpb.Attestation, params.BeaconConfig().DefaultBufferSize),
		stateNotifier:         cfg.StateNotifier,
	}
}

// Start the gRPC server.
func (s *Service) Start() {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		log.Errorf("Could not listen to port in Start() :%s: %v", s.port, err)
	}
	s.listener = lis
	log.WithField("port", fmt.Sprintf(":%s", s.port)).Info("RPC-API listening on port")

	opts := []grpc.ServerOption{
		grpc.StatsHandler(&ocgrpc.ServerHandler{}),
		grpc.StreamInterceptor(middleware.ChainStreamServer(
			recovery.StreamServerInterceptor(
				recovery.WithRecoveryHandlerContext(traceutil.RecoveryHandlerFunc),
			),
			grpc_prometheus.StreamServerInterceptor,
			grpc_opentracing.StreamServerInterceptor(),
		)),
		grpc.UnaryInterceptor(middleware.ChainUnaryServer(
			recovery.UnaryServerInterceptor(
				recovery.WithRecoveryHandlerContext(traceutil.RecoveryHandlerFunc),
			),
			grpc_prometheus.UnaryServerInterceptor,
			grpc_opentracing.UnaryServerInterceptor(),
		)),
	}
	// TODO(#791): Utilize a certificate for secure connections
	// between beacon nodes and validator clients.
	if s.withCert != "" && s.withKey != "" {
		creds, err := credentials.NewServerTLSFromFile(s.withCert, s.withKey)
		if err != nil {
			log.Errorf("Could not load TLS keys: %s", err)
			s.credentialError = err
		}
		opts = append(opts, grpc.Creds(creds))
	} else {
		log.Warn("You are using an insecure gRPC connection! Provide a certificate and key to connect securely")
	}
	s.grpcServer = grpc.NewServer(opts...)
	validatorServer := &validator.Server{
		Ctx:                    s.ctx,
		BeaconDB:               s.beaconDB,
		AttestationCache:       cache.NewAttestationCache(),
		AttPool:                s.attestationsPool,
		HeadFetcher:            s.headFetcher,
		ForkFetcher:            s.forkFetcher,
		CanonicalStateChan:     s.canonicalStateChan,
		BlockFetcher:           s.powChainService,
		DepositFetcher:         s.depositFetcher,
		ChainStartFetcher:      s.chainStartFetcher,
		Eth1InfoFetcher:        s.powChainService,
		SyncChecker:            s.syncService,
		StateNotifier:          s.stateNotifier,
		P2P:                    s.p2p,
		BlockReceiver:          s.blockReceiver,
		MockEth1Votes:          s.mockEth1Votes,
		Eth1BlockFetcher:       s.powChainService,
		PendingDepositsFetcher: s.pendingDepositFetcher,
	}
	nodeServer := &node.Server{
		BeaconDB:           s.beaconDB,
		Server:             s.grpcServer,
		SyncChecker:        s.syncService,
		GenesisTimeFetcher: s.genesisTimeFetcher,
		PeersFetcher:       s.peersFetcher,
	}
	beaconChainServer := &beacon.Server{
		Ctx:                 s.ctx,
		BeaconDB:            s.beaconDB,
		Pool:                s.attestationsPool,
		HeadFetcher:         s.headFetcher,
		FinalizationFetcher: s.finalizationFetcher,
		ChainStartFetcher:   s.chainStartFetcher,
		CanonicalStateChan:  s.canonicalStateChan,
		StateNotifier:       s.stateNotifier,
	}
	aggregatorServer := &aggregator.Server{
		BeaconDB:    s.beaconDB,
		HeadFetcher: s.headFetcher,
		SyncChecker: s.syncService,
	}
	pb.RegisterAggregatorServiceServer(s.grpcServer, aggregatorServer)
	ethpb.RegisterNodeServer(s.grpcServer, nodeServer)
	ethpb.RegisterBeaconChainServer(s.grpcServer, beaconChainServer)
	ethpb.RegisterBeaconNodeValidatorServer(s.grpcServer, validatorServer)

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
	if s.credentialError != nil {
		return s.credentialError
	}
	return nil
}
