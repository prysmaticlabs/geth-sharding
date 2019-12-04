package blocks_test

import (
	"fmt"
	"testing"

	ethpb "github.com/prysmaticlabs/ethereumapis/eth/v1alpha1"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/blocks"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
)

func TestEth1DataHasEnoughSupport(t *testing.T) {
	tests := []struct {
		stateVotes         []*ethpb.Eth1Data
		data               *ethpb.Eth1Data
		hasSupport         bool
		votingPeriodLength uint64
	}{
		{
			stateVotes: []*ethpb.Eth1Data{
				{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				},
			},
			data: &ethpb.Eth1Data{
				DepositCount: 1,
				DepositRoot:  []byte("root"),
			},
			hasSupport:         true,
			votingPeriodLength: 7,
		}, {
			stateVotes: []*ethpb.Eth1Data{
				{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				},
			},
			data: &ethpb.Eth1Data{
				DepositCount: 1,
				DepositRoot:  []byte("root"),
			},
			hasSupport:         false,
			votingPeriodLength: 8,
		}, {
			stateVotes: []*ethpb.Eth1Data{
				{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, {
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				},
			},
			data: &ethpb.Eth1Data{
				DepositCount: 1,
				DepositRoot:  []byte("root"),
			},
			hasSupport:         false,
			votingPeriodLength: 10,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			c := params.BeaconConfig()
			c.SlotsPerEth1VotingPeriod = tt.votingPeriodLength
			params.OverrideBeaconConfig(c)

			s := &pb.BeaconState{
				Eth1DataVotes: tt.stateVotes,
			}
			result, err := blocks.Eth1DataHasEnoughSupport(s, tt.data)
			if err != nil {
				t.Fatal(err)
			}

			if result != tt.hasSupport {
				t.Errorf(
					"blocks.Eth1DataHasEnoughSupport(%+v, %+v) = %t, wanted %t",
					s,
					tt.data,
					result,
					tt.hasSupport,
				)
			}
		})
	}
}
