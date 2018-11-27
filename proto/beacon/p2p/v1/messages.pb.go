// Code generated by protoc-gen-go. DO NOT EDIT.
// source: proto/beacon/p2p/v1/messages.proto

package v1

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Topic int32

const (
	Topic_UNKNOWN                             Topic = 0
	Topic_BEACON_BLOCK_ANNOUNCE               Topic = 1
	Topic_BEACON_BLOCK_REQUEST                Topic = 2
	Topic_BEACON_BLOCK_REQUEST_BY_SLOT_NUMBER Topic = 3
	Topic_BEACON_BLOCK_RESPONSE               Topic = 4
	Topic_BATCHED_BEACON_BLOCK_REQUEST        Topic = 5
	Topic_BATCHED_BEACON_BLOCK_RESPONSE       Topic = 6
	Topic_CHAIN_HEAD_REQUEST                  Topic = 7
	Topic_CHAIN_HEAD_RESPONSE                 Topic = 8
	Topic_CRYSTALLIZED_STATE_HASH_ANNOUNCE    Topic = 9
	Topic_CRYSTALLIZED_STATE_REQUEST          Topic = 10
	Topic_CRYSTALLIZED_STATE_RESPONSE         Topic = 11
	Topic_ACTIVE_STATE_HASH_ANNOUNCE          Topic = 12
	Topic_ACTIVE_STATE_REQUEST                Topic = 13
	Topic_ACTIVE_STATE_RESPONSE               Topic = 14
)

var Topic_name = map[int32]string{
	0:  "UNKNOWN",
	1:  "BEACON_BLOCK_ANNOUNCE",
	2:  "BEACON_BLOCK_REQUEST",
	3:  "BEACON_BLOCK_REQUEST_BY_SLOT_NUMBER",
	4:  "BEACON_BLOCK_RESPONSE",
	5:  "BATCHED_BEACON_BLOCK_REQUEST",
	6:  "BATCHED_BEACON_BLOCK_RESPONSE",
	7:  "CHAIN_HEAD_REQUEST",
	8:  "CHAIN_HEAD_RESPONSE",
	9:  "CRYSTALLIZED_STATE_HASH_ANNOUNCE",
	10: "CRYSTALLIZED_STATE_REQUEST",
	11: "CRYSTALLIZED_STATE_RESPONSE",
	12: "ACTIVE_STATE_HASH_ANNOUNCE",
	13: "ACTIVE_STATE_REQUEST",
	14: "ACTIVE_STATE_RESPONSE",
}
var Topic_value = map[string]int32{
	"UNKNOWN":                             0,
	"BEACON_BLOCK_ANNOUNCE":               1,
	"BEACON_BLOCK_REQUEST":                2,
	"BEACON_BLOCK_REQUEST_BY_SLOT_NUMBER": 3,
	"BEACON_BLOCK_RESPONSE":               4,
	"BATCHED_BEACON_BLOCK_REQUEST":        5,
	"BATCHED_BEACON_BLOCK_RESPONSE":       6,
	"CHAIN_HEAD_REQUEST":                  7,
	"CHAIN_HEAD_RESPONSE":                 8,
	"CRYSTALLIZED_STATE_HASH_ANNOUNCE":    9,
	"CRYSTALLIZED_STATE_REQUEST":          10,
	"CRYSTALLIZED_STATE_RESPONSE":         11,
	"ACTIVE_STATE_HASH_ANNOUNCE":          12,
	"ACTIVE_STATE_REQUEST":                13,
	"ACTIVE_STATE_RESPONSE":               14,
}

func (x Topic) String() string {
	return proto.EnumName(Topic_name, int32(x))
}
func (Topic) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{0}
}

