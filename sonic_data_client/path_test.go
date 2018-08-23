package client

import (
	//"flag"
	"github.com/go-redis/redis"
	"testing"
)

func TestGetDbPath(t *testing.T) {
	//flag.Set("alsologtostderr", "true")
	//flag.Set("v", "6")
	//flag.Parse()
	UseRedisLocalTcpPort = true
	useRedisTcpClient()
	rclient := redis.NewClient(&redis.Options{
		Network:     "tcp",
		Addr:        "localhost:6379",
		Password:    "", // no password set
		DB:          4,
		DialTimeout: 0,
	})
	_, err := rclient.Ping().Result()
	if err != nil {
		t.Fatalf("failed to connect to redis server %v", err)
	}
	defer rclient.Close()

	var tests = []struct {
		desc     string
		input    []string
		wantErr  string
		wantPath []tablePath
	}{
		{
			desc:     "Invalid DB",
			input:    []string{"TEST_DB", "COUNTERS"},
			wantErr:  "invaild db target: TEST_DB",
			wantPath: nil,
		},
		{
			desc:     "Others target",
			input:    []string{"OTHERS", "proc", "meminfo"},
			wantErr:  "invaild db target: OTHERS",
			wantPath: nil,
		},
		{
			desc:     "CONFIG_DB without table",
			input:    []string{"CONFIG_DB"},
			wantErr:  "not support",
			wantPath: nil,
		},
		{
			desc:     "CONFIG_DB invalid table",
			input:    []string{"CONFIG_DB", "COUNTERS"},
			wantErr:  "failed to find CONFIG_DB [CONFIG_DB COUNTERS] <nil> []",
			wantPath: nil,
		},
		{
			desc:    "CONFIG_DB PORT",
			input:   []string{"CONFIG_DB", "PORT"},
			wantErr: "",
			wantPath: []tablePath{
				tablePath{
					dbName:    "CONFIG_DB",
					tableName: "PORT",
					delimitor: "|",
				},
			},
		},
		{
			desc:     "CONFIG_DB PORT invalid key",
			input:    []string{"CONFIG_DB", "PORT", "Ethernet100"},
			wantErr:  "no valid entry found on [CONFIG_DB PORT Ethernet100] with key PORT|Ethernet100",
			wantPath: nil,
		},
		{
			desc:    "CONFIG_DB PORT Ethernet1",
			input:   []string{"CONFIG_DB", "PORT", "Ethernet1"},
			wantErr: "",
			wantPath: []tablePath{
				tablePath{
					dbName:    "CONFIG_DB",
					tableName: "PORT",
					delimitor: "|",
					tableKey:  "Ethernet1",
				},
			},
		},
		{
			desc:    "CONFIG_DB PORT Ethernet1 alias",
			input:   []string{"CONFIG_DB", "PORT", "Ethernet1", "alias"},
			wantErr: "",
			wantPath: []tablePath{
				tablePath{
					dbName:    "CONFIG_DB",
					tableName: "PORT",
					delimitor: "|",
					tableKey:  "Ethernet1",
					fields:    "alias",
				},
			},
		},
		{
			desc:    "CONFIG_DB VLAN_MEMBER Vlan12 PortChannel2",
			input:   []string{"CONFIG_DB", "VLAN_MEMBER", "Vlan12", "PortChannel2"},
			wantErr: "",
			wantPath: []tablePath{
				tablePath{
					dbName:    "CONFIG_DB",
					tableName: "VLAN_MEMBER",
					delimitor: "|",
					tableKey:  "Vlan12|PortChannel2",
				},
			},
		},
		{
			desc:    "CONFIG_DB VLAN_MEMBER Vlan12 PortChannel2 tagging_mode",
			input:   []string{"CONFIG_DB", "VLAN_MEMBER", "Vlan12", "PortChannel2", "tagging_mode"},
			wantErr: "",
			wantPath: []tablePath{
				tablePath{
					dbName:    "CONFIG_DB",
					tableName: "VLAN_MEMBER",
					delimitor: "|",
					tableKey:  "Vlan12|PortChannel2",
					fields:    "tagging_mode",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			rclient.FlushDb()
			rclient.HSet("PORT|Ethernet1", "alias", "1")
			rclient.HSet("PORT|Ethernet1", "fec", "rs")
			rclient.HSet("PORT|Ethernet2", "alias", "2")
			rclient.HSet("PORT|Ethernet2", "fec", "rs")
			rclient.HSet("VLAN_MEMBER|Vlan12|PortChannel2", "tagging_mode", "tagged")

			gsPath := &GSPath{gpath: test.input}
			err := gsPath.GetDbPath(false)
			tp := gsPath.tpath

			if err != nil && test.wantErr != "" {
				if err.Error() != test.wantErr {
					t.Errorf("GetDbPath err (%v) != wantErr (%v)", err, test.wantErr)
				}
			} else {
				if !(err == nil && test.wantErr == "") {
					t.Errorf("GetDbPath err (%v) != wantErr (%v)", err, test.wantErr)
				}
			}
			// This testcase not include multiple tablePath like counterPath
			if (len(tp) > 1) || (len(test.wantPath) > 1) {
				t.Errorf("GetDbPath tablePath length must be smaller than one")
				t.Logf("got  : %v", tp)
				t.Logf("want : %v", test.wantPath)
			} else {
				if tp != nil && test.wantPath != nil {
					if tp[0] != test.wantPath[0] {
						t.Errorf("GetDbPath got tablePath != wantPath")
						t.Logf("got  : %v", tp)
						t.Logf("want : %v", test.wantPath)
					}
				} else {
					if !(tp == nil && test.wantPath == nil) {
						t.Errorf("GetDbPath got tablePath != wantPath")
						t.Logf("got  : %v", tp)
						t.Logf("want : %v", test.wantPath)
					}
				}
			}
		})
	}
}

func TestGetCfgPath(t *testing.T) {
	//flag.Set("alsologtostderr", "true")
	//flag.Set("v", "6")
	//flag.Parse()
	UseRedisLocalTcpPort = true
	useRedisTcpClient()
	rclient := redis.NewClient(&redis.Options{
		Network:     "tcp",
		Addr:        "localhost:6379",
		Password:    "", // no password set
		DB:          4,
		DialTimeout: 0,
	})
	_, err := rclient.Ping().Result()
	if err != nil {
		t.Fatalf("failed to connect to redis server %v", err)
	}
	defer rclient.Close()

	var tests = []struct {
		desc     string
		input    []string
		wantErr  string
		wantPath []tablePath
	}{
		{
			desc:     "Invalid DB",
			input:    []string{"TEST_DB", "COUNTERS"},
			wantErr:  "invaild db target: TEST_DB",
			wantPath: nil,
		},
		{
			desc:     "COUNTERS_DB not supported",
			input:    []string{"COUNTERS_DB", "COUNTERS"},
			wantErr:  "config COUNTERS_DB not supported",
			wantPath: nil,
		},
		{
			desc:     "CONFIG_DB PORT Ethernet1 not supported",
			input:    []string{"CONFIG_DB", "PORT", "Ethernet1"},
			wantErr:  "config [CONFIG_DB PORT Ethernet1] not supported",
			wantPath: nil,
		},
		{
			desc:    "CONFIG_DB TELEMETRY_CLIENT Global",
			input:   []string{"CONFIG_DB", "TELEMETRY_CLIENT", "Global"},
			wantErr: "",
			wantPath: []tablePath{
				tablePath{
					dbName:    "CONFIG_DB",
					tableName: "TELEMETRY_CLIENT",
					tableKey:  "Global",
					delimitor: "|",
				},
			},
		},
		{
			//allow set invalid key for xnet subscribe first time
			desc:    "CONFIG_DB TELEMETRY_CLIENT invalid key",
			input:   []string{"CONFIG_DB", "TELEMETRY_CLIENT", "port"},
			wantErr: "",
			wantPath: []tablePath{
				tablePath{
					dbName:    "CONFIG_DB",
					tableName: "TELEMETRY_CLIENT",
					tableKey:  "port",
					delimitor: "|",
				},
			},
		},
		{
			desc:    "CONFIG_DB TELEMETRY_CLIENT Global retry_interval",
			input:   []string{"CONFIG_DB", "TELEMETRY_CLIENT", "Global", "retry_interval"},
			wantErr: "",
			wantPath: []tablePath{
				tablePath{
					dbName:    "CONFIG_DB",
					tableName: "TELEMETRY_CLIENT",
					tableKey:  "Global",
					delimitor: "|",
					fields:    "retry_interval",
				},
			},
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			rclient.FlushDb()
			rclient.HSet("TELEMETRY_CLIENT|Global", "retry_interval", "30")

			gsPath := &GSPath{gpath: test.input}
			err := gsPath.GetCfgPath()
			tp := gsPath.tpath

			if err != nil && test.wantErr != "" {
				if err.Error() != test.wantErr {
					t.Errorf("GetDbPath err (%v) != wantErr (%v)", err, test.wantErr)
				}
			} else {
				if !(err == nil && test.wantErr == "") {
					t.Errorf("GetDbPath err (%v) != wantErr (%v)", err, test.wantErr)
				}
			}
			// This testcase not include multiple tablePath like counterPath
			if (len(tp) > 1) || (len(test.wantPath) > 1) {
				t.Errorf("GetDbPath tablePath length must be smaller than one")
				t.Logf("got  : %v", tp)
				t.Logf("want : %v", test.wantPath)
			} else {
				if tp != nil && test.wantPath != nil {
					if tp[0] != test.wantPath[0] {
						t.Errorf("GetDbPath got tablePath != wantPath")
						t.Logf("got  : %v", tp)
						t.Logf("want : %v", test.wantPath)
					}
				} else {
					if !(tp == nil && test.wantPath == nil) {
						t.Errorf("GetDbPath got tablePath != wantPath")
						t.Logf("got  : %v", tp)
						t.Logf("want : %v", test.wantPath)
					}
				}
			}
		})
	}

}
