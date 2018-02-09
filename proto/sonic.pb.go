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

func init() {
	proto.RegisterEnum("gnmi.sonic.Target", Target_name, Target_value)
}

func init() { proto.RegisterFile("sonic.proto", fileDescriptor2) }

var fileDescriptor2 = []byte{
	// 205 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x2c, 0xce, 0x4f, 0x4e, 0x84, 0x30,
	0x14, 0xc7, 0x71, 0x19, 0xb5, 0xea, 0x43, 0x33, 0xa4, 0xee, 0xe6, 0x08, 0x2e, 0xc0, 0xc4, 0x13,
	0x8c, 0xa5, 0x20, 0x49, 0x43, 0x09, 0xad, 0x7f, 0x76, 0x44, 0x10, 0x6b, 0x17, 0xb4, 0x04, 0xf1,
	0x28, 0xde, 0xd7, 0xbc, 0xe2, 0xae, 0x9f, 0x6f, 0x7e, 0x6d, 0x0a, 0xf1, 0xb7, 0x77, 0x76, 0x48,
	0xe7, 0xc5, 0xaf, 0x9e, 0x82, 0x71, 0x93, 0x4d, 0x43, 0x39, 0xdc, 0x1b, 0xbb, 0x7e, 0xfd, 0xf4,
	0xe9, 0xe0, 0xa7, 0xcc, 0xcf, 0xa3, 0x1b, 0xbc, 0xfb, 0xb4, 0x26, 0xc3, 0x45, 0x16, 0xd6, 0xdb,
	0x31, 0xdc, 0x08, 0xbe, 0xfb, 0x8d, 0x80, 0xe8, 0xf7, 0xc5, 0x8c, 0x2b, 0x8d, 0xe1, 0xe2, 0xd8,
	0x34, 0xa2, 0xcb, 0x1f, 0x93, 0x93, 0x00, 0x55, 0x31, 0x44, 0x44, 0xf7, 0x10, 0x33, 0xf9, 0x5c,
	0x6b, 0xde, 0x2a, 0x0c, 0x3b, 0x0c, 0x42, 0x96, 0x82, 0xbf, 0xf0, 0x30, 0x3f, 0xa5, 0x37, 0x70,
	0xc5, 0x64, 0x5d, 0x54, 0x25, 0xf2, 0x0c, 0xd9, 0x14, 0xac, 0x7b, 0xcd, 0x91, 0xe7, 0xf4, 0x16,
	0xf6, 0x85, 0xe0, 0x6f, 0xdd, 0xff, 0x23, 0x5b, 0xbc, 0x86, 0x4b, 0xa5, 0x8f, 0x9a, 0xa3, 0x08,
	0x05, 0x20, 0x52, 0x3f, 0xf1, 0x56, 0x25, 0x1f, 0x87, 0x5d, 0x12, 0xf5, 0x24, 0x7c, 0xef, 0xe1,
	0x2f, 0x00, 0x00, 0xff, 0xff, 0xca, 0x4e, 0xf3, 0x7a, 0xeb, 0x00, 0x00, 0x00,
}
