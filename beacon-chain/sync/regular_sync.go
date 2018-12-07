// Package sync defines the utilities for the beacon-chain to sync with the network.
package sync

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/proto"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/types"
	v "github.com/prysmaticlabs/prysm/beacon-chain/core/validators"
	"github.com/prysmaticlabs/prysm/beacon-chain/db"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/event"
	"github.com/prysmaticlabs/prysm/shared/p2p"
	"github.com/sirupsen/logrus"
	"go.opencensus.io/trace"
)

var log = logrus.WithField("prefix", "sync")

type chainService interface {
	IncomingBlockFeed() *event.Feed
}

type attestationService interface {
	IncomingAttestationFeed() *event.Feed
}

type queryService interface {
	IsSynced() (bool, error)
}

type p2pAPI interface {
	Subscribe(msg proto.Message, channel chan p2p.Message) event.Subscription
	Send(msg proto.Message, peer p2p.Peer)
	Broadcast(msg proto.Message)
}

// RegularSync is the gateway and the bridge between the p2p network and the local beacon chain.
// In broad terms, a new block is synced in 4 steps:
//     1. Receive a block hash from a peer
//     2. Request the block for the hash from the network
//     3. Receive the block
//     4. Forward block to the beacon service for full validation
//
//  In addition, RegularSync will handle the following responsibilities:
//     *  Decide which messages are forwarded to other peers
//     *  Filter redundant data and unwanted data
//     *  Drop peers that send invalid data
//     *  Throttle incoming requests
type RegularSync struct {
	ctx                   context.Context
	cancel                context.CancelFunc
	p2p                   p2pAPI
	chainService          chainService
	attestationService    attestationService
	queryService          queryService
	db                    *db.BeaconDB
	blockAnnouncementFeed *event.Feed
	announceBlockBuf      chan p2p.Message
	blockBuf              chan p2p.Message
	blockRequestBySlot    chan p2p.Message
	chainHeadReqBuf       chan p2p.Message
	attestationBuf        chan p2p.Message
}

// Config allows the channel's buffer sizes to be changed.
type Config struct {
	BlockAnnounceBufferSize int
	BlockBufferSize         int
	BlockRequestBufferSize  int
	AttestationBufferSize   int
	ChainHeadReqBufferSize  int
	ChainService            chainService
	AttestService           attestationService
	QueryService            queryService
	BeaconDB                *db.BeaconDB
	P2P                     p2pAPI
}

// DefaultConfig provides the default configuration for a sync service.
func DefaultConfig() *Config {
	return &Config{
		BlockAnnounceBufferSize: 100,
		BlockBufferSize:         100,
		BlockRequestBufferSize:  100,
		ChainHeadReqBufferSize:  100,
		AttestationBufferSize:   100,
	}
}

// NewRegularSyncService accepts a context and returns a new Service.
func NewRegularSyncService(ctx context.Context, cfg *Config) *RegularSync {
	ctx, cancel := context.WithCancel(ctx)
	return &RegularSync{
		ctx:                   ctx,
		cancel:                cancel,
		p2p:                   cfg.P2P,
		chainService:          cfg.ChainService,
		db:                    cfg.BeaconDB,
		attestationService:    cfg.AttestService,
		queryService:          cfg.QueryService,
		blockAnnouncementFeed: new(event.Feed),
		announceBlockBuf:      make(chan p2p.Message, cfg.BlockAnnounceBufferSize),
		blockBuf:              make(chan p2p.Message, cfg.BlockBufferSize),
		blockRequestBySlot:    make(chan p2p.Message, cfg.BlockRequestBufferSize),
		attestationBuf:        make(chan p2p.Message, cfg.AttestationBufferSize),
		chainHeadReqBuf:       make(chan p2p.Message, cfg.ChainHeadReqBufferSize),
	}
}

// IsSyncedWithNetwork queries other nodes in the network
// to determine whether or not the local chain is synced
// with the rest of the network.
// TODO(#661): Implement this method.
func (rs *RegularSync) IsSyncedWithNetwork() bool {
	return false
}

// Start begins the block processing goroutine.
func (rs *RegularSync) Start() {
	synced, err := rs.queryService.IsSynced()
	if err != nil {
		log.Error(err)
	}

	if !synced {
		log.Info("Chain state not detected starting initial sync")
		return
	}

	go rs.run()
}

