// Code generated by protoc-gen-go. DO NOT EDIT.
// source: sonic.proto

package gnmi_sonic

import proto "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"
import _ "github.com/openconfig/gnmi/proto/gnmi"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// target - the name of the target for which the path is a member. Only set in prefix for a path.
type Target int32

const (
	Target_APPL_DB     Target = 0
	Target_ASIC_DB     Target = 1
	Target_COUNTERS_DB Target = 2
	Target_LOGLEVEL_DB Target = 3
	Target_CONFIG_DB   Target = 4
	// PFC_WD_DB shares the the same db number with FLEX_COUNTER_DB
	Target_PFC_WD_DB       Target = 5
	Target_FLEX_COUNTER_DB Target = 5
	Target_STATE_DB        Target = 6
	// For none-DB data
	Target_OTHERS Target = 100
)

var Target_name = map[int32]string{
	0: "APPL_DB",
	1: "ASIC_DB",
	2: "COUNTERS_DB",
	3: "LOGLEVEL_DB",
	4: "CONFIG_DB",
	5: "PFC_WD_DB",
	// Duplicate value: 5: "FLEX_COUNTER_DB",
	6:   "STATE_DB",
	100: "OTHERS",
}
var Target_value = map[string]int32{
	"APPL_DB":         0,
	"ASIC_DB":         1,
	"COUNTERS_DB":     2,
	"LOGLEVEL_DB":     3,
	"CONFIG_DB":       4,
	"PFC_WD_DB":       5,
	"FLEX_COUNTER_DB": 5,
	"STATE_DB":        6,
	"OTHERS":          100,
}

func (x Target) String() string {
	return proto.EnumName(Target_name, int32(x))
}
func (Target) EnumDescriptor() ([]byte, []int) { return fileDescriptor2, []int{0} }

type SupportedBundleVersions struct {
	BundleVersion string `protobuf:"bytes,1,opt,name=bundle_version,json=bundleVersion" json:"bundle_version,omitempty"`
	BaseVersion   string `protobuf:"bytes,2,opt,name=base_version,json=baseVersion" json:"base_version,omitempty"`
}

func (m *SupportedBundleVersions) Reset()                    { *m = SupportedBundleVersions{} }
func (m *SupportedBundleVersions) String() string            { return proto.CompactTextString(m) }
func (*SupportedBundleVersions) ProtoMessage()               {}
func (*SupportedBundleVersions) Descriptor() ([]byte, []int) { return fileDescriptor2, []int{0} }

func (m *SupportedBundleVersions) GetBundleVersion() string {
	if m != nil {
		return m.BundleVersion
	}
	return ""
}

func (m *SupportedBundleVersions) GetBaseVersion() string {
	if m != nil {
		return m.BaseVersion
	}
	return ""
}

type BundleVersion struct {
	Version string `protobuf:"bytes,1,opt,name=version" json:"version,omitempty"`
}

func (m *BundleVersion) Reset()                    { *m = BundleVersion{} }
func (m *BundleVersion) String() string            { return proto.CompactTextString(m) }
func (*BundleVersion) ProtoMessage()               {}
func (*BundleVersion) Descriptor() ([]byte, []int) { return fileDescriptor2, []int{1} }

func (m *BundleVersion) GetVersion() string {
	if m != nil {
		return m.Version
	}
	return ""
}

func init() {
	proto.RegisterType((*SupportedBundleVersions)(nil), "gnmi.sonic.SupportedBundleVersions")
	proto.RegisterType((*BundleVersion)(nil), "gnmi.sonic.BundleVersion")
	proto.RegisterEnum("gnmi.sonic.Target", Target_name, Target_value)
}

func init() { proto.RegisterFile("sonic.proto", fileDescriptor2) }

var fileDescriptor2 = []byte{
	// 279 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x54, 0x90, 0xdf, 0x4e, 0xb3, 0x30,
	0x18, 0x87, 0x3f, 0xf8, 0x94, 0xb9, 0x97, 0xe1, 0x48, 0x3d, 0x70, 0xd9, 0x91, 0x2e, 0x31, 0x51,
	0x0f, 0xc0, 0xc4, 0x2b, 0xd8, 0x58, 0x99, 0x4b, 0xc8, 0x20, 0x80, 0xd3, 0x33, 0xb2, 0x42, 0xc5,
	0x26, 0xae, 0x25, 0xfc, 0xf1, 0x4e, 0xbc, 0x5f, 0xd3, 0x32, 0xa3, 0x3b, 0xeb, 0xf3, 0xe4, 0xe9,
	0xef, 0xe0, 0x05, 0xb3, 0x11, 0x9c, 0xe5, 0x4e, 0x55, 0x8b, 0x56, 0x20, 0x28, 0xf9, 0x9e, 0x39,
	0xca, 0x4c, 0x1f, 0x4a, 0xd6, 0xbe, 0x77, 0xc4, 0xc9, 0xc5, 0xde, 0x15, 0x15, 0xe5, 0xb9, 0xe0,
	0x6f, 0xac, 0x74, 0x65, 0xe1, 0xaa, 0xba, 0x7f, 0xaa, 0x1f, 0x8a, 0x67, 0x39, 0x5c, 0x26, 0x5d,
	0x55, 0x89, 0xba, 0xa5, 0xc5, 0xa2, 0xe3, 0xc5, 0x07, 0xdd, 0xd2, 0xba, 0x61, 0x82, 0x37, 0xe8,
	0x06, 0xce, 0x89, 0x32, 0xd9, 0x67, 0xaf, 0x26, 0xda, 0x95, 0x76, 0x3b, 0x8c, 0x2d, 0xf2, 0xb7,
	0x43, 0xd7, 0x30, 0x22, 0xbb, 0xe6, 0x37, 0xd2, 0x55, 0x64, 0x4a, 0x77, 0x48, 0x66, 0x77, 0x60,
	0x1d, 0x6d, 0xa3, 0x09, 0x0c, 0x8e, 0x37, 0x7f, 0xf0, 0xfe, 0x4b, 0x03, 0x23, 0xdd, 0xd5, 0x25,
	0x6d, 0x91, 0x09, 0x83, 0x79, 0x14, 0x05, 0xd9, 0x72, 0x61, 0xff, 0x53, 0x90, 0xac, 0x3d, 0x09,
	0x1a, 0x1a, 0x83, 0xe9, 0x85, 0xcf, 0x9b, 0x14, 0xc7, 0x89, 0x14, 0xba, 0x14, 0x41, 0xb8, 0x0a,
	0xf0, 0x16, 0xab, 0xfc, 0x3f, 0xb2, 0x60, 0xe8, 0x85, 0x1b, 0x7f, 0xbd, 0x92, 0x78, 0x22, 0x31,
	0xf2, 0xbd, 0xec, 0x65, 0x29, 0xf1, 0x14, 0x5d, 0xc0, 0xd8, 0x0f, 0xf0, 0x6b, 0x76, 0x18, 0xe9,
	0xe5, 0x08, 0xce, 0x92, 0x74, 0x9e, 0x62, 0x49, 0x06, 0x02, 0x30, 0xc2, 0xf4, 0x09, 0xc7, 0x89,
	0x5d, 0x4c, 0x75, 0x5b, 0x23, 0x86, 0x3a, 0xd7, 0xe3, 0x77, 0x00, 0x00, 0x00, 0xff, 0xff, 0xf8,
	0xb8, 0x5a, 0xaf, 0x7b, 0x01, 0x00, 0x00,
}