type BeaconBlockAnnounce struct {
	Hash                 []byte   `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	SlotNumber           uint64   `protobuf:"varint,2,opt,name=slot_number,json=slotNumber,proto3" json:"slot_number,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BeaconBlockAnnounce) Reset()         { *m = BeaconBlockAnnounce{} }
func (m *BeaconBlockAnnounce) String() string { return proto.CompactTextString(m) }
func (*BeaconBlockAnnounce) ProtoMessage()    {}
func (*BeaconBlockAnnounce) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{0}
}
func (m *BeaconBlockAnnounce) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BeaconBlockAnnounce.Unmarshal(m, b)
}
func (m *BeaconBlockAnnounce) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BeaconBlockAnnounce.Marshal(b, m, deterministic)
}
func (dst *BeaconBlockAnnounce) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BeaconBlockAnnounce.Merge(dst, src)
}
func (m *BeaconBlockAnnounce) XXX_Size() int {
	return xxx_messageInfo_BeaconBlockAnnounce.Size(m)
}
func (m *BeaconBlockAnnounce) XXX_DiscardUnknown() {
	xxx_messageInfo_BeaconBlockAnnounce.DiscardUnknown(m)
}

var xxx_messageInfo_BeaconBlockAnnounce proto.InternalMessageInfo

func (m *BeaconBlockAnnounce) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

func (m *BeaconBlockAnnounce) GetSlotNumber() uint64 {
	if m != nil {
		return m.SlotNumber
	}
	return 0
}

type BeaconBlockRequest struct {
	Hash                 []byte   `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BeaconBlockRequest) Reset()         { *m = BeaconBlockRequest{} }
func (m *BeaconBlockRequest) String() string { return proto.CompactTextString(m) }
func (*BeaconBlockRequest) ProtoMessage()    {}
func (*BeaconBlockRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{1}
}
func (m *BeaconBlockRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BeaconBlockRequest.Unmarshal(m, b)
}
func (m *BeaconBlockRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BeaconBlockRequest.Marshal(b, m, deterministic)
}
func (dst *BeaconBlockRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BeaconBlockRequest.Merge(dst, src)
}
func (m *BeaconBlockRequest) XXX_Size() int {
	return xxx_messageInfo_BeaconBlockRequest.Size(m)
}
func (m *BeaconBlockRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_BeaconBlockRequest.DiscardUnknown(m)
}

var xxx_messageInfo_BeaconBlockRequest proto.InternalMessageInfo

func (m *BeaconBlockRequest) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

type BeaconBlockRequestBySlotNumber struct {
	SlotNumber           uint64   `protobuf:"varint,1,opt,name=slot_number,json=slotNumber,proto3" json:"slot_number,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BeaconBlockRequestBySlotNumber) Reset()         { *m = BeaconBlockRequestBySlotNumber{} }
func (m *BeaconBlockRequestBySlotNumber) String() string { return proto.CompactTextString(m) }
func (*BeaconBlockRequestBySlotNumber) ProtoMessage()    {}
func (*BeaconBlockRequestBySlotNumber) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{2}
}
func (m *BeaconBlockRequestBySlotNumber) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BeaconBlockRequestBySlotNumber.Unmarshal(m, b)
}
func (m *BeaconBlockRequestBySlotNumber) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BeaconBlockRequestBySlotNumber.Marshal(b, m, deterministic)
}
func (dst *BeaconBlockRequestBySlotNumber) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BeaconBlockRequestBySlotNumber.Merge(dst, src)
}
func (m *BeaconBlockRequestBySlotNumber) XXX_Size() int {
	return xxx_messageInfo_BeaconBlockRequestBySlotNumber.Size(m)
}
func (m *BeaconBlockRequestBySlotNumber) XXX_DiscardUnknown() {
	xxx_messageInfo_BeaconBlockRequestBySlotNumber.DiscardUnknown(m)
}

var xxx_messageInfo_BeaconBlockRequestBySlotNumber proto.InternalMessageInfo

func (m *BeaconBlockRequestBySlotNumber) GetSlotNumber() uint64 {
	if m != nil {
		return m.SlotNumber
	}
	return 0
}

type BeaconBlockResponse struct {
	Block                *BeaconBlock           `protobuf:"bytes,1,opt,name=block,proto3" json:"block,omitempty"`
	Attestation          *AggregatedAttestation `protobuf:"bytes,2,opt,name=attestation,proto3" json:"attestation,omitempty"`
	XXX_NoUnkeyedLiteral struct{}               `json:"-"`
	XXX_unrecognized     []byte                 `json:"-"`
	XXX_sizecache        int32                  `json:"-"`
}

