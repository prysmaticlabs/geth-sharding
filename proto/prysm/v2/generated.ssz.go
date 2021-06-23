// Code generated by fastssz. DO NOT EDIT.
// Hash: 5cf4cea3807a129cf4c2d8d731961802f00c34e21b6e10714da5287f8966bb44
package v2

import (
	ssz "github.com/ferranbt/fastssz"
	github_com_prysmaticlabs_eth2_types "github.com/prysmaticlabs/eth2-types"
	v1alpha1 "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
)

// MarshalSSZ ssz marshals the SignedBeaconBlockAltair object
func (s *SignedBeaconBlockAltair) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(s)
}

// MarshalSSZTo ssz marshals the SignedBeaconBlockAltair object to a target array
func (s *SignedBeaconBlockAltair) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf
	offset := int(100)

	// Offset (0) 'Block'
	dst = ssz.WriteOffset(dst, offset)
	if s.Block == nil {
		s.Block = new(BeaconBlockAltair)
	}
	offset += s.Block.SizeSSZ()

	// Field (1) 'Signature'
	if len(s.Signature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.Signature...)

	// Field (0) 'Block'
	if dst, err = s.Block.MarshalSSZTo(dst); err != nil {
		return
	}

	return
}

// UnmarshalSSZ ssz unmarshals the SignedBeaconBlockAltair object
func (s *SignedBeaconBlockAltair) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size < 100 {
		return ssz.ErrSize
	}

	tail := buf
	var o0 uint64

	// Offset (0) 'Block'
	if o0 = ssz.ReadOffset(buf[0:4]); o0 > size {
		return ssz.ErrOffset
	}

	if o0 < 100 {
		return ssz.ErrInvalidVariableOffset
	}

	// Field (1) 'Signature'
	if cap(s.Signature) == 0 {
		s.Signature = make([]byte, 0, len(buf[4:100]))
	}
	s.Signature = append(s.Signature, buf[4:100]...)

	// Field (0) 'Block'
	{
		buf = tail[o0:]
		if s.Block == nil {
			s.Block = new(BeaconBlockAltair)
		}
		if err = s.Block.UnmarshalSSZ(buf); err != nil {
			return err
		}
	}
	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the SignedBeaconBlockAltair object
func (s *SignedBeaconBlockAltair) SizeSSZ() (size int) {
	size = 100

	// Field (0) 'Block'
	if s.Block == nil {
		s.Block = new(BeaconBlockAltair)
	}
	size += s.Block.SizeSSZ()

	return
}

// HashTreeRoot ssz hashes the SignedBeaconBlockAltair object
func (s *SignedBeaconBlockAltair) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(s)
}

// HashTreeRootWith ssz hashes the SignedBeaconBlockAltair object with a hasher
func (s *SignedBeaconBlockAltair) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Block'
	if err = s.Block.HashTreeRootWith(hh); err != nil {
		return
	}

	// Field (1) 'Signature'
	if len(s.Signature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.Signature)

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the BeaconBlockAltair object
func (b *BeaconBlockAltair) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(b)
}

// MarshalSSZTo ssz marshals the BeaconBlockAltair object to a target array
func (b *BeaconBlockAltair) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf
	offset := int(84)

	// Field (0) 'Slot'
	dst = ssz.MarshalUint64(dst, uint64(b.Slot))

	// Field (1) 'ProposerIndex'
	dst = ssz.MarshalUint64(dst, uint64(b.ProposerIndex))

	// Field (2) 'ParentRoot'
	if len(b.ParentRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, b.ParentRoot...)

	// Field (3) 'StateRoot'
	if len(b.StateRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, b.StateRoot...)

	// Offset (4) 'Body'
	dst = ssz.WriteOffset(dst, offset)
	if b.Body == nil {
		b.Body = new(BeaconBlockBodyAltair)
	}
	offset += b.Body.SizeSSZ()

	// Field (4) 'Body'
	if dst, err = b.Body.MarshalSSZTo(dst); err != nil {
		return
	}

	return
}

