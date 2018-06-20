// Package proposer defines all relevant functionality for a Proposer actor
// within the minimal sharding protocol.
package proposer

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/sharding/database"
	"github.com/ethereum/go-ethereum/sharding/mainchain"
	"github.com/ethereum/go-ethereum/sharding/p2p"
	"github.com/ethereum/go-ethereum/sharding/params"
	"github.com/ethereum/go-ethereum/sharding/txpool"
)

// Proposer holds functionality required to run a collation proposer
// in a sharded system. Must satisfy the Service interface defined in
// sharding/service.go.
type Proposer struct {
	config       *params.Config
	client       *mainchain.SMCClient
	p2p          *p2p.Server
	txpool       *txpool.TXPool
	txpoolSub    event.Subscription
	shardChainDb *database.ShardDB
	shardID      int
	ctx          context.Context
	cancel       context.CancelFunc
}

// NewProposer creates a struct instance of a proposer service.
// It will have access to a mainchain client, a p2p network,
// and a shard transaction pool.
func NewProposer(config *params.Config, client *mainchain.SMCClient, p2p *p2p.Server, txpool *txpool.TXPool, shardChainDb *database.ShardDB, shardID int) (*Proposer, error) {
	ctx, cancel := context.WithCancel(context.Background())
	return &Proposer{
		config,
		client,
		p2p,
		txpool,
		nil,
		shardChainDb,
		shardID,
		ctx,
		cancel}, nil
}

// Start the main loop for proposing collations.
func (p *Proposer) Start() {
	log.Info(fmt.Sprintf("Starting proposer service in shard %d", p.shardID))
	go p.proposeCollations()
}

// Stop the main loop for proposing collations.
func (p *Proposer) Stop() error {
	log.Info(fmt.Sprintf("Stopping proposer service in shard %d", p.shardID))
	defer p.cancel()
	p.txpoolSub.Unsubscribe()
	return nil
}

// proposeCollations listens to the transaction feed and submits collations over an interval.
func (p *Proposer) proposeCollations() {
	requests := make(chan *types.Transaction)
	p.txpoolSub = p.txpool.TransactionsFeed().Subscribe(requests)

	for {
		select {
		case tx := <-requests:
			log.Info(fmt.Sprintf("Received transaction: %x", tx.Hash()))
			if err := p.createCollation(p.ctx, []*types.Transaction{tx}); err != nil {
				log.Error(fmt.Sprintf("Create collation failed: %v", err))
			}
		case <-p.ctx.Done():
			log.Error("Proposer context closed, exiting goroutine")
			return
		case err := <-p.txpoolSub.Err():
			log.Error(fmt.Sprintf("Subscriber closed: %v", err))
			return
		}
	}
}

func (p *Proposer) createCollation(ctx context.Context, txs []*types.Transaction) error {
	// Get current block number.
	blockNumber, err := p.client.ChainReader().BlockByNumber(ctx, nil)
	if err != nil {
		return err
	}
	period := new(big.Int).Div(blockNumber.Number(), big.NewInt(p.config.PeriodLength))

	// Create collation.
	collation, err := createCollation(p.client, p.client.Account(), p.client, big.NewInt(int64(p.shardID)), period, txs)
	if err != nil {
		return err
	}

	// Check SMC if we can submit header before addHeader
	canAdd, err := checkHeaderAdded(p.client, big.NewInt(int64(p.shardID)), period)
	if err != nil {
		return err
	}
	if canAdd {
		addHeader(p.client, collation)
	}

	return nil
}