func (m *BeaconBlockResponse) Reset()         { *m = BeaconBlockResponse{} }
func (m *BeaconBlockResponse) String() string { return proto.CompactTextString(m) }
func (*BeaconBlockResponse) ProtoMessage()    {}
func (*BeaconBlockResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{3}
}
func (m *BeaconBlockResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BeaconBlockResponse.Unmarshal(m, b)
}
func (m *BeaconBlockResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BeaconBlockResponse.Marshal(b, m, deterministic)
}
func (dst *BeaconBlockResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BeaconBlockResponse.Merge(dst, src)
}
func (m *BeaconBlockResponse) XXX_Size() int {
	return xxx_messageInfo_BeaconBlockResponse.Size(m)
}
func (m *BeaconBlockResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_BeaconBlockResponse.DiscardUnknown(m)
}

var xxx_messageInfo_BeaconBlockResponse proto.InternalMessageInfo

func (m *BeaconBlockResponse) GetBlock() *BeaconBlock {
	if m != nil {
		return m.Block
	}
	return nil
}

func (m *BeaconBlockResponse) GetAttestation() *AggregatedAttestation {
	if m != nil {
		return m.Attestation
	}
	return nil
}

type BatchedBeaconBlockRequest struct {
	SlotNumber           uint64   `protobuf:"varint,1,opt,name=slot_number,json=slotNumber,proto3" json:"slot_number,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BatchedBeaconBlockRequest) Reset()         { *m = BatchedBeaconBlockRequest{} }
func (m *BatchedBeaconBlockRequest) String() string { return proto.CompactTextString(m) }
func (*BatchedBeaconBlockRequest) ProtoMessage()    {}
func (*BatchedBeaconBlockRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{4}
}
func (m *BatchedBeaconBlockRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BatchedBeaconBlockRequest.Unmarshal(m, b)
}
func (m *BatchedBeaconBlockRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BatchedBeaconBlockRequest.Marshal(b, m, deterministic)
}
func (dst *BatchedBeaconBlockRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BatchedBeaconBlockRequest.Merge(dst, src)
}
func (m *BatchedBeaconBlockRequest) XXX_Size() int {
	return xxx_messageInfo_BatchedBeaconBlockRequest.Size(m)
}
func (m *BatchedBeaconBlockRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_BatchedBeaconBlockRequest.DiscardUnknown(m)
}

var xxx_messageInfo_BatchedBeaconBlockRequest proto.InternalMessageInfo

func (m *BatchedBeaconBlockRequest) GetSlotNumber() uint64 {
	if m != nil {
		return m.SlotNumber
	}
	return 0
}

type BatchedBeaconBlockResponse struct {
	BatchedBlocks        []*BeaconBlock `protobuf:"bytes,1,rep,name=batched_blocks,json=batchedBlocks,proto3" json:"batched_blocks,omitempty"`
	XXX_NoUnkeyedLiteral struct{}       `json:"-"`
	XXX_unrecognized     []byte         `json:"-"`
	XXX_sizecache        int32          `json:"-"`
}

func (m *BatchedBeaconBlockResponse) Reset()         { *m = BatchedBeaconBlockResponse{} }
func (m *BatchedBeaconBlockResponse) String() string { return proto.CompactTextString(m) }
func (*BatchedBeaconBlockResponse) ProtoMessage()    {}
func (*BatchedBeaconBlockResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{5}
}
func (m *BatchedBeaconBlockResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BatchedBeaconBlockResponse.Unmarshal(m, b)
}
func (m *BatchedBeaconBlockResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BatchedBeaconBlockResponse.Marshal(b, m, deterministic)
}
func (dst *BatchedBeaconBlockResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BatchedBeaconBlockResponse.Merge(dst, src)
}
func (m *BatchedBeaconBlockResponse) XXX_Size() int {
	return xxx_messageInfo_BatchedBeaconBlockResponse.Size(m)
}
func (m *BatchedBeaconBlockResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_BatchedBeaconBlockResponse.DiscardUnknown(m)
}

var xxx_messageInfo_BatchedBeaconBlockResponse proto.InternalMessageInfo

func (m *BatchedBeaconBlockResponse) GetBatchedBlocks() []*BeaconBlock {
	if m != nil {
		return m.BatchedBlocks
	}
	return nil
}

type ChainHeadRequest struct {
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ChainHeadRequest) Reset()         { *m = ChainHeadRequest{} }
func (m *ChainHeadRequest) String() string { return proto.CompactTextString(m) }
func (*ChainHeadRequest) ProtoMessage()    {}
func (*ChainHeadRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{6}
}
func (m *ChainHeadRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ChainHeadRequest.Unmarshal(m, b)
}
func (m *ChainHeadRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ChainHeadRequest.Marshal(b, m, deterministic)
}
func (dst *ChainHeadRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ChainHeadRequest.Merge(dst, src)
}
func (m *ChainHeadRequest) XXX_Size() int {
	return xxx_messageInfo_ChainHeadRequest.Size(m)
}
func (m *ChainHeadRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ChainHeadRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ChainHeadRequest proto.InternalMessageInfo

type ChainHeadResponse struct {
	Hash                 []byte       `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	Slot                 uint64       `protobuf:"varint,2,opt,name=slot,proto3" json:"slot,omitempty"`
	Block                *BeaconBlock `protobuf:"bytes,3,opt,name=block,proto3" json:"block,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *ChainHeadResponse) Reset()         { *m = ChainHeadResponse{} }
func (m *ChainHeadResponse) String() string { return proto.CompactTextString(m) }
func (*ChainHeadResponse) ProtoMessage()    {}
func (*ChainHeadResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{7}
}
func (m *ChainHeadResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ChainHeadResponse.Unmarshal(m, b)
}
func (m *ChainHeadResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ChainHeadResponse.Marshal(b, m, deterministic)
}
func (dst *ChainHeadResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ChainHeadResponse.Merge(dst, src)
}
func (m *ChainHeadResponse) XXX_Size() int {
	return xxx_messageInfo_ChainHeadResponse.Size(m)
}
func (m *ChainHeadResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ChainHeadResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ChainHeadResponse proto.InternalMessageInfo

func (m *ChainHeadResponse) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

func (m *ChainHeadResponse) GetSlot() uint64 {
	if m != nil {
		return m.Slot
	}
	return 0
}

func (m *ChainHeadResponse) GetBlock() *BeaconBlock {
	if m != nil {
		return m.Block
	}
	return nil
}

type CrystallizedStateHashAnnounce struct {
	Hash                 []byte   `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CrystallizedStateHashAnnounce) Reset()         { *m = CrystallizedStateHashAnnounce{} }
