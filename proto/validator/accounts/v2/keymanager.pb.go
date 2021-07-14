// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.15.8
// source: proto/validator/accounts/v2/keymanager.proto

package ethereum_validator_accounts_v2

import (
	context "context"
	reflect "reflect"
	sync "sync"

	proto "github.com/golang/protobuf/proto"
	empty "github.com/golang/protobuf/ptypes/empty"
	github_com_prysmaticlabs_eth2_types "github.com/prysmaticlabs/eth2-types"
	_ "github.com/prysmaticlabs/prysm/proto/eth/ext"
	v1alpha1 "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	v2 "github.com/prysmaticlabs/prysm/proto/prysm/v2"
	_ "google.golang.org/genproto/googleapis/api/annotations"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
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

type SignResponse_Status int32

const (
	SignResponse_UNKNOWN   SignResponse_Status = 0
	SignResponse_SUCCEEDED SignResponse_Status = 1
	SignResponse_DENIED    SignResponse_Status = 2
	SignResponse_FAILED    SignResponse_Status = 3
)

// Enum value maps for SignResponse_Status.
var (
	SignResponse_Status_name = map[int32]string{
		0: "UNKNOWN",
		1: "SUCCEEDED",
		2: "DENIED",
		3: "FAILED",
	}
	SignResponse_Status_value = map[string]int32{
		"UNKNOWN":   0,
		"SUCCEEDED": 1,
		"DENIED":    2,
		"FAILED":    3,
	}
)

func (x SignResponse_Status) Enum() *SignResponse_Status {
	p := new(SignResponse_Status)
	*p = x
	return p
}

func (x SignResponse_Status) String() string {
	return protoimpl.X.EnumStringOf(x.Descriptor(), protoreflect.EnumNumber(x))
}

func (SignResponse_Status) Descriptor() protoreflect.EnumDescriptor {
	return file_proto_validator_accounts_v2_keymanager_proto_enumTypes[0].Descriptor()
}

func (SignResponse_Status) Type() protoreflect.EnumType {
	return &file_proto_validator_accounts_v2_keymanager_proto_enumTypes[0]
}

func (x SignResponse_Status) Number() protoreflect.EnumNumber {
	return protoreflect.EnumNumber(x)
}

// Deprecated: Use SignResponse_Status.Descriptor instead.
func (SignResponse_Status) EnumDescriptor() ([]byte, []int) {
	return file_proto_validator_accounts_v2_keymanager_proto_rawDescGZIP(), []int{2, 0}
}

type ListPublicKeysResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ValidatingPublicKeys [][]byte `protobuf:"bytes,2,rep,name=validating_public_keys,json=validatingPublicKeys,proto3" json:"validating_public_keys,omitempty"`
}

func (x *ListPublicKeysResponse) Reset() {
	*x = ListPublicKeysResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_validator_accounts_v2_keymanager_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ListPublicKeysResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ListPublicKeysResponse) ProtoMessage() {}

func (x *ListPublicKeysResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_validator_accounts_v2_keymanager_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ListPublicKeysResponse.ProtoReflect.Descriptor instead.
func (*ListPublicKeysResponse) Descriptor() ([]byte, []int) {
	return file_proto_validator_accounts_v2_keymanager_proto_rawDescGZIP(), []int{0}
}

func (x *ListPublicKeysResponse) GetValidatingPublicKeys() [][]byte {
	if x != nil {
		return x.ValidatingPublicKeys
	}
	return nil
}

type SignRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	PublicKey       []byte                                     `protobuf:"bytes,1,opt,name=public_key,json=publicKey,proto3" json:"public_key,omitempty"`
	SigningRoot     []byte                                     `protobuf:"bytes,2,opt,name=signing_root,json=signingRoot,proto3" json:"signing_root,omitempty"`
	SignatureDomain github_com_prysmaticlabs_eth2_types.Domain `protobuf:"bytes,3,opt,name=signature_domain,json=signatureDomain,proto3" json:"signature_domain,omitempty" cast-type:"github.com/prysmaticlabs/eth2-types.Domain" ssz-size:"32"`
	// Types that are assignable to Object:
	//	*SignRequest_Block
	//	*SignRequest_AttestationData
	//	*SignRequest_AggregateAttestationAndProof
	//	*SignRequest_Exit
	//	*SignRequest_Slot
	//	*SignRequest_Epoch
	//	*SignRequest_BlockV2
	Object isSignRequest_Object `protobuf_oneof:"object"`
}