// UnmarshalSSZ ssz unmarshals the BeaconBlockAltair object
func (b *BeaconBlockAltair) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size < 84 {
		return ssz.ErrSize
	}

	tail := buf
	var o4 uint64

	// Field (0) 'Slot'
	b.Slot = github_com_prysmaticlabs_eth2_types.Slot(ssz.UnmarshallUint64(buf[0:8]))

	// Field (1) 'ProposerIndex'
	b.ProposerIndex = github_com_prysmaticlabs_eth2_types.ValidatorIndex(ssz.UnmarshallUint64(buf[8:16]))

	// Field (2) 'ParentRoot'
	if cap(b.ParentRoot) == 0 {
		b.ParentRoot = make([]byte, 0, len(buf[16:48]))
	}
	b.ParentRoot = append(b.ParentRoot, buf[16:48]...)

	// Field (3) 'StateRoot'
	if cap(b.StateRoot) == 0 {
		b.StateRoot = make([]byte, 0, len(buf[48:80]))
	}
	b.StateRoot = append(b.StateRoot, buf[48:80]...)

	// Offset (4) 'Body'
	if o4 = ssz.ReadOffset(buf[80:84]); o4 > size {
		return ssz.ErrOffset
	}

	if o4 < 84 {
		return ssz.ErrInvalidVariableOffset
	}

	// Field (4) 'Body'
	{
		buf = tail[o4:]
		if b.Body == nil {
			b.Body = new(BeaconBlockBodyAltair)
		}
		if err = b.Body.UnmarshalSSZ(buf); err != nil {
			return err
		}
	}
	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the BeaconBlockAltair object
func (b *BeaconBlockAltair) SizeSSZ() (size int) {
	size = 84

	// Field (4) 'Body'
	if b.Body == nil {
		b.Body = new(BeaconBlockBodyAltair)
	}
	size += b.Body.SizeSSZ()

	return
}

// HashTreeRoot ssz hashes the BeaconBlockAltair object
func (b *BeaconBlockAltair) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(b)
}

