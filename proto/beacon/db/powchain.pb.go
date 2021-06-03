// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.15.8
// source: proto/beacon/db/powchain.proto

package db

import (
	proto "github.com/golang/protobuf/proto"
	v1 "github.com/prysmaticlabs/prysm/proto/beacon/p2p/v1"
	v1alpha1 "github.com/prysmaticlabs/prysm/proto/eth/v1alpha1"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
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

type ETH1ChainData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	CurrentEth1Data   *LatestETH1Data     `protobuf:"bytes,1,opt,name=current_eth1_data,json=currentEth1Data,proto3" json:"current_eth1_data,omitempty"`
	ChainstartData    *ChainStartData     `protobuf:"bytes,2,opt,name=chainstart_data,json=chainstartData,proto3" json:"chainstart_data,omitempty"`
	BeaconState       *v1.BeaconState     `protobuf:"bytes,3,opt,name=beacon_state,json=beaconState,proto3" json:"beacon_state,omitempty"`
	Trie              *SparseMerkleTrie   `protobuf:"bytes,4,opt,name=trie,proto3" json:"trie,omitempty"`
	DepositContainers []*DepositContainer `protobuf:"bytes,5,rep,name=deposit_containers,json=depositContainers,proto3" json:"deposit_containers,omitempty"`
}

func (x *ETH1ChainData) Reset() {
	*x = ETH1ChainData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_beacon_db_powchain_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ETH1ChainData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ETH1ChainData) ProtoMessage() {}

func (x *ETH1ChainData) ProtoReflect() protoreflect.Message {
	mi := &file_proto_beacon_db_powchain_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ETH1ChainData.ProtoReflect.Descriptor instead.
func (*ETH1ChainData) Descriptor() ([]byte, []int) {
	return file_proto_beacon_db_powchain_proto_rawDescGZIP(), []int{0}
}

func (x *ETH1ChainData) GetCurrentEth1Data() *LatestETH1Data {
	if x != nil {
		return x.CurrentEth1Data
	}
	return nil
}

func (x *ETH1ChainData) GetChainstartData() *ChainStartData {
	if x != nil {
		return x.ChainstartData
	}
	return nil
}

func (x *ETH1ChainData) GetBeaconState() *v1.BeaconState {
	if x != nil {
		return x.BeaconState
	}
	return nil
}

func (x *ETH1ChainData) GetTrie() *SparseMerkleTrie {
	if x != nil {
		return x.Trie
	}
	return nil
}

func (x *ETH1ChainData) GetDepositContainers() []*DepositContainer {
	if x != nil {
		return x.DepositContainers
	}
	return nil
}

type LatestETH1Data struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	BlockHeight        uint64 `protobuf:"varint,2,opt,name=block_height,json=blockHeight,proto3" json:"block_height,omitempty"`
	BlockTime          uint64 `protobuf:"varint,3,opt,name=block_time,json=blockTime,proto3" json:"block_time,omitempty"`
	BlockHash          []byte `protobuf:"bytes,4,opt,name=block_hash,json=blockHash,proto3" json:"block_hash,omitempty"`
	LastRequestedBlock uint64 `protobuf:"varint,5,opt,name=last_requested_block,json=lastRequestedBlock,proto3" json:"last_requested_block,omitempty"`
}

func (x *LatestETH1Data) Reset() {
	*x = LatestETH1Data{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_beacon_db_powchain_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *LatestETH1Data) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*LatestETH1Data) ProtoMessage() {}

