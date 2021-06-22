// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.15.8
// source: proto/prysm/v2/beacon_block.proto

package v2

import (
	reflect "reflect"
	sync "sync"

	proto "github.com/golang/protobuf/proto"
	github_com_prysmaticlabs_eth2_types "github.com/prysmaticlabs/eth2-types"
	github_com_prysmaticlabs_go_bitfield "github.com/prysmaticlabs/go-bitfield"
	_ "github.com/prysmaticlabs/prysm/proto/eth/ext"
	v1alpha1 "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// This is a compile-time assertion that a sufficiently up-to-date version
// of the legacy proto package is being used.
const _ = proto.ProtoPackageIsVersion4

type SignedBeaconBlockAltair struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Block     *BeaconBlockAltair `protobuf:"bytes,1,opt,name=block,proto3" json:"block,omitempty"`
	Signature []byte             `protobuf:"bytes,2,opt,name=signature,proto3" json:"signature,omitempty" ssz-size:"96"`
}

func (x *SignedBeaconBlockAltair) Reset() {
	*x = SignedBeaconBlockAltair{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_prysm_v2_beacon_block_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignedBeaconBlockAltair) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignedBeaconBlockAltair) ProtoMessage() {}

func (x *SignedBeaconBlockAltair) ProtoReflect() protoreflect.Message {
	mi := &file_proto_prysm_v2_beacon_block_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignedBeaconBlockAltair.ProtoReflect.Descriptor instead.
func (*SignedBeaconBlockAltair) Descriptor() ([]byte, []int) {
	return file_proto_prysm_v2_beacon_block_proto_rawDescGZIP(), []int{0}
}

func (x *SignedBeaconBlockAltair) GetBlock() *BeaconBlockAltair {
	if x != nil {
		return x.Block
	}
	return nil
}

func (x *SignedBeaconBlockAltair) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

type BeaconBlockAltair struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Slot          github_com_prysmaticlabs_eth2_types.Slot           `protobuf:"varint,1,opt,name=slot,proto3" json:"slot,omitempty" cast-type:"github.com/prysmaticlabs/eth2-types.Slot"`
	ProposerIndex github_com_prysmaticlabs_eth2_types.ValidatorIndex `protobuf:"varint,2,opt,name=proposer_index,json=proposerIndex,proto3" json:"proposer_index,omitempty" cast-type:"github.com/prysmaticlabs/eth2-types.ValidatorIndex"`
	ParentRoot    []byte                                             `protobuf:"bytes,3,opt,name=parent_root,json=parentRoot,proto3" json:"parent_root,omitempty" ssz-size:"32"`
	StateRoot     []byte                                             `protobuf:"bytes,4,opt,name=state_root,json=stateRoot,proto3" json:"state_root,omitempty" ssz-size:"32"`
	Body          *BeaconBlockBodyAltair                             `protobuf:"bytes,5,opt,name=body,proto3" json:"body,omitempty"`
}

func (x *BeaconBlockAltair) Reset() {
	*x = BeaconBlockAltair{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_prysm_v2_beacon_block_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BeaconBlockAltair) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BeaconBlockAltair) ProtoMessage() {}

