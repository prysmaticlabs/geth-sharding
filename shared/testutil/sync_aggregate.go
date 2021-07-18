package testutil

import (
	"github.com/pkg/errors"
	"github.com/prysmaticlabs/go-bitfield"
	"github.com/prysmaticlabs/prysm/beacon-chain/core/helpers"
	p2pType "github.com/prysmaticlabs/prysm/beacon-chain/p2p/types"
	iface "github.com/prysmaticlabs/prysm/beacon-chain/state/interface"
	pb "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	prysmv2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	"github.com/prysmaticlabs/prysm/shared/bls"
	"github.com/prysmaticlabs/prysm/shared/bytesutil"
	"github.com/prysmaticlabs/prysm/shared/params"
	"github.com/prysmaticlabs/prysm/shared/version"
)

func generateSyncAggregate(bState iface.BeaconState, privs []bls.SecretKey, parentRoot [32]byte) (*prysmv2.SyncAggregate, error) {
	st, ok := bState.(iface.BeaconStateAltair)
	if !ok || bState.Version() == version.Phase0 {
		return nil, errors.Errorf("state cannot be asserted to altair state")
	}
	nextSlotEpoch := helpers.SlotToEpoch(st.Slot() + 1)
	currEpoch := helpers.SlotToEpoch(st.Slot())

	var syncCommittee *pb.SyncCommittee
	var err error
	if helpers.SyncCommitteePeriod(currEpoch) == helpers.SyncCommitteePeriod(nextSlotEpoch) {
		syncCommittee, err = st.CurrentSyncCommittee()
		if err != nil {
			return nil, err
		}
	} else {
		syncCommittee, err = st.NextSyncCommittee()
		if err != nil {
			return nil, err
		}
	}
	sigs := make([]bls.Signature, 0, len(syncCommittee.Pubkeys))
	var bVector []byte
	currSize := new(prysmv2.SyncAggregate).SyncCommitteeBits.Len()
	switch currSize {
	case 512:
		bVector = bitfield.NewBitvector512()
	case 32:
		bVector = bitfield.NewBitvector32()
	default:
		return nil, errors.New("invalid bit vector size")
	}

	for i, p := range syncCommittee.Pubkeys {
		idx, ok := st.ValidatorIndexByPubkey(bytesutil.ToBytes48(p))
		if !ok {
			continue
		}
		d, err := helpers.Domain(st.Fork(), helpers.SlotToEpoch(st.Slot()), params.BeaconConfig().DomainSyncCommittee, st.GenesisValidatorRoot())
		if err != nil {
			return nil, err
		}
		sszBytes := p2pType.SSZBytes(parentRoot[:])
		r, err := helpers.ComputeSigningRoot(&sszBytes, d)
		if err != nil {
			return nil, err
		}
		sigs = append(sigs, privs[idx].Sign(r[:]))
		if currSize == 512 {
			bitfield.Bitvector512(bVector).SetBitAt(uint64(i), true)
		}
		if currSize == 32 {
			bitfield.Bitvector32(bVector).SetBitAt(uint64(i), true)
		}
	}
	if len(sigs) == 0 {
		fakeSig := [96]byte{0xC0}
		return &prysmv2.SyncAggregate{SyncCommitteeSignature: fakeSig[:], SyncCommitteeBits: bVector}, nil
	}
	aggSig := bls.AggregateSignatures(sigs)
	return &prysmv2.SyncAggregate{SyncCommitteeSignature: aggSig.Marshal(), SyncCommitteeBits: bVector}, nil
}