// HashTreeRootWith ssz hashes the BeaconBlockAltair object with a hasher
func (b *BeaconBlockAltair) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Slot'
	hh.PutUint64(uint64(b.Slot))

	// Field (1) 'ProposerIndex'
	hh.PutUint64(uint64(b.ProposerIndex))

	// Field (2) 'ParentRoot'
	if len(b.ParentRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(b.ParentRoot)

	// Field (3) 'StateRoot'
	if len(b.StateRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(b.StateRoot)

	// Field (4) 'Body'
	if err = b.Body.HashTreeRootWith(hh); err != nil {
		return
	}

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the BeaconBlockBodyAltair object
func (b *BeaconBlockBodyAltair) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(b)
}

// MarshalSSZTo ssz marshals the BeaconBlockBodyAltair object to a target array
func (b *BeaconBlockBodyAltair) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf
	offset := int(380)

	// Field (0) 'RandaoReveal'
	if len(b.RandaoReveal) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, b.RandaoReveal...)

	// Field (1) 'Eth1Data'
	if b.Eth1Data == nil {
		b.Eth1Data = new(v1alpha1.Eth1Data)
	}
	if dst, err = b.Eth1Data.MarshalSSZTo(dst); err != nil {
		return
	}

	// Field (2) 'Graffiti'
	if len(b.Graffiti) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, b.Graffiti...)

	// Offset (3) 'ProposerSlashings'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(b.ProposerSlashings) * 416

	// Offset (4) 'AttesterSlashings'
	dst = ssz.WriteOffset(dst, offset)
	for ii := 0; ii < len(b.AttesterSlashings); ii++ {
		offset += 4
		offset += b.AttesterSlashings[ii].SizeSSZ()
	}

	// Offset (5) 'Attestations'
	dst = ssz.WriteOffset(dst, offset)
	for ii := 0; ii < len(b.Attestations); ii++ {
		offset += 4
		offset += b.Attestations[ii].SizeSSZ()
	}

	// Offset (6) 'Deposits'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(b.Deposits) * 1240

	// Offset (7) 'VoluntaryExits'
	dst = ssz.WriteOffset(dst, offset)
	offset += len(b.VoluntaryExits) * 112

	// Field (8) 'SyncAggregate'
	if b.SyncAggregate == nil {
		b.SyncAggregate = new(SyncAggregate)
	}
	if dst, err = b.SyncAggregate.MarshalSSZTo(dst); err != nil {
		return
	}

	// Field (3) 'ProposerSlashings'
	if len(b.ProposerSlashings) > 16 {
		err = ssz.ErrListTooBig
		return
	}
	for ii := 0; ii < len(b.ProposerSlashings); ii++ {
		if dst, err = b.ProposerSlashings[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	// Field (4) 'AttesterSlashings'
	if len(b.AttesterSlashings) > 2 {
		err = ssz.ErrListTooBig
		return
	}
	{
		offset = 4 * len(b.AttesterSlashings)
		for ii := 0; ii < len(b.AttesterSlashings); ii++ {
			dst = ssz.WriteOffset(dst, offset)
			offset += b.AttesterSlashings[ii].SizeSSZ()
		}
	}
	for ii := 0; ii < len(b.AttesterSlashings); ii++ {
		if dst, err = b.AttesterSlashings[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	// Field (5) 'Attestations'
	if len(b.Attestations) > 128 {
		err = ssz.ErrListTooBig
		return
	}
	{
		offset = 4 * len(b.Attestations)
		for ii := 0; ii < len(b.Attestations); ii++ {
			dst = ssz.WriteOffset(dst, offset)
			offset += b.Attestations[ii].SizeSSZ()
		}
	}
	for ii := 0; ii < len(b.Attestations); ii++ {
		if dst, err = b.Attestations[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	// Field (6) 'Deposits'
	if len(b.Deposits) > 16 {
		err = ssz.ErrListTooBig
		return
	}
	for ii := 0; ii < len(b.Deposits); ii++ {
		if dst, err = b.Deposits[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	// Field (7) 'VoluntaryExits'
	if len(b.VoluntaryExits) > 16 {
		err = ssz.ErrListTooBig
		return
	}
	for ii := 0; ii < len(b.VoluntaryExits); ii++ {
		if dst, err = b.VoluntaryExits[ii].MarshalSSZTo(dst); err != nil {
			return
		}
	}

	return
}

// UnmarshalSSZ ssz unmarshals the BeaconBlockBodyAltair object
func (b *BeaconBlockBodyAltair) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size < 380 {
		return ssz.ErrSize
	}

	tail := buf
	var o3, o4, o5, o6, o7 uint64

	// Field (0) 'RandaoReveal'
	if cap(b.RandaoReveal) == 0 {
		b.RandaoReveal = make([]byte, 0, len(buf[0:96]))
	}
	b.RandaoReveal = append(b.RandaoReveal, buf[0:96]...)

	// Field (1) 'Eth1Data'
	if b.Eth1Data == nil {
		b.Eth1Data = new(v1alpha1.Eth1Data)
	}
	if err = b.Eth1Data.UnmarshalSSZ(buf[96:168]); err != nil {
		return err
	}

	// Field (2) 'Graffiti'
	if cap(b.Graffiti) == 0 {
		b.Graffiti = make([]byte, 0, len(buf[168:200]))
	}
	b.Graffiti = append(b.Graffiti, buf[168:200]...)

	// Offset (3) 'ProposerSlashings'
	if o3 = ssz.ReadOffset(buf[200:204]); o3 > size {
		return ssz.ErrOffset
	}

	if o3 < 380 {
		return ssz.ErrInvalidVariableOffset
	}

	// Offset (4) 'AttesterSlashings'
	if o4 = ssz.ReadOffset(buf[204:208]); o4 > size || o3 > o4 {
		return ssz.ErrOffset
	}

	// Offset (5) 'Attestations'
	if o5 = ssz.ReadOffset(buf[208:212]); o5 > size || o4 > o5 {
		return ssz.ErrOffset
	}

	// Offset (6) 'Deposits'
	if o6 = ssz.ReadOffset(buf[212:216]); o6 > size || o5 > o6 {
		return ssz.ErrOffset
	}

	// Offset (7) 'VoluntaryExits'
	if o7 = ssz.ReadOffset(buf[216:220]); o7 > size || o6 > o7 {
		return ssz.ErrOffset
	}

	// Field (8) 'SyncAggregate'
	if b.SyncAggregate == nil {
		b.SyncAggregate = new(SyncAggregate)
	}
	if err = b.SyncAggregate.UnmarshalSSZ(buf[220:380]); err != nil {
		return err
	}

	// Field (3) 'ProposerSlashings'
	{
		buf = tail[o3:o4]
		num, err := ssz.DivideInt2(len(buf), 416, 16)
		if err != nil {
			return err
		}
		b.ProposerSlashings = make([]*v1alpha1.ProposerSlashing, num)
		for ii := 0; ii < num; ii++ {
			if b.ProposerSlashings[ii] == nil {
				b.ProposerSlashings[ii] = new(v1alpha1.ProposerSlashing)
			}
			if err = b.ProposerSlashings[ii].UnmarshalSSZ(buf[ii*416 : (ii+1)*416]); err != nil {
				return err
			}
		}
	}

	// Field (4) 'AttesterSlashings'
	{
		buf = tail[o4:o5]
		num, err := ssz.DecodeDynamicLength(buf, 2)
		if err != nil {
			return err
		}
		b.AttesterSlashings = make([]*v1alpha1.AttesterSlashing, num)
		err = ssz.UnmarshalDynamic(buf, num, func(indx int, buf []byte) (err error) {
			if b.AttesterSlashings[indx] == nil {
				b.AttesterSlashings[indx] = new(v1alpha1.AttesterSlashing)
			}
			if err = b.AttesterSlashings[indx].UnmarshalSSZ(buf); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Field (5) 'Attestations'
	{
		buf = tail[o5:o6]
		num, err := ssz.DecodeDynamicLength(buf, 128)
		if err != nil {
			return err
		}
		b.Attestations = make([]*v1alpha1.Attestation, num)
		err = ssz.UnmarshalDynamic(buf, num, func(indx int, buf []byte) (err error) {
			if b.Attestations[indx] == nil {
				b.Attestations[indx] = new(v1alpha1.Attestation)
			}
			if err = b.Attestations[indx].UnmarshalSSZ(buf); err != nil {
				return err
			}
			return nil
		})
		if err != nil {
			return err
		}
	}

	// Field (6) 'Deposits'
	{
		buf = tail[o6:o7]
		num, err := ssz.DivideInt2(len(buf), 1240, 16)
		if err != nil {
			return err
		}
		b.Deposits = make([]*v1alpha1.Deposit, num)
		for ii := 0; ii < num; ii++ {
			if b.Deposits[ii] == nil {
				b.Deposits[ii] = new(v1alpha1.Deposit)
			}
			if err = b.Deposits[ii].UnmarshalSSZ(buf[ii*1240 : (ii+1)*1240]); err != nil {
				return err
			}
		}
	}

	// Field (7) 'VoluntaryExits'
	{
		buf = tail[o7:]
		num, err := ssz.DivideInt2(len(buf), 112, 16)
		if err != nil {
			return err
		}
		b.VoluntaryExits = make([]*v1alpha1.SignedVoluntaryExit, num)
		for ii := 0; ii < num; ii++ {
			if b.VoluntaryExits[ii] == nil {
				b.VoluntaryExits[ii] = new(v1alpha1.SignedVoluntaryExit)
			}
			if err = b.VoluntaryExits[ii].UnmarshalSSZ(buf[ii*112 : (ii+1)*112]); err != nil {
				return err
			}
		}
	}
	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the BeaconBlockBodyAltair object
func (b *BeaconBlockBodyAltair) SizeSSZ() (size int) {
	size = 380

	// Field (3) 'ProposerSlashings'
	size += len(b.ProposerSlashings) * 416

	// Field (4) 'AttesterSlashings'
	for ii := 0; ii < len(b.AttesterSlashings); ii++ {
		size += 4
		size += b.AttesterSlashings[ii].SizeSSZ()
	}

	// Field (5) 'Attestations'
	for ii := 0; ii < len(b.Attestations); ii++ {
		size += 4
		size += b.Attestations[ii].SizeSSZ()
	}

	// Field (6) 'Deposits'
	size += len(b.Deposits) * 1240

	// Field (7) 'VoluntaryExits'
	size += len(b.VoluntaryExits) * 112

	return
}

// HashTreeRoot ssz hashes the BeaconBlockBodyAltair object
func (b *BeaconBlockBodyAltair) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(b)
}

// HashTreeRootWith ssz hashes the BeaconBlockBodyAltair object with a hasher
func (b *BeaconBlockBodyAltair) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'RandaoReveal'
	if len(b.RandaoReveal) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(b.RandaoReveal)

	// Field (1) 'Eth1Data'
	if err = b.Eth1Data.HashTreeRootWith(hh); err != nil {
		return
	}

	// Field (2) 'Graffiti'
	if len(b.Graffiti) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(b.Graffiti)

	// Field (3) 'ProposerSlashings'
	{
		subIndx := hh.Index()
		num := uint64(len(b.ProposerSlashings))
		if num > 16 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = b.ProposerSlashings[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 16)
	}

	// Field (4) 'AttesterSlashings'
	{
		subIndx := hh.Index()
		num := uint64(len(b.AttesterSlashings))
		if num > 2 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = b.AttesterSlashings[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 2)
	}

	// Field (5) 'Attestations'
	{
		subIndx := hh.Index()
		num := uint64(len(b.Attestations))
		if num > 128 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = b.Attestations[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 128)
	}

	// Field (6) 'Deposits'
	{
		subIndx := hh.Index()
		num := uint64(len(b.Deposits))
		if num > 16 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = b.Deposits[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 16)
	}

	// Field (7) 'VoluntaryExits'
	{
		subIndx := hh.Index()
		num := uint64(len(b.VoluntaryExits))
		if num > 16 {
			err = ssz.ErrIncorrectListSize
			return
		}
		for i := uint64(0); i < num; i++ {
			if err = b.VoluntaryExits[i].HashTreeRootWith(hh); err != nil {
				return
			}
		}
		hh.MerkleizeWithMixin(subIndx, num, 16)
	}

	// Field (8) 'SyncAggregate'
	if err = b.SyncAggregate.HashTreeRootWith(hh); err != nil {
		return
	}

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the SyncAggregate object
func (s *SyncAggregate) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(s)
}

// MarshalSSZTo ssz marshals the SyncAggregate object to a target array
func (s *SyncAggregate) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf

	// Field (0) 'SyncCommitteeBits'
	if len(s.SyncCommitteeBits) != 64 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.SyncCommitteeBits...)

	// Field (1) 'SyncCommitteeSignature'
	if len(s.SyncCommitteeSignature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.SyncCommitteeSignature...)

	return
}

// UnmarshalSSZ ssz unmarshals the SyncAggregate object
func (s *SyncAggregate) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size != 160 {
		return ssz.ErrSize
	}

	// Field (0) 'SyncCommitteeBits'
	if cap(s.SyncCommitteeBits) == 0 {
		s.SyncCommitteeBits = make([]byte, 0, len(buf[0:64]))
	}
	s.SyncCommitteeBits = append(s.SyncCommitteeBits, buf[0:64]...)

	// Field (1) 'SyncCommitteeSignature'
	if cap(s.SyncCommitteeSignature) == 0 {
		s.SyncCommitteeSignature = make([]byte, 0, len(buf[64:160]))
	}
	s.SyncCommitteeSignature = append(s.SyncCommitteeSignature, buf[64:160]...)

	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the SyncAggregate object
func (s *SyncAggregate) SizeSSZ() (size int) {
	size = 160
	return
}

// HashTreeRoot ssz hashes the SyncAggregate object
func (s *SyncAggregate) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(s)
}

// HashTreeRootWith ssz hashes the SyncAggregate object with a hasher
func (s *SyncAggregate) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'SyncCommitteeBits'
	if len(s.SyncCommitteeBits) != 64 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.SyncCommitteeBits)

	// Field (1) 'SyncCommitteeSignature'
	if len(s.SyncCommitteeSignature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.SyncCommitteeSignature)

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the SyncCommitteeMessage object
func (s *SyncCommitteeMessage) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(s)
}

// MarshalSSZTo ssz marshals the SyncCommitteeMessage object to a target array
func (s *SyncCommitteeMessage) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf

	// Field (0) 'Slot'
	dst = ssz.MarshalUint64(dst, uint64(s.Slot))

	// Field (1) 'BlockRoot'
	if len(s.BlockRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.BlockRoot...)

	// Field (2) 'ValidatorIndex'
	dst = ssz.MarshalUint64(dst, uint64(s.ValidatorIndex))

	// Field (3) 'Signature'
	if len(s.Signature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.Signature...)

	return
}

// UnmarshalSSZ ssz unmarshals the SyncCommitteeMessage object
func (s *SyncCommitteeMessage) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size != 144 {
		return ssz.ErrSize
	}

	// Field (0) 'Slot'
	s.Slot = github_com_prysmaticlabs_eth2_types.Slot(ssz.UnmarshallUint64(buf[0:8]))

	// Field (1) 'BlockRoot'
	if cap(s.BlockRoot) == 0 {
		s.BlockRoot = make([]byte, 0, len(buf[8:40]))
	}
	s.BlockRoot = append(s.BlockRoot, buf[8:40]...)

	// Field (2) 'ValidatorIndex'
	s.ValidatorIndex = github_com_prysmaticlabs_eth2_types.ValidatorIndex(ssz.UnmarshallUint64(buf[40:48]))

	// Field (3) 'Signature'
	if cap(s.Signature) == 0 {
		s.Signature = make([]byte, 0, len(buf[48:144]))
	}
	s.Signature = append(s.Signature, buf[48:144]...)

	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the SyncCommitteeMessage object
func (s *SyncCommitteeMessage) SizeSSZ() (size int) {
	size = 144
	return
}

// HashTreeRoot ssz hashes the SyncCommitteeMessage object
func (s *SyncCommitteeMessage) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(s)
}

// HashTreeRootWith ssz hashes the SyncCommitteeMessage object with a hasher
func (s *SyncCommitteeMessage) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Slot'
	hh.PutUint64(uint64(s.Slot))

	// Field (1) 'BlockRoot'
	if len(s.BlockRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.BlockRoot)

	// Field (2) 'ValidatorIndex'
	hh.PutUint64(uint64(s.ValidatorIndex))

	// Field (3) 'Signature'
	if len(s.Signature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.Signature)

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the SyncCommitteeContribution object
func (s *SyncCommitteeContribution) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(s)
}

