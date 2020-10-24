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
// source: node-health.proto

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

// Message containing the health of a node
type NodeHealth struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Name of the node
	NodeName string `protobuf:"bytes,1,opt,name=node_name,json=nodeName,proto3" json:"node_name,omitempty"`
	// State version
	Version   uint64                `protobuf:"varint,2,opt,name=version,proto3" json:"version,omitempty"`
	WebServer *NodeHealth_WebServer `protobuf:"bytes,5,opt,name=web_server,json=webServer,proto3" json:"web_server,omitempty"`
	Sync      *NodeHealth_Sync      `protobuf:"bytes,6,opt,name=sync,proto3" json:"sync,omitempty"`
	Sites     []*NodeHealth_Site    `protobuf:"bytes,10,rep,name=sites,proto3" json:"sites,omitempty"`
	// Internal use only - do not use
	XError string `protobuf:"bytes,2001,opt,name=_error,json=Error,proto3" json:"_error,omitempty"`
}

func (x *NodeHealth) Reset() {
	*x = NodeHealth{}
	if protoimpl.UnsafeEnabled {
		mi := &file_node_health_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NodeHealth) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeHealth) ProtoMessage() {}

func (x *NodeHealth) ProtoReflect() protoreflect.Message {
	mi := &file_node_health_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeHealth.ProtoReflect.Descriptor instead.
func (*NodeHealth) Descriptor() ([]byte, []int) {
	return file_node_health_proto_rawDescGZIP(), []int{0}
}

func (x *NodeHealth) GetNodeName() string {
	if x != nil {
		return x.NodeName
	}
	return ""
}

func (x *NodeHealth) GetVersion() uint64 {
	if x != nil {
		return x.Version
	}
	return 0
}

func (x *NodeHealth) GetWebServer() *NodeHealth_WebServer {
	if x != nil {
		return x.WebServer
	}
	return nil
}

func (x *NodeHealth) GetSync() *NodeHealth_Sync {
	if x != nil {
		return x.Sync
	}
	return nil
}

func (x *NodeHealth) GetSites() []*NodeHealth_Site {
	if x != nil {
		return x.Sites
	}
	return nil
}

func (x *NodeHealth) GetXError() string {
	if x != nil {
		return x.XError
	}
	return ""
}

// Health of the web server
type NodeHealth_WebServer struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Web server is healthy
	Healthy bool `protobuf:"varint,1,opt,name=healthy,proto3" json:"healthy,omitempty"`
}

func (x *NodeHealth_WebServer) Reset() {
	*x = NodeHealth_WebServer{}
	if protoimpl.UnsafeEnabled {
		mi := &file_node_health_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NodeHealth_WebServer) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeHealth_WebServer) ProtoMessage() {}

func (x *NodeHealth_WebServer) ProtoReflect() protoreflect.Message {
	mi := &file_node_health_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeHealth_WebServer.ProtoReflect.Descriptor instead.
func (*NodeHealth_WebServer) Descriptor() ([]byte, []int) {
	return file_node_health_proto_rawDescGZIP(), []int{0, 0}
}

func (x *NodeHealth_WebServer) GetHealthy() bool {
	if x != nil {
		return x.Healthy
	}
	return false
}

// Sync activity
type NodeHealth_Sync struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Sync is running
	Running bool `protobuf:"varint,1,opt,name=running,proto3" json:"running,omitempty"`
	// Last sync time (UNIX timestamp)
	LastSync int64 `protobuf:"varint,2,opt,name=last_sync,json=lastSync,proto3" json:"last_sync,omitempty"`
	// Last sync error (optional)
	SyncError string `protobuf:"bytes,3,opt,name=sync_error,json=syncError,proto3" json:"sync_error,omitempty"`
}

func (x *NodeHealth_Sync) Reset() {
	*x = NodeHealth_Sync{}
	if protoimpl.UnsafeEnabled {
		mi := &file_node_health_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NodeHealth_Sync) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeHealth_Sync) ProtoMessage() {}

