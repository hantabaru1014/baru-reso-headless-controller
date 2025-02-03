// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.4
// 	protoc        (unknown)
// source: hdlctrl/v1/user.proto

package hdlctrlv1

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
	unsafe "unsafe"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type TokenSetResponse struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Token         string                 `protobuf:"bytes,1,opt,name=token,proto3" json:"token,omitempty"`
	RefreshToken  string                 `protobuf:"bytes,2,opt,name=refresh_token,json=refreshToken,proto3" json:"refresh_token,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *TokenSetResponse) Reset() {
	*x = TokenSetResponse{}
	mi := &file_hdlctrl_v1_user_proto_msgTypes[0]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *TokenSetResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*TokenSetResponse) ProtoMessage() {}

func (x *TokenSetResponse) ProtoReflect() protoreflect.Message {
	mi := &file_hdlctrl_v1_user_proto_msgTypes[0]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use TokenSetResponse.ProtoReflect.Descriptor instead.
func (*TokenSetResponse) Descriptor() ([]byte, []int) {
	return file_hdlctrl_v1_user_proto_rawDescGZIP(), []int{0}
}

func (x *TokenSetResponse) GetToken() string {
	if x != nil {
		return x.Token
	}
	return ""
}

func (x *TokenSetResponse) GetRefreshToken() string {
	if x != nil {
		return x.RefreshToken
	}
	return ""
}

type GetTokenByPasswordRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	Id            string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Password      string                 `protobuf:"bytes,2,opt,name=password,proto3" json:"password,omitempty"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *GetTokenByPasswordRequest) Reset() {
	*x = GetTokenByPasswordRequest{}
	mi := &file_hdlctrl_v1_user_proto_msgTypes[1]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *GetTokenByPasswordRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetTokenByPasswordRequest) ProtoMessage() {}