// ResumeSync resumes normal sync after initial sync is complete.
func (rs *RegularSync) ResumeSync() {
	go rs.run()
}

// Stop kills the block processing goroutine, but does not wait until the goroutine exits.
func (rs *RegularSync) Stop() error {
	log.Info("Stopping service")
	rs.cancel()
	return nil
}

// BlockAnnouncementFeed returns an event feed processes can subscribe to for
// newly received, incoming p2p blocks.
func (rs *RegularSync) BlockAnnouncementFeed() *event.Feed {
	return rs.blockAnnouncementFeed
}

// run handles incoming block sync.
func (rs *RegularSync) run() {
	announceBlockSub := rs.p2p.Subscribe(&pb.BeaconBlockAnnounce{}, rs.announceBlockBuf)
	blockSub := rs.p2p.Subscribe(&pb.BeaconBlockResponse{}, rs.blockBuf)
	blockRequestSub := rs.p2p.Subscribe(&pb.BeaconBlockRequestBySlotNumber{}, rs.blockRequestBySlot)
	attestationSub := rs.p2p.Subscribe(&pb.AggregatedAttestation{}, rs.attestationBuf)
	chainHeadReqSub := rs.p2p.Subscribe(&pb.ChainHeadRequest{}, rs.chainHeadReqBuf)

	defer announceBlockSub.Unsubscribe()
	defer blockSub.Unsubscribe()
	defer blockRequestSub.Unsubscribe()
	defer chainHeadReqSub.Unsubscribe()
	defer attestationSub.Unsubscribe()

	for {
		select {
		case <-rs.ctx.Done():
			log.Debug("Exiting goroutine")
			return
		case msg := <-rs.announceBlockBuf:
			rs.receiveBlockAnnounce(msg)
		case msg := <-rs.attestationBuf:
			rs.receiveAttestation(msg)
		case msg := <-rs.blockBuf:
			rs.receiveBlock(msg)
		case msg := <-rs.blockRequestBySlot:
			rs.handleBlockRequestBySlot(msg)
		case msg := <-rs.chainHeadReqBuf:
			rs.handleChainHeadRequest(msg)
		}
	}
}

// receiveBlockHash accepts a block hash.
// New hashes are forwarded to other peers in the network (unimplemented), and
// the contents of the block are requested if the local chain doesn't have the block.
func (rs *RegularSync) receiveBlockAnnounce(msg p2p.Message) {
	ctx, receiveBlockSpan := trace.StartSpan(msg.Ctx, "receiveBlockHash")
	defer receiveBlockSpan.End()

	data := msg.Data.(*pb.BeaconBlockAnnounce)
	var h [32]byte
	copy(h[:], data.Hash[:32])

	if rs.db.HasBlock(h) {
		log.Debugf("Received a hash for a block that has already been processed: %#x", h)
		return
	}

	log.WithField("blockHash", fmt.Sprintf("%#x", h)).Debug("Received incoming block hash, requesting full block data from sender")
	// Request the full block data from peer that sent the block hash.
	_, sendBlockRequestSpan := trace.StartSpan(ctx, "sendBlockRequest")
	rs.p2p.Send(&pb.BeaconBlockRequest{Hash: h[:]}, msg.Peer)
	sendBlockRequestSpan.End()
}