func (x *NodeHealth_Sync) ProtoReflect() protoreflect.Message {
	mi := &file_node_health_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeHealth_Sync.ProtoReflect.Descriptor instead.
func (*NodeHealth_Sync) Descriptor() ([]byte, []int) {
	return file_node_health_proto_rawDescGZIP(), []int{0, 1}
}

func (x *NodeHealth_Sync) GetRunning() bool {
	if x != nil {
		return x.Running
	}
	return false
}

func (x *NodeHealth_Sync) GetLastSync() int64 {
	if x != nil {
		return x.LastSync
	}
	return 0
}

func (x *NodeHealth_Sync) GetSyncError() string {
	if x != nil {
		return x.SyncError
	}
	return ""
}

// Sites
type NodeHealth_Site struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	// Domain name
	Domain string `protobuf:"bytes,1,opt,name=domain,proto3" json:"domain,omitempty"`
	// Deployed app (optional)
	App string `protobuf:"bytes,2,opt,name=app,proto3" json:"app,omitempty"`
	// App error (optional)
	Error string `protobuf:"bytes,3,opt,name=error,proto3" json:"error,omitempty"`
}

func (x *NodeHealth_Site) Reset() {
	*x = NodeHealth_Site{}
	if protoimpl.UnsafeEnabled {
		mi := &file_node_health_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *NodeHealth_Site) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*NodeHealth_Site) ProtoMessage() {}

