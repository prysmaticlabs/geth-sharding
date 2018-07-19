package node

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/ethereum/go-ethereum/common"
	"github.com/prysmaticlabs/geth-sharding/beacon-chain/blockchain"
	"github.com/prysmaticlabs/geth-sharding/beacon-chain/database"
	"github.com/prysmaticlabs/geth-sharding/beacon-chain/powchain"
	"github.com/prysmaticlabs/geth-sharding/beacon-chain/utils"
	"github.com/prysmaticlabs/geth-sharding/shared"
	"github.com/prysmaticlabs/geth-sharding/shared/cmd"
	"github.com/prysmaticlabs/geth-sharding/shared/debug"
	log "github.com/sirupsen/logrus"
	"github.com/urfave/cli"
)

var beaconChainDBName = "beaconchaindata"

// BeaconNode defines a struct that handles the services running a random beacon chain
// full PoS node. It handles the lifecycle of the entire system and registers
// services to a service registry.
type BeaconNode struct {
	ctx      *cli.Context
	services *shared.ServiceRegistry
	lock     sync.RWMutex
	stop     chan struct{} // Channel to wait for termination notifications.
}

// New creates a new node instance, sets up configuration options, and registers
// every required service to the node.
func New(ctx *cli.Context) (*BeaconNode, error) {
	registry := shared.NewServiceRegistry()

	beacon := &BeaconNode{
		ctx:      ctx,
		services: registry,
		stop:     make(chan struct{}),
	}

	path := ctx.GlobalString(cmd.DataDirFlag.Name)
	if err := beacon.registerBeaconDB(path); err != nil {
		return nil, err
	}

	if err := beacon.registerBlockchainService(); err != nil {
		return nil, err
	}

	if err := beacon.registerPOWChainService(); err != nil {
		return nil, err
	}

	return beacon, nil
}

// Start the BeaconNode and kicks off every registered service.
func (b *BeaconNode) Start() {
	b.lock.Lock()

	log.Info("Starting beacon node")

	b.services.StartAll()

	stop := b.stop
	b.lock.Unlock()

	go func() {
		sigc := make(chan os.Signal, 1)
		signal.Notify(sigc, syscall.SIGINT, syscall.SIGTERM)
		defer signal.Stop(sigc)
		<-sigc
		log.Info("Got interrupt, shutting down...")
		go b.Close()
		for i := 10; i > 0; i-- {
			<-sigc
			if i > 1 {
				log.Info("Already shutting down, interrupt more to panic.", "times", i-1)
			}
		}
		debug.Exit() // Ensure trace and CPU profile data are flushed.
		panic("Panic closing the beacon node")
	}()

	// Wait for stop channel to be closed.
	<-stop
}

// Close handles graceful shutdown of the system.
func (b *BeaconNode) Close() {
	b.lock.Lock()
	defer b.lock.Unlock()

	b.services.StopAll()
	log.Info("Stopping beacon node")
	close(b.stop)
}

func (b *BeaconNode) registerBeaconDB(path string) error {
	config := &database.BeaconDBConfig{DataDir: path, Name: beaconChainDBName, InMemory: false}
	beaconDB, err := database.NewBeaconDB(config)
	if err != nil {
		return fmt.Errorf("could not register beaconDB service: %v", err)
	}
	return b.services.RegisterService(beaconDB)
}

func (b *BeaconNode) registerBlockchainService() error {
	var beaconDB *database.BeaconDB
	if err := b.services.FetchService(&beaconDB); err != nil {
		return err
	}

	blockchainService, err := blockchain.NewChainService(context.TODO(), beaconDB)
	if err != nil {
		return fmt.Errorf("could not register blockchain service: %v", err)
	}
	return b.services.RegisterService(blockchainService)
}

func (b *BeaconNode) registerPOWChainService() error {
	web3Service, err := powchain.NewWeb3Service(context.TODO(), &powchain.Web3ServiceConfig{
		Endpoint: b.ctx.GlobalString(utils.Web3ProviderFlag.Name),
		Pubkey:   b.ctx.GlobalString(utils.PubKeyFlag.Name),
		VrcAddr:  common.HexToAddress(b.ctx.GlobalString(utils.VrcContractFlag.Name)),
	})
	if err != nil {
		return fmt.Errorf("could not register proof-of-work chain web3Service: %v", err)
	}
	return b.services.RegisterService(web3Service)
}