func (x *SignRequest) Reset() {
	*x = SignRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_validator_accounts_v2_keymanager_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignRequest) ProtoMessage() {}

func (x *SignRequest) ProtoReflect() protoreflect.Message {
	mi := &file_proto_validator_accounts_v2_keymanager_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignRequest.ProtoReflect.Descriptor instead.
func (*SignRequest) Descriptor() ([]byte, []int) {
	return file_proto_validator_accounts_v2_keymanager_proto_rawDescGZIP(), []int{1}
}

func (x *SignRequest) GetPublicKey() []byte {
	if x != nil {
		return x.PublicKey
	}
	return nil
}

func (x *SignRequest) GetSigningRoot() []byte {
	if x != nil {
		return x.SigningRoot
	}
	return nil
}

func (x *SignRequest) GetSignatureDomain() github_com_prysmaticlabs_eth2_types.Domain {
	if x != nil {
		return x.SignatureDomain
	}
	return github_com_prysmaticlabs_eth2_types.Domain(nil)
}

func (m *SignRequest) GetObject() isSignRequest_Object {
	if m != nil {
		return m.Object
	}
	return nil
}

func (x *SignRequest) GetBlock() *v1alpha1.BeaconBlock {
	if x, ok := x.GetObject().(*SignRequest_Block); ok {
		return x.Block
	}
	return nil
}

func (x *SignRequest) GetAttestationData() *v1alpha1.AttestationData {
	if x, ok := x.GetObject().(*SignRequest_AttestationData); ok {
		return x.AttestationData
	}
	return nil
}

func (x *SignRequest) GetAggregateAttestationAndProof() *v1alpha1.AggregateAttestationAndProof {
	if x, ok := x.GetObject().(*SignRequest_AggregateAttestationAndProof); ok {
		return x.AggregateAttestationAndProof
	}
	return nil
}

func (x *SignRequest) GetExit() *v1alpha1.VoluntaryExit {
	if x, ok := x.GetObject().(*SignRequest_Exit); ok {
		return x.Exit
	}
	return nil
}

func (x *SignRequest) GetSlot() github_com_prysmaticlabs_eth2_types.Slot {
	if x, ok := x.GetObject().(*SignRequest_Slot); ok {
		return x.Slot
	}
	return github_com_prysmaticlabs_eth2_types.Slot(0)
}

func (x *SignRequest) GetEpoch() github_com_prysmaticlabs_eth2_types.Epoch {
	if x, ok := x.GetObject().(*SignRequest_Epoch); ok {
		return x.Epoch
	}
	return github_com_prysmaticlabs_eth2_types.Epoch(0)
}

func (x *SignRequest) GetBlockV2() *v2.BeaconBlockAltair {
	if x, ok := x.GetObject().(*SignRequest_BlockV2); ok {
		return x.BlockV2
	}
	return nil
}

type isSignRequest_Object interface {
	isSignRequest_Object()
}

type SignRequest_Block struct {
	Block *v1alpha1.BeaconBlock `protobuf:"bytes,101,opt,name=block,proto3,oneof"`
}

type SignRequest_AttestationData struct {
	AttestationData *v1alpha1.AttestationData `protobuf:"bytes,102,opt,name=attestation_data,json=attestationData,proto3,oneof"`
}

type SignRequest_AggregateAttestationAndProof struct {
	AggregateAttestationAndProof *v1alpha1.AggregateAttestationAndProof `protobuf:"bytes,103,opt,name=aggregate_attestation_and_proof,json=aggregateAttestationAndProof,proto3,oneof"`
}

type SignRequest_Exit struct {
	Exit *v1alpha1.VoluntaryExit `protobuf:"bytes,104,opt,name=exit,proto3,oneof"`
}

type SignRequest_Slot struct {
	Slot github_com_prysmaticlabs_eth2_types.Slot `protobuf:"varint,105,opt,name=slot,proto3,oneof" cast-type:"github.com/prysmaticlabs/eth2-types.Slot"`
}

type SignRequest_Epoch struct {
	Epoch github_com_prysmaticlabs_eth2_types.Epoch `protobuf:"varint,106,opt,name=epoch,proto3,oneof" cast-type:"github.com/prysmaticlabs/eth2-types.Epoch"`
}

type SignRequest_BlockV2 struct {
	BlockV2 *v2.BeaconBlockAltair `protobuf:"bytes,107,opt,name=blockV2,proto3,oneof"`
}

func (*SignRequest_Block) isSignRequest_Object() {}

func (*SignRequest_AttestationData) isSignRequest_Object() {}

func (*SignRequest_AggregateAttestationAndProof) isSignRequest_Object() {}

func (*SignRequest_Exit) isSignRequest_Object() {}

func (*SignRequest_Slot) isSignRequest_Object() {}

func (*SignRequest_Epoch) isSignRequest_Object() {}

func (*SignRequest_BlockV2) isSignRequest_Object() {}

type SignResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Signature []byte              `protobuf:"bytes,1,opt,name=signature,proto3" json:"signature,omitempty"`
	Status    SignResponse_Status `protobuf:"varint,2,opt,name=status,proto3,enum=ethereum.validator.accounts.v2.SignResponse_Status" json:"status,omitempty"`
}

