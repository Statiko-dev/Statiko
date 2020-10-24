//
//Copyright © 2020 Alessandro Segala (@ItalyPaleAle)
//
//This program is free software: you can redistribute it and/or modify
//it under the terms of the GNU Affero General Public License as published
//by the Free Software Foundation, version 3 of the License.
//
//This program is distributed in the hope that it will be useful,
//but WITHOUT ANY WARRANTY; without even the implied warranty of
//MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
//GNU Affero General Public License for more details.
//
//You should have received a copy of the GNU Affero General Public License
//along with this program.  If not, see <https://www.gnu.org/licenses/>.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.25.0-devel
// 	protoc        v3.13.0
// source: common.proto

package proto

import (
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

// Site definition
type Site struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Domains: primary and aliases
	Domain  string   `protobuf:"bytes,1,opt,name=domain,proto3" json:"domain,omitempty"`
	Aliases []string `protobuf:"bytes,2,rep,name=aliases,proto3" json:"aliases,omitempty"`
	// Temporary site (e.g. for testing)
	Temporary bool `protobuf:"varint,3,opt,name=temporary,proto3" json:"temporary,omitempty"`
	// ID of the generated TLS certificate
	GeneratedTlsId string `protobuf:"bytes,10,opt,name=generated_tls_id,json=generatedTlsId,proto3" json:"generated_tls_id,omitempty"`
	// Enable using ACME for requesting certificates instead of self-signed ones
	EnableAcme bool `protobuf:"varint,11,opt,name=enable_acme,json=enableAcme,proto3" json:"enable_acme,omitempty"`
	// ID of an imported TLS certificate to use (optional)
	ImportedTlsId string    `protobuf:"bytes,12,opt,name=imported_tls_id,json=importedTlsId,proto3" json:"imported_tls_id,omitempty"`
	App           *Site_App `protobuf:"bytes,20,opt,name=app,proto3" json:"app,omitempty"`
}

func (x *Site) Reset() {
	*x = Site{}
	if protoimpl.UnsafeEnabled {
		mi := &file_common_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Site) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Site) ProtoMessage() {}

func (x *Site) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Site.ProtoReflect.Descriptor instead.
func (*Site) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{0}
}

func (x *Site) GetDomain() string {
	if x != nil {
		return x.Domain
	}
	return ""
}

func (x *Site) GetAliases() []string {
	if x != nil {
		return x.Aliases
	}
	return nil
}

func (x *Site) GetTemporary() bool {
	if x != nil {
		return x.Temporary
	}
	return false
}

func (x *Site) GetGeneratedTlsId() string {
	if x != nil {
		return x.GeneratedTlsId
	}
	return ""
}

func (x *Site) GetEnableAcme() bool {
	if x != nil {
		return x.EnableAcme
	}
	return false
}

func (x *Site) GetImportedTlsId() string {
	if x != nil {
		return x.ImportedTlsId
	}
	return ""
}

func (x *Site) GetApp() *Site_App {
	if x != nil {
		return x.App
	}
	return nil
}

// DH Parameters
type DHParams struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Generation date (UNIX timestamp)
	Date int64 `protobuf:"varint,1,opt,name=date,proto3" json:"date,omitempty"`
	// PEM-encoded value
	Pem string `protobuf:"bytes,2,opt,name=pem,proto3" json:"pem,omitempty"`
}

func (x *DHParams) Reset() {
	*x = DHParams{}
	if protoimpl.UnsafeEnabled {
		mi := &file_common_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *DHParams) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*DHParams) ProtoMessage() {}

func (x *DHParams) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use DHParams.ProtoReflect.Descriptor instead.
func (*DHParams) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{1}
}

func (x *DHParams) GetDate() int64 {
	if x != nil {
		return x.Date
	}
	return 0
}

func (x *DHParams) GetPem() string {
	if x != nil {
		return x.Pem
	}
	return ""
}

// App (optional)
type Site_App struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// App name
	Name string `protobuf:"bytes,1,opt,name=name,proto3" json:"name,omitempty"`
}

func (x *Site_App) Reset() {
	*x = Site_App{}
	if protoimpl.UnsafeEnabled {
		mi := &file_common_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Site_App) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Site_App) ProtoMessage() {}

