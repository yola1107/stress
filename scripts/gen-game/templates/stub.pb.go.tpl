// Code scaffold by stress/scripts/gen-game (见 api/common/pb/stub.pb.go 同形；RawDesc 由 marshalStubFileDescriptorWire 生成)。
//
// source: biz/game/{{ .Pkg }}/pb/stub.proto

// Code generated — style aligned with protoc-gen-go. DO NOT EDIT by hand.

package pb

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that the generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// {{ .Name }}（GameID: {{ .ID }}）

type Stub_BetOrderResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *Stub_BetOrderResponse) Reset() {
	*x = Stub_BetOrderResponse{}
	mi := &file_common_pb_stub_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *Stub_BetOrderResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Stub_BetOrderResponse) ProtoMessage() {}

func (x *Stub_BetOrderResponse) ProtoReflect() protoreflect.Message {
	mi := &file_common_pb_stub_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Stub_BetOrderResponse.ProtoReflect.Descriptor instead.
func (*Stub_BetOrderResponse) Descriptor() ([]byte, []int) {
	return file_common_pb_stub_proto_rawDescGZIP(), []int{0}
}

var File_common_pb_stub_proto protoreflect.FileDescriptor

{{ .RawDescConst }}

var (
	file_common_pb_stub_proto_rawDescOnce sync.Once
	file_common_pb_stub_proto_rawDescData []byte
)

func file_common_pb_stub_proto_rawDescGZIP() []byte {
	file_common_pb_stub_proto_rawDescOnce.Do(func() {
		file_common_pb_stub_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_common_pb_stub_proto_rawDesc), len(file_common_pb_stub_proto_rawDesc)))
	})
	return file_common_pb_stub_proto_rawDescData
}

var file_common_pb_stub_proto_msgTypes = make([]protoimpl.MessageInfo, 1)
var file_common_pb_stub_proto_goTypes = []any{
	(*Stub_BetOrderResponse)(nil), // 0: {{ .ProtoPkg }}.Stub_BetOrderResponse
}
var file_common_pb_stub_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_common_pb_stub_proto_init() }

func file_common_pb_stub_proto_init() {
	if File_common_pb_stub_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_common_pb_stub_proto_rawDesc), len(file_common_pb_stub_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   1,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_common_pb_stub_proto_goTypes,
		DependencyIndexes: file_common_pb_stub_proto_depIdxs,
		MessageInfos:      file_common_pb_stub_proto_msgTypes,
	}.Build()
	File_common_pb_stub_proto = out.File
	file_common_pb_stub_proto_goTypes = nil
	file_common_pb_stub_proto_depIdxs = nil
}