// receiveBlock processes a block from the p2p layer.
func (rs *RegularSync) receiveBlock(msg p2p.Message) {
	ctx, receiveBlockSpan := trace.StartSpan(msg.Ctx, "receiveBlock")
	defer receiveBlockSpan.End()

	response := msg.Data.(*pb.BeaconBlockResponse)
	block := types.NewBlock(response.Block)
	blockHash, err := block.Hash()
	if err != nil {
		log.Errorf("Could not hash received block: %v", err)
	}

	log.Debugf("Processing response to block request: %#x", blockHash)

	if rs.db.HasBlock(blockHash) {
		log.Debug("Received a block that already exists. Exiting...")
		return
	}

	beaconState, err := rs.db.GetState()
	if err != nil {
		log.Errorf("Failed to get beacon state: %v", err)
		return
	}

	if block.SlotNumber() < beaconState.LastFinalizedSlot() {
		log.Debug("Discarding received block with a slot number smaller than the last finalized slot")
		return
	}

	// Verify attestation coming from proposer then forward block to the subscribers.
	attestation := types.NewAttestation(response.Attestation)

	proposerShardID, _, err := v.ProposerShardAndIndex(
		beaconState.ShardAndCommitteesForSlots(),
		beaconState.LastStateRecalculationSlot(),
		block.SlotNumber(),
	)
	if err != nil {
		log.Errorf("Failed to get proposer shard ID: %v", err)
		return
	}

	// TODO(#258): stubbing public key with empty 32 bytes.
	if err := attestation.VerifyProposerAttestation([32]byte{}, proposerShardID); err != nil {
		log.Errorf("Failed to verify proposer attestation: %v", err)
		return
	}

	_, sendAttestationSpan := trace.StartSpan(ctx, "sendAttestation")
	log.WithField("attestationHash", fmt.Sprintf("%#x", attestation.Key())).Debug("Sending newly received attestation to subscribers")
	rs.attestationService.IncomingAttestationFeed().Send(attestation)
	sendAttestationSpan.End()

	_, sendBlockSpan := trace.StartSpan(ctx, "sendBlock")
	log.WithField("blockHash", fmt.Sprintf("%#x", blockHash)).Debug("Sending newly received block to subscribers")
	rs.chainService.IncomingBlockFeed().Send(block)
	sendBlockSpan.End()
}

// handleBlockRequestBySlot processes a block request from the p2p layer.
// if found, the block is sent to the requesting peer.
func (rs *RegularSync) handleBlockRequestBySlot(msg p2p.Message) {
	ctx, blockRequestSpan := trace.StartSpan(msg.Ctx, "blockRequestBySlot")
	defer blockRequestSpan.End()

	request, ok := msg.Data.(*pb.BeaconBlockRequestBySlotNumber)
	if !ok {
		log.Error("Received malformed beacon block request p2p message")
		return
	}

	ctx, getBlockSpan := trace.StartSpan(ctx, "getBlockBySlot")
	block, err := rs.db.GetBlockBySlot(request.GetSlotNumber())
	getBlockSpan.End()
	if err != nil || block == nil {
		log.Errorf("Error retrieving block from db: %v", err)
		return
	}

	_, sendBlockSpan := trace.StartSpan(ctx, "sendBlock")
	log.WithField("slotNumber", fmt.Sprintf("%d", request.GetSlotNumber())).Debug("Sending requested block to peer")
	rs.p2p.Send(block.Proto(), msg.Peer)
	sendBlockSpan.End()
}

func (rs *RegularSync) handleChainHeadRequest(msg p2p.Message) {
	if _, ok := msg.Data.(*pb.ChainHeadRequest); !ok {
		log.Errorf("message is of the incorrect type")
		return
	}

	block, err := rs.db.GetChainHead()
	if err != nil {
		log.Errorf("Could not retrieve chain head %v", err)
		return
	}

	hash, err := block.Hash()
	if err != nil {
		log.Errorf("Could not hash block %v", err)
		return
	}

	req := &pb.ChainHeadResponse{
		Slot:  block.SlotNumber(),
		Hash:  hash[:],
		Block: block.Proto(),
	}

	rs.p2p.Send(req, msg.Peer)
}

// receiveAttestation accepts an broadcasted attestation from the p2p layer,
// discard the attestation if we have gotten before, send it to attestation
// service if we have not.
func (rs *RegularSync) receiveAttestation(msg p2p.Message) {
	data := msg.Data.(*pb.AggregatedAttestation)
	a := types.NewAttestation(data)
	h := a.Key()

	attestation, err := rs.db.GetAttestation(h)
	if err != nil {
		log.Errorf("Could not check for attestation in DB: %v", err)
		return
	}
	if attestation != nil {
		validatorExists := attestation.ContainsValidator(a.AttesterBitfield())
		if validatorExists {
			log.Debugf("Received attestation %#x already", h)
			return
		}
	}

	log.WithField("attestationHash", fmt.Sprintf("%#x", h)).Debug("Forwarding attestation to subscribed services")
	// Request the full block data from peer that sent the block hash.
	rs.attestationService.IncomingAttestationFeed().Send(a)
}