func (x *BeaconBlockAltair) ProtoReflect() protoreflect.Message {
	mi := &file_proto_prysm_v2_beacon_block_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BeaconBlockAltair.ProtoReflect.Descriptor instead.
func (*BeaconBlockAltair) Descriptor() ([]byte, []int) {
	return file_proto_prysm_v2_beacon_block_proto_rawDescGZIP(), []int{1}
}

func (x *BeaconBlockAltair) GetSlot() github_com_prysmaticlabs_eth2_types.Slot {
	if x != nil {
		return x.Slot
	}
	return github_com_prysmaticlabs_eth2_types.Slot(0)
}

func (x *BeaconBlockAltair) GetProposerIndex() github_com_prysmaticlabs_eth2_types.ValidatorIndex {
	if x != nil {
		return x.ProposerIndex
	}
	return github_com_prysmaticlabs_eth2_types.ValidatorIndex(0)
}

func (x *BeaconBlockAltair) GetParentRoot() []byte {
	if x != nil {
		return x.ParentRoot
	}
	return nil
}

func (x *BeaconBlockAltair) GetStateRoot() []byte {
	if x != nil {
		return x.StateRoot
	}
	return nil
}

func (x *BeaconBlockAltair) GetBody() *BeaconBlockBodyAltair {
	if x != nil {
		return x.Body
	}
	return nil
}

type BeaconBlockBodyAltair struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	RandaoReveal      []byte                          `protobuf:"bytes,1,opt,name=randao_reveal,json=randaoReveal,proto3" json:"randao_reveal,omitempty" ssz-size:"96"`
	Eth1Data          *v1alpha1.Eth1Data              `protobuf:"bytes,2,opt,name=eth1_data,json=eth1Data,proto3" json:"eth1_data,omitempty"`
	Graffiti          []byte                          `protobuf:"bytes,3,opt,name=graffiti,proto3" json:"graffiti,omitempty" ssz-size:"32"`
	ProposerSlashings []*v1alpha1.ProposerSlashing    `protobuf:"bytes,4,rep,name=proposer_slashings,json=proposerSlashings,proto3" json:"proposer_slashings,omitempty" ssz-max:"16"`
	AttesterSlashings []*v1alpha1.AttesterSlashing    `protobuf:"bytes,5,rep,name=attester_slashings,json=attesterSlashings,proto3" json:"attester_slashings,omitempty" ssz-max:"2"`
	Attestations      []*v1alpha1.Attestation         `protobuf:"bytes,6,rep,name=attestations,proto3" json:"attestations,omitempty" ssz-max:"128"`
	Deposits          []*v1alpha1.Deposit             `protobuf:"bytes,7,rep,name=deposits,proto3" json:"deposits,omitempty" ssz-max:"16"`
	VoluntaryExits    []*v1alpha1.SignedVoluntaryExit `protobuf:"bytes,8,rep,name=voluntary_exits,json=voluntaryExits,proto3" json:"voluntary_exits,omitempty" ssz-max:"16"`
	SyncAggregate     *SyncAggregate                  `protobuf:"bytes,9,opt,name=sync_aggregate,json=syncAggregate,proto3" json:"sync_aggregate,omitempty"`
}

func (x *BeaconBlockBodyAltair) Reset() {
	*x = BeaconBlockBodyAltair{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_prysm_v2_beacon_block_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *BeaconBlockBodyAltair) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*BeaconBlockBodyAltair) ProtoMessage() {}

func (x *BeaconBlockBodyAltair) ProtoReflect() protoreflect.Message {
	mi := &file_proto_prysm_v2_beacon_block_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use BeaconBlockBodyAltair.ProtoReflect.Descriptor instead.
func (*BeaconBlockBodyAltair) Descriptor() ([]byte, []int) {
	return file_proto_prysm_v2_beacon_block_proto_rawDescGZIP(), []int{2}
}

func (x *BeaconBlockBodyAltair) GetRandaoReveal() []byte {
	if x != nil {
		return x.RandaoReveal
	}
	return nil
}

func (x *BeaconBlockBodyAltair) GetEth1Data() *v1alpha1.Eth1Data {
	if x != nil {
		return x.Eth1Data
	}
	return nil
}

func (x *BeaconBlockBodyAltair) GetGraffiti() []byte {
	if x != nil {
		return x.Graffiti
	}
	return nil
}

func (x *BeaconBlockBodyAltair) GetProposerSlashings() []*v1alpha1.ProposerSlashing {
	if x != nil {
		return x.ProposerSlashings
	}
	return nil
}

func (x *BeaconBlockBodyAltair) GetAttesterSlashings() []*v1alpha1.AttesterSlashing {
	if x != nil {
		return x.AttesterSlashings
	}
	return nil
}

func (x *BeaconBlockBodyAltair) GetAttestations() []*v1alpha1.Attestation {
	if x != nil {
		return x.Attestations
	}
	return nil
}

func (x *BeaconBlockBodyAltair) GetDeposits() []*v1alpha1.Deposit {
	if x != nil {
		return x.Deposits
	}
	return nil
}

func (x *BeaconBlockBodyAltair) GetVoluntaryExits() []*v1alpha1.SignedVoluntaryExit {
	if x != nil {
		return x.VoluntaryExits
	}
	return nil
}

func (x *BeaconBlockBodyAltair) GetSyncAggregate() *SyncAggregate {
	if x != nil {
		return x.SyncAggregate
	}
	return nil
}

type SyncAggregate struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	SyncCommitteeBits      github_com_prysmaticlabs_go_bitfield.Bitvector512 `protobuf:"bytes,1,opt,name=sync_committee_bits,json=syncCommitteeBits,proto3" json:"sync_committee_bits,omitempty" cast-type:"github.com/prysmaticlabs/go-bitfield.Bitvector512" ssz-size:"64"`
	SyncCommitteeSignature []byte                                            `protobuf:"bytes,2,opt,name=sync_committee_signature,json=syncCommitteeSignature,proto3" json:"sync_committee_signature,omitempty" ssz-size:"96"`
}

