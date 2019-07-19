package blocks

import (
	"fmt"

	"github.com/prysmaticlabs/prysm/beacon-chain/cache"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"testing"
)

func TestEth1DataHasEnoughSupport(t *testing.T) {
	tests := []struct {
		stateVotes         []*pb.Eth1Data
		data               *pb.Eth1Data
		hasSupport         bool
		votingPeriodLength uint64
	}{
		{
			stateVotes: []*pb.Eth1Data{
				&pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				},
			},
			data: &pb.Eth1Data{
				DepositCount: 1,
				DepositRoot:  []byte("root"),
			},
			hasSupport:         true,
			votingPeriodLength: 7,
		}, {
			stateVotes: []*pb.Eth1Data{
				&pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				},
			},
			data: &pb.Eth1Data{
				DepositCount: 1,
				DepositRoot:  []byte("root"),
			},
			hasSupport:         false,
			votingPeriodLength: 8,
		}, {
			stateVotes: []*pb.Eth1Data{
				&pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				}, &pb.Eth1Data{
					DepositCount: 1,
					DepositRoot:  []byte("root"),
				},
			},
			data: &pb.Eth1Data{
				DepositCount: 1,
				DepositRoot:  []byte("root"),
			},
			hasSupport:         false,
			votingPeriodLength: 10,
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			eth1DataCache = cache.NewEth1DataVoteCache()

			c := params.BeaconConfig()
			c.SlotsPerEth1VotingPeriod = tt.votingPeriodLength
			params.OverrideBeaconConfig(c)

			s := &pb.BeaconState{
				Eth1DataVotes: tt.stateVotes,
			}
			result, err := Eth1DataHasEnoughSupport(s, tt.data)
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