func (x *SignResponse) Reset() {
	*x = SignResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_validator_accounts_v2_keymanager_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SignResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SignResponse) ProtoMessage() {}

func (x *SignResponse) ProtoReflect() protoreflect.Message {
	mi := &file_proto_validator_accounts_v2_keymanager_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SignResponse.ProtoReflect.Descriptor instead.
func (*SignResponse) Descriptor() ([]byte, []int) {
	return file_proto_validator_accounts_v2_keymanager_proto_rawDescGZIP(), []int{2}
}

func (x *SignResponse) GetSignature() []byte {
	if x != nil {
		return x.Signature
	}
	return nil
}

func (x *SignResponse) GetStatus() SignResponse_Status {
	if x != nil {
		return x.Status
	}
	return SignResponse_UNKNOWN
}

var File_proto_validator_accounts_v2_keymanager_proto protoreflect.FileDescriptor

var file_proto_validator_accounts_v2_keymanager_proto_rawDesc = []byte{
	0x0a, 0x2c, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f,
	0x72, 0x2f, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73, 0x2f, 0x76, 0x32, 0x2f, 0x6b, 0x65,
	0x79, 0x6d, 0x61, 0x6e, 0x61, 0x67, 0x65, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x1e,
	0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74,
	0x6f, 0x72, 0x2e, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73, 0x2e, 0x76, 0x32, 0x1a, 0x1b,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x74, 0x68, 0x2f, 0x65, 0x78, 0x74, 0x2f, 0x6f, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x24, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x2f, 0x65, 0x74, 0x68, 0x2f, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f,
	0x61, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x1a, 0x25, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x74, 0x68, 0x2f, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x5f, 0x62, 0x6c, 0x6f,
	0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x21, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f,
	0x70, 0x72, 0x79, 0x73, 0x6d, 0x2f, 0x76, 0x32, 0x2f, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x5f,
	0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1c, 0x67, 0x6f, 0x6f,
	0x67, 0x6c, 0x65, 0x2f, 0x61, 0x70, 0x69, 0x2f, 0x61, 0x6e, 0x6e, 0x6f, 0x74, 0x61, 0x74, 0x69,
	0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1b, 0x67, 0x6f, 0x6f, 0x67, 0x6c,
	0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2f, 0x65, 0x6d, 0x70, 0x74, 0x79,
	0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0x4e, 0x0a, 0x16, 0x4c, 0x69, 0x73, 0x74, 0x50, 0x75,
	0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65, 0x79, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65,
	0x12, 0x34, 0x0a, 0x16, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x69, 0x6e, 0x67, 0x5f, 0x70,
	0x75, 0x62, 0x6c, 0x69, 0x63, 0x5f, 0x6b, 0x65, 0x79, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x0c,
	0x52, 0x14, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x69, 0x6e, 0x67, 0x50, 0x75, 0x62, 0x6c,
	0x69, 0x63, 0x4b, 0x65, 0x79, 0x73, 0x22, 0xd2, 0x05, 0x0a, 0x0b, 0x53, 0x69, 0x67, 0x6e, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x70, 0x75, 0x62, 0x6c, 0x69, 0x63,
	0x5f, 0x6b, 0x65, 0x79, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x70, 0x75, 0x62, 0x6c,
	0x69, 0x63, 0x4b, 0x65, 0x79, 0x12, 0x21, 0x0a, 0x0c, 0x73, 0x69, 0x67, 0x6e, 0x69, 0x6e, 0x67,
	0x5f, 0x72, 0x6f, 0x6f, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x73, 0x69, 0x67,
	0x6e, 0x69, 0x6e, 0x67, 0x52, 0x6f, 0x6f, 0x74, 0x12, 0x5f, 0x0a, 0x10, 0x73, 0x69, 0x67, 0x6e,
	0x61, 0x74, 0x75, 0x72, 0x65, 0x5f, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x18, 0x03, 0x20, 0x01,
	0x28, 0x0c, 0x42, 0x34, 0x82, 0xb5, 0x18, 0x2a, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x61, 0x74, 0x69, 0x63, 0x6c, 0x61, 0x62, 0x73,
	0x2f, 0x65, 0x74, 0x68, 0x32, 0x2d, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x44, 0x6f, 0x6d, 0x61,
	0x69, 0x6e, 0x8a, 0xb5, 0x18, 0x02, 0x33, 0x32, 0x52, 0x0f, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74,
	0x75, 0x72, 0x65, 0x44, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x12, 0x3a, 0x0a, 0x05, 0x62, 0x6c, 0x6f,
	0x63, 0x6b, 0x18, 0x65, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x22, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72,
	0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0x2e, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x48, 0x00, 0x52, 0x05,
	0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x53, 0x0a, 0x10, 0x61, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61,
	0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x18, 0x66, 0x20, 0x01, 0x28, 0x0b, 0x32,
	0x26, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76,
	0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x41, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x44, 0x61, 0x74, 0x61, 0x48, 0x00, 0x52, 0x0f, 0x61, 0x74, 0x74, 0x65, 0x73,
	0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x44, 0x61, 0x74, 0x61, 0x12, 0x7c, 0x0a, 0x1f, 0x61, 0x67,
	0x67, 0x72, 0x65, 0x67, 0x61, 0x74, 0x65, 0x5f, 0x61, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x5f, 0x61, 0x6e, 0x64, 0x5f, 0x70, 0x72, 0x6f, 0x6f, 0x66, 0x18, 0x67, 0x20,
	0x01, 0x28, 0x0b, 0x32, 0x33, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x65,
	0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x41, 0x67, 0x67, 0x72,
	0x65, 0x67, 0x61, 0x74, 0x65, 0x41, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x41, 0x6e, 0x64, 0x50, 0x72, 0x6f, 0x6f, 0x66, 0x48, 0x00, 0x52, 0x1c, 0x61, 0x67, 0x67, 0x72,
	0x65, 0x67, 0x61, 0x74, 0x65, 0x41, 0x74, 0x74, 0x65, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6f, 0x6e,
	0x41, 0x6e, 0x64, 0x50, 0x72, 0x6f, 0x6f, 0x66, 0x12, 0x3a, 0x0a, 0x04, 0x65, 0x78, 0x69, 0x74,
	0x18, 0x68, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x24, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75,
	0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x56,
	0x6f, 0x6c, 0x75, 0x6e, 0x74, 0x61, 0x72, 0x79, 0x45, 0x78, 0x69, 0x74, 0x48, 0x00, 0x52, 0x04,
	0x65, 0x78, 0x69, 0x74, 0x12, 0x42, 0x0a, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x18, 0x69, 0x20, 0x01,
	0x28, 0x04, 0x42, 0x2c, 0x82, 0xb5, 0x18, 0x28, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x61, 0x74, 0x69, 0x63, 0x6c, 0x61, 0x62, 0x73,
	0x2f, 0x65, 0x74, 0x68, 0x32, 0x2d, 0x74, 0x79, 0x70, 0x65, 0x73, 0x2e, 0x53, 0x6c, 0x6f, 0x74,
	0x48, 0x00, 0x52, 0x04, 0x73, 0x6c, 0x6f, 0x74, 0x12, 0x45, 0x0a, 0x05, 0x65, 0x70, 0x6f, 0x63,
	0x68, 0x18, 0x6a, 0x20, 0x01, 0x28, 0x04, 0x42, 0x2d, 0x82, 0xb5, 0x18, 0x29, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x61, 0x74, 0x69,
	0x63, 0x6c, 0x61, 0x62, 0x73, 0x2f, 0x65, 0x74, 0x68, 0x32, 0x2d, 0x74, 0x79, 0x70, 0x65, 0x73,
	0x2e, 0x45, 0x70, 0x6f, 0x63, 0x68, 0x48, 0x00, 0x52, 0x05, 0x65, 0x70, 0x6f, 0x63, 0x68, 0x12,
	0x40, 0x0a, 0x07, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x56, 0x32, 0x18, 0x6b, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x24, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x70, 0x72, 0x79, 0x73,
	0x6d, 0x2e, 0x76, 0x32, 0x2e, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x42, 0x6c, 0x6f, 0x63, 0x6b,
	0x41, 0x6c, 0x74, 0x61, 0x69, 0x72, 0x48, 0x00, 0x52, 0x07, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x56,
	0x32, 0x42, 0x08, 0x0a, 0x06, 0x6f, 0x62, 0x6a, 0x65, 0x63, 0x74, 0x22, 0xb7, 0x01, 0x0a, 0x0c,
	0x53, 0x69, 0x67, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1c, 0x0a, 0x09,
	0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0c, 0x52,
	0x09, 0x73, 0x69, 0x67, 0x6e, 0x61, 0x74, 0x75, 0x72, 0x65, 0x12, 0x4b, 0x0a, 0x06, 0x73, 0x74,
	0x61, 0x74, 0x75, 0x73, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0e, 0x32, 0x33, 0x2e, 0x65, 0x74, 0x68,
	0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x2e,
	0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x53, 0x69, 0x67, 0x6e,
	0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x2e, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x52,
	0x06, 0x73, 0x74, 0x61, 0x74, 0x75, 0x73, 0x22, 0x3c, 0x0a, 0x06, 0x53, 0x74, 0x61, 0x74, 0x75,
	0x73, 0x12, 0x0b, 0x0a, 0x07, 0x55, 0x4e, 0x4b, 0x4e, 0x4f, 0x57, 0x4e, 0x10, 0x00, 0x12, 0x0d,
	0x0a, 0x09, 0x53, 0x55, 0x43, 0x43, 0x45, 0x45, 0x44, 0x45, 0x44, 0x10, 0x01, 0x12, 0x0a, 0x0a,
	0x06, 0x44, 0x45, 0x4e, 0x49, 0x45, 0x44, 0x10, 0x02, 0x12, 0x0a, 0x0a, 0x06, 0x46, 0x41, 0x49,
	0x4c, 0x45, 0x44, 0x10, 0x03, 0x32, 0xa7, 0x02, 0x0a, 0x0c, 0x52, 0x65, 0x6d, 0x6f, 0x74, 0x65,
	0x53, 0x69, 0x67, 0x6e, 0x65, 0x72, 0x12, 0x90, 0x01, 0x0a, 0x18, 0x4c, 0x69, 0x73, 0x74, 0x56,
	0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x69, 0x6e, 0x67, 0x50, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x4b,
	0x65, 0x79, 0x73, 0x12, 0x16, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x45, 0x6d, 0x70, 0x74, 0x79, 0x1a, 0x36, 0x2e, 0x65, 0x74,
	0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72,
	0x2e, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73, 0x2e, 0x76, 0x32, 0x2e, 0x4c, 0x69, 0x73,
	0x74, 0x50, 0x75, 0x62, 0x6c, 0x69, 0x63, 0x4b, 0x65, 0x79, 0x73, 0x52, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x22, 0x24, 0x82, 0xd3, 0xe4, 0x93, 0x02, 0x1e, 0x12, 0x1c, 0x2f, 0x61, 0x63,
	0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73, 0x2f, 0x76, 0x32, 0x2f, 0x72, 0x65, 0x6d, 0x6f, 0x74, 0x65,
	0x2f, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73, 0x12, 0x83, 0x01, 0x0a, 0x04, 0x53, 0x69,
	0x67, 0x6e, 0x12, 0x2b, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x76, 0x61,
	0x6c, 0x69, 0x64, 0x61, 0x74, 0x6f, 0x72, 0x2e, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73,
	0x2e, 0x76, 0x32, 0x2e, 0x53, 0x69, 0x67, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x2c, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x76, 0x61, 0x6c, 0x69, 0x64,
	0x61, 0x74, 0x6f, 0x72, 0x2e, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73, 0x2e, 0x76, 0x32,
	0x2e, 0x53, 0x69, 0x67, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x20, 0x82,
	0xd3, 0xe4, 0x93, 0x02, 0x1a, 0x22, 0x18, 0x2f, 0x61, 0x63, 0x63, 0x6f, 0x75, 0x6e, 0x74, 0x73,
	0x2f, 0x76, 0x32, 0x2f, 0x72, 0x65, 0x6d, 0x6f, 0x74, 0x65, 0x2f, 0x73, 0x69, 0x67, 0x6e, 0x62,
	0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_validator_accounts_v2_keymanager_proto_rawDescOnce sync.Once
	file_proto_validator_accounts_v2_keymanager_proto_rawDescData = file_proto_validator_accounts_v2_keymanager_proto_rawDesc
)

func file_proto_validator_accounts_v2_keymanager_proto_rawDescGZIP() []byte {
	file_proto_validator_accounts_v2_keymanager_proto_rawDescOnce.Do(func() {
		file_proto_validator_accounts_v2_keymanager_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_validator_accounts_v2_keymanager_proto_rawDescData)
	})
	return file_proto_validator_accounts_v2_keymanager_proto_rawDescData
}

