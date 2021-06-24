package p2putils

import (
	"reflect"
	"testing"

	types "github.com/prysmaticlabs/eth2-types"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
)

func TestFork(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	tests := []struct {
		name        string
		targetEpoch types.Epoch
		want        *pb.Fork
		wantErr     bool
		setConfg    func()
	}{
		{
			name:        "genesis fork",
			targetEpoch: 0,
			want: &pb.Fork{
				Epoch:           0,
				CurrentVersion:  []byte{'A', 'B', 'C', 'D'},
				PreviousVersion: []byte{'A', 'B', 'C', 'D'},
			},
			wantErr: false,
			setConfg: func() {
				cfg := params.BeaconConfig()
				cfg.GenesisForkVersion = []byte{'A', 'B', 'C', 'D'}
				cfg.ForkVersionSchedule = map[[4]byte]types.Epoch{
					[4]byte{'A', 'B', 'C', 'D'}: 0,
				}
				params.OverrideBeaconConfig(cfg)
			},
		},
		{
			name:        "altair pre-fork",
			targetEpoch: 0,
			want: &pb.Fork{
				Epoch:           0,
				CurrentVersion:  []byte{'A', 'B', 'C', 'D'},
				PreviousVersion: []byte{'A', 'B', 'C', 'D'},
			},
			wantErr: false,
			setConfg: func() {
				cfg := params.BeaconConfig()
				cfg.GenesisForkVersion = []byte{'A', 'B', 'C', 'D'}
				cfg.AltairForkVersion = []byte{'A', 'B', 'C', 'F'}
				cfg.ForkVersionSchedule = map[[4]byte]types.Epoch{
					[4]byte{'A', 'B', 'C', 'D'}: 0,
					[4]byte{'A', 'B', 'C', 'F'}: 10,
				}
				params.OverrideBeaconConfig(cfg)
			},
		},
		{
			name:        "altair on fork",
			targetEpoch: 10,
			want: &pb.Fork{
				Epoch:           10,
				CurrentVersion:  []byte{'A', 'B', 'C', 'F'},
				PreviousVersion: []byte{'A', 'B', 'C', 'D'},
			},
			wantErr: false,
			setConfg: func() {
				cfg := params.BeaconConfig()
				cfg.GenesisForkVersion = []byte{'A', 'B', 'C', 'D'}
				cfg.AltairForkVersion = []byte{'A', 'B', 'C', 'F'}
				cfg.ForkVersionSchedule = map[[4]byte]types.Epoch{
					[4]byte{'A', 'B', 'C', 'D'}: 0,
					[4]byte{'A', 'B', 'C', 'F'}: 10,
				}
				params.OverrideBeaconConfig(cfg)
			},
		},

		{
			name:        "altair post fork",
			targetEpoch: 10,
			want: &pb.Fork{
				Epoch:           10,
				CurrentVersion:  []byte{'A', 'B', 'C', 'F'},
				PreviousVersion: []byte{'A', 'B', 'C', 'D'},
			},
			wantErr: false,
			setConfg: func() {
				cfg := params.BeaconConfig()
				cfg.GenesisForkVersion = []byte{'A', 'B', 'C', 'D'}
				cfg.AltairForkVersion = []byte{'A', 'B', 'C', 'F'}
				cfg.ForkVersionSchedule = map[[4]byte]types.Epoch{
					[4]byte{'A', 'B', 'C', 'D'}: 0,
					[4]byte{'A', 'B', 'C', 'F'}: 10,
				}
				params.OverrideBeaconConfig(cfg)
			},
		},

		{
			name:        "3 forks, pre-fork",
			targetEpoch: 20,
			want: &pb.Fork{
				Epoch:           10,
				CurrentVersion:  []byte{'A', 'B', 'C', 'F'},
				PreviousVersion: []byte{'A', 'B', 'C', 'D'},
			},
			wantErr: false,
			setConfg: func() {
				cfg := params.BeaconConfig()
				cfg.GenesisForkVersion = []byte{'A', 'B', 'C', 'D'}
				cfg.ForkVersionSchedule = map[[4]byte]types.Epoch{
					[4]byte{'A', 'B', 'C', 'D'}: 0,
					[4]byte{'A', 'B', 'C', 'F'}: 10,
					[4]byte{'A', 'B', 'C', 'Z'}: 100,
				}
				params.OverrideBeaconConfig(cfg)
			},
		},
		{
			name:        "3 forks, on fork",
			targetEpoch: 100,
			want: &pb.Fork{
				Epoch:           100,
				CurrentVersion:  []byte{'A', 'B', 'C', 'Z'},
				PreviousVersion: []byte{'A', 'B', 'C', 'F'},
			},
			wantErr: false,
			setConfg: func() {
				cfg := params.BeaconConfig()
				cfg.GenesisForkVersion = []byte{'A', 'B', 'C', 'D'}
				cfg.ForkVersionSchedule = map[[4]byte]types.Epoch{
					[4]byte{'A', 'B', 'C', 'D'}: 0,
					[4]byte{'A', 'B', 'C', 'F'}: 10,
					[4]byte{'A', 'B', 'C', 'Z'}: 100,
				}
				params.OverrideBeaconConfig(cfg)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setConfg()
			got, err := Fork(tt.targetEpoch)
			if (err != nil) != tt.wantErr {
				t.Errorf("Fork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Fork() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetrieveForkDataFromDigest(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	cfg := params.BeaconConfig()
	cfg.GenesisForkVersion = []byte{'A', 'B', 'C', 'D'}
	cfg.ForkVersionSchedule = map[[4]byte]types.Epoch{
		[4]byte{'A', 'B', 'C', 'D'}: 0,
		[4]byte{'A', 'B', 'C', 'F'}: 10,
		[4]byte{'A', 'B', 'C', 'Z'}: 100,
	}
	params.OverrideBeaconConfig(cfg)
	genValRoot := [32]byte{'A', 'B', 'C', 'D'}
	digest, err := helpers.ComputeForkDigest([]byte{'A', 'B', 'C', 'F'}, genValRoot[:])
	assert.NoError(t, err)

	version, epoch, err := RetrieveForkDataFromDigest(digest, genValRoot[:])
	assert.NoError(t, err)
	assert.Equal(t, [4]byte{'A', 'B', 'C', 'F'}, version)
	assert.Equal(t, epoch, types.Epoch(10))

	digest, err = helpers.ComputeForkDigest([]byte{'A', 'B', 'C', 'Z'}, genValRoot[:])
	assert.NoError(t, err)

	version, epoch, err = RetrieveForkDataFromDigest(digest, genValRoot[:])
	assert.NoError(t, err)
	assert.Equal(t, [4]byte{'A', 'B', 'C', 'Z'}, version)
	assert.Equal(t, epoch, types.Epoch(100))
}
