// Copyright 2017 Lei Ni (nilei81@gmail.com).
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.28.1
// 	protoc        v3.21.5
// source: multiraft.proto

package multiraftpb

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

// Session is the session object used to track proposals for the
// specified raft cluster. SeriesID is a sequential id used to identify
// proposals, RespondedTo is a sequential id used to track the last
// responded proposal.
type Session struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ShardID     uint64 `protobuf:"varint,1,opt,name=ShardID,proto3" json:"ShardID,omitempty"`
	ClientID    uint64 `protobuf:"varint,2,opt,name=ClientID,proto3" json:"ClientID,omitempty"`
	SeriesID    uint64 `protobuf:"varint,3,opt,name=SeriesID,proto3" json:"SeriesID,omitempty"`
	RespondedTo uint64 `protobuf:"varint,4,opt,name=RespondedTo,proto3" json:"RespondedTo,omitempty"`
}

func (x *Session) Reset() {
	*x = Session{}
	if protoimpl.UnsafeEnabled {
		mi := &file_multiraft_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *Session) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*Session) ProtoMessage() {}

func (x *Session) ProtoReflect() protoreflect.Message {
	mi := &file_multiraft_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use Session.ProtoReflect.Descriptor instead.
func (*Session) Descriptor() ([]byte, []int) {
	return file_multiraft_proto_rawDescGZIP(), []int{0}
}

func (x *Session) GetShardID() uint64 {
	if x != nil {
		return x.ShardID
	}
	return 0
}

func (x *Session) GetClientID() uint64 {
	if x != nil {
		return x.ClientID
	}
	return 0
}

func (x *Session) GetSeriesID() uint64 {
	if x != nil {
		return x.SeriesID
	}
	return 0
}

func (x *Session) GetRespondedTo() uint64 {
	if x != nil {
		return x.RespondedTo
	}
	return 0
}

// SessionRequest is the message used to specified the interested raft
// cluster.
type SessionRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ShardId uint64 `protobuf:"varint,1,opt,name=shard_id,json=shardId,proto3" json:"shard_id,omitempty"`
}

func (x *SessionRequest) Reset() {
	*x = SessionRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_multiraft_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SessionRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SessionRequest) ProtoMessage() {}

func (x *SessionRequest) ProtoReflect() protoreflect.Message {
	mi := &file_multiraft_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SessionRequest.ProtoReflect.Descriptor instead.
func (*SessionRequest) Descriptor() ([]byte, []int) {
	return file_multiraft_proto_rawDescGZIP(), []int{1}
}

func (x *SessionRequest) GetShardId() uint64 {
	if x != nil {
		return x.ShardId
	}
	return 0
}

// SessionResponse is the message used to indicate whether the
// Session object is successfully closed.
type SessionResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Completed bool `protobuf:"varint,1,opt,name=completed,proto3" json:"completed,omitempty"`
}

func (x *SessionResponse) Reset() {
	*x = SessionResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_multiraft_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *SessionResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*SessionResponse) ProtoMessage() {}