var file_proto_validator_accounts_v2_keymanager_proto_enumTypes = make([]protoimpl.EnumInfo, 1)
var file_proto_validator_accounts_v2_keymanager_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_proto_validator_accounts_v2_keymanager_proto_goTypes = []interface{}{
	(SignResponse_Status)(0),                      // 0: ethereum.validator.accounts.v2.SignResponse.Status
	(*ListPublicKeysResponse)(nil),                // 1: ethereum.validator.accounts.v2.ListPublicKeysResponse
	(*SignRequest)(nil),                           // 2: ethereum.validator.accounts.v2.SignRequest
	(*SignResponse)(nil),                          // 3: ethereum.validator.accounts.v2.SignResponse
	(*v1alpha1.BeaconBlock)(nil),                  // 4: ethereum.eth.v1alpha1.BeaconBlock
	(*v1alpha1.AttestationData)(nil),              // 5: ethereum.eth.v1alpha1.AttestationData
	(*v1alpha1.AggregateAttestationAndProof)(nil), // 6: ethereum.eth.v1alpha1.AggregateAttestationAndProof
	(*v1alpha1.VoluntaryExit)(nil),                // 7: ethereum.eth.v1alpha1.VoluntaryExit
	(*v2.BeaconBlockAltair)(nil),                  // 8: ethereum.prysm.v2.BeaconBlockAltair
	(*empty.Empty)(nil),                           // 9: google.protobuf.Empty
}
var file_proto_validator_accounts_v2_keymanager_proto_depIdxs = []int32{
	4, // 0: ethereum.validator.accounts.v2.SignRequest.block:type_name -> ethereum.eth.v1alpha1.BeaconBlock
	5, // 1: ethereum.validator.accounts.v2.SignRequest.attestation_data:type_name -> ethereum.eth.v1alpha1.AttestationData
	6, // 2: ethereum.validator.accounts.v2.SignRequest.aggregate_attestation_and_proof:type_name -> ethereum.eth.v1alpha1.AggregateAttestationAndProof
	7, // 3: ethereum.validator.accounts.v2.SignRequest.exit:type_name -> ethereum.eth.v1alpha1.VoluntaryExit
	8, // 4: ethereum.validator.accounts.v2.SignRequest.blockV2:type_name -> ethereum.prysm.v2.BeaconBlockAltair
	0, // 5: ethereum.validator.accounts.v2.SignResponse.status:type_name -> ethereum.validator.accounts.v2.SignResponse.Status
	9, // 6: ethereum.validator.accounts.v2.RemoteSigner.ListValidatingPublicKeys:input_type -> google.protobuf.Empty
	2, // 7: ethereum.validator.accounts.v2.RemoteSigner.Sign:input_type -> ethereum.validator.accounts.v2.SignRequest
	1, // 8: ethereum.validator.accounts.v2.RemoteSigner.ListValidatingPublicKeys:output_type -> ethereum.validator.accounts.v2.ListPublicKeysResponse
	3, // 9: ethereum.validator.accounts.v2.RemoteSigner.Sign:output_type -> ethereum.validator.accounts.v2.SignResponse
	8, // [8:10] is the sub-list for method output_type
	6, // [6:8] is the sub-list for method input_type
	6, // [6:6] is the sub-list for extension type_name
	6, // [6:6] is the sub-list for extension extendee
	0, // [0:6] is the sub-list for field type_name
}

