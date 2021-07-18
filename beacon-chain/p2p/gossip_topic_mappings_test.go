package p2p

import (
	"reflect"
	"testing"

	eth2types "github.com/prysmaticlabs/eth2-types"
	ethpb "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/testutil/assert"
)

func TestMappingHasNoDuplicates(t *testing.T) {
	m := make(map[reflect.Type]bool)
	for _, v := range gossipTopicMappings {
		if _, ok := m[reflect.TypeOf(v)]; ok {
			t.Errorf("%T is duplicated in the topic mapping", v)
		}
		m[reflect.TypeOf(v)] = true
	}
}

func TestGossipTopicMappings_CorrectBlockType(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	bCfg := params.BeaconConfig()
	forkEpoch := eth2types.Epoch(100)
	bCfg.AltairForkEpoch = forkEpoch
	bCfg.ForkVersionSchedule[bytesutil.ToBytes4(bCfg.AltairForkVersion)] = eth2types.Epoch(100)
	params.OverrideBeaconConfig(bCfg)

	// Before Fork
	pMessage := GossipTopicMappings(BlockSubnetTopicFormat, 0)
	_, ok := pMessage.(*ethpb.SignedBeaconBlock)
	assert.Equal(t, true, ok)

	// After Fork
	pMessage = GossipTopicMappings(BlockSubnetTopicFormat, forkEpoch)
	_, ok = pMessage.(*prysmv2.SignedBeaconBlock)
	assert.Equal(t, true, ok)
}