func (x *LatestETH1Data) ProtoReflect() protoreflect.Message {
	mi := &file_proto_beacon_db_powchain_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use LatestETH1Data.ProtoReflect.Descriptor instead.
func (*LatestETH1Data) Descriptor() ([]byte, []int) {
	return file_proto_beacon_db_powchain_proto_rawDescGZIP(), []int{1}
}

func (x *LatestETH1Data) GetBlockHeight() uint64 {
	if x != nil {
		return x.BlockHeight
	}
	return 0
}

func (x *LatestETH1Data) GetBlockTime() uint64 {
	if x != nil {
		return x.BlockTime
	}
	return 0
}

func (x *LatestETH1Data) GetBlockHash() []byte {
	if x != nil {
		return x.BlockHash
	}
	return nil
}

func (x *LatestETH1Data) GetLastRequestedBlock() uint64 {
	if x != nil {
		return x.LastRequestedBlock
	}
	return 0
}

type ChainStartData struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Chainstarted       bool                `protobuf:"varint,1,opt,name=chainstarted,proto3" json:"chainstarted,omitempty"`
	GenesisTime        uint64              `protobuf:"varint,2,opt,name=genesis_time,json=genesisTime,proto3" json:"genesis_time,omitempty"`
	GenesisBlock       uint64              `protobuf:"varint,3,opt,name=genesis_block,json=genesisBlock,proto3" json:"genesis_block,omitempty"`
	Eth1Data           *v1alpha1.Eth1Data  `protobuf:"bytes,4,opt,name=eth1_data,json=eth1Data,proto3" json:"eth1_data,omitempty"`
	ChainstartDeposits []*v1alpha1.Deposit `protobuf:"bytes,5,rep,name=chainstart_deposits,json=chainstartDeposits,proto3" json:"chainstart_deposits,omitempty"`
}

func (x *ChainStartData) Reset() {
	*x = ChainStartData{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_beacon_db_powchain_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ChainStartData) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChainStartData) ProtoMessage() {}

func (x *ChainStartData) ProtoReflect() protoreflect.Message {
	mi := &file_proto_beacon_db_powchain_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChainStartData.ProtoReflect.Descriptor instead.
func (*ChainStartData) Descriptor() ([]byte, []int) {
	return file_proto_beacon_db_powchain_proto_rawDescGZIP(), []int{2}
}

func (x *ChainStartData) GetChainstarted() bool {
	if x != nil {
		return x.Chainstarted
	}
	return false
}

func (x *ChainStartData) GetGenesisTime() uint64 {
	if x != nil {
		return x.GenesisTime
	}
	return 0
}

func (x *ChainStartData) GetGenesisBlock() uint64 {
	if x != nil {
		return x.GenesisBlock
	}
	return 0
}

func (x *ChainStartData) GetEth1Data() *v1alpha1.Eth1Data {
	if x != nil {
		return x.Eth1Data
	}
	return nil
}

func (x *ChainStartData) GetChainstartDeposits() []*v1alpha1.Deposit {
	if x != nil {
		return x.ChainstartDeposits
	}
	return nil
}

type SparseMerkleTrie struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Depth         uint64       `protobuf:"varint,1,opt,name=depth,proto3" json:"depth,omitempty"`
	Layers        []*TrieLayer `protobuf:"bytes,2,rep,name=layers,proto3" json:"layers,omitempty"`
	OriginalItems [][]byte     `protobuf:"bytes,3,rep,name=original_items,json=originalItems,proto3" json:"original_items,omitempty"`
}

func (x *SparseMerkleTrie) Reset() {
	*x = SparseMerkleTrie{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_beacon_db_powchain_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SparseMerkleTrie) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SparseMerkleTrie) ProtoMessage() {}

func (x *SparseMerkleTrie) ProtoReflect() protoreflect.Message {
	mi := &file_proto_beacon_db_powchain_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SparseMerkleTrie.ProtoReflect.Descriptor instead.
func (*SparseMerkleTrie) Descriptor() ([]byte, []int) {
	return file_proto_beacon_db_powchain_proto_rawDescGZIP(), []int{3}
}

func (x *SparseMerkleTrie) GetDepth() uint64 {
	if x != nil {
		return x.Depth
	}
	return 0
}

func (x *SparseMerkleTrie) GetLayers() []*TrieLayer {
	if x != nil {
		return x.Layers
	}
	return nil
}

func (x *SparseMerkleTrie) GetOriginalItems() [][]byte {
	if x != nil {
		return x.OriginalItems
	}
	return nil
}

type TrieLayer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Layer [][]byte `protobuf:"bytes,1,rep,name=layer,proto3" json:"layer,omitempty"`
}

func (x *TrieLayer) Reset() {
	*x = TrieLayer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_beacon_db_powchain_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *TrieLayer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TrieLayer) ProtoMessage() {}