func init() { file_proto_validator_accounts_v2_keymanager_proto_init() }
func file_proto_validator_accounts_v2_keymanager_proto_init() {
	if File_proto_validator_accounts_v2_keymanager_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_validator_accounts_v2_keymanager_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ListPublicKeysResponse); i {
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
		file_proto_validator_accounts_v2_keymanager_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignRequest); i {
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
		file_proto_validator_accounts_v2_keymanager_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SignResponse); i {
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
	file_proto_validator_accounts_v2_keymanager_proto_msgTypes[1].OneofWrappers = []interface{}{
		(*SignRequest_Block)(nil),
		(*SignRequest_AttestationData)(nil),
		(*SignRequest_AggregateAttestationAndProof)(nil),
		(*SignRequest_Exit)(nil),
		(*SignRequest_Slot)(nil),
		(*SignRequest_Epoch)(nil),
		(*SignRequest_BlockV2)(nil),
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_validator_accounts_v2_keymanager_proto_rawDesc,
			NumEnums:      1,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_proto_validator_accounts_v2_keymanager_proto_goTypes,
		DependencyIndexes: file_proto_validator_accounts_v2_keymanager_proto_depIdxs,
		EnumInfos:         file_proto_validator_accounts_v2_keymanager_proto_enumTypes,
		MessageInfos:      file_proto_validator_accounts_v2_keymanager_proto_msgTypes,
	}.Build()
	File_proto_validator_accounts_v2_keymanager_proto = out.File
	file_proto_validator_accounts_v2_keymanager_proto_rawDesc = nil
	file_proto_validator_accounts_v2_keymanager_proto_goTypes = nil
	file_proto_validator_accounts_v2_keymanager_proto_depIdxs = nil
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConnInterface

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion6

// RemoteSignerClient is the client API for RemoteSigner service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type RemoteSignerClient interface {
	ListValidatingPublicKeys(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*ListPublicKeysResponse, error)
	Sign(ctx context.Context, in *SignRequest, opts ...grpc.CallOption) (*SignResponse, error)
}

