package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/params"
	recaptcha "github.com/prestonvanloon/go-recaptcha"
	faucetpb "github.com/prysmaticlabs/prysm/proto/faucet"
	"github.com/prysmaticlabs/prysm/shared/roughtime"
	"google.golang.org/grpc/metadata"
)

const ipLimit = 3

var fundingAmount = big.NewInt(3.5 * params.Ether)
var funded = make(map[string]bool)
var ipCounter = make(map[string]int)
var fundingLock sync.Mutex

type faucetServer struct {
	r        recaptcha.Recaptcha
	client   *ethclient.Client
	funder   common.Address
	pk       *ecdsa.PrivateKey
	minScore float64
}

func newFaucetServer(
	r recaptcha.Recaptcha,
	rpcPath string,
	funderPrivateKey string,
	minScore float64,
) *faucetServer {
	client, err := ethclient.DialContext(context.Background(), rpcPath)
	if err != nil {
		panic(err)
	}

	pk, err := crypto.HexToECDSA(funderPrivateKey)
	if err != nil {
		panic(err)
	}

	funder := crypto.PubkeyToAddress(pk.PublicKey)

	bal, err := client.BalanceAt(context.Background(), funder, nil)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Funder is %s\n", funder.Hex())
	fmt.Printf("Funder has %d\n", bal)

	return &faucetServer{
		r:        r,
		client:   client,
		funder:   funder,
		pk:       pk,
		minScore: minScore,
	}
}

func (s *faucetServer) verifyRecaptcha(peer string, req *faucetpb.FundingRequest) error {
	fmt.Printf("Sending captcha request for peer %s\n", peer)

	rr, err := s.r.Check(peer, req.RecaptchaResponse)
	if err != nil {
		return err
	}
	if !rr.Success {
		fmt.Printf("Unsuccessful recaptcha request. Error codes: %+v\n", rr.ErrorCodes)
		return errors.New("failed")
	}
	if rr.Score < s.minScore {
		return fmt.Errorf("recaptcha score too low (%f)", rr.Score)
	}
	if roughtime.Now().After(rr.ChallengeTS.Add(2 * time.Minute)) {
		return errors.New("captcha challenge too old")
	}
	if rr.Action != req.WalletAddress {
		return fmt.Errorf("action was %s, wanted %s", rr.Action, req.WalletAddress)
	}
	if !strings.HasSuffix(rr.Hostname, "prylabs.net") && !strings.HasSuffix(rr.Hostname, "prylabs.network") {
		return fmt.Errorf("expected hostname (%s) to end in prylabs.net", rr.Hostname)
	}

	return nil
}

// RequestFunds from the ethereum 1.x faucet. Requires a valid captcha
// response.
func (s *faucetServer) RequestFunds(ctx context.Context, req *faucetpb.FundingRequest) (*faucetpb.FundingResponse, error) {
	peer, err := s.getPeer(ctx)
	if err != nil {
		fmt.Printf("peer failure %v\n", err)
		return &faucetpb.FundingResponse{Error: "peer error"}, nil
	}

	if err := s.verifyRecaptcha(peer, req); err != nil {
		fmt.Printf("Recaptcha failure %v\n", err)
		return &faucetpb.FundingResponse{Error: "recaptcha error"}, nil
	}

	fundingLock.Lock()
	exceedPeerLimit := ipCounter[peer] >= ipLimit
	if funded[req.WalletAddress] || exceedPeerLimit {
		if exceedPeerLimit {
			fmt.Printf("peer %s trying to get funded despite being over peer limit\n", peer)
		}
		fundingLock.Unlock()
		return &faucetpb.FundingResponse{Error: "funded too recently"}, nil
	}
	funded[req.WalletAddress] = true
	fundingLock.Unlock()

	txHash, err := s.fundAndWait(common.HexToAddress(req.WalletAddress))
	if err != nil {
		return &faucetpb.FundingResponse{Error: fmt.Sprintf("Failed to send transaction %v", err)}, nil
	}
	fundingLock.Lock()
	ipCounter[peer]++
	fundingLock.Unlock()

	fmt.Printf("Funded with TX %s\n", txHash)

	return &faucetpb.FundingResponse{
		Amount:          fundingAmount.String(),
		TransactionHash: txHash,
	}, nil
}

func (s *faucetServer) fundAndWait(to common.Address) (string, error) {
	nonce := uint64(0)
	nonce, err := s.client.PendingNonceAt(context.Background(), s.funder)
	if err != nil {
		return "", err
	}

	tx := types.NewTransaction(nonce, to, fundingAmount, 40000, big.NewInt(1*params.GWei), nil /*data*/)

	tx, err = types.SignTx(tx, types.NewEIP155Signer(big.NewInt(5)), s.pk)
	if err != nil {
		return "", err
	}

	if err := s.client.SendTransaction(context.Background(), tx); err != nil {
		return "", err
	}

	// Wait for contract to mine
	for pending := true; pending; _, pending, err = s.client.TransactionByHash(context.Background(), tx.Hash()) {
		if err != nil {
			log.Fatal(err)
		}
		time.Sleep(1 * time.Second)
	}

	return tx.Hash().Hex(), nil
}

func (s *faucetServer) getPeer(ctx context.Context) (string, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md.Get("x-forwarded-for")) < 1 {
		return "", errors.New("metadata not ok")
	}
	peer := md.Get("x-forwarded-for")[0]
	return peer, nil
}