func (m *CrystallizedStateHashAnnounce) String() string { return proto.CompactTextString(m) }
func (*CrystallizedStateHashAnnounce) ProtoMessage()    {}
func (*CrystallizedStateHashAnnounce) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{8}
}
func (m *CrystallizedStateHashAnnounce) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CrystallizedStateHashAnnounce.Unmarshal(m, b)
}
func (m *CrystallizedStateHashAnnounce) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CrystallizedStateHashAnnounce.Marshal(b, m, deterministic)
}
func (dst *CrystallizedStateHashAnnounce) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CrystallizedStateHashAnnounce.Merge(dst, src)
}
func (m *CrystallizedStateHashAnnounce) XXX_Size() int {
	return xxx_messageInfo_CrystallizedStateHashAnnounce.Size(m)
}
func (m *CrystallizedStateHashAnnounce) XXX_DiscardUnknown() {
	xxx_messageInfo_CrystallizedStateHashAnnounce.DiscardUnknown(m)
}

var xxx_messageInfo_CrystallizedStateHashAnnounce proto.InternalMessageInfo

func (m *CrystallizedStateHashAnnounce) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

type CrystallizedStateRequest struct {
	Hash                 []byte   `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CrystallizedStateRequest) Reset()         { *m = CrystallizedStateRequest{} }
func (m *CrystallizedStateRequest) String() string { return proto.CompactTextString(m) }
func (*CrystallizedStateRequest) ProtoMessage()    {}
func (*CrystallizedStateRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{9}
}
func (m *CrystallizedStateRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CrystallizedStateRequest.Unmarshal(m, b)
}
func (m *CrystallizedStateRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CrystallizedStateRequest.Marshal(b, m, deterministic)
}
func (dst *CrystallizedStateRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CrystallizedStateRequest.Merge(dst, src)
}
func (m *CrystallizedStateRequest) XXX_Size() int {
	return xxx_messageInfo_CrystallizedStateRequest.Size(m)
}
func (m *CrystallizedStateRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CrystallizedStateRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CrystallizedStateRequest proto.InternalMessageInfo

func (m *CrystallizedStateRequest) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

type CrystallizedStateResponse struct {
	CrystallizedState    *CrystallizedState `protobuf:"bytes,1,opt,name=crystallized_state,json=crystallizedState,proto3" json:"crystallized_state,omitempty"`
	XXX_NoUnkeyedLiteral struct{}           `json:"-"`
	XXX_unrecognized     []byte             `json:"-"`
	XXX_sizecache        int32              `json:"-"`
}

func (m *CrystallizedStateResponse) Reset()         { *m = CrystallizedStateResponse{} }
func (m *CrystallizedStateResponse) String() string { return proto.CompactTextString(m) }
func (*CrystallizedStateResponse) ProtoMessage()    {}
func (*CrystallizedStateResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{10}
}
func (m *CrystallizedStateResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CrystallizedStateResponse.Unmarshal(m, b)
}
func (m *CrystallizedStateResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CrystallizedStateResponse.Marshal(b, m, deterministic)
}
func (dst *CrystallizedStateResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CrystallizedStateResponse.Merge(dst, src)
}
func (m *CrystallizedStateResponse) XXX_Size() int {
	return xxx_messageInfo_CrystallizedStateResponse.Size(m)
}
func (m *CrystallizedStateResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CrystallizedStateResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CrystallizedStateResponse proto.InternalMessageInfo

func (m *CrystallizedStateResponse) GetCrystallizedState() *CrystallizedState {
	if m != nil {
		return m.CrystallizedState
	}
	return nil
}

type ActiveStateHashAnnounce struct {
	Hash                 []byte   `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ActiveStateHashAnnounce) Reset()         { *m = ActiveStateHashAnnounce{} }