func (x *SessionResponse) ProtoReflect() protoreflect.Message {
	mi := &file_multiraft_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use SessionResponse.ProtoReflect.Descriptor instead.
func (*SessionResponse) Descriptor() ([]byte, []int) {
	return file_multiraft_proto_rawDescGZIP(), []int{2}
}

func (x *SessionResponse) GetCompleted() bool {
	if x != nil {
		return x.Completed
	}
	return false
}

// RaftProposal is the message used to describe the proposal to be made on the
// selected raft cluster.
type RaftProposal struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Session *Session `protobuf:"bytes,1,opt,name=session,proto3" json:"session,omitempty"`
	Data    []byte   `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *RaftProposal) Reset() {
	*x = RaftProposal{}
	if protoimpl.UnsafeEnabled {
		mi := &file_multiraft_proto_msgTypes[3]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RaftProposal) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RaftProposal) ProtoMessage() {}

func (x *RaftProposal) ProtoReflect() protoreflect.Message {
	mi := &file_multiraft_proto_msgTypes[3]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RaftProposal.ProtoReflect.Descriptor instead.
func (*RaftProposal) Descriptor() ([]byte, []int) {
	return file_multiraft_proto_rawDescGZIP(), []int{3}
}

func (x *RaftProposal) GetSession() *Session {
	if x != nil {
		return x.Session
	}
	return nil
}

func (x *RaftProposal) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

// RaftReadIndex is the message used to describe the input to the ReadIndex
// protocol. The ReadIndex protocol is used for making linearizable read on
// the selected raft cluster.
type RaftReadIndex struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ShardId uint64 `protobuf:"varint,1,opt,name=shard_id,json=shardId,proto3" json:"shard_id,omitempty"`
	Data    []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *RaftReadIndex) Reset() {
	*x = RaftReadIndex{}
	if protoimpl.UnsafeEnabled {
		mi := &file_multiraft_proto_msgTypes[4]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RaftReadIndex) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RaftReadIndex) ProtoMessage() {}

func (x *RaftReadIndex) ProtoReflect() protoreflect.Message {
	mi := &file_multiraft_proto_msgTypes[4]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RaftReadIndex.ProtoReflect.Descriptor instead.
func (*RaftReadIndex) Descriptor() ([]byte, []int) {
	return file_multiraft_proto_rawDescGZIP(), []int{4}
}

func (x *RaftReadIndex) GetShardId() uint64 {
	if x != nil {
		return x.ShardId
	}
	return 0
}

func (x *RaftReadIndex) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

// RaftResponse is the message used to describe the response produced by
// the Update or Lookup function of the IDataStore instance.
type RaftResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Result uint64 `protobuf:"varint,1,opt,name=result,proto3" json:"result,omitempty"`
	Data   []byte `protobuf:"bytes,2,opt,name=data,proto3" json:"data,omitempty"`
}

func (x *RaftResponse) Reset() {
	*x = RaftResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_multiraft_proto_msgTypes[5]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *RaftResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*RaftResponse) ProtoMessage() {}

func (x *RaftResponse) ProtoReflect() protoreflect.Message {
	mi := &file_multiraft_proto_msgTypes[5]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use RaftResponse.ProtoReflect.Descriptor instead.
func (*RaftResponse) Descriptor() ([]byte, []int) {
	return file_multiraft_proto_rawDescGZIP(), []int{5}
}

func (x *RaftResponse) GetResult() uint64 {
	if x != nil {
		return x.Result
	}
	return 0
}

func (x *RaftResponse) GetData() []byte {
	if x != nil {
		return x.Data
	}
	return nil
}

var File_multiraft_proto protoreflect.FileDescriptor

var file_multiraft_proto_rawDesc = []byte{
	0x0a, 0x0f, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x2e, 0x70, 0x72, 0x6f, 0x74,
	0x6f, 0x12, 0x0b, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x22, 0x7d,
	0x0a, 0x07, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x18, 0x0a, 0x07, 0x53, 0x68, 0x61,
	0x72, 0x64, 0x49, 0x44, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52, 0x07, 0x53, 0x68, 0x61, 0x72,
	0x64, 0x49, 0x44, 0x12, 0x1a, 0x0a, 0x08, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x49, 0x44, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x04, 0x52, 0x08, 0x43, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x49, 0x44, 0x12,
	0x1a, 0x0a, 0x08, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x49, 0x44, 0x18, 0x03, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x08, 0x53, 0x65, 0x72, 0x69, 0x65, 0x73, 0x49, 0x44, 0x12, 0x20, 0x0a, 0x0b, 0x52,
	0x65, 0x73, 0x70, 0x6f, 0x6e, 0x64, 0x65, 0x64, 0x54, 0x6f, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04,
	0x52, 0x0b, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x64, 0x65, 0x64, 0x54, 0x6f, 0x22, 0x2b, 0x0a,
	0x0e, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12,
	0x19, 0x0a, 0x08, 0x73, 0x68, 0x61, 0x72, 0x64, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x04, 0x52, 0x07, 0x73, 0x68, 0x61, 0x72, 0x64, 0x49, 0x64, 0x22, 0x2f, 0x0a, 0x0f, 0x53, 0x65,
	0x73, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1c, 0x0a,
	0x09, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x08,
	0x52, 0x09, 0x63, 0x6f, 0x6d, 0x70, 0x6c, 0x65, 0x74, 0x65, 0x64, 0x22, 0x52, 0x0a, 0x0c, 0x52,
	0x61, 0x66, 0x74, 0x50, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x61, 0x6c, 0x12, 0x2e, 0x0a, 0x07, 0x73,
	0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x0b, 0x32, 0x14, 0x2e, 0x6d,
	0x75, 0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2e, 0x53, 0x65, 0x73, 0x73, 0x69,
	0x6f, 0x6e, 0x52, 0x07, 0x73, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x12, 0x0a, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22,
	0x3e, 0x0a, 0x0d, 0x52, 0x61, 0x66, 0x74, 0x52, 0x65, 0x61, 0x64, 0x49, 0x6e, 0x64, 0x65, 0x78,
	0x12, 0x19, 0x0a, 0x08, 0x73, 0x68, 0x61, 0x72, 0x64, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01,
	0x28, 0x04, 0x52, 0x07, 0x73, 0x68, 0x61, 0x72, 0x64, 0x49, 0x64, 0x12, 0x12, 0x0a, 0x04, 0x64,
	0x61, 0x74, 0x61, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x22,
	0x3a, 0x0a, 0x0c, 0x52, 0x61, 0x66, 0x74, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12,
	0x16, 0x0a, 0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x18, 0x01, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x06, 0x72, 0x65, 0x73, 0x75, 0x6c, 0x74, 0x12, 0x12, 0x0a, 0x04, 0x64, 0x61, 0x74, 0x61, 0x18,
	0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x04, 0x64, 0x61, 0x74, 0x61, 0x32, 0x9a, 0x02, 0x0a, 0x0b,
	0x4e, 0x6f, 0x64, 0x65, 0x68, 0x6f, 0x73, 0x74, 0x41, 0x50, 0x49, 0x12, 0x41, 0x0a, 0x0a, 0x47,
	0x65, 0x74, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x1b, 0x2e, 0x6d, 0x75, 0x6c, 0x74,
	0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2e, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x14, 0x2e, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x72, 0x61,
	0x66, 0x74, 0x70, 0x62, 0x2e, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x22, 0x00, 0x12, 0x44,
	0x0a, 0x0c, 0x43, 0x6c, 0x6f, 0x73, 0x65, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x14,
	0x2e, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2e, 0x53, 0x65, 0x73,
	0x73, 0x69, 0x6f, 0x6e, 0x1a, 0x1c, 0x2e, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74,
	0x70, 0x62, 0x2e, 0x53, 0x65, 0x73, 0x73, 0x69, 0x6f, 0x6e, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e,
	0x73, 0x65, 0x22, 0x00, 0x12, 0x41, 0x0a, 0x07, 0x50, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x65, 0x12,
	0x19, 0x2e, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2e, 0x52, 0x61,
	0x66, 0x74, 0x50, 0x72, 0x6f, 0x70, 0x6f, 0x73, 0x61, 0x6c, 0x1a, 0x19, 0x2e, 0x6d, 0x75, 0x6c,
	0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2e, 0x52, 0x61, 0x66, 0x74, 0x52, 0x65, 0x73,
	0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x12, 0x3f, 0x0a, 0x04, 0x52, 0x65, 0x61, 0x64, 0x12,
	0x1a, 0x2e, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2e, 0x52, 0x61,
	0x66, 0x74, 0x52, 0x65, 0x61, 0x64, 0x49, 0x6e, 0x64, 0x65, 0x78, 0x1a, 0x19, 0x2e, 0x6d, 0x75,
	0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x2e, 0x52, 0x61, 0x66, 0x74, 0x52, 0x65,
	0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x22, 0x00, 0x42, 0x24, 0x5a, 0x22, 0x67, 0x69, 0x74, 0x68,
	0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x6c, 0x6e, 0x69, 0x2f, 0x64, 0x72, 0x75, 0x6d, 0x6d,
	0x65, 0x72, 0x2f, 0x6d, 0x75, 0x6c, 0x74, 0x69, 0x72, 0x61, 0x66, 0x74, 0x70, 0x62, 0x62, 0x06,
	0x70, 0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_multiraft_proto_rawDescOnce sync.Once
	file_multiraft_proto_rawDescData = file_multiraft_proto_rawDesc
)

func file_multiraft_proto_rawDescGZIP() []byte {
	file_multiraft_proto_rawDescOnce.Do(func() {
		file_multiraft_proto_rawDescData = protoimpl.X.CompressGZIP(file_multiraft_proto_rawDescData)
	})
	return file_multiraft_proto_rawDescData
}

var file_multiraft_proto_msgTypes = make([]protoimpl.MessageInfo, 6)
var file_multiraft_proto_goTypes = []interface{}{
	(*Session)(nil),         // 0: multiraftpb.Session
	(*SessionRequest)(nil),  // 1: multiraftpb.SessionRequest
	(*SessionResponse)(nil), // 2: multiraftpb.SessionResponse
	(*RaftProposal)(nil),    // 3: multiraftpb.RaftProposal
	(*RaftReadIndex)(nil),   // 4: multiraftpb.RaftReadIndex
	(*RaftResponse)(nil),    // 5: multiraftpb.RaftResponse
}
var file_multiraft_proto_depIdxs = []int32{
	0, // 0: multiraftpb.RaftProposal.session:type_name -> multiraftpb.Session
	1, // 1: multiraftpb.NodehostAPI.GetSession:input_type -> multiraftpb.SessionRequest
	0, // 2: multiraftpb.NodehostAPI.CloseSession:input_type -> multiraftpb.Session
	3, // 3: multiraftpb.NodehostAPI.Propose:input_type -> multiraftpb.RaftProposal
	4, // 4: multiraftpb.NodehostAPI.Read:input_type -> multiraftpb.RaftReadIndex
	0, // 5: multiraftpb.NodehostAPI.GetSession:output_type -> multiraftpb.Session
	2, // 6: multiraftpb.NodehostAPI.CloseSession:output_type -> multiraftpb.SessionResponse
	5, // 7: multiraftpb.NodehostAPI.Propose:output_type -> multiraftpb.RaftResponse
	5, // 8: multiraftpb.NodehostAPI.Read:output_type -> multiraftpb.RaftResponse
	5, // [5:9] is the sub-list for method output_type
	1, // [1:5] is the sub-list for method input_type
	1, // [1:1] is the sub-list for extension type_name
	1, // [1:1] is the sub-list for extension extendee
	0, // [0:1] is the sub-list for field type_name
}

func init() { file_multiraft_proto_init() }
func file_multiraft_proto_init() {
	if File_multiraft_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_multiraft_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*Session); i {
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
		file_multiraft_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SessionRequest); i {
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
		file_multiraft_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*SessionResponse); i {
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
		file_multiraft_proto_msgTypes[3].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RaftProposal); i {
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
		file_multiraft_proto_msgTypes[4].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RaftReadIndex); i {
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
		file_multiraft_proto_msgTypes[5].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*RaftResponse); i {
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
			RawDescriptor: file_multiraft_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   6,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_multiraft_proto_goTypes,
		DependencyIndexes: file_multiraft_proto_depIdxs,
		MessageInfos:      file_multiraft_proto_msgTypes,
	}.Build()
	File_multiraft_proto = out.File
	file_multiraft_proto_rawDesc = nil
	file_multiraft_proto_goTypes = nil
	file_multiraft_proto_depIdxs = nil
}
