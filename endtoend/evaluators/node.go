// Package evaluators defines functions which can peer into end to end
// tests to determine if a chain is running as required.
package evaluators

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/pkg/errors"
	types "github.com/prysmaticlabs/eth2-types"
	e2e "github.com/prysmaticlabs/prysm/endtoend/params"
	"github.com/prysmaticlabs/prysm/endtoend/policies"
	e2etypes "github.com/prysmaticlabs/prysm/endtoend/types"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Allow a very short delay after disconnecting to prevent connection refused issues.
var connTimeDelay = 50 * time.Millisecond

// PeersConnect checks all beacon nodes and returns whether they are connected to each other as peers.
var PeersConnect = e2etypes.Evaluator{
	Name:       "peers_connect_epoch_%d",
	Policy:     policies.OnEpoch(0),
	Evaluation: peersConnect,
}

// HealthzCheck pings healthz and errors if it doesn't have the expected OK status.
var HealthzCheck = e2etypes.Evaluator{
	Name:       "healthz_check_epoch_%d",
	Policy:     policies.AfterNthEpoch(0),
	Evaluation: healthzCheck,
}

// FinishedSyncing returns whether the beacon node with the given rpc port has finished syncing.
var FinishedSyncing = e2etypes.Evaluator{
	Name:       "finished_syncing_%d",
	Policy:     policies.AllEpochs,
	Evaluation: finishedSyncing,
}

// AllNodesHaveSameHead ensures all nodes have the same head epoch. Checks finality and justification as well.
// Not checking head block root as it may change irregularly for the validator connected nodes.
var AllNodesHaveSameHead = e2etypes.Evaluator{
	Name:       "all_nodes_have_same_head_%d",
	Policy:     policies.AllEpochs,
	Evaluation: allNodesHaveSameHead,
}

func healthzCheck(conns ...*grpc.ClientConn) error {
	count := len(conns)
	for i := 0; i < count; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", e2e.TestParams.BeaconNodeMetricsPort+i))
		if err != nil {
			// Continue if the connection fails, regular flake.
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			return fmt.Errorf("expected status code OK for beacon node %d, received %v with body %s", i, resp.StatusCode, body)
		}
		if err = resp.Body.Close(); err != nil {
			return err
		}
		time.Sleep(connTimeDelay)
	}

	for i := 0; i < count; i++ {
		resp, err := http.Get(fmt.Sprintf("http://localhost:%d/healthz", e2e.TestParams.ValidatorMetricsPort+i))
		if err != nil {
			// Continue if the connection fails, regular flake.
			continue
		}
		if resp.StatusCode != http.StatusOK {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			return fmt.Errorf("expected status code OK for validator client %d, received %v with body %s", i, resp.StatusCode, body)
		}
		if err = resp.Body.Close(); err != nil {
			return err
		}
		time.Sleep(connTimeDelay)
	}
	return nil
}

func peersConnect(conns ...*grpc.ClientConn) error {
	if len(conns) == 1 {
		return nil
	}
	ctx := context.Background()
	for _, conn := range conns {
		nodeClient := eth.NewNodeClient(conn)
		peersResp, err := nodeClient.ListPeers(ctx, &emptypb.Empty{})
		if err != nil {
			return err
		}
		expectedPeers := len(conns) - 1
		if expectedPeers != len(peersResp.Peers) {
			return fmt.Errorf("unexpected amount of peers, expected %d, received %d", expectedPeers, len(peersResp.Peers))
		}
		time.Sleep(connTimeDelay)
	}
	return nil
}

func finishedSyncing(conns ...*grpc.ClientConn) error {
	conn := conns[0]
	syncNodeClient := eth.NewNodeClient(conn)
	syncStatus, err := syncNodeClient.GetSyncStatus(context.Background(), &emptypb.Empty{})
	if err != nil {
		return err
	}
	if syncStatus.Syncing {
		return errors.New("expected node to have completed sync")
	}
	return nil
}

func allNodesHaveSameHead(conns ...*grpc.ClientConn) error {
	headEpochs := make([]types.Epoch, len(conns))
	justifiedRoots := make([][]byte, len(conns))
	prevJustifiedRoots := make([][]byte, len(conns))
	finalizedRoots := make([][]byte, len(conns))
	for i, conn := range conns {
		beaconClient := eth.NewBeaconChainClient(conn)
		chainHead, err := beaconClient.GetChainHead(context.Background(), &emptypb.Empty{})
		if err != nil {
			return err
		}
		headEpochs[i] = chainHead.HeadEpoch
		justifiedRoots[i] = chainHead.JustifiedBlockRoot
		prevJustifiedRoots[i] = chainHead.PreviousJustifiedBlockRoot
		finalizedRoots[i] = chainHead.FinalizedBlockRoot
		time.Sleep(connTimeDelay)
	}

	for i := 0; i < len(conns); i++ {
		if headEpochs[0] != headEpochs[i] {
			return fmt.Errorf(
				"received conflicting head epochs on node %d, expected %d, received %d",
				i,
				headEpochs[0],
				headEpochs[i],
			)
		}
		if !bytes.Equal(justifiedRoots[0], justifiedRoots[i]) {
			return fmt.Errorf(
				"received conflicting justified block roots on node %d, expected %#x, received %#x",
				i,
				justifiedRoots[0],
				justifiedRoots[i],
			)
		}
		if !bytes.Equal(prevJustifiedRoots[0], prevJustifiedRoots[i]) {
			return fmt.Errorf(
				"received conflicting previous justified block roots on node %d, expected %#x, received %#x",
				i,
				prevJustifiedRoots[0],
				prevJustifiedRoots[i],
			)
		}
		if !bytes.Equal(finalizedRoots[0], finalizedRoots[i]) {
			return fmt.Errorf(
				"received conflicting finalized epoch roots on node %d, expected %#x, received %#x",
				i,
				finalizedRoots[0],
				finalizedRoots[i],
			)
		}
	}

	return nil
}