// MarshalSSZTo ssz marshals the SyncCommitteeContribution object to a target array
func (s *SyncCommitteeContribution) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf

	// Field (0) 'Slot'
	dst = ssz.MarshalUint64(dst, uint64(s.Slot))

	// Field (1) 'BlockRoot'
	if len(s.BlockRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.BlockRoot...)

	// Field (2) 'SubcommitteeIndex'
	dst = ssz.MarshalUint64(dst, s.SubcommitteeIndex)

	// Field (3) 'AggregationBits'
	if len(s.AggregationBits) != 16 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.AggregationBits...)

	// Field (4) 'Signature'
	if len(s.Signature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.Signature...)

	return
}

// UnmarshalSSZ ssz unmarshals the SyncCommitteeContribution object
func (s *SyncCommitteeContribution) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size != 160 {
		return ssz.ErrSize
	}

	// Field (0) 'Slot'
	s.Slot = github_com_prysmaticlabs_eth2_types.Slot(ssz.UnmarshallUint64(buf[0:8]))

	// Field (1) 'BlockRoot'
	if cap(s.BlockRoot) == 0 {
		s.BlockRoot = make([]byte, 0, len(buf[8:40]))
	}
	s.BlockRoot = append(s.BlockRoot, buf[8:40]...)

	// Field (2) 'SubcommitteeIndex'
	s.SubcommitteeIndex = ssz.UnmarshallUint64(buf[40:48])

	// Field (3) 'AggregationBits'
	if cap(s.AggregationBits) == 0 {
		s.AggregationBits = make([]byte, 0, len(buf[48:64]))
	}
	s.AggregationBits = append(s.AggregationBits, buf[48:64]...)

	// Field (4) 'Signature'
	if cap(s.Signature) == 0 {
		s.Signature = make([]byte, 0, len(buf[64:160]))
	}
	s.Signature = append(s.Signature, buf[64:160]...)

	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the SyncCommitteeContribution object
