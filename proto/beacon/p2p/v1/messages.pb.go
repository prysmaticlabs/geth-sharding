// Code generated by protoc-gen-go. DO NOT EDIT.
// source: messages.proto

package ethereum_beacon_p2p_v1

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import timestamp "github.com/golang/protobuf/ptypes/timestamp"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type BeaconBlockHashAnnounce struct {
	Hash                 []byte   `protobuf:"bytes,1,opt,name=hash,proto3" json:"hash,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *BeaconBlockHashAnnounce) Reset()         { *m = BeaconBlockHashAnnounce{} }
func (m *BeaconBlockHashAnnounce) String() string { return proto.CompactTextString(m) }
func (*BeaconBlockHashAnnounce) ProtoMessage()    {}
func (*BeaconBlockHashAnnounce) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_c712bbee4f06215c, []int{0}
}
func (m *BeaconBlockHashAnnounce) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_BeaconBlockHashAnnounce.Unmarshal(m, b)
}
func (m *BeaconBlockHashAnnounce) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_BeaconBlockHashAnnounce.Marshal(b, m, deterministic)
}
func (dst *BeaconBlockHashAnnounce) XXX_Merge(src proto.Message) {
	xxx_messageInfo_BeaconBlockHashAnnounce.Merge(dst, src)
}
func (m *BeaconBlockHashAnnounce) XXX_Size() int {
	return xxx_messageInfo_BeaconBlockHashAnnounce.Size(m)
}
func (m *BeaconBlockHashAnnounce) XXX_DiscardUnknown() {
	xxx_messageInfo_BeaconBlockHashAnnounce.DiscardUnknown(m)
}

var xxx_messageInfo_BeaconBlockHashAnnounce proto.InternalMessageInfo

func (m *BeaconBlockHashAnnounce) GetHash() []byte {
	if m != nil {
		return m.Hash
	}
	return nil
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
	return fileDescriptor_messages_c712bbee4f06215c, []int{1}
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
	return fileDescriptor_messages_c712bbee4f06215c, []int{2}
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
	ParentHash              []byte               `protobuf:"bytes,1,opt,name=parent_hash,json=parentHash,proto3" json:"parent_hash,omitempty"`
	SlotNumber              uint64               `protobuf:"varint,2,opt,name=slot_number,json=slotNumber,proto3" json:"slot_number,omitempty"`
	RandaoReveal            []byte               `protobuf:"bytes,3,opt,name=randao_reveal,json=randaoReveal,proto3" json:"randao_reveal,omitempty"`
	AttestationBitmask      []byte               `protobuf:"bytes,4,opt,name=attestation_bitmask,json=attestationBitmask,proto3" json:"attestation_bitmask,omitempty"`
	AttestationAggregateSig []uint32             `protobuf:"varint,5,rep,packed,name=attestation_aggregate_sig,json=attestationAggregateSig,proto3" json:"attestation_aggregate_sig,omitempty"`
	ShardAggregateVotes     []*AggregateVote     `protobuf:"bytes,6,rep,name=shard_aggregate_votes,json=shardAggregateVotes,proto3" json:"shard_aggregate_votes,omitempty"`
	MainChainRef            []byte               `protobuf:"bytes,7,opt,name=main_chain_ref,json=mainChainRef,proto3" json:"main_chain_ref,omitempty"`
	ActiveStateHash         []byte               `protobuf:"bytes,8,opt,name=active_state_hash,json=activeStateHash,proto3" json:"active_state_hash,omitempty"`
	CrystallizedStateHash   []byte               `protobuf:"bytes,9,opt,name=crystallized_state_hash,json=crystallizedStateHash,proto3" json:"crystallized_state_hash,omitempty"`
	Timestamp               *timestamp.Timestamp `protobuf:"bytes,10,opt,name=timestamp,proto3" json:"timestamp,omitempty"`
	XXX_NoUnkeyedLiteral    struct{}             `json:"-"`
	XXX_unrecognized        []byte               `json:"-"`
	XXX_sizecache           int32                `json:"-"`
}

func (m *BeaconBlockResponse) Reset()         { *m = BeaconBlockResponse{} }
func (m *BeaconBlockResponse) String() string { return proto.CompactTextString(m) }
func (*BeaconBlockResponse) ProtoMessage()    {}
func (*BeaconBlockResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_c712bbee4f06215c, []int{3}
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

func (m *BeaconBlockResponse) GetParentHash() []byte {
	if m != nil {
		return m.ParentHash
	}
	return nil
}

func (m *BeaconBlockResponse) GetSlotNumber() uint64 {
	if m != nil {
		return m.SlotNumber
	}
	return 0
}

func (m *BeaconBlockResponse) GetRandaoReveal() []byte {
	if m != nil {
		return m.RandaoReveal
	}
	return nil
}

func (m *BeaconBlockResponse) GetAttestationBitmask() []byte {
	if m != nil {
		return m.AttestationBitmask
	}
	return nil
}

func (m *BeaconBlockResponse) GetAttestationAggregateSig() []uint32 {
	if m != nil {
		return m.AttestationAggregateSig
	}
	return nil
}

func (m *BeaconBlockResponse) GetShardAggregateVotes() []*AggregateVote {
	if m != nil {
		return m.ShardAggregateVotes
	}
	return nil
}

func (m *BeaconBlockResponse) GetMainChainRef() []byte {
	if m != nil {
		return m.MainChainRef
	}
	return nil
}

func (m *BeaconBlockResponse) GetActiveStateHash() []byte {
	if m != nil {
		return m.ActiveStateHash
	}
	return nil
}

func (m *BeaconBlockResponse) GetCrystallizedStateHash() []byte {
	if m != nil {
		return m.CrystallizedStateHash
	}
	return nil
}

func (m *BeaconBlockResponse) GetTimestamp() *timestamp.Timestamp {
	if m != nil {
		return m.Timestamp
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
	return fileDescriptor_messages_c712bbee4f06215c, []int{4}
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
	return fileDescriptor_messages_c712bbee4f06215c, []int{5}
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
	ActiveValidators      []*ValidatorRecord `protobuf:"bytes,1,rep,name=active_validators,json=activeValidators,proto3" json:"active_validators,omitempty"`
	QueuedValidators      []*ValidatorRecord `protobuf:"bytes,2,rep,name=queued_validators,json=queuedValidators,proto3" json:"queued_validators,omitempty"`
	ExitedValidators      []*ValidatorRecord `protobuf:"bytes,3,rep,name=exited_validators,json=exitedValidators,proto3" json:"exited_validators,omitempty"`
	CurrentEpochShuffling []uint64           `protobuf:"varint,4,rep,packed,name=current_epoch_shuffling,json=currentEpochShuffling,proto3" json:"current_epoch_shuffling,omitempty"`
	CurrentEpoch          uint64             `protobuf:"varint,5,opt,name=current_epoch,json=currentEpoch,proto3" json:"current_epoch,omitempty"`
	LastJustifiedEpoch    uint64             `protobuf:"varint,6,opt,name=last_justified_epoch,json=lastJustifiedEpoch,proto3" json:"last_justified_epoch,omitempty"`
	LastFinalizedEpoch    uint64             `protobuf:"varint,7,opt,name=last_finalized_epoch,json=lastFinalizedEpoch,proto3" json:"last_finalized_epoch,omitempty"`
	CurrentDynasty        uint64             `protobuf:"varint,8,opt,name=current_dynasty,json=currentDynasty,proto3" json:"current_dynasty,omitempty"`
	NextShard             uint64             `protobuf:"varint,9,opt,name=next_shard,json=nextShard,proto3" json:"next_shard,omitempty"`
	CurrentCheckPoint     []byte             `protobuf:"bytes,10,opt,name=current_check_point,json=currentCheckPoint,proto3" json:"current_check_point,omitempty"`
	TotalDeposits         uint64             `protobuf:"varint,11,opt,name=total_deposits,json=totalDeposits,proto3" json:"total_deposits,omitempty"`
	DynastySeed           []byte             `protobuf:"bytes,12,opt,name=dynasty_seed,json=dynastySeed,proto3" json:"dynasty_seed,omitempty"`
	DynastySeedLastReset  uint64             `protobuf:"varint,13,opt,name=dynasty_seed_last_reset,json=dynastySeedLastReset,proto3" json:"dynasty_seed_last_reset,omitempty"`
	XXX_NoUnkeyedLiteral  struct{}           `json:"-"`
	XXX_unrecognized      []byte             `json:"-"`
	XXX_sizecache         int32              `json:"-"`
}

func (m *CrystallizedStateResponse) Reset()         { *m = CrystallizedStateResponse{} }
func (m *CrystallizedStateResponse) String() string { return proto.CompactTextString(m) }
func (*CrystallizedStateResponse) ProtoMessage()    {}
func (*CrystallizedStateResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_c712bbee4f06215c, []int{6}
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

func (m *CrystallizedStateResponse) GetActiveValidators() []*ValidatorRecord {
	if m != nil {
		return m.ActiveValidators
	}
	return nil
}

func (m *CrystallizedStateResponse) GetQueuedValidators() []*ValidatorRecord {
	if m != nil {
		return m.QueuedValidators
	}
	return nil
}

func (m *CrystallizedStateResponse) GetExitedValidators() []*ValidatorRecord {
	if m != nil {
		return m.ExitedValidators
	}
	return nil
}

func (m *CrystallizedStateResponse) GetCurrentEpochShuffling() []uint64 {
	if m != nil {
		return m.CurrentEpochShuffling
	}
	return nil
}

func (m *CrystallizedStateResponse) GetCurrentEpoch() uint64 {
	if m != nil {
		return m.CurrentEpoch
	}
	return 0
}

func (m *CrystallizedStateResponse) GetLastJustifiedEpoch() uint64 {
	if m != nil {
		return m.LastJustifiedEpoch
	}
	return 0
}

func (m *CrystallizedStateResponse) GetLastFinalizedEpoch() uint64 {
	if m != nil {
		return m.LastFinalizedEpoch
	}
	return 0
}

func (m *CrystallizedStateResponse) GetCurrentDynasty() uint64 {
	if m != nil {
		return m.CurrentDynasty
	}
	return 0
}

func (m *CrystallizedStateResponse) GetNextShard() uint64 {
	if m != nil {
		return m.NextShard
	}
	return 0
}

func (m *CrystallizedStateResponse) GetCurrentCheckPoint() []byte {
	if m != nil {
		return m.CurrentCheckPoint
	}
	return nil
}

func (m *CrystallizedStateResponse) GetTotalDeposits() uint64 {
	if m != nil {
		return m.TotalDeposits
	}
	return 0
}

func (m *CrystallizedStateResponse) GetDynastySeed() []byte {
	if m != nil {
		return m.DynastySeed
	}
	return nil
}

func (m *CrystallizedStateResponse) GetDynastySeedLastReset() uint64 {
	if m != nil {
		return m.DynastySeedLastReset
	}
	return 0
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
	return fileDescriptor_messages_c712bbee4f06215c, []int{7}
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
	return fileDescriptor_messages_c712bbee4f06215c, []int{8}
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
	TotalAttesterDeposits uint64   `protobuf:"varint,1,opt,name=total_attester_deposits,json=totalAttesterDeposits,proto3" json:"total_attester_deposits,omitempty"`
	AttesterBitfield      []byte   `protobuf:"bytes,2,opt,name=attester_bitfield,json=attesterBitfield,proto3" json:"attester_bitfield,omitempty"`
	XXX_NoUnkeyedLiteral  struct{} `json:"-"`
	XXX_unrecognized      []byte   `json:"-"`
	XXX_sizecache         int32    `json:"-"`
}

func (m *ActiveStateResponse) Reset()         { *m = ActiveStateResponse{} }
func (m *ActiveStateResponse) String() string { return proto.CompactTextString(m) }
func (*ActiveStateResponse) ProtoMessage()    {}
func (*ActiveStateResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_c712bbee4f06215c, []int{9}
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

func (m *ActiveStateResponse) GetTotalAttesterDeposits() uint64 {
	if m != nil {
		return m.TotalAttesterDeposits
	}
	return 0
}

func (m *ActiveStateResponse) GetAttesterBitfield() []byte {
	if m != nil {
		return m.AttesterBitfield
	}
	return nil
}

type AggregateVote struct {
	ShardId              uint32   `protobuf:"varint,1,opt,name=shard_id,json=shardId,proto3" json:"shard_id,omitempty"`
	ShardBlockHash       []byte   `protobuf:"bytes,2,opt,name=shard_block_hash,json=shardBlockHash,proto3" json:"shard_block_hash,omitempty"`
	SignerBitmask        []byte   `protobuf:"bytes,3,opt,name=signer_bitmask,json=signerBitmask,proto3" json:"signer_bitmask,omitempty"`
	AggregateSig         []uint32 `protobuf:"varint,4,rep,packed,name=aggregate_sig,json=aggregateSig,proto3" json:"aggregate_sig,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *AggregateVote) Reset()         { *m = AggregateVote{} }
func (m *AggregateVote) String() string { return proto.CompactTextString(m) }
func (*AggregateVote) ProtoMessage()    {}
func (*AggregateVote) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_c712bbee4f06215c, []int{10}
}
func (m *AggregateVote) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_AggregateVote.Unmarshal(m, b)
}
func (m *AggregateVote) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_AggregateVote.Marshal(b, m, deterministic)
}
func (dst *AggregateVote) XXX_Merge(src proto.Message) {
	xxx_messageInfo_AggregateVote.Merge(dst, src)
}
func (m *AggregateVote) XXX_Size() int {
	return xxx_messageInfo_AggregateVote.Size(m)
}
func (m *AggregateVote) XXX_DiscardUnknown() {
	xxx_messageInfo_AggregateVote.DiscardUnknown(m)
}

