package powchain

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	gethTypes "github.com/ethereum/go-ethereum/core/types"
	logTest "github.com/sirupsen/logrus/hooks/test"
)

type badReader struct{}

func (b *badReader) SubscribeNewHead(ctx context.Context, ch chan<- *gethTypes.Header) (ethereum.Subscription, error) {
	return nil, errors.New("subscription has failed")
}

type goodReader struct{}

func (g *goodReader) SubscribeNewHead(ctx context.Context, ch chan<- *gethTypes.Header) (ethereum.Subscription, error) {
	return nil, nil
}

func TestNewWeb3Service(t *testing.T) {
	endpoint := "http://127.0.0.1"
	ctx := context.Background()
	if _, err := NewWeb3Service(ctx, endpoint, common.Address{}, ""); err == nil {
		t.Errorf("passing in an HTTP endpoint should throw an error, received nil")
	}
	endpoint = "ftp://127.0.0.1"
	if _, err := NewWeb3Service(ctx, endpoint, common.Address{}, ""); err == nil {
		t.Errorf("passing in a non-ws, wss, or ipc endpoint should throw an error, received nil")
	}
	endpoint = "ws://127.0.0.1"
	if _, err := NewWeb3Service(ctx, endpoint, common.Address{}, ""); err != nil {
		t.Errorf("passing in as ws endpoint should not throw error, received %v", err)
	}
	endpoint = "ipc://geth.ipc"
	if _, err := NewWeb3Service(ctx, endpoint, common.Address{}, ""); err != nil {
		t.Errorf("passing in an ipc endpoint should not throw error, received %v", err)
	}
}

func TestStart(t *testing.T) {
	hook := logTest.NewGlobal()

	endpoint := "ws://127.0.0.1"
	web3Service, err := NewWeb3Service(context.Background(), endpoint, common.Address{}, "")
	if err != nil {
		t.Fatalf("unable to setup web3 PoW chain service: %v", err)
	}

	web3Service.Start()

	msg := hook.LastEntry().Message
	want := "Cannot connect to PoW chain RPC client"
	if strings.Contains(want, msg) {
		t.Errorf("incorrect log, expected %s, got %s", want, msg)
	}
	hook.Reset()
}

func TestStop(t *testing.T) {
	hook := logTest.NewGlobal()

	endpoint := "ws://127.0.0.1"
	web3Service, err := NewWeb3Service(context.Background(), endpoint, common.Address{}, "")
	if err != nil {
		t.Fatalf("unable to setup web3 PoW chain service: %v", err)
	}

	if err := web3Service.Stop(); err != nil {
		t.Fatalf("Unable to stop web3 PoW chain service: %v", err)
	}

	msg := hook.LastEntry().Message
	want := "Stopping web3 PoW chain service"
	if msg != want {
		t.Errorf("incorrect log, expected %s, got %s", want, msg)
	}

	// The context should have been canceled.
	if web3Service.ctx.Err() == nil {
		t.Error("context was not canceled")
	}
	hook.Reset()
}

func TestBadReader(t *testing.T) {
	hook := logTest.NewGlobal()
	endpoint := "ws://127.0.0.1"
	web3Service, err := NewWeb3Service(context.Background(), endpoint, common.Address{}, "")
	if err != nil {
		t.Fatalf("unable to setup web3 PoW chain service: %v", err)
	}
	web3Service.latestPOWChainInfo(&badReader{}, web3Service.ctx.Done())
	msg := hook.LastEntry().Message
	want := "Unable to subscribe to incoming PoW chain headers: subscription has failed"
	if msg != want {
		t.Errorf("incorrect log, expected %s, got %s", want, msg)
	}
	hook.Reset()
}

func TestLatestMainchainInfo(t *testing.T) {
	endpoint := "ws://127.0.0.1"
	web3Service, err := NewWeb3Service(context.Background(), endpoint, common.Address{}, "")
	if err != nil {
		t.Fatalf("unable to setup web3 PoW chain service: %v", err)
	}

	doneChan := make(chan struct{})
	exitRoutine := make(chan bool)

	go func() {
		web3Service.latestPOWChainInfo(&goodReader{}, doneChan)
		<-exitRoutine
	}()

	header := &gethTypes.Header{Number: big.NewInt(42)}

	web3Service.headerChan <- header
	doneChan <- struct{}{}
	exitRoutine <- true

	if web3Service.blockNumber.Cmp(header.Number) != 0 {
		t.Errorf("block number not set, expected %v, got %v", header.Number, web3Service.blockNumber)
	}

	if web3Service.blockHash.Hex() != header.Hash().Hex() {
		t.Errorf("block hash not set, expected %v, got %v", header.Hash().Hex(), web3Service.blockHash.Hex())
	}
}

func TestListenVRCLog(t *testing.T) {

	endpoint := "ws://127.0.0.1"
	_, err := NewWeb3Service(context.Background(), endpoint, common.Address{}, "")
	if err != nil {
		t.Fatalf("unable to setup web3 PoW chain service: %v", err)
	}

	exitRoutine := make(chan bool)

	go func() {
		<-exitRoutine
	}()
}