type remoteSignerClient struct {
	cc grpc.ClientConnInterface
}

func NewRemoteSignerClient(cc grpc.ClientConnInterface) RemoteSignerClient {
	return &remoteSignerClient{cc}
}

func (c *remoteSignerClient) ListValidatingPublicKeys(ctx context.Context, in *empty.Empty, opts ...grpc.CallOption) (*ListPublicKeysResponse, error) {
	out := new(ListPublicKeysResponse)
	err := c.cc.Invoke(ctx, "/ethereum.validator.accounts.v2.RemoteSigner/ListValidatingPublicKeys", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *remoteSignerClient) Sign(ctx context.Context, in *SignRequest, opts ...grpc.CallOption) (*SignResponse, error) {
	out := new(SignResponse)
	err := c.cc.Invoke(ctx, "/ethereum.validator.accounts.v2.RemoteSigner/Sign", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RemoteSignerServer is the server API for RemoteSigner service.
type RemoteSignerServer interface {
	ListValidatingPublicKeys(context.Context, *empty.Empty) (*ListPublicKeysResponse, error)
	Sign(context.Context, *SignRequest) (*SignResponse, error)
}

// UnimplementedRemoteSignerServer can be embedded to have forward compatible implementations.
type UnimplementedRemoteSignerServer struct {
}

func (*UnimplementedRemoteSignerServer) ListValidatingPublicKeys(context.Context, *empty.Empty) (*ListPublicKeysResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListValidatingPublicKeys not implemented")
}
func (*UnimplementedRemoteSignerServer) Sign(context.Context, *SignRequest) (*SignResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Sign not implemented")
}

func RegisterRemoteSignerServer(s *grpc.Server, srv RemoteSignerServer) {
	s.RegisterService(&_RemoteSigner_serviceDesc, srv)
}

func _RemoteSigner_ListValidatingPublicKeys_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(empty.Empty)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RemoteSignerServer).ListValidatingPublicKeys(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ethereum.validator.accounts.v2.RemoteSigner/ListValidatingPublicKeys",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RemoteSignerServer).ListValidatingPublicKeys(ctx, req.(*empty.Empty))
	}
	return interceptor(ctx, in, info, handler)
}

func _RemoteSigner_Sign_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SignRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(RemoteSignerServer).Sign(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/ethereum.validator.accounts.v2.RemoteSigner/Sign",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(RemoteSignerServer).Sign(ctx, req.(*SignRequest))
	}
	return interceptor(ctx, in, info, handler)
}

var _RemoteSigner_serviceDesc = grpc.ServiceDesc{
	ServiceName: "ethereum.validator.accounts.v2.RemoteSigner",
	HandlerType: (*RemoteSignerServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ListValidatingPublicKeys",
			Handler:    _RemoteSigner_ListValidatingPublicKeys_Handler,
		},
		{
			MethodName: "Sign",
			Handler:    _RemoteSigner_Sign_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/validator/accounts/v2/keymanager.proto",
}