func (s *SyncCommitteeContribution) SizeSSZ() (size int) {
	size = 160
	return
}

// HashTreeRoot ssz hashes the SyncCommitteeContribution object
func (s *SyncCommitteeContribution) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(s)
}

// HashTreeRootWith ssz hashes the SyncCommitteeContribution object with a hasher
func (s *SyncCommitteeContribution) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Slot'
	hh.PutUint64(uint64(s.Slot))

	// Field (1) 'BlockRoot'
	if len(s.BlockRoot) != 32 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.BlockRoot)

	// Field (2) 'SubcommitteeIndex'
	hh.PutUint64(s.SubcommitteeIndex)

	// Field (3) 'AggregationBits'
	if len(s.AggregationBits) != 16 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.AggregationBits)

	// Field (4) 'Signature'
	if len(s.Signature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.Signature)

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the ContributionAndProof object
func (c *ContributionAndProof) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(c)
}

// MarshalSSZTo ssz marshals the ContributionAndProof object to a target array
func (c *ContributionAndProof) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf

	// Field (0) 'AggregatorIndex'
	dst = ssz.MarshalUint64(dst, uint64(c.AggregatorIndex))

	// Field (1) 'Contribution'
	if c.Contribution == nil {
		c.Contribution = new(SyncCommitteeContribution)
	}
	if dst, err = c.Contribution.MarshalSSZTo(dst); err != nil {
		return
	}

	// Field (2) 'SelectionProof'
	if len(c.SelectionProof) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, c.SelectionProof...)

	return
}

