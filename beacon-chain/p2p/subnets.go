package p2p

import (
	"github.com/ethereum/go-ethereum/p2p/enode"
	"github.com/ethereum/go-ethereum/p2p/enr"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/shared/params"
)

var attestationSubnetCount = params.BeaconNetworkConfig().AttestationSubnetCount

var attSubnetEnrKey = params.BeaconNetworkConfig().AttSubnetKey

func intializeAttSubnets(node *enode.LocalNode) *enode.LocalNode {
	bitV := bitfield.NewBitvector64()
	entry := enr.WithEntry(attSubnetEnrKey, bitV.Bytes())
	node.Set(entry)
	return node
}

func retrieveAttSubnets(record *enr.Record) ([]uint64, error) {
	bitV, err := retrieveBitvector(record)
	if err != nil {
		return nil, err
	}
	committeeIdxs := []uint64{}
	for i := uint64(0); i < attestationSubnetCount; i++ {
		if bitV.BitAt(i) {
			committeeIdxs = append(committeeIdxs, i)
		}
	}
	return committeeIdxs, nil
}

func retrieveBitvector(record *enr.Record) (bitfield.Bitvector64, error) {
	bitV := bitfield.NewBitvector64()
	entry := enr.WithEntry(attSubnetEnrKey, &bitV)
	err := record.Load(entry)
	if err != nil {
		return nil, err
	}
	return bitV, nil
}
