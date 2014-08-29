// Code generated by protoc-gen-gogo.
// source: resource_limits.proto
// DO NOT EDIT!

package warden

import proto "code.google.com/p/gogoprotobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type ResourceLimits struct {
	As               *uint64 `protobuf:"varint,1,opt,name=as" json:"as,omitempty"`
	Core             *uint64 `protobuf:"varint,2,opt,name=core" json:"core,omitempty"`
	Cpu              *uint64 `protobuf:"varint,3,opt,name=cpu" json:"cpu,omitempty"`
	Data             *uint64 `protobuf:"varint,4,opt,name=data" json:"data,omitempty"`
	Fsize            *uint64 `protobuf:"varint,5,opt,name=fsize" json:"fsize,omitempty"`
	Locks            *uint64 `protobuf:"varint,6,opt,name=locks" json:"locks,omitempty"`
	Memlock          *uint64 `protobuf:"varint,7,opt,name=memlock" json:"memlock,omitempty"`
	Msgqueue         *uint64 `protobuf:"varint,8,opt,name=msgqueue" json:"msgqueue,omitempty"`
	Nice             *uint64 `protobuf:"varint,9,opt,name=nice" json:"nice,omitempty"`
	Nofile           *uint64 `protobuf:"varint,10,opt,name=nofile" json:"nofile,omitempty"`
	Nproc            *uint64 `protobuf:"varint,11,opt,name=nproc" json:"nproc,omitempty"`
	Rss              *uint64 `protobuf:"varint,12,opt,name=rss" json:"rss,omitempty"`
	Rtprio           *uint64 `protobuf:"varint,13,opt,name=rtprio" json:"rtprio,omitempty"`
	Sigpending       *uint64 `protobuf:"varint,14,opt,name=sigpending" json:"sigpending,omitempty"`
	Stack            *uint64 `protobuf:"varint,15,opt,name=stack" json:"stack,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *ResourceLimits) Reset()         { *m = ResourceLimits{} }
func (m *ResourceLimits) String() string { return proto.CompactTextString(m) }
func (*ResourceLimits) ProtoMessage()    {}

func (m *ResourceLimits) GetAs() uint64 {
	if m != nil && m.As != nil {
		return *m.As
	}
	return 0
}

func (m *ResourceLimits) GetCore() uint64 {
	if m != nil && m.Core != nil {
		return *m.Core
	}
	return 0
}

func (m *ResourceLimits) GetCpu() uint64 {
	if m != nil && m.Cpu != nil {
		return *m.Cpu
	}
	return 0
}

func (m *ResourceLimits) GetData() uint64 {
	if m != nil && m.Data != nil {
		return *m.Data
	}
	return 0
}

func (m *ResourceLimits) GetFsize() uint64 {
	if m != nil && m.Fsize != nil {
		return *m.Fsize
	}
	return 0
}

func (m *ResourceLimits) GetLocks() uint64 {
	if m != nil && m.Locks != nil {
		return *m.Locks
	}
	return 0
}

func (m *ResourceLimits) GetMemlock() uint64 {
	if m != nil && m.Memlock != nil {
		return *m.Memlock
	}
	return 0
}

func (m *ResourceLimits) GetMsgqueue() uint64 {
	if m != nil && m.Msgqueue != nil {
		return *m.Msgqueue
	}
	return 0
}

func (m *ResourceLimits) GetNice() uint64 {
	if m != nil && m.Nice != nil {
		return *m.Nice
	}
	return 0
}

func (m *ResourceLimits) GetNofile() uint64 {
	if m != nil && m.Nofile != nil {
		return *m.Nofile
	}
	return 0
}

func (m *ResourceLimits) GetNproc() uint64 {
	if m != nil && m.Nproc != nil {
		return *m.Nproc
	}
	return 0
}

func (m *ResourceLimits) GetRss() uint64 {
	if m != nil && m.Rss != nil {
		return *m.Rss
	}
	return 0
}

func (m *ResourceLimits) GetRtprio() uint64 {
	if m != nil && m.Rtprio != nil {
		return *m.Rtprio
	}
	return 0
}

func (m *ResourceLimits) GetSigpending() uint64 {
	if m != nil && m.Sigpending != nil {
		return *m.Sigpending
	}
	return 0
}

func (m *ResourceLimits) GetStack() uint64 {
	if m != nil && m.Stack != nil {
		return *m.Stack
	}
	return 0
}

func init() {
}