func (x *GetTokenByPasswordRequest) ProtoReflect() protoreflect.Message {
	mi := &file_hdlctrl_v1_user_proto_msgTypes[1]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetTokenByPasswordRequest.ProtoReflect.Descriptor instead.
func (*GetTokenByPasswordRequest) Descriptor() ([]byte, []int) {
	return file_hdlctrl_v1_user_proto_rawDescGZIP(), []int{1}
}

func (x *GetTokenByPasswordRequest) GetId() string {
	if x != nil {
		return x.Id
	}
	return ""
}

func (x *GetTokenByPasswordRequest) GetPassword() string {
	if x != nil {
		return x.Password
	}
	return ""
}

// 既に持っているトークンをheaderに付与してリクエストする
type RefreshTokenRequest struct {
	state         protoimpl.MessageState `protogen:"open.v1"`
	unknownFields protoimpl.UnknownFields
	sizeCache     protoimpl.SizeCache
}

func (x *RefreshTokenRequest) Reset() {
	*x = RefreshTokenRequest{}
	mi := &file_hdlctrl_v1_user_proto_msgTypes[2]
	ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
	ms.StoreMessageInfo(mi)
}

func (x *RefreshTokenRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RefreshTokenRequest) ProtoMessage() {}

func (x *RefreshTokenRequest) ProtoReflect() protoreflect.Message {
	mi := &file_hdlctrl_v1_user_proto_msgTypes[2]
	if x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RefreshTokenRequest.ProtoReflect.Descriptor instead.
func (*RefreshTokenRequest) Descriptor() ([]byte, []int) {
	return file_hdlctrl_v1_user_proto_rawDescGZIP(), []int{2}
}

var File_hdlctrl_v1_user_proto protoreflect.FileDescriptor

var file_hdlctrl_v1_user_proto_rawDesc = string([]byte{
	0x0a, 0x15, 0x68, 0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c, 0x2f, 0x76, 0x31, 0x2f, 0x75, 0x73, 0x65,
	0x72, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x0a, 0x68, 0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c,
	0x2e, 0x76, 0x31, 0x22, 0x4d, 0x0a, 0x10, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x53, 0x65, 0x74, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x14, 0x0a, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x12, 0x23, 0x0a,
	0x0d, 0x72, 0x65, 0x66, 0x72, 0x65, 0x73, 0x68, 0x5f, 0x74, 0x6f, 0x6b, 0x65, 0x6e, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0c, 0x72, 0x65, 0x66, 0x72, 0x65, 0x73, 0x68, 0x54, 0x6f, 0x6b,
	0x65, 0x6e, 0x22, 0x47, 0x0a, 0x19, 0x47, 0x65, 0x74, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x42, 0x79,
	0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x0e, 0x0a, 0x02, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x02, 0x69, 0x64, 0x12,
	0x1a, 0x0a, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x18, 0x02, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x70, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x22, 0x15, 0x0a, 0x13, 0x52,
	0x65, 0x66, 0x72, 0x65, 0x73, 0x68, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65,
	0x73, 0x74, 0x32, 0xbb, 0x01, 0x0a, 0x0b, 0x55, 0x73, 0x65, 0x72, 0x53, 0x65, 0x72, 0x76, 0x69,
	0x63, 0x65, 0x12, 0x5b, 0x0a, 0x12, 0x47, 0x65, 0x74, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x42, 0x79,
	0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x12, 0x25, 0x2e, 0x68, 0x64, 0x6c, 0x63, 0x74,
	0x72, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x42, 0x79,
	0x50, 0x61, 0x73, 0x73, 0x77, 0x6f, 0x72, 0x64, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a,
	0x1c, 0x2e, 0x68, 0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x54, 0x6f, 0x6b,
	0x65, 0x6e, 0x53, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12,
	0x4f, 0x0a, 0x0c, 0x52, 0x65, 0x66, 0x72, 0x65, 0x73, 0x68, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x12,
	0x1f, 0x2e, 0x68, 0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x52, 0x65, 0x66,
	0x72, 0x65, 0x73, 0x68, 0x54, 0x6f, 0x6b, 0x65, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x1a, 0x1c, 0x2e, 0x68, 0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c, 0x2e, 0x76, 0x31, 0x2e, 0x54, 0x6f,
	0x6b, 0x65, 0x6e, 0x53, 0x65, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00,
	0x42, 0xb7, 0x01, 0x0a, 0x0e, 0x63, 0x6f, 0x6d, 0x2e, 0x68, 0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c,
	0x2e, 0x76, 0x31, 0x42, 0x09, 0x55, 0x73, 0x65, 0x72, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01,
	0x5a, 0x51, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x61, 0x6e,
	0x74, 0x61, 0x62, 0x61, 0x72, 0x75, 0x31, 0x30, 0x31, 0x34, 0x2f, 0x62, 0x61, 0x72, 0x75, 0x2d,
	0x72, 0x65, 0x73, 0x6f, 0x2d, 0x68, 0x65, 0x61, 0x64, 0x6c, 0x65, 0x73, 0x73, 0x2d, 0x63, 0x6f,
	0x6e, 0x74, 0x72, 0x6f, 0x6c, 0x6c, 0x65, 0x72, 0x2f, 0x70, 0x62, 0x67, 0x65, 0x6e, 0x2f, 0x68,
	0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c, 0x2f, 0x76, 0x31, 0x3b, 0x68, 0x64, 0x6c, 0x63, 0x74, 0x72,
	0x6c, 0x76, 0x31, 0xa2, 0x02, 0x03, 0x48, 0x58, 0x58, 0xaa, 0x02, 0x0a, 0x48, 0x64, 0x6c, 0x63,
	0x74, 0x72, 0x6c, 0x2e, 0x56, 0x31, 0xca, 0x02, 0x0a, 0x48, 0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c,
	0x5c, 0x56, 0x31, 0xe2, 0x02, 0x16, 0x48, 0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c, 0x5c, 0x56, 0x31,
	0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x0b, 0x48,
	0x64, 0x6c, 0x63, 0x74, 0x72, 0x6c, 0x3a, 0x3a, 0x56, 0x31, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x33,
})

var (
	file_hdlctrl_v1_user_proto_rawDescOnce sync.Once
	file_hdlctrl_v1_user_proto_rawDescData []byte
)

func file_hdlctrl_v1_user_proto_rawDescGZIP() []byte {
	file_hdlctrl_v1_user_proto_rawDescOnce.Do(func() {
		file_hdlctrl_v1_user_proto_rawDescData = protoimpl.X.CompressGZIP(unsafe.Slice(unsafe.StringData(file_hdlctrl_v1_user_proto_rawDesc), len(file_hdlctrl_v1_user_proto_rawDesc)))
	})
	return file_hdlctrl_v1_user_proto_rawDescData
}

var file_hdlctrl_v1_user_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_hdlctrl_v1_user_proto_goTypes = []any{
	(*TokenSetResponse)(nil),          // 0: hdlctrl.v1.TokenSetResponse
	(*GetTokenByPasswordRequest)(nil), // 1: hdlctrl.v1.GetTokenByPasswordRequest
	(*RefreshTokenRequest)(nil),       // 2: hdlctrl.v1.RefreshTokenRequest
}
var file_hdlctrl_v1_user_proto_depIdxs = []int32{
	1, // 0: hdlctrl.v1.UserService.GetTokenByPassword:input_type -> hdlctrl.v1.GetTokenByPasswordRequest
	2, // 1: hdlctrl.v1.UserService.RefreshToken:input_type -> hdlctrl.v1.RefreshTokenRequest
	0, // 2: hdlctrl.v1.UserService.GetTokenByPassword:output_type -> hdlctrl.v1.TokenSetResponse
	0, // 3: hdlctrl.v1.UserService.RefreshToken:output_type -> hdlctrl.v1.TokenSetResponse
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_hdlctrl_v1_user_proto_init() }
func file_hdlctrl_v1_user_proto_init() {
	if File_hdlctrl_v1_user_proto != nil {
		return
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: unsafe.Slice(unsafe.StringData(file_hdlctrl_v1_user_proto_rawDesc), len(file_hdlctrl_v1_user_proto_rawDesc)),
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_hdlctrl_v1_user_proto_goTypes,
		DependencyIndexes: file_hdlctrl_v1_user_proto_depIdxs,
		MessageInfos:      file_hdlctrl_v1_user_proto_msgTypes,
	}.Build()
	File_hdlctrl_v1_user_proto = out.File
	file_hdlctrl_v1_user_proto_goTypes = nil
	file_hdlctrl_v1_user_proto_depIdxs = nil
}
