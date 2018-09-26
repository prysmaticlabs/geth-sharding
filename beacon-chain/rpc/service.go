// Package rpc defines the services that the beacon-chain uses to communicate via gRPC.
package rpc

import (
	"context"
	"fmt"
	"net"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/prysmaticlabs/prysm/beacon-chain/casper"
	"github.com/prysmaticlabs/prysm/beacon-chain/types"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var log = logrus.WithField("prefix", "rpc")

// canonicalFetcher defines a struct with methods that can be
// called on-demand to fetch the latest canonical head
// and crystallized state as well as methods that stream
// latest canonical head events to clients
// These functions are called by a validator client upon
// establishing an initial connection to a beacon node via gRPC.
type canonicalFetcher interface {
	// These methods can be called on-demand by a validator
	// to fetch canonical head and state.
	CanonicalHead() (*types.Block, error)
	CanonicalCrystallizedState() *types.CrystallizedState
	// These methods are not called on-demand by a validator
	// but instead streamed to connected validators every
	// time the canonical head changes in the chain service.
	CanonicalBlockFeed() *event.Feed
	CanonicalCrystallizedStateFeed() *event.Feed
}

type chainService interface {
	IncomingBlockFeed() *event.Feed
	CurrentCrystallizedState() *types.CrystallizedState
}

type attestationService interface {
	IncomingAttestationFeed() *event.Feed
}

type powChainService interface {
	LatestBlockHash() common.Hash
}

// Service defining an RPC server for a beacon node.
type Service struct {
	ctx                 context.Context
	cancel              context.CancelFunc
	fetcher             canonicalFetcher
	chainService        chainService
	powChainService     powChainService
	attestationService  attestationService
	port                string
	listener            net.Listener
	withCert            string
	withKey             string
	grpcServer          *grpc.Server
	canonicalBlockChan  chan *types.Block
	canonicalStateChan  chan *types.CrystallizedState
	incomingAttestation chan *types.Attestation
	devMode             bool
}

// Config options for the beacon node RPC server.
type Config struct {
	Port               string
	CertFlag           string
	KeyFlag            string
	SubscriptionBuf    int
	CanonicalFetcher   canonicalFetcher
	ChainService       chainService
	POWChainService    powChainService
	AttestationService attestationService
	DevMode            bool
}

// NewRPCService creates a new instance of a struct implementing the BeaconServiceServer
// interface.
func NewRPCService(ctx context.Context, cfg *Config) *Service {
	ctx, cancel := context.WithCancel(ctx)
	return &Service{
		ctx:                 ctx,
		cancel:              cancel,
		fetcher:             cfg.CanonicalFetcher,
		chainService:        cfg.ChainService,
		powChainService:     cfg.POWChainService,
		attestationService:  cfg.AttestationService,
		port:                cfg.Port,
		withCert:            cfg.CertFlag,
		withKey:             cfg.KeyFlag,
		canonicalBlockChan:  make(chan *types.Block, cfg.SubscriptionBuf),
		canonicalStateChan:  make(chan *types.CrystallizedState, cfg.SubscriptionBuf),
		incomingAttestation: make(chan *types.Attestation, cfg.SubscriptionBuf),
		devMode:             cfg.DevMode,
	}
}

// Start the gRPC server.
func (s *Service) Start() {
	log.Info("Starting service")

	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", s.port))
	if err != nil {
		log.Errorf("Could not listen to port :%s: %v", s.port, err)
		return
	}
	s.listener = lis
	log.Infof("RPC server listening on port :%s", s.port)

	if s.withCert != "" && s.withKey != "" {
		creds, err := credentials.NewServerTLSFromFile(s.withCert, s.withKey)
		if err != nil {
			log.Errorf("could not load TLS keys: %s", err)
		}
		s.grpcServer = grpc.NewServer(grpc.Creds(creds))
	} else {
		log.Warn("You are using an insecure gRPC connection! Please provide a certificate and key to use a secure connection")
		s.grpcServer = grpc.NewServer()
	}

	pb.RegisterBeaconServiceServer(s.grpcServer, s)
	pb.RegisterProposerServiceServer(s.grpcServer, s)
	go func() {
		err = s.grpcServer.Serve(lis)
		if err != nil {
			log.Errorf("Could not serve gRPC: %v", err)
		}
	}()
}

// Stop the service.
func (s *Service) Stop() error {
	log.Info("Stopping service")
	s.cancel()
	if s.listener != nil {
		s.grpcServer.GracefulStop()
		log.Debug("Initiated graceful stop of gRPC server")
	}
	return nil
}

// CanonicalHead of the current beacon chain. This method is requested on-demand
// by a validator when it is their time to propose or attest.
func (s *Service) CanonicalHead(ctx context.Context, req *empty.Empty) (*pbp2p.BeaconBlock, error) {
	block, err := s.fetcher.CanonicalHead()
	if err != nil {
		return nil, fmt.Errorf("could not get canonical head block: %v", err)
	}
	return block.Proto(), nil
}

// GenesisTimeAndCanonicalState returns the genesis timestamp and crystallized state
// determined as canonical. Validator clients send this request
// once upon establishing a connection to the beacon node in order to determine
// their role and assigned slot initially and setup an internal ticker.
func (s *Service) GenesisTimeAndCanonicalState(ctx context.Context, req *empty.Empty) (*pb.GenesisTimeAndStateResponse, error) {
	genesis := types.NewGenesisBlock([32]byte{}, [32]byte{})
	crystallized := s.fetcher.CanonicalCrystallizedState()
	return &pb.GenesisTimeAndStateResponse{
		GenesisTimestamp:        genesis.Proto().GetTimestamp(),
		LatestCrystallizedState: crystallized.Proto(),
	}, nil
}

// ProposeBlock is called by a proposer in a sharding validator and a full beacon node
// sends the request into a beacon block that can then be included in a canonical chain.
func (s *Service) ProposeBlock(ctx context.Context, req *pb.ProposeRequest) (*pb.ProposeResponse, error) {
	var powChainHash common.Hash
	if s.devMode {
		powChainHash = common.BytesToHash([]byte("stub"))
	} else {
		powChainHash = s.powChainService.LatestBlockHash()
	}
	data := &pbp2p.BeaconBlock{
		SlotNumber:  req.GetSlotNumber(),
		PowChainRef: powChainHash[:],
		ParentHash:  req.GetParentHash(),
		Timestamp:   req.GetTimestamp(),
	}
	block := types.NewBlock(data)
	h, err := block.Hash()
	if err != nil {
		return nil, fmt.Errorf("could not hash block: %v", err)
	}
	log.WithField("blockHash", fmt.Sprintf("0x%x", h)).Debugf("Block proposal received via RPC")
	// We relay the received block from the proposer to the chain service for processing.
	s.chainService.IncomingBlockFeed().Send(block)
	return &pb.ProposeResponse{BlockHash: h[:]}, nil
}

// AttestHead is a function called by an attester in a sharding validator to vote
// on a block.
func (s *Service) AttestHead(ctx context.Context, req *pb.AttestRequest) (*pb.AttestResponse, error) {
	attestation := types.NewAttestation(req.Attestation)
	h, err := attestation.Hash()
	if err != nil {
		return nil, fmt.Errorf("could not hash attestation: %v", err)
	}

	// Relays the attestation to chain service.
	s.attestationService.IncomingAttestationFeed().Send(attestation)

	return &pb.AttestResponse{AttestationHash: h[:]}, nil
}

// LatestCrystallizedState streams the latest beacon crystallized state.
func (s *Service) LatestCrystallizedState(req *empty.Empty, stream pb.BeaconService_LatestCrystallizedStateServer) error {
	// Right now, this streams every newly created crystallized state but should only
	// stream canonical states.
	sub := s.fetcher.CanonicalCrystallizedStateFeed().Subscribe(s.canonicalStateChan)
	defer sub.Unsubscribe()
	for {
		select {
		case state := <-s.canonicalStateChan:
			log.Info("Sending crystallized state to RPC clients")
			if err := stream.Send(state.Proto()); err != nil {
				return err
			}
		case <-sub.Err():
			log.Debug("Subscriber closed, exiting goroutine")
			return nil
		case <-s.ctx.Done():
			log.Debug("RPC context closed, exiting goroutine")
			return nil
		}
	}
}

// ValidatorAssignment streams validator assignments every slot to clients that request
// to watch a subset of public keys in the CrystallizedState's active validator set.
func (s *Service) ValidatorAssignment(req *pb.ValidatorAssignmentRequest, stream pb.ValidatorService_ValidatorAssignmentServer) error {
	return nil
}

// ValidatorShardID is called by a validator to get the shard ID of where it's suppose
// to proposer or attest.
func (s *Service) ValidatorShardID(ctx context.Context, req *pb.PublicKey) (*pb.ShardIDResponse, error) {
	cState := s.chainService.CurrentCrystallizedState()

	shardID, err := casper.ValidatorShardID(
		req.PublicKey,
		cState.CurrentDynasty(),
		cState.Validators(),
		cState.ShardAndCommitteesForSlots(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not get validator shard ID: %v", err)
	}

	return &pb.ShardIDResponse{ShardId: shardID}, nil
}

// ValidatorSlotAndResponsibility fetches a validator's assigned slot number
// and whether it should act as a proposer/attester.
func (s *Service) ValidatorSlotAndResponsibility(ctx context.Context, req *pb.PublicKey) (*pb.SlotResponsibilityResponse, error) {
	cState := s.chainService.CurrentCrystallizedState()

	slot, responsibility, err := casper.ValidatorSlotAndResponsibility(
		req.PublicKey,
		cState.CurrentDynasty(),
		cState.Validators(),
		cState.ShardAndCommitteesForSlots(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not get validator slot for attester/propose: %v", err)
	}

	var role pb.ValidatorRole
	if responsibility == "proposer" {
		role = pb.ValidatorRole_PROPOSER
	} else if responsibility == "attester" {
		role = pb.ValidatorRole_ATTESTER
	}

	return &pb.SlotResponsibilityResponse{Slot: slot, Role: role}, nil
}

// ValidatorIndex is called by a validator to get its index location that corresponds
// to the attestation bit fields.
func (s *Service) ValidatorIndex(ctx context.Context, req *pb.PublicKey) (*pb.IndexResponse, error) {
	cState := s.chainService.CurrentCrystallizedState()

	index, err := casper.ValidatorIndex(
		req.PublicKey,
		cState.CurrentDynasty(),
		cState.Validators(),
	)
	if err != nil {
		return nil, fmt.Errorf("could not get validator index: %v", err)
	}

	return &pb.IndexResponse{Index: index}, nil
}

// LatestAttestation streams the latest processed attestations to the rpc clients.
func (s *Service) LatestAttestation(req *empty.Empty, stream pb.BeaconService_LatestAttestationServer) error {
	sub := s.attestationService.IncomingAttestationFeed().Subscribe(s.incomingAttestation)
	defer sub.Unsubscribe()

	for {
		select {
		case attestation := <-s.incomingAttestation:
			log.Info("Sending attestation to RPC clients")
			if err := stream.Send(attestation.Proto()); err != nil {
				return err
			}
		case <-sub.Err():
			log.Debug("Subscriber closed, exiting goroutine")
			return nil
		case <-s.ctx.Done():
			log.Debug("RPC context closed, exiting goroutine")
			return nil
		}
	}
}