func (x *TrieLayer) ProtoReflect() protoreflect.Message {
	mi := &file_proto_beacon_db_powchain_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TrieLayer.ProtoReflect.Descriptor instead.
func (*TrieLayer) Descriptor() ([]byte, []int) {
	return file_proto_beacon_db_powchain_proto_rawDescGZIP(), []int{4}
}

func (x *TrieLayer) GetLayer() [][]byte {
	if x != nil {
		return x.Layer
	}
	return nil
}

type DepositContainer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Index           int64             `protobuf:"varint,1,opt,name=index,proto3" json:"index,omitempty"`
	Eth1BlockHeight uint64            `protobuf:"varint,2,opt,name=eth1_block_height,json=eth1BlockHeight,proto3" json:"eth1_block_height,omitempty"`
	Deposit         *v1alpha1.Deposit `protobuf:"bytes,3,opt,name=deposit,proto3" json:"deposit,omitempty"`
	DepositRoot     []byte            `protobuf:"bytes,4,opt,name=deposit_root,json=depositRoot,proto3" json:"deposit_root,omitempty"`
}

func (x *DepositContainer) Reset() {
	*x = DepositContainer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_proto_beacon_db_powchain_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DepositContainer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DepositContainer) ProtoMessage() {}

func (x *DepositContainer) ProtoReflect() protoreflect.Message {
	mi := &file_proto_beacon_db_powchain_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DepositContainer.ProtoReflect.Descriptor instead.
func (*DepositContainer) Descriptor() ([]byte, []int) {
	return file_proto_beacon_db_powchain_proto_rawDescGZIP(), []int{5}
}

func (x *DepositContainer) GetIndex() int64 {
	if x != nil {
		return x.Index
	}
	return 0
}

func (x *DepositContainer) GetEth1BlockHeight() uint64 {
	if x != nil {
		return x.Eth1BlockHeight
	}
	return 0
}

func (x *DepositContainer) GetDeposit() *v1alpha1.Deposit {
	if x != nil {
		return x.Deposit
	}
	return nil
}

func (x *DepositContainer) GetDepositRoot() []byte {
	if x != nil {
		return x.DepositRoot
	}
	return nil
}

var File_proto_beacon_db_powchain_proto protoreflect.FileDescriptor

var file_proto_beacon_db_powchain_proto_rawDesc = []byte{
	0x0a, 0x1e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2f, 0x64,
	0x62, 0x2f, 0x70, 0x6f, 0x77, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x12, 0x0f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2e, 0x64,
	0x62, 0x1a, 0x25, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x74, 0x68, 0x2f, 0x76, 0x31, 0x61,
	0x6c, 0x70, 0x68, 0x61, 0x31, 0x2f, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x5f, 0x62, 0x6c, 0x6f,
	0x63, 0x6b, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x1a, 0x1f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f,
	0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2f, 0x70, 0x32, 0x70, 0x2f, 0x76, 0x31, 0x2f, 0x74, 0x79,
	0x70, 0x65, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x22, 0xf7, 0x02, 0x0a, 0x0d, 0x45, 0x54,
	0x48, 0x31, 0x43, 0x68, 0x61, 0x69, 0x6e, 0x44, 0x61, 0x74, 0x61, 0x12, 0x4b, 0x0a, 0x11, 0x63,
	0x75, 0x72, 0x72, 0x65, 0x6e, 0x74, 0x5f, 0x65, 0x74, 0x68, 0x31, 0x5f, 0x64, 0x61, 0x74, 0x61,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x62,
	0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2e, 0x64, 0x62, 0x2e, 0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x45,
	0x54, 0x48, 0x31, 0x44, 0x61, 0x74, 0x61, 0x52, 0x0f, 0x63, 0x75, 0x72, 0x72, 0x65, 0x6e, 0x74,
	0x45, 0x74, 0x68, 0x31, 0x44, 0x61, 0x74, 0x61, 0x12, 0x48, 0x0a, 0x0f, 0x63, 0x68, 0x61, 0x69,
	0x6e, 0x73, 0x74, 0x61, 0x72, 0x74, 0x5f, 0x64, 0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x0b, 0x32, 0x1f, 0x2e, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e,
	0x2e, 0x64, 0x62, 0x2e, 0x43, 0x68, 0x61, 0x69, 0x6e, 0x53, 0x74, 0x61, 0x72, 0x74, 0x44, 0x61,
	0x74, 0x61, 0x52, 0x0e, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x72, 0x74, 0x44, 0x61,
	0x74, 0x61, 0x12, 0x46, 0x0a, 0x0c, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x5f, 0x73, 0x74, 0x61,
	0x74, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x23, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72,
	0x65, 0x75, 0x6d, 0x2e, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2e, 0x70, 0x32, 0x70, 0x2e, 0x76,
	0x31, 0x2e, 0x42, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x53, 0x74, 0x61, 0x74, 0x65, 0x52, 0x0b, 0x62,
	0x65, 0x61, 0x63, 0x6f, 0x6e, 0x53, 0x74, 0x61, 0x74, 0x65, 0x12, 0x35, 0x0a, 0x04, 0x74, 0x72,
	0x69, 0x65, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x21, 0x2e, 0x70, 0x72, 0x79, 0x73, 0x6d,
	0x2e, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2e, 0x64, 0x62, 0x2e, 0x53, 0x70, 0x61, 0x72, 0x73,
	0x65, 0x4d, 0x65, 0x72, 0x6b, 0x6c, 0x65, 0x54, 0x72, 0x69, 0x65, 0x52, 0x04, 0x74, 0x72, 0x69,
	0x65, 0x12, 0x50, 0x0a, 0x12, 0x64, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x5f, 0x63, 0x6f, 0x6e,
	0x74, 0x61, 0x69, 0x6e, 0x65, 0x72, 0x73, 0x18, 0x05, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x21, 0x2e,
	0x70, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x62, 0x65, 0x61, 0x63, 0x6f, 0x6e, 0x2e, 0x64, 0x62, 0x2e,
	0x44, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e, 0x65, 0x72,
	0x52, 0x11, 0x64, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x43, 0x6f, 0x6e, 0x74, 0x61, 0x69, 0x6e,
	0x65, 0x72, 0x73, 0x22, 0xa3, 0x01, 0x0a, 0x0e, 0x4c, 0x61, 0x74, 0x65, 0x73, 0x74, 0x45, 0x54,
	0x48, 0x31, 0x44, 0x61, 0x74, 0x61, 0x12, 0x21, 0x0a, 0x0c, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f,
	0x68, 0x65, 0x69, 0x67, 0x68, 0x74, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x1d, 0x0a, 0x0a, 0x62, 0x6c, 0x6f,
	0x63, 0x6b, 0x5f, 0x74, 0x69, 0x6d, 0x65, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x09, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x54, 0x69, 0x6d, 0x65, 0x12, 0x1d, 0x0a, 0x0a, 0x62, 0x6c, 0x6f, 0x63,
	0x6b, 0x5f, 0x68, 0x61, 0x73, 0x68, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x09, 0x62, 0x6c,
	0x6f, 0x63, 0x6b, 0x48, 0x61, 0x73, 0x68, 0x12, 0x30, 0x0a, 0x14, 0x6c, 0x61, 0x73, 0x74, 0x5f,
	0x72, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x65, 0x64, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x18,
	0x05, 0x20, 0x01, 0x28, 0x04, 0x52, 0x12, 0x6c, 0x61, 0x73, 0x74, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x65, 0x64, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x22, 0x8b, 0x02, 0x0a, 0x0e, 0x43, 0x68,
	0x61, 0x69, 0x6e, 0x53, 0x74, 0x61, 0x72, 0x74, 0x44, 0x61, 0x74, 0x61, 0x12, 0x22, 0x0a, 0x0c,
	0x63, 0x68, 0x61, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x08, 0x52, 0x0c, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x72, 0x74, 0x65, 0x64,
	0x12, 0x21, 0x0a, 0x0c, 0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x5f, 0x74, 0x69, 0x6d, 0x65,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0b, 0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x54,
	0x69, 0x6d, 0x65, 0x12, 0x23, 0x0a, 0x0d, 0x67, 0x65, 0x6e, 0x65, 0x73, 0x69, 0x73, 0x5f, 0x62,
	0x6c, 0x6f, 0x63, 0x6b, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0c, 0x67, 0x65, 0x6e, 0x65,
	0x73, 0x69, 0x73, 0x42, 0x6c, 0x6f, 0x63, 0x6b, 0x12, 0x3c, 0x0a, 0x09, 0x65, 0x74, 0x68, 0x31,
	0x5f, 0x64, 0x61, 0x74, 0x61, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1f, 0x2e, 0x65, 0x74,
	0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70,
	0x68, 0x61, 0x31, 0x2e, 0x45, 0x74, 0x68, 0x31, 0x44, 0x61, 0x74, 0x61, 0x52, 0x08, 0x65, 0x74,
	0x68, 0x31, 0x44, 0x61, 0x74, 0x61, 0x12, 0x4f, 0x0a, 0x13, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x73,
	0x74, 0x61, 0x72, 0x74, 0x5f, 0x64, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x73, 0x18, 0x05, 0x20,
	0x03, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x65,
	0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31, 0x2e, 0x44, 0x65, 0x70, 0x6f,
	0x73, 0x69, 0x74, 0x52, 0x12, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x73, 0x74, 0x61, 0x72, 0x74, 0x44,
	0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x73, 0x22, 0x83, 0x01, 0x0a, 0x10, 0x53, 0x70, 0x61, 0x72,
	0x73, 0x65, 0x4d, 0x65, 0x72, 0x6b, 0x6c, 0x65, 0x54, 0x72, 0x69, 0x65, 0x12, 0x14, 0x0a, 0x05,
	0x64, 0x65, 0x70, 0x74, 0x68, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x05, 0x64, 0x65, 0x70,
	0x74, 0x68, 0x12, 0x32, 0x0a, 0x06, 0x6c, 0x61, 0x79, 0x65, 0x72, 0x73, 0x18, 0x02, 0x20, 0x03,
	0x28, 0x0b, 0x32, 0x1a, 0x2e, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2e, 0x62, 0x65, 0x61, 0x63, 0x6f,
	0x6e, 0x2e, 0x64, 0x62, 0x2e, 0x54, 0x72, 0x69, 0x65, 0x4c, 0x61, 0x79, 0x65, 0x72, 0x52, 0x06,
	0x6c, 0x61, 0x79, 0x65, 0x72, 0x73, 0x12, 0x25, 0x0a, 0x0e, 0x6f, 0x72, 0x69, 0x67, 0x69, 0x6e,
	0x61, 0x6c, 0x5f, 0x69, 0x74, 0x65, 0x6d, 0x73, 0x18, 0x03, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x0d,
	0x6f, 0x72, 0x69, 0x67, 0x69, 0x6e, 0x61, 0x6c, 0x49, 0x74, 0x65, 0x6d, 0x73, 0x22, 0x21, 0x0a,
	0x09, 0x54, 0x72, 0x69, 0x65, 0x4c, 0x61, 0x79, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x6c, 0x61,
	0x79, 0x65, 0x72, 0x18, 0x01, 0x20, 0x03, 0x28, 0x0c, 0x52, 0x05, 0x6c, 0x61, 0x79, 0x65, 0x72,
	0x22, 0xb1, 0x01, 0x0a, 0x10, 0x44, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x43, 0x6f, 0x6e, 0x74,
	0x61, 0x69, 0x6e, 0x65, 0x72, 0x12, 0x14, 0x0a, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x03, 0x52, 0x05, 0x69, 0x6e, 0x64, 0x65, 0x78, 0x12, 0x2a, 0x0a, 0x11, 0x65,
	0x74, 0x68, 0x31, 0x5f, 0x62, 0x6c, 0x6f, 0x63, 0x6b, 0x5f, 0x68, 0x65, 0x69, 0x67, 0x68, 0x74,
	0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x0f, 0x65, 0x74, 0x68, 0x31, 0x42, 0x6c, 0x6f, 0x63,
	0x6b, 0x48, 0x65, 0x69, 0x67, 0x68, 0x74, 0x12, 0x38, 0x0a, 0x07, 0x64, 0x65, 0x70, 0x6f, 0x73,
	0x69, 0x74, 0x18, 0x03, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1e, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72,
	0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x76, 0x31, 0x61, 0x6c, 0x70, 0x68, 0x61, 0x31,
	0x2e, 0x44, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x52, 0x07, 0x64, 0x65, 0x70, 0x6f, 0x73, 0x69,
	0x74, 0x12, 0x21, 0x0a, 0x0c, 0x64, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74, 0x5f, 0x72, 0x6f, 0x6f,
	0x74, 0x18, 0x04, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x0b, 0x64, 0x65, 0x70, 0x6f, 0x73, 0x69, 0x74,
	0x52, 0x6f, 0x6f, 0x74, 0x42, 0x30, 0x5a, 0x2e, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63,
	0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x61, 0x74, 0x69, 0x63, 0x6c, 0x61, 0x62, 0x73,
	0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x62, 0x65, 0x61,
	0x63, 0x6f, 0x6e, 0x2f, 0x64, 0x62, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_proto_beacon_db_powchain_proto_rawDescOnce sync.Once
	file_proto_beacon_db_powchain_proto_rawDescData = file_proto_beacon_db_powchain_proto_rawDesc
)

func file_proto_beacon_db_powchain_proto_rawDescGZIP() []byte {
	file_proto_beacon_db_powchain_proto_rawDescOnce.Do(func() {
		file_proto_beacon_db_powchain_proto_rawDescData = protoimpl.X.CompressGZIP(file_proto_beacon_db_powchain_proto_rawDescData)
	})
	return file_proto_beacon_db_powchain_proto_rawDescData
}

var file_proto_beacon_db_powchain_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_proto_beacon_db_powchain_proto_goTypes = []interface{}{
	(*ETH1ChainData)(nil),     // 0: prysm.beacon.db.ETH1ChainData
	(*LatestETH1Data)(nil),    // 1: prysm.beacon.db.LatestETH1Data
	(*ChainStartData)(nil),    // 2: prysm.beacon.db.ChainStartData
	(*SparseMerkleTrie)(nil),  // 3: prysm.beacon.db.SparseMerkleTrie
	(*TrieLayer)(nil),         // 4: prysm.beacon.db.TrieLayer
	(*DepositContainer)(nil),  // 5: prysm.beacon.db.DepositContainer
	(*v1.BeaconState)(nil),    // 6: ethereum.beacon.p2p.v1.BeaconState
	(*v1alpha1.Eth1Data)(nil), // 7: ethereum.eth.v1alpha1.Eth1Data
	(*v1alpha1.Deposit)(nil),  // 8: ethereum.eth.v1alpha1.Deposit
}
var file_proto_beacon_db_powchain_proto_depIdxs = []int32{
	1, // 0: prysm.beacon.db.ETH1ChainData.current_eth1_data:type_name -> prysm.beacon.db.LatestETH1Data
	2, // 1: prysm.beacon.db.ETH1ChainData.chainstart_data:type_name -> prysm.beacon.db.ChainStartData
	6, // 2: prysm.beacon.db.ETH1ChainData.beacon_state:type_name -> ethereum.beacon.p2p.v1.BeaconState
	3, // 3: prysm.beacon.db.ETH1ChainData.trie:type_name -> prysm.beacon.db.SparseMerkleTrie
	5, // 4: prysm.beacon.db.ETH1ChainData.deposit_containers:type_name -> prysm.beacon.db.DepositContainer
	7, // 5: prysm.beacon.db.ChainStartData.eth1_data:type_name -> ethereum.eth.v1alpha1.Eth1Data
	8, // 6: prysm.beacon.db.ChainStartData.chainstart_deposits:type_name -> ethereum.eth.v1alpha1.Deposit
	4, // 7: prysm.beacon.db.SparseMerkleTrie.layers:type_name -> prysm.beacon.db.TrieLayer
	8, // 8: prysm.beacon.db.DepositContainer.deposit:type_name -> ethereum.eth.v1alpha1.Deposit
	9, // [9:9] is the sub-list for method output_type
	9, // [9:9] is the sub-list for method input_type
	9, // [9:9] is the sub-list for extension type_name
	9, // [9:9] is the sub-list for extension extendee
	0, // [0:9] is the sub-list for field type_name
}

func init() { file_proto_beacon_db_powchain_proto_init() }
func file_proto_beacon_db_powchain_proto_init() {
	if File_proto_beacon_db_powchain_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_proto_beacon_db_powchain_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ETH1ChainData); i {
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
		file_proto_beacon_db_powchain_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*LatestETH1Data); i {
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
		file_proto_beacon_db_powchain_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ChainStartData); i {
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
		file_proto_beacon_db_powchain_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SparseMerkleTrie); i {
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
		file_proto_beacon_db_powchain_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*TrieLayer); i {
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
		file_proto_beacon_db_powchain_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DepositContainer); i {
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
			RawDescriptor: file_proto_beacon_db_powchain_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_proto_beacon_db_powchain_proto_goTypes,
		DependencyIndexes: file_proto_beacon_db_powchain_proto_depIdxs,
		MessageInfos:      file_proto_beacon_db_powchain_proto_msgTypes,
	}.Build()
	File_proto_beacon_db_powchain_proto = out.File
	file_proto_beacon_db_powchain_proto_rawDesc = nil
	file_proto_beacon_db_powchain_proto_goTypes = nil
	file_proto_beacon_db_powchain_proto_depIdxs = nil
}
