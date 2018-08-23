package client

import (
	"sort"
	"testing"
)

type tblPathSlice []tablePath

func (a tblPathSlice) Len() int {
	return len(a)
}

func (a tblPathSlice) Swap(i, j int) {
	a[i], a[j] = a[j], a[i]
}

func (a tblPathSlice) Less(i, j int) bool {
	if a[i].dbName != a[j].dbName {
		return a[i].dbName < a[j].dbName
	}
	if a[i].tableName != a[j].tableName {
		return a[i].tableName < a[j].tableName
	}
	if a[i].tableKey != a[j].tableKey {
		return a[i].tableKey < a[j].tableKey
	}
	if a[i].delimitor != a[j].delimitor {
		return a[i].delimitor < a[j].delimitor
	}
	if a[i].fields != a[j].fields {
		return a[i].fields < a[j].fields
	}
	if a[i].jsonTableKey != a[j].jsonTableKey {
		return a[i].jsonTableKey < a[j].jsonTableKey
	}

	return a[i].jsonFields < a[j].jsonFields
}

func mock_initCountersNameMap() {
	countersNameOidTbls["COUNTERS_PORT_NAME_MAP"] = map[string]string{
		"Ethernet1": "oid:0x1000000000001",
		"Ethernet2": "oid:0x1000000000002",
	}

	countersNameOidTbls["COUNTERS_QUEUE_NAME_MAP"] = map[string]string{
		"Ethernet1:0": "oid:0x15000000000010",
		"Ethernet1:1": "oid:0x15000000000011",
		"Ethernet2:0": "oid:0x15000000000020",
		"Ethernet2:1": "oid:0x15000000000021",
	}

	countersNameOidTbls["COUNTERS_BUFFER_POOL_NAME_MAP"] = map[string]string{
		"BUFFER_POOL_0": "oid:0x18000000000000",
		"BUFFER_POOL_1": "oid:0x18000000000001",
	}

	supportedCounterFields["COUNTERS_PORT_NAME_MAP"] = []string{
		"SAI_PORT_STAT_IF_IN_OCTETS",
		"SAI_PORT_STAT_IF_IN_UCAST_PKTS",
		"SAI_PORT_STAT_IF_OUT_OCTETS",
		"SAI_PORT_STAT_IF_OUT_UCAST_PKTS",
	}

	supportedCounterFields["COUNTERS_QUEUE_NAME_MAP"] = []string{
		"SAI_QUEUE_STAT_PACKETS",
		"SAI_QUEUE_STAT_BYTES",
		"SAI_QUEUE_STAT_DROPPED_PACKETS",
		"SAI_QUEUE_STAT_DROPPED_BYTES",
	}

	supportedCounterFields["COUNTERS_BUFFER_POOL_NAME_MAP"] = []string{
		"SAI_BUFFER_POOL_STAT_CURR_OCCUPANCY_BYTES",
		"SAI_BUFFER_POOL_STAT_WATERMARK_BYTES",
	}
}