// UnmarshalSSZ ssz unmarshals the ContributionAndProof object
func (c *ContributionAndProof) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size != 264 {
		return ssz.ErrSize
	}

	// Field (0) 'AggregatorIndex'
	c.AggregatorIndex = github_com_prysmaticlabs_eth2_types.ValidatorIndex(ssz.UnmarshallUint64(buf[0:8]))

	// Field (1) 'Contribution'
	if c.Contribution == nil {
		c.Contribution = new(SyncCommitteeContribution)
	}
	if err = c.Contribution.UnmarshalSSZ(buf[8:168]); err != nil {
		return err
	}

	// Field (2) 'SelectionProof'
	if cap(c.SelectionProof) == 0 {
		c.SelectionProof = make([]byte, 0, len(buf[168:264]))
	}
	c.SelectionProof = append(c.SelectionProof, buf[168:264]...)

	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the ContributionAndProof object
func (c *ContributionAndProof) SizeSSZ() (size int) {
	size = 264
	return
}

// HashTreeRoot ssz hashes the ContributionAndProof object
func (c *ContributionAndProof) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(c)
}

// HashTreeRootWith ssz hashes the ContributionAndProof object with a hasher
func (c *ContributionAndProof) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'AggregatorIndex'
	hh.PutUint64(uint64(c.AggregatorIndex))

	// Field (1) 'Contribution'
	if err = c.Contribution.HashTreeRootWith(hh); err != nil {
		return
	}

	// Field (2) 'SelectionProof'
	if len(c.SelectionProof) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(c.SelectionProof)

	hh.Merkleize(indx)
	return
}

