// Code generated by protoc-gen-go. DO NOT EDIT.
// source: proto/sharding/p2p/v1/messages.proto

package ethereum_sharding_p2p_v1

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
	Topic_UNKNOWN                    Topic = 0
	Topic_COLLATION_BODY_REQUEST     Topic = 1
	Topic_COLLATION_BODY_RESPONSE    Topic = 2
	Topic_TRANSACTIONS               Topic = 3
	Topic_BEACON_BLOCK_HASH_ANNOUNCE Topic = 4
	Topic_BEACON_BLOCK_REQUEST       Topic = 5
	Topic_BEACON_BLOCK_RESPONSE      Topic = 6
)

var Topic_name = map[int32]string{
	0: "UNKNOWN",
	1: "COLLATION_BODY_REQUEST",
	2: "COLLATION_BODY_RESPONSE",
	3: "TRANSACTIONS",
	4: "BEACON_BLOCK_HASH_ANNOUNCE",
	5: "BEACON_BLOCK_REQUEST",
	6: "BEACON_BLOCK_RESPONSE",
}
var Topic_value = map[string]int32{
	"UNKNOWN":                    0,
	"COLLATION_BODY_REQUEST":     1,
	"COLLATION_BODY_RESPONSE":    2,
	"TRANSACTIONS":               3,
	"BEACON_BLOCK_HASH_ANNOUNCE": 4,
	"BEACON_BLOCK_REQUEST":       5,
	"BEACON_BLOCK_RESPONSE":      6,
}

func (x Topic) String() string {
	return proto.EnumName(Topic_name, int32(x))
}
func (Topic) EnumDescriptor() ([]byte, []int) {
	return fileDescriptor_messages_a2b5338ab8a3879b, []int{0}
}

type CollationBodyRequest struct {
	ShardId              uint64   `protobuf:"varint,1,opt,name=shard_id,json=shardId,proto3" json:"shard_id,omitempty"`
	Period               uint64   `protobuf:"varint,2,opt,name=period,proto3" json:"period,omitempty"`
	ChunkRoot            []byte   `protobuf:"bytes,3,opt,name=chunk_root,json=chunkRoot,proto3" json:"chunk_root,omitempty"`
	ProposerAddress      []byte   `protobuf:"bytes,4,opt,name=proposer_address,json=proposerAddress,proto3" json:"proposer_address,omitempty"`
	Signature            []byte   `protobuf:"bytes,5,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CollationBodyRequest) Reset()         { *m = CollationBodyRequest{} }
func (m *CollationBodyRequest) String() string { return proto.CompactTextString(m) }
func (*CollationBodyRequest) ProtoMessage()    {}
func (*CollationBodyRequest) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_a2b5338ab8a3879b, []int{0}
}
func (m *CollationBodyRequest) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CollationBodyRequest.Unmarshal(m, b)
}
func (m *CollationBodyRequest) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CollationBodyRequest.Marshal(b, m, deterministic)
}
func (dst *CollationBodyRequest) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CollationBodyRequest.Merge(dst, src)
}
func (m *CollationBodyRequest) XXX_Size() int {
	return xxx_messageInfo_CollationBodyRequest.Size(m)
}
func (m *CollationBodyRequest) XXX_DiscardUnknown() {
	xxx_messageInfo_CollationBodyRequest.DiscardUnknown(m)
}

var xxx_messageInfo_CollationBodyRequest proto.InternalMessageInfo

func (m *CollationBodyRequest) GetShardId() uint64 {
	if m != nil {
		return m.ShardId
	}
	return 0
}

func (m *CollationBodyRequest) GetPeriod() uint64 {
	if m != nil {
		return m.Period
	}
	return 0
}

func (m *CollationBodyRequest) GetChunkRoot() []byte {
	if m != nil {
		return m.ChunkRoot
	}
	return nil
}

func (m *CollationBodyRequest) GetProposerAddress() []byte {
	if m != nil {
		return m.ProposerAddress
	}
	return nil
}

func (m *CollationBodyRequest) GetSignature() []byte {
	if m != nil {
		return m.Signature
	}
	return nil
}

type CollationBodyResponse struct {
	HeaderHash           []byte   `protobuf:"bytes,1,opt,name=header_hash,json=headerHash,proto3" json:"header_hash,omitempty"`
	Body                 []byte   `protobuf:"bytes,2,opt,name=body,proto3" json:"body,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *CollationBodyResponse) Reset()         { *m = CollationBodyResponse{} }
func (m *CollationBodyResponse) String() string { return proto.CompactTextString(m) }
func (*CollationBodyResponse) ProtoMessage()    {}
func (*CollationBodyResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_a2b5338ab8a3879b, []int{1}
}
func (m *CollationBodyResponse) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_CollationBodyResponse.Unmarshal(m, b)
}
func (m *CollationBodyResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_CollationBodyResponse.Marshal(b, m, deterministic)
}
func (dst *CollationBodyResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_CollationBodyResponse.Merge(dst, src)
}
func (m *CollationBodyResponse) XXX_Size() int {
	return xxx_messageInfo_CollationBodyResponse.Size(m)
}
func (m *CollationBodyResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_CollationBodyResponse.DiscardUnknown(m)
}