func (x *NodeHealth_Site) ProtoReflect() protoreflect.Message {
	mi := &file_node_health_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use NodeHealth_Site.ProtoReflect.Descriptor instead.
func (*NodeHealth_Site) Descriptor() ([]byte, []int) {
	return file_node_health_proto_rawDescGZIP(), []int{0, 2}
}

func (x *NodeHealth_Site) GetDomain() string {
	if x != nil {
		return x.Domain
	}
	return ""
}

func (x *NodeHealth_Site) GetApp() string {
	if x != nil {
		return x.App
	}
	return ""
}

func (x *NodeHealth_Site) GetError() string {
	if x != nil {
		return x.Error
	}
	return ""
}

var File_node_health_proto protoreflect.FileDescriptor

var file_node_health_proto_rawDesc = []byte{
	0x0a, 0x11, 0x6e, 0x6f, 0x64, 0x65, 0x2d, 0x68, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x2e, 0x70, 0x72,
	0x6f, 0x74, 0x6f, 0x12, 0x07, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f, 0x22, 0xc4, 0x03, 0x0a,
	0x0a, 0x4e, 0x6f, 0x64, 0x65, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x12, 0x1b, 0x0a, 0x09, 0x6e,
	0x6f, 0x64, 0x65, 0x5f, 0x6e, 0x61, 0x6d, 0x65, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08,
	0x6e, 0x6f, 0x64, 0x65, 0x4e, 0x61, 0x6d, 0x65, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73,
	0x69, 0x6f, 0x6e, 0x18, 0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69,
	0x6f, 0x6e, 0x12, 0x3c, 0x0a, 0x0a, 0x77, 0x65, 0x62, 0x5f, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72,
	0x18, 0x05, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x1d, 0x2e, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f,
	0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x48, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x2e, 0x57, 0x65, 0x62, 0x53,
	0x65, 0x72, 0x76, 0x65, 0x72, 0x52, 0x09, 0x77, 0x65, 0x62, 0x53, 0x65, 0x72, 0x76, 0x65, 0x72,
	0x12, 0x2c, 0x0a, 0x04, 0x73, 0x79, 0x6e, 0x63, 0x18, 0x06, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x18,
	0x2e, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x48, 0x65, 0x61,
	0x6c, 0x74, 0x68, 0x2e, 0x53, 0x79, 0x6e, 0x63, 0x52, 0x04, 0x73, 0x79, 0x6e, 0x63, 0x12, 0x2e,
	0x0a, 0x05, 0x73, 0x69, 0x74, 0x65, 0x73, 0x18, 0x0a, 0x20, 0x03, 0x28, 0x0b, 0x32, 0x18, 0x2e,
	0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f, 0x2e, 0x4e, 0x6f, 0x64, 0x65, 0x48, 0x65, 0x61, 0x6c,
	0x74, 0x68, 0x2e, 0x53, 0x69, 0x74, 0x65, 0x52, 0x05, 0x73, 0x69, 0x74, 0x65, 0x73, 0x12, 0x16,
	0x0a, 0x06, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0xd1, 0x0f, 0x20, 0x01, 0x28, 0x09, 0x52,
	0x05, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x1a, 0x25, 0x0a, 0x09, 0x57, 0x65, 0x62, 0x53, 0x65, 0x72,
	0x76, 0x65, 0x72, 0x12, 0x18, 0x0a, 0x07, 0x68, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x79, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x68, 0x65, 0x61, 0x6c, 0x74, 0x68, 0x79, 0x1a, 0x5c, 0x0a,
	0x04, 0x53, 0x79, 0x6e, 0x63, 0x12, 0x18, 0x0a, 0x07, 0x72, 0x75, 0x6e, 0x6e, 0x69, 0x6e, 0x67,
	0x18, 0x01, 0x20, 0x01, 0x28, 0x08, 0x52, 0x07, 0x72, 0x75, 0x6e, 0x6e, 0x69, 0x6e, 0x67, 0x12,
	0x1b, 0x0a, 0x09, 0x6c, 0x61, 0x73, 0x74, 0x5f, 0x73, 0x79, 0x6e, 0x63, 0x18, 0x02, 0x20, 0x01,
	0x28, 0x03, 0x52, 0x08, 0x6c, 0x61, 0x73, 0x74, 0x53, 0x79, 0x6e, 0x63, 0x12, 0x1d, 0x0a, 0x0a,
	0x73, 0x79, 0x6e, 0x63, 0x5f, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09,
	0x52, 0x09, 0x73, 0x79, 0x6e, 0x63, 0x45, 0x72, 0x72, 0x6f, 0x72, 0x1a, 0x46, 0x0a, 0x04, 0x53,
	0x69, 0x74, 0x65, 0x12, 0x16, 0x0a, 0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x06, 0x64, 0x6f, 0x6d, 0x61, 0x69, 0x6e, 0x12, 0x10, 0x0a, 0x03, 0x61,
	0x70, 0x70, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x03, 0x61, 0x70, 0x70, 0x12, 0x14, 0x0a,
	0x05, 0x65, 0x72, 0x72, 0x6f, 0x72, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x05, 0x65, 0x72,
	0x72, 0x6f, 0x72, 0x42, 0x2d, 0x5a, 0x2b, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f,
	0x6d, 0x2f, 0x73, 0x74, 0x61, 0x74, 0x69, 0x6b, 0x6f, 0x2d, 0x64, 0x65, 0x76, 0x2f, 0x73, 0x74,
	0x61, 0x74, 0x69, 0x6b, 0x6f, 0x2f, 0x73, 0x68, 0x61, 0x72, 0x65, 0x64, 0x2f, 0x70, 0x72, 0x6f,
	0x74, 0x6f, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_node_health_proto_rawDescOnce sync.Once
	file_node_health_proto_rawDescData = file_node_health_proto_rawDesc
)

func file_node_health_proto_rawDescGZIP() []byte {
	file_node_health_proto_rawDescOnce.Do(func() {
		file_node_health_proto_rawDescData = protoimpl.X.CompressGZIP(file_node_health_proto_rawDescData)
	})
	return file_node_health_proto_rawDescData
}

var file_node_health_proto_msgTypes = make([]protoimpl.MessageInfo, 4)
var file_node_health_proto_goTypes = []interface{}{
	(*NodeHealth)(nil),           // 0: statiko.NodeHealth
	(*NodeHealth_WebServer)(nil), // 1: statiko.NodeHealth.WebServer
	(*NodeHealth_Sync)(nil),      // 2: statiko.NodeHealth.Sync
	(*NodeHealth_Site)(nil),      // 3: statiko.NodeHealth.Site
}
var file_node_health_proto_depIdxs = []int32{
	1, // 0: statiko.NodeHealth.web_server:type_name -> statiko.NodeHealth.WebServer
	2, // 1: statiko.NodeHealth.sync:type_name -> statiko.NodeHealth.Sync
	3, // 2: statiko.NodeHealth.sites:type_name -> statiko.NodeHealth.Site
	3, // [3:3] is the sub-list for method output_type
	3, // [3:3] is the sub-list for method input_type
	3, // [3:3] is the sub-list for extension type_name
	3, // [3:3] is the sub-list for extension extendee
	0, // [0:3] is the sub-list for field type_name
}

func init() { file_node_health_proto_init() }
func file_node_health_proto_init() {
	if File_node_health_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_node_health_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NodeHealth); i {
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
		file_node_health_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NodeHealth_WebServer); i {
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
		file_node_health_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NodeHealth_Sync); i {
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
		file_node_health_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*NodeHealth_Site); i {
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
			RawDescriptor: file_node_health_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   4,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_node_health_proto_goTypes,
		DependencyIndexes: file_node_health_proto_depIdxs,
		MessageInfos:      file_node_health_proto_msgTypes,
	}.Build()
	File_node_health_proto = out.File
	file_node_health_proto_rawDesc = nil
	file_node_health_proto_goTypes = nil
	file_node_health_proto_depIdxs = nil
}