func TestGetV2RPath(t *testing.T) {
	mock_initCountersNameMap()

	var tests = []struct {
		desc  string
		input []string
		want  []tablePath
	}{
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet*",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet*"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x1000000000001",
					delimitor:    ":",
					fields:       "",
					jsonTableKey: "Ethernet1",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x1000000000002",
					delimitor:    ":",
					fields:       "",
					jsonTableKey: "Ethernet2",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet*/SAI_PORT_STAT_IF_IN_OCTETS",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet*", "SAI_PORT_STAT_IF_IN_OCTETS"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x1000000000001",
					delimitor:    ":",
					fields:       "SAI_PORT_STAT_IF_IN_OCTETS",
					jsonTableKey: "Ethernet1",
					jsonFields:   "SAI_PORT_STAT_IF_IN_OCTETS",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x1000000000002",
					delimitor:    ":",
					fields:       "SAI_PORT_STAT_IF_IN_OCTETS",
					jsonTableKey: "Ethernet2",
					jsonFields:   "SAI_PORT_STAT_IF_IN_OCTETS",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet*/SAI_PORT_STAT_IF_IN_*",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet*", "SAI_PORT_STAT_IF_IN_*"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x1000000000001",
					delimitor:    ":",
					fields:       "SAI_PORT_STAT_IF_IN_OCTETS,SAI_PORT_STAT_IF_IN_UCAST_PKTS",
					jsonTableKey: "Ethernet1",
					jsonFields:   "SAI_PORT_STAT_IF_IN_OCTETS,SAI_PORT_STAT_IF_IN_UCAST_PKTS",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x1000000000002",
					delimitor:    ":",
					fields:       "SAI_PORT_STAT_IF_IN_OCTETS,SAI_PORT_STAT_IF_IN_UCAST_PKTS",
					jsonTableKey: "Ethernet2",
					jsonFields:   "SAI_PORT_STAT_IF_IN_OCTETS,SAI_PORT_STAT_IF_IN_UCAST_PKTS",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet2",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet2"},
			want: []tablePath{
				{
					dbName:    "COUNTERS_DB",
					tableName: "COUNTERS",
					tableKey:  "oid:0x1000000000002",
					delimitor: ":",
					fields:    "",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet2/SAI_PORT_STAT_IF_IN_OCTETS",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet2", "SAI_PORT_STAT_IF_IN_OCTETS"},
			want: []tablePath{
				{
					dbName:    "COUNTERS_DB",
					tableName: "COUNTERS",
					tableKey:  "oid:0x1000000000002",
					delimitor: ":",
					fields:    "SAI_PORT_STAT_IF_IN_OCTETS",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet2/SAI_PORT_STAT_IF_IN_*",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet2", "SAI_PORT_STAT_IF_IN_*"},
			want: []tablePath{
				{
					dbName:     "COUNTERS_DB",
					tableName:  "COUNTERS",
					tableKey:   "oid:0x1000000000002",
					delimitor:  ":",
					fields:     "SAI_PORT_STAT_IF_IN_OCTETS,SAI_PORT_STAT_IF_IN_UCAST_PKTS",
					jsonFields: "SAI_PORT_STAT_IF_IN_OCTETS,SAI_PORT_STAT_IF_IN_UCAST_PKTS",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet*/Queues",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet*", "Queues"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000010",
					delimitor:    ":",
					fields:       "",
					jsonTableKey: "Ethernet1:0",
					jsonFields:   "",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000011",
					delimitor:    ":",
					fields:       "",
					jsonTableKey: "Ethernet1:1",
					jsonFields:   "",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000020",
					delimitor:    ":",
					fields:       "",
					jsonTableKey: "Ethernet2:0",
					jsonFields:   "",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000021",
					delimitor:    ":",
					fields:       "",
					jsonTableKey: "Ethernet2:1",
					jsonFields:   "",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet*/Queues/SAI_QUEUE_STAT_DROPPED_PACKETS",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet*", "Queues", "SAI_QUEUE_STAT_DROPPED_PACKETS"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000010",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS",
					jsonTableKey: "Ethernet1:0",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000011",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS",
					jsonTableKey: "Ethernet1:1",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000020",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS",
					jsonTableKey: "Ethernet2:0",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000021",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS",
					jsonTableKey: "Ethernet2:1",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet*/Queues/SAI_QUEUE_STAT_DROPPED*",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet*", "Queues", "SAI_QUEUE_STAT_DROPPED*"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000010",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
					jsonTableKey: "Ethernet1:0",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000011",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
					jsonTableKey: "Ethernet1:1",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000020",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
					jsonTableKey: "Ethernet2:0",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000021",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
					jsonTableKey: "Ethernet2:1",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet2/Queues",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet2", "Queues"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000020",
					delimitor:    ":",
					fields:       "",
					jsonTableKey: "Ethernet2:0",
					jsonFields:   "",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000021",
					delimitor:    ":",
					fields:       "",
					jsonTableKey: "Ethernet2:1",
					jsonFields:   "",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet2/Queues/SAI_QUEUE_STAT_DROPPED_PACKETS",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet2", "Queues", "SAI_QUEUE_STAT_DROPPED_PACKETS"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000020",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS",
					jsonTableKey: "Ethernet2:0",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000021",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS",
					jsonTableKey: "Ethernet2:1",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS",
				},
			},
		},
		{
			desc:  "COUNTERS_DB/COUNTERS/Ethernet2/Queues/SAI_QUEUE_STAT_DROPPED*",
			input: []string{"COUNTERS_DB", "COUNTERS", "Ethernet2", "Queues", "SAI_QUEUE_STAT_DROPPED*"},
			want: []tablePath{
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000020",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
					jsonTableKey: "Ethernet2:0",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
				},
				{
					dbName:       "COUNTERS_DB",
					tableName:    "COUNTERS",
					tableKey:     "oid:0x15000000000021",
					delimitor:    ":",
					fields:       "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
					jsonTableKey: "Ethernet2:1",
					jsonFields:   "SAI_QUEUE_STAT_DROPPED_PACKETS,SAI_QUEUE_STAT_DROPPED_BYTES",
				},
			},
		},
	}

	for _, test := range tests {
		//got, err := getv2rPath(test.input)
		t.Run(test.desc, func(t *testing.T) {
			got, err := getv2rPath(test.input)

			if len(got) != len(test.want) {
				t.Errorf("getv2rPath err: %v  input: %q got != want", err, test.input)
				t.Logf("got : %v", got)
				t.Logf("want: %v", test.want)
			} else {
				sort.Sort(tblPathSlice(got))
				sort.Sort(tblPathSlice(test.want))
				for i, g := range got {
					if g != test.want[i] {
						t.Errorf("getv2rPath err: %v  input: (%q) element[%v] isn't wanted %v", err, test.input, i, g)
						t.Logf("got  = %v", got)
						t.Logf("want = %v", test.want)
					}
				}
			}
		})
	}

}