var xxx_messageInfo_CollationBodyResponse proto.InternalMessageInfo

func (m *CollationBodyResponse) GetHeaderHash() []byte {
	if m != nil {
		return m.HeaderHash
	}
	return nil
}

func (m *CollationBodyResponse) GetBody() []byte {
	if m != nil {
		return m.Body
	}
	return nil
}

type Transaction struct {
	Nonce                uint64     `protobuf:"varint,1,opt,name=nonce,proto3" json:"nonce,omitempty"`
	GasPrice             uint64     `protobuf:"varint,2,opt,name=gas_price,json=gasPrice,proto3" json:"gas_price,omitempty"`
	GasLimit             uint64     `protobuf:"varint,3,opt,name=gas_limit,json=gasLimit,proto3" json:"gas_limit,omitempty"`
	Recipient            []byte     `protobuf:"bytes,4,opt,name=recipient,proto3" json:"recipient,omitempty"`
	Value                uint64     `protobuf:"varint,5,opt,name=value,proto3" json:"value,omitempty"`
	Input                []byte     `protobuf:"bytes,6,opt,name=input,proto3" json:"input,omitempty"`
	Signature            *Signature `protobuf:"bytes,7,opt,name=signature,proto3" json:"signature,omitempty"`
	XXX_NoUnkeyedLiteral struct{}   `json:"-"`
	XXX_unrecognized     []byte     `json:"-"`
	XXX_sizecache        int32      `json:"-"`
}

func (m *Transaction) Reset()         { *m = Transaction{} }
func (m *Transaction) String() string { return proto.CompactTextString(m) }
func (*Transaction) ProtoMessage()    {}
func (*Transaction) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_a2b5338ab8a3879b, []int{2}
}
func (m *Transaction) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Transaction.Unmarshal(m, b)
}
func (m *Transaction) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Transaction.Marshal(b, m, deterministic)
}
func (dst *Transaction) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Transaction.Merge(dst, src)
}
func (m *Transaction) XXX_Size() int {
	return xxx_messageInfo_Transaction.Size(m)
}
func (m *Transaction) XXX_DiscardUnknown() {
	xxx_messageInfo_Transaction.DiscardUnknown(m)
}

var xxx_messageInfo_Transaction proto.InternalMessageInfo

func (m *Transaction) GetNonce() uint64 {
	if m != nil {
		return m.Nonce
	}
	return 0
}

func (m *Transaction) GetGasPrice() uint64 {
	if m != nil {
		return m.GasPrice
	}
	return 0
}

func (m *Transaction) GetGasLimit() uint64 {
	if m != nil {
		return m.GasLimit
	}
	return 0
}

func (m *Transaction) GetRecipient() []byte {
	if m != nil {
		return m.Recipient
	}
	return nil
}

func (m *Transaction) GetValue() uint64 {
	if m != nil {
		return m.Value
	}
	return 0
}

func (m *Transaction) GetInput() []byte {
	if m != nil {
		return m.Input
	}
	return nil
}

func (m *Transaction) GetSignature() *Signature {
	if m != nil {
		return m.Signature
	}
	return nil
}

