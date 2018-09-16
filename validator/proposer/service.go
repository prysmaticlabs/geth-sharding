// Package proposer defines all relevant functionality for a Proposer actor
// within Ethereum 2.0.
package proposer

import (
	"bytes"
	"context"
	"sync"

	"github.com/ethereum/go-ethereum/event"
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/rpc/v1"
	"github.com/sirupsen/logrus"
	blake2b "golang.org/x/crypto/blake2b"
)

var log = logrus.WithField("prefix", "proposer")

type rpcClientService interface {
	ProposerServiceClient() pb.ProposerServiceClient
}

type assignmentAnnouncer interface {
	ProposerAssignmentFeed() *event.Feed
}

type rpcAttestationService interface {
	ProcessedAttestationFeed() *event.Feed
}

// Proposer holds functionality required to run a block proposer
// in Ethereum 2.0. Must satisfy the Service interface defined in
// sharding/service.go.
type Proposer struct {
	ctx                context.Context
	cancel             context.CancelFunc
	assigner           assignmentAnnouncer
	rpcClientService   rpcClientService
	assignmentChan     chan *pbp2p.BeaconBlock
	attestationService rpcAttestationService
	attestationChan    chan *pbp2p.AggregatedAttestation
	pendingAttestation []*pbp2p.AggregatedAttestation
	mutex              *sync.Mutex
}

// Config options for proposer service.
type Config struct {
	AssignmentBuf         int
	AttestationBufferSize int
	Assigner              assignmentAnnouncer
	AttesterFeed          rpcAttestationService
	Client                rpcClientService
}

// NewProposer creates a new attester instance.
func NewProposer(ctx context.Context, cfg *Config) *Proposer {
	ctx, cancel := context.WithCancel(ctx)
	return &Proposer{
		ctx:                ctx,
		cancel:             cancel,
		assigner:           cfg.Assigner,
		rpcClientService:   cfg.Client,
		attestationService: cfg.AttesterFeed,
		assignmentChan:     make(chan *pbp2p.BeaconBlock, cfg.AssignmentBuf),
		attestationChan:    make(chan *pbp2p.AggregatedAttestation, cfg.AttestationBufferSize),
		pendingAttestation: make([]*pbp2p.AggregatedAttestation, 0),
		mutex:              &sync.Mutex{},
	}
}

// Start the main routine for a proposer.
func (p *Proposer) Start() {
	log.Info("Starting service")
	client := p.rpcClientService.ProposerServiceClient()

	go p.run(p.ctx.Done(), client)
	go p.processAttestation(p.ctx.Done())

}

// Stop the main loop.
func (p *Proposer) Stop() error {
	defer p.cancel()
	log.Info("Stopping service")
	return nil
}

// DoesAttestationExist checks if an attester has already attested to a block.
func (p *Proposer) DoesAttestationExist(attestation *pbp2p.AggregatedAttestation) bool {
	exists := false
	for _, record := range p.pendingAttestation {
		if bytes.Equal(record.GetAttesterBitfield(), attestation.GetAttesterBitfield()) {
			exists = true
			break
		}
	}
	return exists
}

// AddPendingAttestation adds a pending attestation to the memory so that it can be included
// in the next proposed block.
func (p *Proposer) AddPendingAttestation(attestation *pbp2p.AggregatedAttestation) {
	p.pendingAttestation = append(p.pendingAttestation, attestation)
}

// AggregateAllSignatures aggregates all the signatures of the attesters. This is currently a
// stub for now till BLS/other signature schemes are implemented.
func (p *Proposer) AggregateAllSignatures(attestations []*pbp2p.AggregatedAttestation) []uint32 {
	// TODO: Implement Signature Aggregation.
	return []uint32{}
}

// GenerateBitmask creates the attestation bitmask from all the attester bitfields in the
// attestation records.
func (p *Proposer) GenerateBitmask(attestations []*pbp2p.AggregatedAttestation) []byte {
	// TODO: Implement bitmask where all attesters bitfields are aggregated.
	return []byte{}
}

// processAttestation processes incoming broadcasted attestations from the beacon node.
func (p *Proposer) processAttestation(done <-chan struct{}) {
	attestationSub := p.attestationService.ProcessedAttestationFeed().Subscribe(p.attestationChan)
	defer attestationSub.Unsubscribe()

	for {
		select {
		case <-done:
			log.Debug("Proposer context closed, exiting goroutine")
			return
		case attestationRecord := <-p.attestationChan:

			attestationExists := p.DoesAttestationExist(attestationRecord)
			if !attestationExists {
				p.AddPendingAttestation(attestationRecord)
				log.Info("Attestation stored in memory")
			}
		}

	}
}

// run the main event loop that listens for a proposer assignment.
func (p *Proposer) run(done <-chan struct{}, client pb.ProposerServiceClient) {
	sub := p.assigner.ProposerAssignmentFeed().Subscribe(p.assignmentChan)
	defer sub.Unsubscribe()

	for {
		select {
		case <-done:
			log.Debug("Proposer context closed, exiting goroutine")
			return

		// TODO: On the beacon node side, calculate active and crystallized and update the
		// active/crystallize state hash values in the proposed block.

		// When we receive an assignment on a slot, we leverage the fields
		// from the latest canonical beacon block to perform a proposal responsibility.
		case latestBeaconBlock := <-p.assignmentChan:
			log.Info("Performing proposer responsibility")

			// Extract the hash of the latest beacon block to use as parent hash in
			// the proposal.
			data, err := proto.Marshal(latestBeaconBlock)
			if err != nil {
				log.Errorf("Could not marshal latest beacon block: %v", err)
				continue
			}
			latestBlockHash := blake2b.Sum512(data)

			// To prevent any unaccounted attestations from being added.
			p.mutex.Lock()

			agSig := p.AggregateAllSignatures(p.pendingAttestation)
			bitmask := p.GenerateBitmask(p.pendingAttestation)

			// TODO: Implement real proposals with randao reveals and attestation fields.
			req := &pb.ProposeRequest{
				ParentHash:              latestBlockHash[:],
				SlotNumber:              latestBeaconBlock.GetSlotNumber() + 1,
				RandaoReveal:            []byte{},
				AttestationBitmask:      bitmask,
				AttestationAggregateSig: agSig,
				Timestamp:               ptypes.TimestampNow(),
			}

			res, err := client.ProposeBlock(p.ctx, req)
			if err != nil {
				log.Errorf("Could not propose block: %v", err)
				continue
			}

			log.Infof("Block proposed successfully with hash 0x%x", res.BlockHash)
			p.pendingAttestation = nil
			p.mutex.Unlock()
		}
	}
}