// MarshalSSZ ssz marshals the SignedContributionAndProof object
func (s *SignedContributionAndProof) MarshalSSZ() ([]byte, error) {
	return ssz.MarshalSSZ(s)
}

// MarshalSSZTo ssz marshals the SignedContributionAndProof object to a target array
func (s *SignedContributionAndProof) MarshalSSZTo(buf []byte) (dst []byte, err error) {
	dst = buf

	// Field (0) 'Message'
	if s.Message == nil {
		s.Message = new(ContributionAndProof)
	}
	if dst, err = s.Message.MarshalSSZTo(dst); err != nil {
		return
	}

	// Field (1) 'Signature'
	if len(s.Signature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	dst = append(dst, s.Signature...)

	return
}

// UnmarshalSSZ ssz unmarshals the SignedContributionAndProof object
func (s *SignedContributionAndProof) UnmarshalSSZ(buf []byte) error {
	var err error
	size := uint64(len(buf))
	if size != 360 {
		return ssz.ErrSize
	}

	// Field (0) 'Message'
	if s.Message == nil {
		s.Message = new(ContributionAndProof)
	}
	if err = s.Message.UnmarshalSSZ(buf[0:264]); err != nil {
		return err
	}

	// Field (1) 'Signature'
	if cap(s.Signature) == 0 {
		s.Signature = make([]byte, 0, len(buf[264:360]))
	}
	s.Signature = append(s.Signature, buf[264:360]...)

	return err
}

// SizeSSZ returns the ssz encoded size in bytes for the SignedContributionAndProof object
func (s *SignedContributionAndProof) SizeSSZ() (size int) {
	size = 360
	return
}

// HashTreeRoot ssz hashes the SignedContributionAndProof object
func (s *SignedContributionAndProof) HashTreeRoot() ([32]byte, error) {
	return ssz.HashWithDefaultHasher(s)
}

// HashTreeRootWith ssz hashes the SignedContributionAndProof object with a hasher
func (s *SignedContributionAndProof) HashTreeRootWith(hh *ssz.Hasher) (err error) {
	indx := hh.Index()

	// Field (0) 'Message'
	if err = s.Message.HashTreeRootWith(hh); err != nil {
		return
	}

	// Field (1) 'Signature'
	if len(s.Signature) != 96 {
		err = ssz.ErrBytesLength
		return
	}
	hh.PutBytes(s.Signature)

	hh.Merkleize(indx)
	return
}