func (x *SyncAggregate) Reset() {
	*x = SyncAggregate{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_prysm_v2_beacon_block_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SyncAggregate) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SyncAggregate) ProtoMessage() {}

func (x *SyncAggregate) ProtoReflect() protoreflect.Message {
	mi := &file_proto_prysm_v2_beacon_block_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SyncAggregate.ProtoReflect.Descriptor instead.
func (*SyncAggregate) Descriptor() ([]byte, []int) {
	return file_proto_prysm_v2_beacon_block_proto_rawDescGZIP(), []int{3}
}

func (x *SyncAggregate) GetSyncCommitteeBits() github_com_prysmaticlabs_go_bitfield.Bitvector512 {
	if x != nil {
		return x.SyncCommitteeBits
	}
	return github_com_prysmaticlabs_go_bitfield.Bitvector512(nil)
}

func (x *SyncAggregate) GetSyncCommitteeSignature() []byte {
	if x != nil {
		return x.SyncCommitteeSignature
	}
	return nil
}

var File_proto_prysm_v2_beacon_block_proto protoreflect.FileDescriptor

var file_proto_prysm_v2_beacon_block_proto_rawDesc = []byte{
	0x0a, 0x21, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2f, 0x76, 0x32,
	0x2f, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x11, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x70, 0x72,
	0x79, 0x73, 0x6d, 0x2e, 0x76, 0x32, 0x1a, 0x1b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x74,
	0x68, 0x2f, 0x65, 0x78, 0x74, 0x2f, 0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x1a, 0x25, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x74, 0x68, 0x2f, 0x76,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x5f, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x24, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x2f, 0x65, 0x74, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x61,
	0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x22, 0x7b, 0x0a, 0x17, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e,
	0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x41, 0x6c, 0x74, 0x61, 0x69, 0x72, 0x12, 0x3a, 0x0a, 0x05, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x65, 0x74, 0x68,
	0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x76, 0x32, 0x2e, 0x42,
	0x65, 0x61, 0x63, 0x6f, 0x6e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x41, 0x6c, 0x74, 0x61, 0x69, 0x72,
	0x52, 0x05, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x24, 0x0a, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61,
	0x74, 0x75, 0x72, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x8a, 0xb5, 0x18, 0x02,
	0x39, 0x36, 0x52, 0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x22, 0xc2, 0x02,
	0x0a, 0x11, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x41, 0x6c, 0x74,
	0x61, 0x69, 0x72, 0x12, 0x40, 0x0a, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x04, 0x42, 0x2c, 0x82, 0xb5, 0x18, 0x28, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x61, 0x74, 0x69, 0x63, 0x6c, 0x61, 0x62, 0x73, 0x2f,
	0x65, 0x74, 0x68, 0x32, 0x2d, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74, 0x52,
	0x04, 0x73, 0x6c, 0x6f, 0x74, 0x12, 0x5d, 0x0a, 0x0e, 0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65,
	0x72, 0x5f, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x42, 0x36, 0x82,
	0xb5, 0x18, 0x32, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x72,
	0x79, 0x73, 0x6d, 0x61, 0x74, 0x69, 0x63, 0x6c, 0x61, 0x62, 0x73, 0x2f, 0x65, 0x74, 0x68, 0x32,
	0x2d, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x56, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72,
	0x49, 0x6e, 0x64, 0x65, 0x78, 0x52, 0x0d, 0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65, 0x72, 0x49,
	0x6e, 0x64, 0x65, 0x78, 0x12, 0x27, 0x0a, 0x0b, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x72,
	0x6f, 0x6f, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x8a, 0xb5, 0x18, 0x02, 0x33,
	0x32, 0x52, 0x0a, 0x70, 0x61, 0x72, 0x65, 0x6e, 0x74, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x25, 0x0a,
	0x0a, 0x73, 0x74, 0x61, 0x74, 0x65, 0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x04, 0x20, 0x01, 0x28,
	0x0c, 0x42, 0x06, 0x8a, 0xb5, 0x18, 0x02, 0x33, 0x32, 0x52, 0x09, 0x73, 0x74, 0x61, 0x74, 0x65,
	0x52, 0x6f, 0x6f, 0x74, 0x12, 0x3c, 0x0a, 0x04, 0x62, 0x6f, 0x64, 0x79, 0x18, 0x05, 0x20, 0x01,
	0x28, 0x0b, 0x32, 0x28, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x70, 0x72,
	0x79, 0x73, 0x6d, 0x2e, 0x76, 0x32, 0x2e, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x42, 0x6c, 0x6f,
	0x63, 0x6b, 0x42, 0x6f, 0x64, 0x79, 0x41, 0x6c, 0x74, 0x61, 0x69, 0x72, 0x52, 0x04, 0x62, 0x6f,
	0x64, 0x79, 0x22, 0xa0, 0x05, 0x0a, 0x15, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x42, 0x6c, 0x6f,
	0x63, 0x6b, 0x42, 0x6f, 0x64, 0x79, 0x41, 0x6c, 0x74, 0x61, 0x69, 0x72, 0x12, 0x2b, 0x0a, 0x0d,
	0x72, 0x61, 0x6e, 0x64, 0x61, 0x6f, 0x5f, 0x72, 0x65, 0x76, 0x65, 0x61, 0x6c, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x0c, 0x42, 0x06, 0x8a, 0xb5, 0x18, 0x02, 0x39, 0x36, 0x52, 0x0c, 0x72, 0x61, 0x6e,
	0x64, 0x61, 0x6f, 0x52, 0x65, 0x76, 0x65, 0x61, 0x6c, 0x12, 0x3c, 0x0a, 0x09, 0x65, 0x74, 0x68,
	0x31, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x65,
	0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c,
	0x70, 0x68, 0x61, 0x31, 0x2e, 0x45, 0x74, 0x68, 0x31, 0x44, 0x61, 0x74, 0x61, 0x52, 0x08, 0x65,
	0x74, 0x68, 0x31, 0x44, 0x61, 0x74, 0x61, 0x12, 0x22, 0x0a, 0x08, 0x67, 0x72, 0x61, 0x66, 0x66,
	0x69, 0x74, 0x69, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x8a, 0xb5, 0x18, 0x02, 0x33,
	0x32, 0x52, 0x08, 0x67, 0x72, 0x61, 0x66, 0x66, 0x69, 0x74, 0x69, 0x12, 0x5e, 0x0a, 0x12, 0x70,
	0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65, 0x72, 0x5f, 0x73, 0x6c, 0x61, 0x73, 0x68, 0x69, 0x6e, 0x67,
	0x73, 0x18, 0x04, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x27, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65,
	0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e,
	0x50, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65, 0x72, 0x53, 0x6c, 0x61, 0x73, 0x68, 0x69, 0x6e, 0x67,
	0x42, 0x06, 0x92, 0xb5, 0x18, 0x02, 0x31, 0x36, 0x52, 0x11, 0x70, 0x72, 0x6f, 0x70, 0x6f, 0x73,
	0x65, 0x72, 0x53, 0x6c, 0x61, 0x73, 0x68, 0x69, 0x6e, 0x67, 0x73, 0x12, 0x5d, 0x0a, 0x12, 0x61,
	0x74, 0x74, 0x65, 0x73, 0x74, 0x65, 0x72, 0x5f, 0x73, 0x6c, 0x61, 0x73, 0x68, 0x69, 0x6e, 0x67,
	0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x27, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65,
	0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e,
	0x41, 0x74, 0x74, 0x65, 0x73, 0x74, 0x65, 0x72, 0x53, 0x6c, 0x61, 0x73, 0x68, 0x69, 0x6e, 0x67,
	0x42, 0x05, 0x92, 0xb5, 0x18, 0x01, 0x32, 0x52, 0x11, 0x61, 0x74, 0x74, 0x65, 0x73, 0x74, 0x65,
	0x72, 0x53, 0x6c, 0x61, 0x73, 0x68, 0x69, 0x6e, 0x67, 0x73, 0x12, 0x4f, 0x0a, 0x0c, 0x61, 0x74,
	0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0x06, 0x20, 0x03, 0x28, 0x0b,
	0x32, 0x22, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e,
	0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x41, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x42, 0x07, 0x92, 0xb5, 0x18, 0x03, 0x31, 0x32, 0x38, 0x52, 0x0c, 0x61,
	0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x12, 0x42, 0x0a, 0x08, 0x64,
	0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x73, 0x18, 0x07, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x1e, 0x2e,
	0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x44, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x42, 0x06, 0x92,
	0xb5, 0x18, 0x02, 0x31, 0x36, 0x52, 0x08, 0x64, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x73, 0x12,
	0x5b, 0x0a, 0x0f, 0x76, 0x6f, 0x6c, 0x75, 0x6e, 0x74, 0x61, 0x72, 0x79, 0x5f, 0x65, 0x78, 0x69,
	0x74, 0x73, 0x18, 0x08, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x2a, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72,
	0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0x2e, 0x53, 0x69, 0x67, 0x6e, 0x65, 0x64, 0x56, 0x6f, 0x6c, 0x75, 0x6e, 0x74, 0x61, 0x72, 0x79,
	0x45, 0x78, 0x69, 0x74, 0x42, 0x06, 0x92, 0xb5, 0x18, 0x02, 0x31, 0x36, 0x52, 0x0e, 0x76, 0x6f,
	0x6c, 0x75, 0x6e, 0x74, 0x61, 0x72, 0x79, 0x45, 0x78, 0x69, 0x74, 0x73, 0x12, 0x47, 0x0a, 0x0e,
	0x73, 0x79, 0x6e, 0x63, 0x5f, 0x61, 0x67, 0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x18, 0x09,
	0x20, 0x01, 0x28, 0x0b, 0x32, 0x20, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e,
	0x70, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x76, 0x32, 0x2e, 0x53, 0x79, 0x6e, 0x63, 0x41, 0x67, 0x67,
	0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x52, 0x0d, 0x73, 0x79, 0x6e, 0x63, 0x41, 0x67, 0x67, 0x72,
	0x65, 0x67, 0x61, 0x74, 0x65, 0x22, 0xbe, 0x01, 0x0a, 0x0d, 0x53, 0x79, 0x6e, 0x63, 0x41, 0x67,
	0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x12, 0x6b, 0x0a, 0x13, 0x73, 0x79, 0x6e, 0x63, 0x5f,
	0x63, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x74, 0x65, 0x65, 0x5f, 0x62, 0x69, 0x74, 0x73, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x0c, 0x42, 0x3b, 0x8a, 0xb5, 0x18, 0x02, 0x36, 0x34, 0x82, 0xb5, 0x18, 0x31,
	0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d,
	0x61, 0x74, 0x69, 0x63, 0x6c, 0x61, 0x62, 0x73, 0x2f, 0x67, 0x6f, 0x2d, 0x62, 0x69, 0x74, 0x66,
	0x69, 0x65, 0x6c, 0x64, 0x2e, 0x42, 0x69, 0x74, 0x76, 0x65, 0x63, 0x74, 0x6f, 0x72, 0x35, 0x31,
	0x32, 0x52, 0x11, 0x73, 0x79, 0x6e, 0x63, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x74, 0x65, 0x65,
	0x42, 0x69, 0x74, 0x73, 0x12, 0x40, 0x0a, 0x18, 0x73, 0x79, 0x6e, 0x63, 0x5f, 0x63, 0x6f, 0x6d,
	0x6d, 0x69, 0x74, 0x74, 0x65, 0x65, 0x5f, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x42, 0x06, 0x8a, 0xb5, 0x18, 0x02, 0x39, 0x36, 0x52, 0x16,
	0x73, 0x79, 0x6e, 0x63, 0x43, 0x6f, 0x6d, 0x6d, 0x69, 0x74, 0x74, 0x65, 0x65, 0x53, 0x69, 0x67,
	0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x42, 0x85, 0x01, 0x0a, 0x15, 0x6f, 0x72, 0x67, 0x2e, 0x65,
	0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x76, 0x32,
	0x42, 0x10, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x50, 0x72, 0x6f,
	0x74, 0x6f, 0x50, 0x01, 0x5a, 0x30, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d,
	0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x61, 0x74, 0x69, 0x63, 0x6c, 0x61, 0x62, 0x73, 0x2f, 0x70,
	0x72, 0x79, 0x73, 0x6d, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d,
	0x2f, 0x76, 0x32, 0x3b, 0x76, 0x32, 0xaa, 0x02, 0x11, 0x45, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75,
	0x6d, 0x2e, 0x50, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x56, 0x32, 0xca, 0x02, 0x11, 0x45, 0x74, 0x68,
	0x65, 0x72, 0x65, 0x75, 0x6d, 0x5c, 0x50, 0x72, 0x79, 0x73, 0x6d, 0x5c, 0x76, 0x32, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_prysm_v2_beacon_block_proto_rawDescOnce sync.Once
	file_proto_prysm_v2_beacon_block_proto_rawDescData = file_proto_prysm_v2_beacon_block_proto_rawDesc
)

func file_proto_prysm_v2_beacon_block_proto_rawDescGZIP() []byte {
	file_proto_prysm_v2_beacon_block_proto_rawDescOnce.Do(func() {
		file_proto_prysm_v2_beacon_block_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_prysm_v2_beacon_block_proto_rawDescData)
	})
	return file_proto_prysm_v2_beacon_block_proto_rawDescData
}