func (x *Site_App) ProtoReflect() protoreflect.Message {
	mi := &file_common_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Site_App.ProtoReflect.Descriptor instead.
func (*Site_App) Descriptor() ([]byte, []int) {
	return file_common_proto_rawDescGZIP(), []int{0, 0}
}

func (x *Site_App) GetName() string {
	if x != nil {
		return x.Name
	}
	return ""
}

var File_common_proto protoreflect.FileDescriptor

var file_common_proto_rawDesc = []byte{
	0x0a, 0x0c, 0x63, 0x6f, 0x6d, 0x6d, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07,
	0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f, 0x22, 0x89, 0x02, 0x0a, 0x04, 0x53, 0x69, 0x74, 0x65,
	0x12, 0x16, 0x0a, 0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x61, 0x6c, 0x69, 0x61,
	0x73, 0x65, 0x73, 0x18, 0x02, 0x20, 0x03, 0x28, 0x09, 0x52, 0x07, 0x61, 0x6c, 0x69, 0x61, 0x73,
	0x65, 0x73, 0x12, 0x1c, 0x0a, 0x09, 0x74, 0x65, 0x6d, 0x70, 0x6f, 0x72, 0x61, 0x72, 0x79, 0x18,
	0x03, 0x20, 0x01, 0x28, 0x08, 0x52, 0x09, 0x74, 0x65, 0x6d, 0x70, 0x6f, 0x72, 0x61, 0x72, 0x79,
	0x12, 0x28, 0x0a, 0x10, 0x67, 0x65, 0x6e, 0x65, 0x72, 0x61, 0x74, 0x65, 0x64, 0x5f, 0x74, 0x6c,
	0x73, 0x5f, 0x69, 0x64, 0x18, 0x0a, 0x20, 0x01, 0x28, 0x09, 0x52, 0x0e, 0x67, 0x65, 0x6e, 0x65,
	0x72, 0x61, 0x74, 0x65, 0x64, 0x54, 0x6c, 0x73, 0x49, 0x64, 0x12, 0x1f, 0x0a, 0x0b, 0x65, 0x6e,
	0x61, 0x62, 0x6c, 0x65, 0x5f, 0x61, 0x63, 0x6d, 0x65, 0x18, 0x0b, 0x20, 0x01, 0x28, 0x08, 0x52,
	0x0a, 0x65, 0x6e, 0x61, 0x62, 0x6c, 0x65, 0x41, 0x63, 0x6d, 0x65, 0x12, 0x26, 0x0a, 0x0f, 0x69,
	0x6d, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x5f, 0x74, 0x6c, 0x73, 0x5f, 0x69, 0x64, 0x18, 0x0c,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x0d, 0x69, 0x6d, 0x70, 0x6f, 0x72, 0x74, 0x65, 0x64, 0x54, 0x6c,
	0x73, 0x49, 0x64, 0x12, 0x23, 0x0a, 0x03, 0x61, 0x70, 0x70, 0x18, 0x14, 0x20, 0x01, 0x28, 0x0b,
	0x32, 0x11, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f, 0x2e, 0x53, 0x69, 0x74, 0x65, 0x2e,
	0x41, 0x70, 0x70, 0x52, 0x03, 0x61, 0x70, 0x70, 0x1a, 0x19, 0x0a, 0x03, 0x41, 0x70, 0x70, 0x12,
	0x12, 0x0a, 0x04, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x04, 0x6e,
	0x61, 0x6d, 0x65, 0x22, 0x30, 0x0a, 0x08, 0x44, 0x48, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x73, 0x12,
	0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x03, 0x52, 0x04, 0x64,
	0x61, 0x74, 0x65, 0x12, 0x10, 0x0a, 0x03, 0x70, 0x65, 0x6d, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x03, 0x70, 0x65, 0x6d, 0x42, 0x2d, 0x5a, 0x2b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e,
	0x63, 0x6f, 0x6d, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f, 0x2d, 0x64, 0x65, 0x76, 0x2f,
	0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f, 0x2f, 0x73, 0x68, 0x61, 0x72, 0x65, 0x64, 0x2f, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_common_proto_rawDescOnce sync.Once
	file_common_proto_rawDescData = file_common_proto_rawDesc
)

func file_common_proto_rawDescGZIP() []byte {
	file_common_proto_rawDescOnce.Do(func() {
		file_common_proto_rawDescData = protoimpl.X.CompressGZIP(file_common_proto_rawDescData)
	})
	return file_common_proto_rawDescData
}

var file_common_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_common_proto_goTypes = []interface{}{
	(*Site)(nil),     // 0: statiko.Site
	(*DHParams)(nil), // 1: statiko.DHParams
	(*Site_App)(nil), // 2: statiko.Site.App
}
var file_common_proto_depIdxs = []int32{
	2, // 0: statiko.Site.app:type_name -> statiko.Site.App
	1, // [1:1] is the sub-list for method output_type
	1, // [1:1] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_common_proto_init() }
func file_common_proto_init() {
	if File_common_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_common_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Site); i {
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
		file_common_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*DHParams); i {
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
		file_common_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Site_App); i {
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
			RawDescriptor: file_common_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_common_proto_goTypes,
		DependencyIndexes: file_common_proto_depIdxs,
		MessageInfos:      file_common_proto_msgTypes,
	}.Build()
	File_common_proto = out.File
	file_common_proto_rawDesc = nil
	file_common_proto_goTypes = nil
	file_common_proto_depIdxs = nil
}