type Signature struct {
	V                    uint64   `protobuf:"varint,1,opt,name=v,proto3" json:"v,omitempty"`
	R                    uint64   `protobuf:"varint,2,opt,name=r,proto3" json:"r,omitempty"`
	S                    uint64   `protobuf:"varint,3,opt,name=s,proto3" json:"s,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Signature) Reset()         { *m = Signature{} }
func (m *Signature) String() string { return proto.CompactTextString(m) }
func (*Signature) ProtoMessage()    {}
func (*Signature) Descriptor() ([]byte, []int) {
	return fileDescriptor_messages_a2b5338ab8a3879b, []int{3}
}
func (m *Signature) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Signature.Unmarshal(m, b)
}
func (m *Signature) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Signature.Marshal(b, m, deterministic)
}
func (dst *Signature) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Signature.Merge(dst, src)
}
func (m *Signature) XXX_Size() int {
	return xxx_messageInfo_Signature.Size(m)
}
func (m *Signature) XXX_DiscardUnknown() {
	xxx_messageInfo_Signature.DiscardUnknown(m)
}

var xxx_messageInfo_Signature proto.InternalMessageInfo

func (m *Signature) GetV() uint64 {
	if m != nil {
		return m.V
	}
	return 0
}

func (m *Signature) GetR() uint64 {
	if m != nil {
		return m.R
	}
	return 0
}

func (m *Signature) GetS() uint64 {
	if m != nil {
		return m.S
	}
	return 0
}

func init() {
	proto.RegisterType((*CollationBodyRequest)(nil), "ethereum.sharding.p2p.v1.CollationBodyRequest")
	proto.RegisterType((*CollationBodyResponse)(nil), "ethereum.sharding.p2p.v1.CollationBodyResponse")
	proto.RegisterType((*Transaction)(nil), "ethereum.sharding.p2p.v1.Transaction")
	proto.RegisterType((*Signature)(nil), "ethereum.sharding.p2p.v1.Signature")
	proto.RegisterEnum("ethereum.sharding.p2p.v1.Topic", Topic_name, Topic_value)
}

func init() {
	proto.RegisterFile("proto/sharding/p2p/v1/messages.proto", fileDescriptor_messages_a2b5338ab8a3879b)
}

var fileDescriptor_messages_a2b5338ab8a3879b = []byte{
	// 503 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x74, 0x92, 0xdd, 0x6a, 0xdb, 0x3c,
	0x18, 0xc7, 0x5f, 0xb5, 0xf9, 0x68, 0x9e, 0x04, 0x5e, 0x23, 0xd2, 0xce, 0x6d, 0xf7, 0x11, 0xb2,
	0x1d, 0x64, 0x3b, 0x70, 0x68, 0xc6, 0x2e, 0xc0, 0xf1, 0x02, 0x29, 0x0d, 0x76, 0x67, 0x27, 0x8c,
	0x1d, 0x19, 0xd5, 0x16, 0xb1, 0x58, 0x62, 0x69, 0x92, 0x1d, 0xe8, 0x65, 0x0d, 0x76, 0x55, 0xbb,
	0x8a, 0x21, 0xd9, 0xee, 0xba, 0x8e, 0x9d, 0xf9, 0xff, 0x81, 0x9e, 0xe7, 0x27, 0x0b, 0xde, 0x08,
	0xc9, 0x0b, 0x3e, 0x55, 0x19, 0x91, 0x29, 0xcb, 0xb7, 0x53, 0x31, 0x13, 0xd3, 0xc3, 0xd5, 0x74,
	0x4f, 0x95, 0x22, 0x5b, 0xaa, 0x1c, 0x13, 0x63, 0x9b, 0x16, 0x19, 0x95, 0xb4, 0xdc, 0x3b, 0x4d,
	0xd1, 0x11, 0x33, 0xe1, 0x1c, 0xae, 0xc6, 0xdf, 0x11, 0x0c, 0x3d, 0xbe, 0xdb, 0x91, 0x82, 0xf1,
	0x7c, 0xce, 0xd3, 0xfb, 0x90, 0x7e, 0x2b, 0xa9, 0x2a, 0xf0, 0x39, 0x9c, 0x98, 0x6e, 0xcc, 0x52,
	0x1b, 0x8d, 0xd0, 0xa4, 0x15, 0x76, 0x8d, 0xbe, 0x4e, 0xf1, 0x19, 0x74, 0x04, 0x95, 0x8c, 0xa7,
	0xf6, 0x91, 0x09, 0x6a, 0x85, 0x5f, 0x00, 0x24, 0x59, 0x99, 0x7f, 0x8d, 0x25, 0xe7, 0x85, 0x7d,
	0x3c, 0x42, 0x93, 0x41, 0xd8, 0x33, 0x4e, 0xc8, 0x79, 0x81, 0xdf, 0x82, 0x25, 0x24, 0x17, 0x5c,
	0x51, 0x19, 0x93, 0x34, 0x95, 0x54, 0x29, 0xbb, 0x65, 0x4a, 0xff, 0x37, 0xbe, 0x5b, 0xd9, 0xf8,
	0x39, 0xf4, 0x14, 0xdb, 0xe6, 0xa4, 0x28, 0x25, 0xb5, 0xdb, 0xd5, 0x41, 0x0f, 0xc6, 0x78, 0x05,
	0xa7, 0x4f, 0x56, 0x56, 0x82, 0xe7, 0x8a, 0xe2, 0x57, 0xd0, 0xcf, 0x28, 0x49, 0xa9, 0x8c, 0x33,
	0xa2, 0x32, 0xb3, 0xf6, 0x20, 0x84, 0xca, 0x5a, 0x12, 0x95, 0x61, 0x0c, 0xad, 0x3b, 0x9e, 0xde,
	0x9b, 0xbd, 0x07, 0xa1, 0xf9, 0x1e, 0xff, 0x44, 0xd0, 0x5f, 0x4b, 0x92, 0x2b, 0x92, 0xe8, 0x03,
	0xf1, 0x10, 0xda, 0x39, 0xcf, 0x13, 0x5a, 0x53, 0x57, 0x02, 0x5f, 0x42, 0x6f, 0x4b, 0x54, 0x2c,
	0x24, 0x4b, 0x68, 0x8d, 0x7d, 0xb2, 0x25, 0xea, 0x56, 0xeb, 0x26, 0xdc, 0xb1, 0x3d, 0xab, 0xb8,
	0xab, 0x70, 0xa5, 0xb5, 0x66, 0x91, 0x34, 0x61, 0x82, 0xd1, 0xbc, 0xa8, 0x79, 0x7f, 0x1b, 0x7a,
	0xda, 0x81, 0xec, 0xca, 0x8a, 0xb2, 0x15, 0x56, 0x42, 0xbb, 0x2c, 0x17, 0x65, 0x61, 0x77, 0x4c,
	0xbf, 0x12, 0xd8, 0x7d, 0x7c, 0x2b, 0xdd, 0x11, 0x9a, 0xf4, 0x67, 0xaf, 0x9d, 0x7f, 0xfd, 0x59,
	0x27, 0x6a, 0xaa, 0x8f, 0xaf, 0xee, 0x03, 0xf4, 0x1e, 0x7c, 0x3c, 0x00, 0x74, 0xa8, 0x29, 0xd1,
	0x41, 0x2b, 0x59, 0x93, 0x21, 0xa9, 0x95, 0xaa, 0x51, 0x90, 0x7a, 0xf7, 0x03, 0x41, 0x7b, 0xcd,
	0x05, 0x4b, 0x70, 0x1f, 0xba, 0x1b, 0xff, 0xc6, 0x0f, 0x3e, 0xfb, 0xd6, 0x7f, 0xf8, 0x02, 0xce,
	0xbc, 0x60, 0xb5, 0x72, 0xd7, 0xd7, 0x81, 0x1f, 0xcf, 0x83, 0x8f, 0x5f, 0xe2, 0x70, 0xf1, 0x69,
	0xb3, 0x88, 0xd6, 0x16, 0xc2, 0x97, 0xf0, 0xec, 0xaf, 0x2c, 0xba, 0x0d, 0xfc, 0x68, 0x61, 0x1d,
	0x61, 0x0b, 0x06, 0xeb, 0xd0, 0xf5, 0x23, 0xd7, 0xd3, 0x71, 0x64, 0x1d, 0xe3, 0x97, 0x70, 0x31,
	0x5f, 0xb8, 0x9e, 0xee, 0xae, 0x02, 0xef, 0x26, 0x5e, 0xba, 0xd1, 0x32, 0x76, 0x7d, 0x3f, 0xd8,
	0xf8, 0xde, 0xc2, 0x6a, 0x61, 0x1b, 0x86, 0x7f, 0xe4, 0xcd, 0xa0, 0x36, 0x3e, 0x87, 0xd3, 0x27,
	0x49, 0x3d, 0xa6, 0x73, 0xd7, 0x31, 0xaf, 0xff, 0xfd, 0xaf, 0x00, 0x00, 0x00, 0xff, 0xff, 0x01,
	0x09, 0x20, 0xaf, 0x25, 0x03, 0x00, 0x00,
}
