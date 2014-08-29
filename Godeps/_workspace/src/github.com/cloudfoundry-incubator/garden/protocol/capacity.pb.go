// Code generated by protoc-gen-gogo.
// source: capacity.proto
// DO NOT EDIT!

package warden

import proto "code.google.com/p/gogoprotobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type CapacityRequest struct {
	XXX_unrecognized []byte `json:"-"`
}

func (m *CapacityRequest) Reset()         { *m = CapacityRequest{} }
func (m *CapacityRequest) String() string { return proto.CompactTextString(m) }
func (*CapacityRequest) ProtoMessage()    {}

type CapacityResponse struct {
	MemoryInBytes    *uint64 `protobuf:"varint,1,req,name=memory_in_bytes" json:"memory_in_bytes,omitempty"`
	DiskInBytes      *uint64 `protobuf:"varint,2,req,name=disk_in_bytes" json:"disk_in_bytes,omitempty"`
	MaxContainers    *uint64 `protobuf:"varint,3,req,name=max_containers" json:"max_containers,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *CapacityResponse) Reset()         { *m = CapacityResponse{} }
func (m *CapacityResponse) String() string { return proto.CompactTextString(m) }
func (*CapacityResponse) ProtoMessage()    {}

func (m *CapacityResponse) GetMemoryInBytes() uint64 {
	if m != nil && m.MemoryInBytes != nil {
		return *m.MemoryInBytes
	}
	return 0
}

func (m *CapacityResponse) GetDiskInBytes() uint64 {
	if m != nil && m.DiskInBytes != nil {
		return *m.DiskInBytes
	}
	return 0
}

func (m *CapacityResponse) GetMaxContainers() uint64 {
	if m != nil && m.MaxContainers != nil {
		return *m.MaxContainers
	}
	return 0
}

func init() {
}