var file_proto_prysm_v2_beacon_block_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_proto_prysm_v2_beacon_block_proto_goTypes = []interface{}{
	(*SignedBeaconBlockAltair)(nil),      // 0: ethereum.prysm.v2.SignedBeaconBlockAltair
	(*BeaconBlockAltair)(nil),            // 1: ethereum.prysm.v2.BeaconBlockAltair
	(*BeaconBlockBodyAltair)(nil),        // 2: ethereum.prysm.v2.BeaconBlockBodyAltair
	(*SyncAggregate)(nil),                // 3: ethereum.prysm.v2.SyncAggregate
	(*v1alpha1.Eth1Data)(nil),            // 4: ethereum.eth.v1alpha1.Eth1Data
	(*v1alpha1.ProposerSlashing)(nil),    // 5: ethereum.eth.v1alpha1.ProposerSlashing
	(*v1alpha1.AttesterSlashing)(nil),    // 6: ethereum.eth.v1alpha1.AttesterSlashing
	(*v1alpha1.Attestation)(nil),         // 7: ethereum.eth.v1alpha1.Attestation
	(*v1alpha1.Deposit)(nil),             // 8: ethereum.eth.v1alpha1.Deposit
	(*v1alpha1.SignedVoluntaryExit)(nil), // 9: ethereum.eth.v1alpha1.SignedVoluntaryExit
}
var file_proto_prysm_v2_beacon_block_proto_depIdxs = []int32{
	1, // 0: ethereum.prysm.v2.SignedBeaconBlockAltair.block:type_name -> ethereum.prysm.v2.BeaconBlockAltair
	2, // 1: ethereum.prysm.v2.BeaconBlockAltair.body:type_name -> ethereum.prysm.v2.BeaconBlockBodyAltair
	4, // 2: ethereum.prysm.v2.BeaconBlockBodyAltair.eth1_data:type_name -> ethereum.eth.v1alpha1.Eth1Data
	5, // 3: ethereum.prysm.v2.BeaconBlockBodyAltair.proposer_slashings:type_name -> ethereum.eth.v1alpha1.ProposerSlashing
	6, // 4: ethereum.prysm.v2.BeaconBlockBodyAltair.attester_slashings:type_name -> ethereum.eth.v1alpha1.AttesterSlashing
	7, // 5: ethereum.prysm.v2.BeaconBlockBodyAltair.attestations:type_name -> ethereum.eth.v1alpha1.Attestation
	8, // 6: ethereum.prysm.v2.BeaconBlockBodyAltair.deposits:type_name -> ethereum.eth.v1alpha1.Deposit
	9, // 7: ethereum.prysm.v2.BeaconBlockBodyAltair.voluntary_exits:type_name -> ethereum.eth.v1alpha1.SignedVoluntaryExit
	3, // 8: ethereum.prysm.v2.BeaconBlockBodyAltair.sync_aggregate:type_name -> ethereum.prysm.v2.SyncAggregate
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_proto_prysm_v2_beacon_block_proto_init() }
func file_proto_prysm_v2_beacon_block_proto_init() {
	if File_proto_prysm_v2_beacon_block_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_prysm_v2_beacon_block_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignedBeaconBlockAltair); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_prysm_v2_beacon_block_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BeaconBlockAltair); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_prysm_v2_beacon_block_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*BeaconBlockBodyAltair); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_proto_prysm_v2_beacon_block_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SyncAggregate); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_prysm_v2_beacon_block_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto_prysm_v2_beacon_block_proto_goTypes,
		DependencyIndexes: file_proto_prysm_v2_beacon_block_proto_depIdxs,
		MessageInfos:      file_proto_prysm_v2_beacon_block_proto_msgTypes,
	}.Build()
	File_proto_prysm_v2_beacon_block_proto = out.File
	file_proto_prysm_v2_beacon_block_proto_rawDesc = nil
	file_proto_prysm_v2_beacon_block_proto_goTypes = nil
	file_proto_prysm_v2_beacon_block_proto_depIdxs = nil
}