var xxx_messageInfo_AggregateVote proto.InternalMessageInfo

func (m *AggregateVote) GetShardId() uint32 {
	if m != nil {
		return m.ShardId
	}
	return 0
}

func (m *AggregateVote) GetShardBlockHash() []byte {
	if m != nil {
		return m.ShardBlockHash
	}
	return nil
}

func (m *AggregateVote) GetSignerBitmask() []byte {
	if m != nil {
		return m.SignerBitmask
	}
	return nil
}

func (m *AggregateVote) GetAggregateSig() []uint32 {
	if m != nil {
		return m.AggregateSig
	}
	return nil
}

type ValidatorRecord struct {
	PublicKey            uint64   `protobuf:"varint,1,opt,name=public_key,json=publicKey,proto3" json:"public_key,omitempty"`
	WithdrawalShard      uint64   `protobuf:"varint,2,opt,name=withdrawal_shard,json=withdrawalShard,proto3" json:"withdrawal_shard,omitempty"`
	WithdrawalAddress    []byte   `protobuf:"bytes,3,opt,name=withdrawal_address,json=withdrawalAddress,proto3" json:"withdrawal_address,omitempty"`
	RandaoCommitment     []byte   `protobuf:"bytes,4,opt,name=randao_commitment,json=randaoCommitment,proto3" json:"randao_commitment,omitempty"`
	Balance              uint64   `protobuf:"varint,5,opt,name=balance,proto3" json:"balance,omitempty"`
	SwitchDynasty        uint64   `protobuf:"varint,6,opt,name=switch_dynasty,json=switchDynasty,proto3" json:"switch_dynasty,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *ValidatorRecord) Reset()         { *m = ValidatorRecord{} }
func (m *ValidatorRecord) String() string { return proto.CompactTextString(m) }
func (*ValidatorRecord) ProtoMessage()    {}
func (*ValidatorRecord) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_c712bbee4f06215c, []int{11}
}
func (m *ValidatorRecord) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_ValidatorRecord.Unmarshal(m, b)
}
func (m *ValidatorRecord) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_ValidatorRecord.Marshal(b, m, deterministic)
}
func (dst *ValidatorRecord) XXX_Merge(src proto.Message) {
	xxx_messageInfo_ValidatorRecord.Merge(dst, src)
}
func (m *ValidatorRecord) XXX_Size() int {
	return xxx_messageInfo_ValidatorRecord.Size(m)
}
func (m *ValidatorRecord) XXX_DiscardUnknown() {
	xxx_messageInfo_ValidatorRecord.DiscardUnknown(m)
}

var xxx_messageInfo_ValidatorRecord proto.InternalMessageInfo

func (m *ValidatorRecord) GetPublicKey() uint64 {
	if m != nil {
		return m.PublicKey
	}
	return 0
}

func (m *ValidatorRecord) GetWithdrawalShard() uint64 {
	if m != nil {
		return m.WithdrawalShard
	}
	return 0
}

func (m *ValidatorRecord) GetWithdrawalAddress() []byte {
	if m != nil {
		return m.WithdrawalAddress
	}
	return nil
}

func (m *ValidatorRecord) GetRandaoCommitment() []byte {
	if m != nil {
		return m.RandaoCommitment
	}
	return nil
}

func (m *ValidatorRecord) GetBalance() uint64 {
	if m != nil {
		return m.Balance
	}
	return 0
}

func (m *ValidatorRecord) GetSwitchDynasty() uint64 {
	if m != nil {
		return m.SwitchDynasty
	}
	return 0
}

func init() {
	proto.RegisterType((*BeaconBlockHashAnnounce)(nil), "ethereum.beacon.p2p.v1.BeaconBlockHashAnnounce")
	proto.RegisterType((*BeaconBlockRequest)(nil), "ethereum.beacon.p2p.v1.BeaconBlockRequest")
	proto.RegisterType((*BeaconBlockRequestBySlotNumber)(nil), "ethereum.beacon.p2p.v1.BeaconBlockRequestBySlotNumber")
	proto.RegisterType((*BeaconBlockResponse)(nil), "ethereum.beacon.p2p.v1.BeaconBlockResponse")
	proto.RegisterType((*CrystallizedStateHashAnnounce)(nil), "ethereum.beacon.p2p.v1.CrystallizedStateHashAnnounce")
	proto.RegisterType((*CrystallizedStateRequest)(nil), "ethereum.beacon.p2p.v1.CrystallizedStateRequest")
	proto.RegisterType((*CrystallizedStateResponse)(nil), "ethereum.beacon.p2p.v1.CrystallizedStateResponse")
	proto.RegisterType((*ActiveStateHashAnnounce)(nil), "ethereum.beacon.p2p.v1.ActiveStateHashAnnounce")
	proto.RegisterType((*ActiveStateRequest)(nil), "ethereum.beacon.p2p.v1.ActiveStateRequest")
	proto.RegisterType((*ActiveStateResponse)(nil), "ethereum.beacon.p2p.v1.ActiveStateResponse")
	proto.RegisterType((*AggregateVote)(nil), "ethereum.beacon.p2p.v1.AggregateVote")
	proto.RegisterType((*ValidatorRecord)(nil), "ethereum.beacon.p2p.v1.ValidatorRecord")
}

func init() { proto.RegisterFile("messages.proto", fileDescriptor_messages_c712bbee4f06215c) }

var fileDescriptor_messages_c712bbee4f06215c = []byte{
	// 951 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x94, 0x55, 0xdd, 0x6e, 0xdb, 0x36,
	0x14, 0x86, 0x6b, 0x27, 0x69, 0x8e, 0x7f, 0xe2, 0xd0, 0xcd, 0xac, 0x14, 0xe8, 0xea, 0xb9, 0x2b,
	0xea, 0x6d, 0xa8, 0xb2, 0xa5, 0x58, 0x31, 0xec, 0xce, 0x4e, 0x37, 0xec, 0x0f, 0xc3, 0x20, 0x17,
	0x05, 0x76, 0x25, 0xd0, 0xd2, 0xb1, 0xc4, 0x45, 0x12, 0x55, 0x91, 0x72, 0xea, 0x3e, 0xc1, 0x1e,
	0x63, 0x6f, 0xb5, 0x57, 0xd9, 0xe5, 0x40, 0x52, 0x92, 0xa5, 0x34, 0xcb, 0x90, 0x9b, 0x20, 0xfa,
	0xfe, 0x2c, 0x1d, 0x9e, 0x73, 0x08, 0x83, 0x18, 0x85, 0xa0, 0x01, 0x0a, 0x3b, 0xcd, 0xb8, 0xe4,
	0xe4, 0x23, 0x94, 0x21, 0x66, 0x98, 0xc7, 0xf6, 0x0a, 0xa9, 0xc7, 0x13, 0x3b, 0x3d, 0x4f, 0xed,
	0xcd, 0x57, 0x0f, 0x1f, 0x07, 0x9c, 0x07, 0x11, 0x9e, 0x69, 0xd5, 0x2a, 0x5f, 0x9f, 0x49, 0x16,
	0xa3, 0x90, 0x34, 0x4e, 0x8d, 0x71, 0xfa, 0x1c, 0xc6, 0x0b, 0xed, 0x58, 0x44, 0xdc, 0xbb, 0xfc,
	0x81, 0x8a, 0x70, 0x9e, 0x24, 0x3c, 0x4f, 0x3c, 0x24, 0x04, 0x3a, 0x21, 0x15, 0xa1, 0xd5, 0x9a,
	0xb4, 0x66, 0x3d, 0x47, 0xff, 0x3f, 0x9d, 0x01, 0xa9, 0xc9, 0x1d, 0x7c, 0x9b, 0xa3, 0x90, 0x37,
	0x2a, 0xe7, 0xf0, 0xf1, 0x87, 0xca, 0xc5, 0x76, 0x19, 0x71, 0xf9, 0x6b, 0x1e, 0xaf, 0x30, 0x23,
	0x8f, 0xa1, 0x2b, 0x22, 0x2e, 0xdd, 0x44, 0x3f, 0x6a, 0x73, 0xc7, 0x01, 0x51, 0x09, 0xa6, 0x7f,
	0x76, 0x60, 0xd4, 0xc8, 0x10, 0x29, 0x4f, 0x04, 0x2a, 0x63, 0x4a, 0x33, 0x4c, 0xa4, 0x5b, 0xfb,
	0x55, 0x30, 0x90, 0xfa, 0x82, 0xeb, 0xc9, 0xf7, 0xae, 0x27, 0x93, 0x27, 0xd0, 0xcf, 0x68, 0xe2,
	0x53, 0xee, 0x66, 0xb8, 0x41, 0x1a, 0x59, 0x6d, 0x9d, 0xd1, 0x33, 0xa0, 0xa3, 0x31, 0x72, 0x06,
	0x23, 0x2a, 0xa5, 0xaa, 0x96, 0x64, 0x3c, 0x71, 0x57, 0x4c, 0xc6, 0x54, 0x5c, 0x5a, 0x1d, 0x2d,
	0x25, 0x35, 0x6a, 0x61, 0x18, 0xf2, 0x2d, 0x9c, 0xd6, 0x0d, 0x34, 0x08, 0x32, 0x0c, 0xa8, 0x44,
	0x57, 0xb0, 0xc0, 0xda, 0x9b, 0xb4, 0x67, 0x7d, 0x67, 0x5c, 0x13, 0xcc, 0x4b, 0x7e, 0xc9, 0x02,
	0xf2, 0x3b, 0x9c, 0x88, 0x90, 0x66, 0x7e, 0xcd, 0xb5, 0xe1, 0x12, 0x85, 0xb5, 0x3f, 0x69, 0xcf,
	0xba, 0xe7, 0x4f, 0xed, 0x9b, 0x0f, 0xd8, 0xae, 0x42, 0xde, 0x70, 0x89, 0xce, 0x48, 0x67, 0x34,
	0x30, 0x41, 0x3e, 0x85, 0x41, 0x4c, 0x59, 0xe2, 0x7a, 0xa1, 0xfa, 0x9b, 0xe1, 0xda, 0x3a, 0x30,
	0x5f, 0xab, 0xd0, 0x0b, 0x05, 0x3a, 0xb8, 0x26, 0x9f, 0xc3, 0x31, 0xf5, 0x24, 0xdb, 0xa0, 0xab,
	0x5e, 0x0f, 0x4d, 0x69, 0xef, 0x6b, 0xe1, 0x91, 0x21, 0x96, 0x0a, 0xd7, 0xf5, 0x7d, 0x09, 0x63,
	0x2f, 0xdb, 0x0a, 0x49, 0xa3, 0x88, 0xbd, 0x47, 0xbf, 0xee, 0x38, 0xd4, 0x8e, 0x93, 0x3a, 0xbd,
	0xf3, 0x7d, 0x03, 0x87, 0x55, 0xff, 0x59, 0x30, 0x69, 0xcd, 0xba, 0xe7, 0x0f, 0x6d, 0xd3, 0xa1,
	0x76, 0xd9, 0xa1, 0xf6, 0xeb, 0x52, 0xe1, 0xec, 0xc4, 0xd3, 0x17, 0xf0, 0xe8, 0xe2, 0xa6, 0xc8,
	0x5b, 0x9b, 0xd5, 0x06, 0xeb, 0x03, 0xd3, 0x6d, 0x2d, 0xfb, 0xf7, 0x1e, 0x9c, 0xde, 0x60, 0x28,
	0xba, 0xee, 0x75, 0x55, 0xa0, 0x0d, 0x8d, 0x98, 0x4f, 0x25, 0xcf, 0x84, 0xd5, 0xd2, 0xa7, 0xf3,
	0xec, 0xbf, 0x4e, 0xe7, 0x4d, 0xa9, 0x74, 0xd0, 0xe3, 0x99, 0xef, 0x0c, 0x4d, 0x42, 0x05, 0x0b,
	0x95, 0xfa, 0x36, 0xc7, 0x1c, 0xfd, 0x7a, 0xea, 0xbd, 0x3b, 0xa6, 0x9a, 0x84, 0x66, 0x2a, 0xbe,
	0x63, 0xb2, 0x99, 0xda, 0xbe, 0x63, 0xaa, 0x49, 0xa8, 0xa5, 0xaa, 0x63, 0xcf, 0x33, 0x3d, 0x78,
	0x98, 0x72, 0x2f, 0x74, 0x45, 0x98, 0xaf, 0xd7, 0x11, 0x4b, 0x02, 0xab, 0x33, 0x69, 0xcf, 0x3a,
	0xce, 0x49, 0x41, 0x7f, 0xa7, 0xd8, 0x65, 0x49, 0xaa, 0x69, 0x6b, 0xf8, 0xac, 0x3d, 0x3d, 0x90,
	0xbd, 0xba, 0x9a, 0x7c, 0x09, 0x0f, 0x22, 0x2a, 0xa4, 0xfb, 0x47, 0x2e, 0x24, 0x5b, 0x33, 0xf4,
	0x0b, 0xed, 0xbe, 0xd6, 0x12, 0xc5, 0xfd, 0x54, 0x52, 0x4d, 0xc7, 0x9a, 0x25, 0xd4, 0xf4, 0xa1,
	0x71, 0x1c, 0xec, 0x1c, 0xdf, 0x97, 0x94, 0x71, 0x3c, 0x83, 0xa3, 0xf2, 0x45, 0xfc, 0x6d, 0x42,
	0x85, 0xdc, 0xea, 0x0e, 0xef, 0x38, 0x83, 0x02, 0x7e, 0x65, 0x50, 0xf2, 0x08, 0x20, 0xc1, 0x77,
	0xd2, 0xd5, 0xe3, 0xa4, 0x7b, 0xba, 0xe3, 0x1c, 0x2a, 0x64, 0xa9, 0x00, 0x62, 0xc3, 0xa8, 0xcc,
	0xf1, 0x42, 0xf4, 0x2e, 0xdd, 0x94, 0xb3, 0x44, 0xea, 0x8e, 0xee, 0x39, 0xc7, 0x05, 0x75, 0xa1,
	0x98, 0xdf, 0x14, 0x41, 0x9e, 0xc2, 0x40, 0x72, 0x49, 0x23, 0xd7, 0xc7, 0x94, 0x0b, 0x26, 0x85,
	0xd5, 0xd5, 0x91, 0x7d, 0x8d, 0xbe, 0x2a, 0x40, 0xf2, 0x09, 0xf4, 0x8a, 0xd7, 0x72, 0x05, 0xa2,
	0x6f, 0xf5, 0x74, 0x5e, 0xb7, 0xc0, 0x96, 0x88, 0x3e, 0xf9, 0x1a, 0xc6, 0x75, 0x89, 0xab, 0x0b,
	0x90, 0xa1, 0x40, 0x69, 0xf5, 0x75, 0xe4, 0x83, 0x9a, 0xfa, 0x17, 0x2a, 0xa4, 0xa3, 0x38, 0xb5,
	0xe5, 0xe7, 0xcd, 0x19, 0xfe, 0xbf, 0x2d, 0x5f, 0x93, 0xdf, 0x36, 0x32, 0xef, 0x61, 0xd4, 0x50,
	0x16, 0xb3, 0xf2, 0x12, 0xc6, 0xe6, 0x83, 0xcd, 0xba, 0xc3, 0x6c, 0xf7, 0xe5, 0x66, 0xcd, 0x9f,
	0x68, 0x7a, 0x5e, 0xb0, 0x55, 0x05, 0xbe, 0x80, 0xe3, 0xca, 0xb1, 0x62, 0x72, 0xcd, 0x30, 0xf2,
	0xf5, 0xfa, 0xee, 0x39, 0xc3, 0x92, 0x58, 0x14, 0xf8, 0xf4, 0xaf, 0x16, 0xf4, 0x1b, 0xab, 0x8e,
	0x9c, 0xc2, 0x7d, 0xb3, 0x44, 0x99, 0xaf, 0x7f, 0xa7, 0xef, 0x1c, 0xe8, 0xe7, 0x1f, 0x7d, 0x32,
	0x83, 0xa1, 0xa1, 0x56, 0xea, 0x2a, 0x31, 0xbb, 0xca, 0x04, 0x0f, 0x34, 0x5e, 0x5d, 0x7f, 0xea,
	0xb0, 0x04, 0x0b, 0x12, 0xf3, 0x06, 0x7a, 0xe3, 0x9b, 0xcb, 0xa1, 0x6f, 0xd0, 0x72, 0xd9, 0x3f,
	0x81, 0x7e, 0x73, 0xc1, 0x77, 0xf4, 0x82, 0xef, 0xd1, 0xda, 0x56, 0x9f, 0xfe, 0xd3, 0x82, 0xa3,
	0x6b, 0x73, 0xa5, 0x7a, 0x2b, 0xcd, 0x57, 0x11, 0xf3, 0xdc, 0x4b, 0xdc, 0x16, 0xe5, 0x38, 0x34,
	0xc8, 0xcf, 0xb8, 0x25, 0x9f, 0xc1, 0xf0, 0x8a, 0xc9, 0xd0, 0xcf, 0xe8, 0x15, 0x8d, 0x8a, 0x06,
	0x34, 0x17, 0xd8, 0xd1, 0x0e, 0x37, 0x6d, 0xf8, 0x1c, 0x48, 0x4d, 0x4a, 0x7d, 0x3f, 0x43, 0x21,
	0x8a, 0xb7, 0x3d, 0xde, 0x31, 0x73, 0x43, 0xa8, 0xe2, 0x16, 0x97, 0x9e, 0xc7, 0xe3, 0x98, 0xc9,
	0x18, 0x13, 0x59, 0xdc, 0x66, 0x43, 0x43, 0x5c, 0x54, 0x38, 0xb1, 0xe0, 0x60, 0x45, 0x23, 0x9a,
	0x78, 0x58, 0x4c, 0x6b, 0xf9, 0xa8, 0xeb, 0x73, 0xc5, 0xa4, 0x17, 0x56, 0x33, 0x64, 0x46, 0xb4,
	0x6f, 0xd0, 0x62, 0x84, 0x56, 0xfb, 0x7a, 0xa1, 0xbf, 0xf8, 0x37, 0x00, 0x00, 0xff, 0xff, 0x33,
	0xe4, 0x31, 0x92, 0xaa, 0x08, 0x00, 0x00,
}
