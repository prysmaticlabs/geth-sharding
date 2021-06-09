// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0
// 	protoc        v3.15.8
// source: proto/eth/ext/options.proto

package ext

import (
	reflect "reflect"

	proto "github.com/golang/protobuf/proto"
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
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

var file_proto_eth_ext_options_proto_extTypes = []protoimpl.ExtensionInfo{
	{
		ExtendedType:  (*descriptorpb.FieldOptions)(nil),
		ExtensionType: (*string)(nil),
		Field:         50000,
		Name:          "ethereum.eth.ext.cast_type",
		Tag:           "bytes,50000,opt,name=cast_type",
		Filename:      "proto/eth/ext/options.proto",
	},
	{
		ExtendedType:  (*descriptorpb.FieldOptions)(nil),
		ExtensionType: (*string)(nil),
		Field:         50001,
		Name:          "ethereum.eth.ext.ssz_size",
		Tag:           "bytes,50001,opt,name=ssz_size",
		Filename:      "proto/eth/ext/options.proto",
	},
	{
		ExtendedType:  (*descriptorpb.FieldOptions)(nil),
		ExtensionType: (*string)(nil),
		Field:         50002,
		Name:          "ethereum.eth.ext.ssz_max",
		Tag:           "bytes,50002,opt,name=ssz_max",
		Filename:      "proto/eth/ext/options.proto",
	},
	{
		ExtendedType:  (*descriptorpb.FieldOptions)(nil),
		ExtensionType: (*string)(nil),
		Field:         50003,
		Name:          "ethereum.eth.ext.spec_name",
		Tag:           "bytes,50003,opt,name=spec_name",
		Filename:      "proto/eth/ext/options.proto",
	},
}

// Extension fields to descriptorpb.FieldOptions.
var (
	// optional string cast_type = 50000;
	E_CastType = &file_proto_eth_ext_options_proto_extTypes[0]
	// optional string ssz_size = 50001;
	E_SszSize = &file_proto_eth_ext_options_proto_extTypes[1]
	// optional string ssz_max = 50002;
	E_SszMax = &file_proto_eth_ext_options_proto_extTypes[2]
	// optional string spec_name = 50003;
	E_SpecName = &file_proto_eth_ext_options_proto_extTypes[3]
)

var File_proto_eth_ext_options_proto protoreflect.FileDescriptor

var file_proto_eth_ext_options_proto_rawDesc = []byte{
	0x0a, 0x1b, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x2f, 0x65, 0x74, 0x68, 0x2f, 0x65, 0x78, 0x74, 0x2f,
	0x6f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x10, 0x65,
	0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x65, 0x78, 0x74, 0x1a,
	0x20, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2f, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2f, 0x64, 0x65, 0x73, 0x63, 0x72, 0x69, 0x70, 0x74, 0x6f, 0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x3a, 0x3c, 0x0a, 0x09, 0x63, 0x61, 0x73, 0x74, 0x5f, 0x74, 0x79, 0x70, 0x65, 0x12, 0x1d,
	0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66,
	0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xd0, 0x86,
	0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x63, 0x61, 0x73, 0x74, 0x54, 0x79, 0x70, 0x65, 0x3a,
	0x3a, 0x0a, 0x08, 0x73, 0x73, 0x7a, 0x5f, 0x73, 0x69, 0x7a, 0x65, 0x12, 0x1d, 0x2e, 0x67, 0x6f,
	0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x46, 0x69,
	0x65, 0x6c, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xd1, 0x86, 0x03, 0x20, 0x01,
	0x28, 0x09, 0x52, 0x07, 0x73, 0x73, 0x7a, 0x53, 0x69, 0x7a, 0x65, 0x3a, 0x38, 0x0a, 0x07, 0x73,
	0x73, 0x7a, 0x5f, 0x6d, 0x61, 0x78, 0x12, 0x1d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x62, 0x75, 0x66, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x4f, 0x70,
	0x74, 0x69, 0x6f, 0x6e, 0x73, 0x18, 0xd2, 0x86, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x06, 0x73,
	0x73, 0x7a, 0x4d, 0x61, 0x78, 0x3a, 0x3c, 0x0a, 0x09, 0x73, 0x70, 0x65, 0x63, 0x5f, 0x6e, 0x61,
	0x6d, 0x65, 0x12, 0x1d, 0x2e, 0x67, 0x6f, 0x6f, 0x67, 0x6c, 0x65, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x62, 0x75, 0x66, 0x2e, 0x46, 0x69, 0x65, 0x6c, 0x64, 0x4f, 0x70, 0x74, 0x69, 0x6f, 0x6e,
	0x73, 0x18, 0xd3, 0x86, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x73, 0x70, 0x65, 0x63, 0x4e,
	0x61, 0x6d, 0x65, 0x42, 0x7b, 0x0a, 0x14, 0x6f, 0x72, 0x67, 0x2e, 0x65, 0x74, 0x68, 0x65, 0x72,
	0x65, 0x75, 0x6d, 0x2e, 0x65, 0x74, 0x68, 0x2e, 0x65, 0x78, 0x74, 0x42, 0x0c, 0x4f, 0x70, 0x74,
	0x69, 0x6f, 0x6e, 0x73, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x2d, 0x67, 0x69, 0x74,
	0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x70, 0x72, 0x79, 0x73, 0x6d, 0x61, 0x74, 0x69,
	0x63, 0x6c, 0x61, 0x62, 0x73, 0x2f, 0x65, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x61, 0x70,
	0x69, 0x73, 0x2f, 0x65, 0x74, 0x68, 0x2f, 0x65, 0x78, 0x74, 0xaa, 0x02, 0x10, 0x45, 0x74, 0x68,
	0x65, 0x72, 0x65, 0x75, 0x6d, 0x2e, 0x45, 0x74, 0x68, 0x2e, 0x65, 0x78, 0x74, 0xca, 0x02, 0x10,
	0x45, 0x74, 0x68, 0x65, 0x72, 0x65, 0x75, 0x6d, 0x5c, 0x45, 0x74, 0x68, 0x5c, 0x65, 0x78, 0x74,
	0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var file_proto_eth_ext_options_proto_goTypes = []interface{}{
	(*descriptorpb.FieldOptions)(nil), // 0: google.protobuf.FieldOptions
}
var file_proto_eth_ext_options_proto_depIdxs = []int32{
	0, // 0: ethereum.eth.ext.cast_type:extendee -> google.protobuf.FieldOptions
	0, // 1: ethereum.eth.ext.ssz_size:extendee -> google.protobuf.FieldOptions
	0, // 2: ethereum.eth.ext.ssz_max:extendee -> google.protobuf.FieldOptions
	0, // 3: ethereum.eth.ext.spec_name:extendee -> google.protobuf.FieldOptions
	4, // [4:4] is the sub-list for method output_type
	4, // [4:4] is the sub-list for method input_type
	4, // [4:4] is the sub-list for extension type_name
	0, // [0:4] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_proto_eth_ext_options_proto_init() }
func file_proto_eth_ext_options_proto_init() {
	if File_proto_eth_ext_options_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_proto_eth_ext_options_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   0,
			NumExtensions: 4,
			NumServices:   0,
		},
		GoTypes:           file_proto_eth_ext_options_proto_goTypes,
		DependencyIndexes: file_proto_eth_ext_options_proto_depIdxs,
		ExtensionInfos:    file_proto_eth_ext_options_proto_extTypes,
	}.Build()
	File_proto_eth_ext_options_proto = out.File
	file_proto_eth_ext_options_proto_rawDesc = nil
	file_proto_eth_ext_options_proto_goTypes = nil
	file_proto_eth_ext_options_proto_depIdxs = nil
}