package types

import (
	"github.com/prysmaticlabs/prysm/proto/beacon/p2p"
	pbp2p "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	"github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1/wrapper"
	eth "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	wrapperv1 "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1/wrapper"
	"github.com/prysmaticlabs/prysm/proto/interfaces"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	wrapperv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2/wrapper"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"google.golang.org/protobuf/proto"
)

func init() {
	// Initialize data maps.
	InitializeDataMaps()
}

// This file provides a mapping of fork version to the respective data type. This is
// to allow any service to appropriately use the correct data type with a provided
// fork-version.

var (
	// BlockMap maps the fork-version to the underlying data type for that
	// particular fork period.
	BlockMap map[[4]byte]func() interfaces.SignedBeaconBlock
	// StateMap maps the fork-version to the underlying data type for that
	// particular fork period.
	StateMap map[[4]byte]proto.Message
	// MetaDataMap maps the fork-version to the underlying data type for that
	// particular fork period.
	MetaDataMap map[[4]byte]func() p2p.Metadata
)

// InitializeDataMaps initializes all the relevant object maps. This function is called to
// reset maps and reinitialize them.
func InitializeDataMaps() {
	// Reset our block map.
	BlockMap = map[[4]byte]func() interfaces.SignedBeaconBlock{
		bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion): func() interfaces.SignedBeaconBlock {
			return wrapperv1.WrappedPhase0SignedBeaconBlock(&eth.SignedBeaconBlock{})
		},
		bytesutil.ToBytes4(params.BeaconConfig().AltairForkVersion): func() interfaces.SignedBeaconBlock {
			return wrapperv2.WrappedAltairSignedBeaconBlock(&prysmv2.SignedBeaconBlock{})
		},
	}

	// Reset our state map.
	StateMap = map[[4]byte]proto.Message{
		bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion): &pbp2p.BeaconState{},
		bytesutil.ToBytes4(params.BeaconConfig().AltairForkVersion):  &pbp2p.BeaconStateAltair{},
	}

	// Reset our metadata map.
	MetaDataMap = map[[4]byte]func() p2p.Metadata{
		bytesutil.ToBytes4(params.BeaconConfig().GenesisForkVersion): func() p2p.Metadata {
			return wrapper.WrappedMetadataV0(&pbp2p.MetaDataV0{})
		},
		bytesutil.ToBytes4(params.BeaconConfig().AltairForkVersion): func() p2p.Metadata {
			return wrapper.WrappedMetadataV1(&pbp2p.MetaDataV1{})
		},
	}
}