func (m *ActiveStateHashAnnounce) String() string { return proto.CompactTextString(m) }
func (*ActiveStateHashAnnounce) ProtoMessage()    {}
func (*ActiveStateHashAnnounce) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{11}
}
func (m *ActiveStateHashAnnounce) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ActiveStateHashAnnounce.Unmarshal(m, b)
}
func (m *ActiveStateHashAnnounce) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ActiveStateHashAnnounce.Marshal(b, m, deterministic)
}
func (dst *ActiveStateHashAnnounce) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActiveStateHashAnnounce.Merge(dst, src)
}
func (m *ActiveStateHashAnnounce) XXX_Size() int {
	return xxx_messageInfo_ActiveStateHashAnnounce.Size(m)
}
func (m *ActiveStateHashAnnounce) XXX_DiscardUnknown() {
	xxx_messageInfo_ActiveStateHashAnnounce.DiscardUnknown(m)
}

var xxx_messageInfo_ActiveStateHashAnnounce proto.InternalMessageInfo

func (m *ActiveStateHashAnnounce) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

type ActiveStateRequest struct {
	Hash                 []byte   `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ActiveStateRequest) Reset()         { *m = ActiveStateRequest{} }
func (m *ActiveStateRequest) String() string { return proto.CompactTextString(m) }
func (*ActiveStateRequest) ProtoMessage()    {}
func (*ActiveStateRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{12}
}
func (m *ActiveStateRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ActiveStateRequest.Unmarshal(m, b)
}
func (m *ActiveStateRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ActiveStateRequest.Marshal(b, m, deterministic)
}
func (dst *ActiveStateRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActiveStateRequest.Merge(dst, src)
}
func (m *ActiveStateRequest) XXX_Size() int {
	return xxx_messageInfo_ActiveStateRequest.Size(m)
}
func (m *ActiveStateRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_ActiveStateRequest.DiscardUnknown(m)
}

var xxx_messageInfo_ActiveStateRequest proto.InternalMessageInfo

func (m *ActiveStateRequest) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
}

type ActiveStateResponse struct {
	ActiveState          *ActiveState `protobuf:"bytes,1,opt,name=active_state,json=activeState,proto3" json:"active_state,omitempty"`
	XXX_NoUnkeyedLiteral struct{}     `json:"-"`
	XXX_unrecognized     []byte       `json:"-"`
	XXX_sizecache        int32        `json:"-"`
}

func (m *ActiveStateResponse) Reset()         { *m = ActiveStateResponse{} }
func (m *ActiveStateResponse) String() string { return proto.CompactTextString(m) }
func (*ActiveStateResponse) ProtoMessage()    {}
func (*ActiveStateResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_8ab03e3ae1769904, []int{13}
}
func (m *ActiveStateResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ActiveStateResponse.Unmarshal(m, b)
}
func (m *ActiveStateResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ActiveStateResponse.Marshal(b, m, deterministic)
}
func (dst *ActiveStateResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ActiveStateResponse.Merge(dst, src)
}
func (m *ActiveStateResponse) XXX_Size() int {
	return xxx_messageInfo_ActiveStateResponse.Size(m)
}
func (m *ActiveStateResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_ActiveStateResponse.DiscardUnknown(m)
}

var xxx_messageInfo_ActiveStateResponse proto.InternalMessageInfo

func (m *ActiveStateResponse) GetActiveState() *ActiveState {
	if m != nil {
		return m.ActiveState
	}
	return nil
}

func init() {
	proto.RegisterType((*BeaconBlockAnnounce)(nil), "ethereum.beacon.p2p.v1.BeaconBlockAnnounce")
	proto.RegisterType((*BeaconBlockRequest)(nil), "ethereum.beacon.p2p.v1.BeaconBlockRequest")
	proto.RegisterType((*BeaconBlockRequestBySlotNumber)(nil), "ethereum.beacon.p2p.v1.BeaconBlockRequestBySlotNumber")
	proto.RegisterType((*BeaconBlockResponse)(nil), "ethereum.beacon.p2p.v1.BeaconBlockResponse")
	proto.RegisterType((*BatchedBeaconBlockRequest)(nil), "ethereum.beacon.p2p.v1.BatchedBeaconBlockRequest")
	proto.RegisterType((*BatchedBeaconBlockResponse)(nil), "ethereum.beacon.p2p.v1.BatchedBeaconBlockResponse")
	proto.RegisterType((*ChainHeadRequest)(nil), "ethereum.beacon.p2p.v1.ChainHeadRequest")
	proto.RegisterType((*ChainHeadResponse)(nil), "ethereum.beacon.p2p.v1.ChainHeadResponse")
	proto.RegisterType((*CrystallizedStateHashAnnounce)(nil), "ethereum.beacon.p2p.v1.CrystallizedStateHashAnnounce")
	proto.RegisterType((*CrystallizedStateRequest)(nil), "ethereum.beacon.p2p.v1.CrystallizedStateRequest")
	proto.RegisterType((*CrystallizedStateResponse)(nil), "ethereum.beacon.p2p.v1.CrystallizedStateResponse")
	proto.RegisterType((*ActiveStateHashAnnounce)(nil), "ethereum.beacon.p2p.v1.ActiveStateHashAnnounce")
	proto.RegisterType((*ActiveStateRequest)(nil), "ethereum.beacon.p2p.v1.ActiveStateRequest")
	proto.RegisterType((*ActiveStateResponse)(nil), "ethereum.beacon.p2p.v1.ActiveStateResponse")
	proto.RegisterEnum("ethereum.beacon.p2p.v1.Topic", Topic_name, Topic_value)
}

func init() {
	proto.RegisterFile("proto/beacon/p2p/v1/messages.proto", fileDescriptor_messages_8ab03e3ae1769904)
}

var fileDescriptor_messages_8ab03e3ae1769904 = []byte{
	// 633 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x54, 0x61, 0x4f, 0xd3, 0x5c,
	0x14, 0x7e, 0xcb, 0x06, 0xbc, 0x9e, 0x02, 0x19, 0x17, 0x85, 0x81, 0x02, 0xb3, 0x98, 0x38, 0x4d,
	0xe8, 0xc2, 0xf8, 0x64, 0xe2, 0x97, 0xdb, 0x52, 0x53, 0x60, 0xb6, 0xda, 0x76, 0x2a, 0x26, 0xa6,
	0xb9, 0xeb, 0x6e, 0xd6, 0xc5, 0xd1, 0xd6, 0xdd, 0xbb, 0x25, 0xf8, 0x6f, 0xfc, 0x25, 0xfe, 0x35,
	0xd3, 0x52, 0x58, 0xc7, 0xee, 0x10, 0xbf, 0xf5, 0x9e, 0xe7, 0x3c, 0xcf, 0x39, 0xcf, 0x39, 0x27,
	0x05, 0x25, 0x19, 0xc6, 0x3c, 0x6e, 0x74, 0x28, 0x09, 0xe2, 0xa8, 0x91, 0x34, 0x93, 0xc6, 0xf8,
	0xa8, 0x71, 0x49, 0x19, 0x23, 0x3d, 0xca, 0xd4, 0x0c, 0x44, 0x9b, 0x94, 0x87, 0x74, 0x48, 0x47,
	0x97, 0xea, 0x75, 0x9a, 0x9a, 0x34, 0x13, 0x75, 0x7c, 0xb4, 0xb3, 0x2f, 0xe2, 0xf2, 0xab, 0xe4,
	0x86, 0xa8, 0x9c, 0xc1, 0x86, 0x96, 0x81, 0xda, 0x20, 0x0e, 0xbe, 0xe3, 0x28, 0x8a, 0x47, 0x51,
	0x40, 0x11, 0x82, 0x72, 0x48, 0x58, 0x58, 0x95, 0x6a, 0x52, 0x7d, 0xc5, 0xc9, 0xbe, 0xd1, 0x3e,
	0xc8, 0x6c, 0x10, 0x73, 0x3f, 0x1a, 0x5d, 0x76, 0xe8, 0xb0, 0xba, 0x50, 0x93, 0xea, 0x65, 0x07,
	0xd2, 0x90, 0x95, 0x45, 0x94, 0x3a, 0xa0, 0x82, 0x96, 0x43, 0x7f, 0x8c, 0x28, 0xe3, 0x22, 0x29,
	0x05, 0xc3, 0xde, 0x6c, 0xa6, 0x76, 0xe5, 0xde, 0x6a, 0xdd, 0x2d, 0x26, 0xcd, 0x14, 0xfb, 0x25,
	0x4d, 0x75, 0xee, 0x50, 0x96, 0xc4, 0x11, 0xa3, 0xe8, 0x0d, 0x2c, 0x76, 0xd2, 0x40, 0x46, 0x91,
	0x9b, 0x07, 0xaa, 0x78, 0x32, 0x6a, 0x91, 0x7b, 0xcd, 0x40, 0x36, 0xc8, 0x84, 0x73, 0xca, 0x38,
	0xe1, 0xfd, 0x38, 0xca, 0x0c, 0xca, 0xcd, 0xc3, 0x79, 0x02, 0xb8, 0xd7, 0x1b, 0xd2, 0x1e, 0xe1,
	0xb4, 0x8b, 0x27, 0x24, 0xa7, 0xa8, 0xa0, 0xbc, 0x85, 0x6d, 0x8d, 0xf0, 0x20, 0xa4, 0x5d, 0xc1,
	0x5c, 0xfe, 0xea, 0x30, 0x84, 0x1d, 0x11, 0x3b, 0xf7, 0x79, 0x06, 0x6b, 0x9d, 0x6b, 0xd4, 0xcf,
	0xba, 0x67, 0x55, 0xa9, 0x56, 0x7a, 0xa8, 0xe1, 0xd5, 0x9c, 0x9a, 0xbd, 0x98, 0x82, 0xa0, 0xa2,
	0x87, 0xa4, 0x1f, 0x99, 0x94, 0x74, 0xf3, 0xf6, 0x94, 0x31, 0xac, 0x17, 0x62, 0x79, 0x51, 0xd1,
	0x59, 0x20, 0x28, 0xa7, 0x4d, 0xe7, 0xf7, 0x90, 0x7d, 0x4f, 0x96, 0x50, 0xfa, 0xd7, 0x25, 0x28,
	0xc7, 0xb0, 0xab, 0x0f, 0xaf, 0x18, 0x27, 0x83, 0x41, 0xff, 0x27, 0xed, 0xba, 0x9c, 0x70, 0x6a,
	0x12, 0x16, 0xde, 0x77, 0x9a, 0x8a, 0x0a, 0xd5, 0x19, 0xd2, 0x7d, 0xf7, 0x37, 0x82, 0x6d, 0x41,
	0x7e, 0x6e, 0xf2, 0x0b, 0xa0, 0xa0, 0x00, 0xfa, 0xe9, 0x36, 0x69, 0x7e, 0x4e, 0xaf, 0xe6, 0x39,
	0x99, 0x95, 0x5b, 0x0f, 0xee, 0x86, 0x94, 0x43, 0xd8, 0xc2, 0x01, 0xef, 0x8f, 0xe9, 0xc3, 0x5c,
	0xd5, 0x01, 0x15, 0xd2, 0xef, 0xf3, 0xf3, 0x0d, 0x36, 0xa6, 0x32, 0x73, 0x27, 0xef, 0x60, 0x85,
	0x64, 0xe1, 0x29, 0x0f, 0x73, 0xb7, 0x51, 0x94, 0x90, 0xc9, 0xe4, 0xf1, 0xfa, 0x77, 0x09, 0x16,
	0xbd, 0x38, 0xe9, 0x07, 0x48, 0x86, 0xe5, 0xb6, 0x75, 0x6e, 0xd9, 0x9f, 0xad, 0xca, 0x7f, 0x68,
	0x1b, 0x9e, 0x68, 0x06, 0xd6, 0x6d, 0xcb, 0xd7, 0x5a, 0xb6, 0x7e, 0xee, 0x63, 0xcb, 0xb2, 0xdb,
	0x96, 0x6e, 0x54, 0x24, 0x54, 0x85, 0xc7, 0x53, 0x90, 0x63, 0x7c, 0x6c, 0x1b, 0xae, 0x57, 0x59,
	0x40, 0x2f, 0xe1, 0x40, 0x84, 0xf8, 0xda, 0x85, 0xef, 0xb6, 0x6c, 0xcf, 0xb7, 0xda, 0xef, 0x35,
	0xc3, 0xa9, 0x94, 0x66, 0xd4, 0x1d, 0xc3, 0xfd, 0x60, 0x5b, 0xae, 0x51, 0x29, 0xa3, 0x1a, 0x3c,
	0xd3, 0xb0, 0xa7, 0x9b, 0xc6, 0x89, 0x2f, 0xac, 0xb2, 0x88, 0x9e, 0xc3, 0xee, 0x9c, 0x8c, 0x5c,
	0x64, 0x09, 0x6d, 0x02, 0xd2, 0x4d, 0x7c, 0x6a, 0xf9, 0xa6, 0x81, 0x4f, 0x6e, 0xa9, 0xcb, 0x68,
	0x0b, 0x36, 0xa6, 0xe2, 0x39, 0xe1, 0x7f, 0xf4, 0x02, 0x6a, 0xba, 0x73, 0xe1, 0x7a, 0xb8, 0xd5,
	0x3a, 0xfd, 0x6a, 0x9c, 0xf8, 0xae, 0x87, 0x3d, 0xc3, 0x37, 0xb1, 0x6b, 0x4e, 0x9c, 0x3f, 0x42,
	0x7b, 0xb0, 0x23, 0xc8, 0xba, 0x91, 0x07, 0xb4, 0x0f, 0x4f, 0x85, 0x78, 0x5e, 0x46, 0x4e, 0x05,
	0xb0, 0xee, 0x9d, 0x7e, 0x32, 0x84, 0x05, 0x56, 0xd2, 0xd1, 0x4e, 0xe1, 0x37, 0xd2, 0xab, 0xe9,
	0xc4, 0xee, 0x20, 0xb9, 0xe8, 0x5a, 0x67, 0x29, 0xfb, 0xdb, 0x1f, 0xff, 0x09, 0x00, 0x00, 0xff,
	0xff, 0x5a, 0x1e, 0x46, 0x7a, 0x4c, 0x06, 0x00, 0x00,
}
