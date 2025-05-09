// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.36.6
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

const file_hdlctrl_v1_user_proto_rawDesc = "" +
	"\n" +
	"\x15hdlctrl/v1/user.proto\x12\n" +
	"hdlctrl.v1\"M\n" +
	"\x10TokenSetResponse\x12\x14\n" +
	"\x05token\x18\x01 \x01(\tR\x05token\x12#\n" +
	"\rrefresh_token\x18\x02 \x01(\tR\frefreshToken\"G\n" +
	"\x19GetTokenByPasswordRequest\x12\x0e\n" +
	"\x02id\x18\x01 \x01(\tR\x02id\x12\x1a\n" +
	"\bpassword\x18\x02 \x01(\tR\bpassword\"\x15\n" +
	"\x13RefreshTokenRequest2\xbb\x01\n" +
	"\vUserService\x12[\n" +
	"\x12GetTokenByPassword\x12%.hdlctrl.v1.GetTokenByPasswordRequest\x1a\x1c.hdlctrl.v1.TokenSetResponse\"\x00\x12O\n" +
	"\fRefreshToken\x12\x1f.hdlctrl.v1.RefreshTokenRequest\x1a\x1c.hdlctrl.v1.TokenSetResponse\"\x00B\xb7\x01\n" +
	"\x0ecom.hdlctrl.v1B\tUserProtoP\x01ZQgithub.com/hantabaru1014/baru-reso-headless-controller/pbgen/hdlctrl/v1;hdlctrlv1\xa2\x02\x03HXX\xaa\x02\n" +
	"Hdlctrl.V1\xca\x02\n" +
	"Hdlctrl\\V1\xe2\x02\x16Hdlctrl\\V1\\GPBMetadata\xea\x02\vHdlctrl::V1b\x06proto3"

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
