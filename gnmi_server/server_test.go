package gnmi

// server_test covers gNMI get, subscribe (stream and poll) test
// Prerequisite: redis-server should be running.

import (
	"crypto/tls"
	"encoding/json"
	"fmt"

	testcert "github.com/Azure/sonic-telemetry/testdata/tls"
	"github.com/go-redis/redis"
	"github.com/golang/protobuf/proto"

	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/kylelemons/godebug/pretty"
	"github.com/openconfig/gnmi/client"
	pb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/openconfig/gnmi/value"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"

	// Register supported client types.
	sdc "github.com/Azure/sonic-telemetry/sonic_data_client"
	sdcfg "github.com/Azure/sonic-telemetry/sonic_db_config"
	gclient "github.com/jipanyang/gnmi/client/gnmi"
)

var clientTypes = []string{gclient.Type}

func loadConfig(t *testing.T, key string, in []byte) map[string]interface{} {
	var fvp map[string]interface{}

	err := json.Unmarshal(in, &fvp)
	if err != nil {
		t.Errorf("Failed to Unmarshal %v err: %v", in, err)
	}
	if key != "" {
		kv := map[string]interface{}{}
		kv[key] = fvp
		return kv
	}
	return fvp
}

// assuming input data is in key field/value pair format
func loadDB(t *testing.T, rclient *redis.Client, mpi map[string]interface{}) {
	for key, fv := range mpi {
		switch fv.(type) {
		case map[string]interface{}:
			_, err := rclient.HMSet(key, fv.(map[string]interface{})).Result()
			if err != nil {
				t.Errorf("Invalid data for db:  %v : %v %v", key, fv, err)
			}
		default:
			t.Errorf("Invalid data for db: %v : %v", key, fv)
		}
	}
}
func loadDBNotStrict(t *testing.T, rclient *redis.Client, mpi map[string]interface{}) {
	for key, fv := range mpi {
		switch fv.(type) {
		case map[string]interface{}:
			rclient.HMSet(key, fv.(map[string]interface{})).Result()

		}
	}
}

func createServer(t *testing.T) *Server {
	certificate, err := testcert.NewCert()
	if err != nil {
		t.Errorf("could not load server key pair: %s", err)
	}
	tlsCfg := &tls.Config{
		ClientAuth:   tls.RequestClientCert,
		Certificates: []tls.Certificate{certificate},
	}

	opts := []grpc.ServerOption{grpc.Creds(credentials.NewTLS(tlsCfg))}
	cfg := &Config{Port: 8081}
	s, err := NewServer(cfg, opts)
	if err != nil {
		t.Errorf("Failed to create gNMI server: %v", err)
	}
	return s
}

// runTestGet requests a path from the server by Get grpc call, and compares if
// the return code and response value are expected.
func runTestGet(t *testing.T, ctx context.Context, gClient pb.GNMIClient, pathTarget string,
	textPbPath string, wantRetCode codes.Code, wantRespVal interface{}, valTest bool) {
	//var retCodeOk bool
	// Send request

	var pbPath pb.Path
	if err := proto.UnmarshalText(textPbPath, &pbPath); err != nil {
		t.Fatalf("error in unmarshaling path: %v %v", textPbPath, err)
	}
	prefix := pb.Path{Target: pathTarget}
	req := &pb.GetRequest{
		Prefix:   &prefix,
		Path:     []*pb.Path{&pbPath},
		Encoding: pb.Encoding_JSON_IETF,
	}

	resp, err := gClient.Get(ctx, req)
	// Check return code
	gotRetStatus, ok := status.FromError(err)
	if !ok {
		t.Fatal("got a non-grpc error from grpc call")
	}

	if gotRetStatus.Code() != wantRetCode {
		t.Log("err: ", err)
		t.Fatalf("got return code %v, want %v", gotRetStatus.Code(), wantRetCode)
	}

	// Check response value
	if valTest {
		var gotVal interface{}
		if resp != nil {
			notifs := resp.GetNotification()
			if len(notifs) != 1 {
				t.Fatalf("got %d notifications, want 1", len(notifs))
			}
			updates := notifs[0].GetUpdate()
			if len(updates) != 1 {
				t.Fatalf("got %d updates in the notification, want 1", len(updates))
			}
			val := updates[0].GetVal()
			if val.GetJsonIetfVal() == nil {
				gotVal, err = value.ToScalar(val)
				if err != nil {
					t.Errorf("got: %v, want a scalar value", gotVal)
				}
			} else {
				// Unmarshal json data to gotVal container for comparison
				if err := json.Unmarshal(val.GetJsonIetfVal(), &gotVal); err != nil {
					t.Fatalf("error in unmarshaling IETF JSON data to json container: %v", err)
				}
				var wantJSONStruct interface{}
				if err := json.Unmarshal(wantRespVal.([]byte), &wantJSONStruct); err != nil {
					t.Fatalf("error in unmarshaling IETF JSON data to json container: %v", err)
				}
				wantRespVal = wantJSONStruct
			}
		}

		if !reflect.DeepEqual(gotVal, wantRespVal) {
			t.Errorf("got: %v (%T),\nwant %v (%T)", gotVal, gotVal, wantRespVal, wantRespVal)
		}
	}
}

func extractJSON(val string) []byte {
	jsonBytes, err := ioutil.ReadFile(val)
	if err == nil {
		return jsonBytes
	}
	return []byte(val)
}

type op_t int

const (
	Delete  op_t = 1
	Replace op_t = 2
)

func runTestSet(t *testing.T, ctx context.Context, gClient pb.GNMIClient, pathTarget string,
	textPbPath string, wantRetCode codes.Code, wantRespVal interface{}, attributeData string, op op_t) {
	// Send request
	var pbPath pb.Path
	if err := proto.UnmarshalText(textPbPath, &pbPath); err != nil {
		t.Fatalf("error in unmarshaling path: %v %v", textPbPath, err)
	}
	req := &pb.SetRequest{}
	switch op {
	case Replace:
		//prefix := pb.Path{Target: pathTarget}
		var v *pb.TypedValue
		v = &pb.TypedValue{
			Value: &pb.TypedValue_JsonIetfVal{JsonIetfVal: extractJSON(attributeData)}}

		req = &pb.SetRequest{
			Replace: []*pb.Update{&pb.Update{Path: &pbPath, Val: v}},
		}
	case Delete:
		req = &pb.SetRequest{
			Delete: []*pb.Path{&pbPath},
		}
	}
	_, err := gClient.Set(ctx, req)
	gotRetStatus, ok := status.FromError(err)
	if !ok {
		t.Fatal("got a non-grpc error from grpc call")
	}
	if gotRetStatus.Code() != wantRetCode {
		t.Log("err: ", err)
		t.Fatalf("got return code %v, want %v", gotRetStatus.Code(), wantRetCode)
	} else {
	}
}

func runServer(t *testing.T, s *Server) {
	//t.Log("Starting RPC server on address:", s.Address())
	err := s.Serve() // blocks until close
	if err != nil {
		t.Fatalf("gRPC server err: %v", err)
	}
	//t.Log("Exiting RPC server on address", s.Address())
}

func getRedisClientN(t *testing.T, n int) *redis.Client {
	rclient := redis.NewClient(&redis.Options{
		Network:     "tcp",
		Addr:        sdcfg.GetDbTcpAddr("COUNTERS_DB"),
		Password:    "", // no password set
		DB:          n,
		DialTimeout: 0,
	})
	_, err := rclient.Ping().Result()
	if err != nil {
		t.Fatalf("failed to connect to redis server %v", err)
	}
	return rclient
}

func getRedisClient(t *testing.T) *redis.Client {
	rclient := redis.NewClient(&redis.Options{
		Network:     "tcp",
		Addr:        sdcfg.GetDbTcpAddr("COUNTERS_DB"),
		Password:    "", // no password set
		DB:          sdcfg.GetDbId("COUNTERS_DB"),
		DialTimeout: 0,
	})
	_, err := rclient.Ping().Result()
	if err != nil {
		t.Fatalf("failed to connect to redis server %v", err)
	}
	return rclient
}

func getConfigDbClient(t *testing.T) *redis.Client {
	rclient := redis.NewClient(&redis.Options{
		Network:     "tcp",
		Addr:        sdcfg.GetDbTcpAddr("CONFIG_DB"),
		Password:    "", // no password set
		DB:          sdcfg.GetDbId("CONFIG_DB"),
		DialTimeout: 0,
	})
	_, err := rclient.Ping().Result()
	if err != nil {
		t.Fatalf("failed to connect to redis server %v", err)
	}
	return rclient
}

func loadConfigDB(t *testing.T, rclient *redis.Client, mpi map[string]interface{}) {
	for key, fv := range mpi {
		switch fv.(type) {
		case map[string]interface{}:
			_, err := rclient.HMSet(key, fv.(map[string]interface{})).Result()
			if err != nil {
				t.Errorf("Invalid data for db: %v : %v %v", key, fv, err)
			}
		default:
			t.Errorf("Invalid data for db: %v : %v", key, fv)
		}
	}
}

func prepareConfigDb(t *testing.T) {
	rclient := getConfigDbClient(t)
	defer rclient.Close()
	rclient.FlushDB()

	fileName := "../testdata/COUNTERS_PORT_ALIAS_MAP.txt"
	countersPortAliasMapByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	mpi_alias_map := loadConfig(t, "", countersPortAliasMapByte)
	loadConfigDB(t, rclient, mpi_alias_map)

	fileName = "../testdata/CONFIG_PFCWD_PORTS.txt"
	configPfcwdByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	mpi_pfcwd_map := loadConfig(t, "", configPfcwdByte)
	loadConfigDB(t, rclient, mpi_pfcwd_map)
}

func prepareDb(t *testing.T) {
	rclient := getRedisClient(t)
	defer rclient.Close()
	rclient.FlushDB()
	//Enable keysapce notification
	os.Setenv("PATH", "/usr/bin:/sbin:/bin:/usr/local/bin")
	cmd := exec.Command("redis-cli", "config", "set", "notify-keyspace-events", "KEA")
	_, err := cmd.Output()
	if err != nil {
		t.Fatal("failed to enable redis keyspace notification ", err)
	}

	fileName := "../testdata/COUNTERS_PORT_NAME_MAP.txt"
	countersPortNameMapByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	mpi_name_map := loadConfig(t, "COUNTERS_PORT_NAME_MAP", countersPortNameMapByte)
	loadDB(t, rclient, mpi_name_map)

	fileName = "../testdata/COUNTERS_QUEUE_NAME_MAP.txt"
	countersQueueNameMapByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	mpi_qname_map := loadConfig(t, "COUNTERS_QUEUE_NAME_MAP", countersQueueNameMapByte)
	loadDB(t, rclient, mpi_qname_map)

	fileName = "../testdata/COUNTERS:Ethernet68.txt"
	countersEthernet68Byte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	// "Ethernet68": "oid:0x1000000000039",
	mpi_counter := loadConfig(t, "COUNTERS:oid:0x1000000000039", countersEthernet68Byte)
	loadDB(t, rclient, mpi_counter)

	fileName = "../testdata/COUNTERS:Ethernet1.txt"
	countersEthernet1Byte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	// "Ethernet1": "oid:0x1000000000003",
	mpi_counter = loadConfig(t, "COUNTERS:oid:0x1000000000003", countersEthernet1Byte)
	loadDB(t, rclient, mpi_counter)

	// "Ethernet64:0": "oid:0x1500000000092a"  : queue counter, to work as data noise
	fileName = "../testdata/COUNTERS:oid:0x1500000000092a.txt"
	counters92aByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	mpi_counter = loadConfig(t, "COUNTERS:oid:0x1500000000092a", counters92aByte)
	loadDB(t, rclient, mpi_counter)

	// "Ethernet68:1": "oid:0x1500000000091c"  : queue counter, for COUNTERS/Ethernet68/Queue vpath test
	fileName = "../testdata/COUNTERS:oid:0x1500000000091c.txt"
	countersEeth68_1Byte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	mpi_counter = loadConfig(t, "COUNTERS:oid:0x1500000000091c", countersEeth68_1Byte)
	loadDB(t, rclient, mpi_counter)

	// "Ethernet68:3": "oid:0x1500000000091e"  : lossless queue counter, for COUNTERS/Ethernet68/Pfcwd vpath test
	fileName = "../testdata/COUNTERS:oid:0x1500000000091e.txt"
	countersEeth68_3Byte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	mpi_counter = loadConfig(t, "COUNTERS:oid:0x1500000000091e", countersEeth68_3Byte)
	loadDB(t, rclient, mpi_counter)

	// "Ethernet68:4": "oid:0x1500000000091f"  : lossless queue counter, for COUNTERS/Ethernet68/Pfcwd vpath test
	fileName = "../testdata/COUNTERS:oid:0x1500000000091f.txt"
	countersEeth68_4Byte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	mpi_counter = loadConfig(t, "COUNTERS:oid:0x1500000000091f", countersEeth68_4Byte)
	loadDB(t, rclient, mpi_counter)

	// Load CONFIG_DB for alias translation
	prepareConfigDb(t)
}

func prepareDbTranslib(t *testing.T) {
	rclient := getRedisClient(t)
	rclient.FlushDB()
	rclient.Close()

	//Enable keysapce notification
	os.Setenv("PATH", "/usr/bin:/sbin:/bin:/usr/local/bin")
	cmd := exec.Command("redis-cli", "config", "set", "notify-keyspace-events", "KEA")
	_, err := cmd.Output()
	if err != nil {
		t.Fatal("failed to enable redis keyspace notification ", err)
	}

	fileName := "../testdata/db_dump.json"
	countersPortNameMapByte, err := ioutil.ReadFile(fileName)
	var rj []map[string]interface{}
	json.Unmarshal(countersPortNameMapByte, &rj)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	for n, v := range rj {
		rclient := getRedisClientN(t, n)
		loadDBNotStrict(t, rclient, v)
		rclient.Close()
	}
}

func TestGnmiSet(t *testing.T) {
	if !READ_WRITE_MODE {
		t.Skip("skipping test in read-only mode.")
	}
	s := createServer(t)
	go runServer(t, s)

	prepareDbTranslib(t)

	//t.Log("Start gNMI client")
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))}

	targetAddr := "127.0.0.1:8081"
	conn, err := grpc.Dial(targetAddr, opts...)
	if err != nil {
		t.Fatalf("Dialing to %q failed: %v", targetAddr, err)
	}
	defer conn.Close()

	gClient := pb.NewGNMIClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var emptyRespVal interface{}

	tds := []struct {
		desc          string
		pathTarget    string
		textPbPath    string
		wantRetCode   codes.Code
		wantRespVal   interface{}
		attributeData string
		operation     op_t
		valTest       bool
	}{
		{
			desc:       "Set OC Interface MTU",
			pathTarget: "OC_YANG",
			textPbPath: `
                        elem: <name: "openconfig-interfaces:interfaces" > elem:<name:"interface" key:<key:"name" value:"Ethernet4" > >
                `,
			attributeData: "../testdata/set_interface_mtu.json",
			wantRetCode:   codes.OK,
			wantRespVal:   emptyRespVal,
			operation:     Replace,
			valTest:       false,
		},
		{
			desc:       "Set OC Interface IP",
			pathTarget: "OC_YANG",
			textPbPath: `
                    elem:<name:"openconfig-interfaces:interfaces" > elem:<name:"interface" key:<key:"name" value:"Ethernet4" > > elem:<name:"subinterfaces" > elem:<name:"subinterface" key:<key:"index" value:"0" > >
                `,
			attributeData: "../testdata/set_interface_ipv4.json",
			wantRetCode:   codes.OK,
			wantRespVal:   emptyRespVal,
			operation:     Replace,
			valTest:       false,
		},
		// {
		//         desc:       "Check OC Interface values set",
		//         pathTarget: "OC_YANG",
		//         textPbPath: `
		//                 elem: <name: "openconfig-interfaces:interfaces" > elem: <name: "interface" key:<key:"name" value:"Ethernet4" > >
		//         `,
		//         wantRetCode: codes.OK,
		//         wantRespVal: interfaceData,
		//         valTest:true,
		// },
		{
			desc:       "Delete OC Interface IP",
			pathTarget: "OC_YANG",
			textPbPath: `
                    elem:<name:"openconfig-interfaces:interfaces" > elem:<name:"interface" key:<key:"name" value:"Ethernet4" > > elem:<name:"subinterfaces" > elem:<name:"subinterface" key:<key:"index" value:"0" > > elem:<name: "ipv4" > elem:<name: "addresses" > elem:<name:"address" key:<key:"ip" value:"9.9.9.9" > >
                `,
			attributeData: "",
			wantRetCode:   codes.OK,
			wantRespVal:   emptyRespVal,
			operation:     Delete,
			valTest:       false,
		},
	}

	for _, td := range tds {
		if td.valTest == true {
			// wait for 2 seconds for change to sync
			time.Sleep(2 * time.Second)
			t.Run(td.desc, func(t *testing.T) {
				runTestGet(t, ctx, gClient, td.pathTarget, td.textPbPath, td.wantRetCode, td.wantRespVal, td.valTest)
			})
		} else {
			t.Run(td.desc, func(t *testing.T) {
				runTestSet(t, ctx, gClient, td.pathTarget, td.textPbPath, td.wantRetCode, td.wantRespVal, td.attributeData, td.operation)
			})
		}
	}
	s.s.Stop()
}

func TestGnmiGet(t *testing.T) {
	//t.Log("Start server")
	s := createServer(t)
	go runServer(t, s)

	prepareDb(t)

	//t.Log("Start gNMI client")
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))}

	targetAddr := "127.0.0.1:8081"
	conn, err := grpc.Dial(targetAddr, opts...)
	if err != nil {
		t.Fatalf("Dialing to %q failed: %v", targetAddr, err)
	}
	defer conn.Close()

	gClient := pb.NewGNMIClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	fileName := "../testdata/COUNTERS_PORT_NAME_MAP.txt"
	countersPortNameMapByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}

	fileName = "../testdata/COUNTERS:Ethernet68.txt"
	countersEthernet68Byte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}

	fileName = "../testdata/COUNTERS:Ethernet68:Pfcwd.txt"
	countersEthernet68PfcwdByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}

	fileName = "../testdata/COUNTERS:Ethernet68:Pfcwd_alias.txt"
	countersEthernet68PfcwdAliasByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}

	fileName = "../testdata/COUNTERS:Ethernet_wildcard_alias.txt"
	countersEthernetWildcardByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}

	fileName = "../testdata/COUNTERS:Ethernet_wildcard_PFC_7_RX_alias.txt"
	countersEthernetWildcardPfcByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}

	fileName = "../testdata/COUNTERS:Ethernet_wildcard_Pfcwd_alias.txt"
	countersEthernetWildcardPfcwdByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}

	type testCase struct {
		desc        string
		pathTarget  string
		textPbPath  string
		wantRetCode codes.Code
		wantRespVal interface{}
		valTest     bool
		testInit    func()
	}

	// A helper function create test cases for 'osversion/build' queries.
	createBuildVersionTestCase := func(desc string, wantedVersion string, versionFileContent string, fileReadErr error) testCase {
		return testCase{
			desc:       desc,
			pathTarget: "OTHERS",
			textPbPath: `
						elem: <name: "osversion" >
						elem: <name: "build" >
					`,
			wantRetCode: codes.OK,
			valTest:     true,
			wantRespVal: []byte(wantedVersion),
			testInit: func() {
				// Override file read function to mock file content.
				sdc.ImplIoutilReadFile = func(filePath string) ([]byte, error) {
					if filePath == sdc.SonicVersionFilePath {
						if fileReadErr != nil {
							return nil, fileReadErr
						}
						return []byte(versionFileContent), nil
					}
					return ioutil.ReadFile(filePath)
				}

				// Reset the cache so that the content gets loaded again.
				sdc.InvalidateVersionFileStash()
			},
		}
	}

	tds := []testCase{{
		desc:       "Test non-existing path Target",
		pathTarget: "MY_DB",
		textPbPath: `
			elem: <name: "MyCounters" >
		`,
		wantRetCode: codes.NotFound,
	}, {
		desc:       "Test empty path target",
		pathTarget: "",
		textPbPath: `
			elem: <name: "MyCounters" >
		`,
		wantRetCode: codes.Unimplemented,
	}, {
		desc:       "Get valid but non-existing node",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
			elem: <name: "MyCounters" >
		`,
		wantRetCode: codes.NotFound,
	}, {
		desc:       "Get COUNTERS_PORT_NAME_MAP",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
			elem: <name: "COUNTERS_PORT_NAME_MAP" >
		`,
		wantRetCode: codes.OK,
		wantRespVal: countersPortNameMapByte,
	}, {
		desc:       "get COUNTERS:Ethernet68",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet68" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: countersEthernet68Byte,
	}, {
		desc:       "get COUNTERS:Ethernet68 SAI_PORT_STAT_PFC_7_RX_PKTS",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet68" >
					elem: <name: "SAI_PORT_STAT_PFC_7_RX_PKTS" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: "2",
	}, {
		desc:       "get COUNTERS:Ethernet68 Pfcwd",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet68" >
					elem: <name: "Pfcwd" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: countersEthernet68PfcwdByte,
	}, {
		desc:       "get COUNTERS (use vendor alias):Ethernet68/1",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet68/1" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: countersEthernet68Byte,
	}, {
		desc:       "get COUNTERS (use vendor alias):Ethernet68/1 SAI_PORT_STAT_PFC_7_RX_PKTS",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet68/1" >
					elem: <name: "SAI_PORT_STAT_PFC_7_RX_PKTS" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: "2",
	}, {
		desc:       "get COUNTERS (use vendor alias):Ethernet68/1 Pfcwd",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet68/1" >
					elem: <name: "Pfcwd" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: countersEthernet68PfcwdAliasByte,
	}, {
		desc:       "get COUNTERS:Ethernet*",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet*" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: countersEthernetWildcardByte,
	}, {
		desc:       "get COUNTERS:Ethernet* SAI_PORT_STAT_PFC_7_RX_PKTS",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet*" >
					elem: <name: "SAI_PORT_STAT_PFC_7_RX_PKTS" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: countersEthernetWildcardPfcByte,
	}, {
		desc:       "get COUNTERS:Ethernet* Pfcwd",
		pathTarget: "COUNTERS_DB",
		textPbPath: `
					elem: <name: "COUNTERS" >
					elem: <name: "Ethernet*" >
					elem: <name: "Pfcwd" >
				`,
		wantRetCode: codes.OK,
		wantRespVal: countersEthernetWildcardPfcwdByte,
	},
		// Happy path
		createBuildVersionTestCase(
			"get osversion/build",                                  // query path
			`{"build_version": "sonic.12345678.90", "error":""}`,   // expected response
			"build_version: '12345678.90'\ndebian_version: '9.13'", // YAML file content
			nil), // mock file reading error

		// File reading error
		createBuildVersionTestCase(
			"get osversion/build file load error",
			`{"build_version": "sonic.NA", "error":"Cannot access '/etc/sonic/sonic_version.yml'"}`,
			"",
			fmt.Errorf("Cannot access '%v'", sdc.SonicVersionFilePath)),

		// File content is not valid YAML
		createBuildVersionTestCase(
			"get osversion/build file parse error",
			`{"build_version": "sonic.NA", "error":"yaml: unmarshal errors:\n  line 1: cannot unmarshal !!str `+"`not a v...`"+` into client.SonicVersionInfo"}`,
			"not a valid YAML content",
			nil),

		// Happy path with different value
		createBuildVersionTestCase(
			"get osversion/build different value",
			`{"build_version": "sonic.23456789.01", "error":""}`,
			"build_version: '23456789.01'\ndebian_version: '9.15'",
			nil),
	}

	for _, td := range tds {
		if td.testInit != nil {
			td.testInit()
		}

		t.Run(td.desc, func(t *testing.T) {
			runTestGet(t, ctx, gClient, td.pathTarget, td.textPbPath, td.wantRetCode, td.wantRespVal, td.valTest)
		})
	}
	s.s.Stop()
}

func TestGnmiGetTranslib(t *testing.T) {
	//t.Log("Start server")
	s := createServer(t)
	go runServer(t, s)

	prepareDbTranslib(t)

	//t.Log("Start gNMI client")
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))}

	targetAddr := "127.0.0.1:8081"
	conn, err := grpc.Dial(targetAddr, opts...)
	if err != nil {
		t.Fatalf("Dialing to %q failed: %v", targetAddr, err)
	}
	defer conn.Close()

	gClient := pb.NewGNMIClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var emptyRespVal interface{}
	tds := []struct {
		desc        string
		pathTarget  string
		textPbPath  string
		wantRetCode codes.Code
		wantRespVal interface{}
		valTest     bool
	}{

		//These tests only work on the real switch platform, since they rely on files in the /proc and another running service
		// 	{
		// 	desc:       "Get OC Platform",
		// 	pathTarget: "OC_YANG",
		// 	textPbPath: `
		//                        elem: <name: "openconfig-platform:components" >
		//                `,
		// 	wantRetCode: codes.OK,
		// 	wantRespVal: emptyRespVal,
		// 	valTest:     false,
		// },
		// 	{
		// 		desc:       "Get OC System State",
		// 		pathTarget: "OC_YANG",
		// 		textPbPath: `
		//                        elem: <name: "openconfig-system:system" > elem: <name: "state" >
		//                `,
		// 		wantRetCode: codes.OK,
		// 		wantRespVal: emptyRespVal,
		// 		valTest:     false,
		// 	},
		// 	{
		// 		desc:       "Get OC System CPU",
		// 		pathTarget: "OC_YANG",
		// 		textPbPath: `
		//                        elem: <name: "openconfig-system:system" > elem: <name: "cpus" >
		//                `,
		// 		wantRetCode: codes.OK,
		// 		wantRespVal: emptyRespVal,
		// 		valTest:     false,
		// 	},
		// 	{
		// 		desc:       "Get OC System memory",
		// 		pathTarget: "OC_YANG",
		// 		textPbPath: `
		//                        elem: <name: "openconfig-system:system" > elem: <name: "memory" >
		//                `,
		// 		wantRetCode: codes.OK,
		// 		wantRespVal: emptyRespVal,
		// 		valTest:     false,
		// 	},
		// 	{
		// 		desc:       "Get OC System processes",
		// 		pathTarget: "OC_YANG",
		// 		textPbPath: `
		//                        elem: <name: "openconfig-system:system" > elem: <name: "processes" >
		//                `,
		// 		wantRetCode: codes.OK,
		// 		wantRespVal: emptyRespVal,
		// 		valTest:     false,
		// 	},
		{
			desc:       "Get OC Interfaces",
			pathTarget: "OC_YANG",
			textPbPath: `
                        elem: <name: "openconfig-interfaces:interfaces" >
                `,
			wantRetCode: codes.OK,
			wantRespVal: emptyRespVal,
			valTest:     false,
		},
		{
			desc:       "Get OC Interface",
			pathTarget: "OC_YANG",
			textPbPath: `
                        elem: <name: "openconfig-interfaces:interfaces" > elem: <name: "interface" key:<key:"name" value:"Ethernet4" > >
                `,
			wantRetCode: codes.OK,
			wantRespVal: emptyRespVal,
			valTest:     false,
		},
		{
			desc:       "Get OC Interface admin-status",
			pathTarget: "OC_YANG",
			textPbPath: `
                        elem: <name: "openconfig-interfaces:interfaces" > elem: <name: "interface" key:<key:"name" value:"Ethernet4" > > elem: <name: "state" > elem: <name: "admin-status" >
                `,
			wantRetCode: codes.OK,
			wantRespVal: emptyRespVal,
			valTest:     false,
		},
		{
			desc:       "Get OC Interface ifindex",
			pathTarget: "OC_YANG",
			textPbPath: `
                        elem: <name: "openconfig-interfaces:interfaces" > elem: <name: "interface" key:<key:"name" value:"Ethernet4" > > elem: <name: "state" > elem: <name: "ifindex" >
                `,
			wantRetCode: codes.OK,
			wantRespVal: emptyRespVal,
			valTest:     false,
		},
		{
			desc:       "Get OC Interface mtu",
			pathTarget: "OC_YANG",
			textPbPath: `
                        elem: <name: "openconfig-interfaces:interfaces" > elem: <name: "interface" key:<key:"name" value:"Ethernet4" > > elem: <name: "state" > elem: <name: "mtu" >
                `,
			wantRetCode: codes.OK,
			wantRespVal: emptyRespVal,
			valTest:     false,
		},
	}

	for _, td := range tds {
		t.Run(td.desc, func(t *testing.T) {
			runTestGet(t, ctx, gClient, td.pathTarget, td.textPbPath, td.wantRetCode, td.wantRespVal, td.valTest)
		})
	}
	s.s.Stop()
}

type tablePathValue struct {
	dbName    string
	tableName string
	tableKey  string
	delimitor string
	field     string
	value     string
	op        string
}

// runTestSubscribe subscribe DB path in stream mode or poll mode.
// The return code and response value are compared with expected code and value.
func runTestSubscribe(t *testing.T) {
	fileName := "../testdata/COUNTERS_PORT_NAME_MAP.txt"
	countersPortNameMapByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersPortNameMapJson interface{}
	json.Unmarshal(countersPortNameMapByte, &countersPortNameMapJson)
	var tmp interface{}
	json.Unmarshal(countersPortNameMapByte, &tmp)
	countersPortNameMapJsonUpdate := tmp.(map[string]interface{})
	countersPortNameMapJsonUpdate["test_field"] = "test_value"

	// for table key subscription
	fileName = "../testdata/COUNTERS:Ethernet68.txt"
	countersEthernet68Byte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersEthernet68Json interface{}
	json.Unmarshal(countersEthernet68Byte, &countersEthernet68Json)

	var tmp2 interface{}
	json.Unmarshal(countersEthernet68Byte, &tmp2)
	countersEthernet68JsonUpdate := tmp2.(map[string]interface{})
	countersEthernet68JsonUpdate["test_field"] = "test_value"

	var tmp3 interface{}
	json.Unmarshal(countersEthernet68Byte, &tmp3)
	countersEthernet68JsonPfcUpdate := tmp3.(map[string]interface{})
	// field SAI_PORT_STAT_PFC_7_RX_PKTS has new value of 4
	countersEthernet68JsonPfcUpdate["SAI_PORT_STAT_PFC_7_RX_PKTS"] = "4"

	// for Ethernet68/Pfcwd subscription
	fileName = "../testdata/COUNTERS:Ethernet68:Pfcwd.txt"
	countersEthernet68PfcwdByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersEthernet68PfcwdJson interface{}
	json.Unmarshal(countersEthernet68PfcwdByte, &countersEthernet68PfcwdJson)

	var tmp4 interface{}
	json.Unmarshal(countersEthernet68PfcwdByte, &tmp4)
	countersEthernet68PfcwdJsonUpdate := map[string]interface{}{}
	countersEthernet68PfcwdJsonUpdate["Ethernet68:3"] = tmp4.(map[string]interface{})["Ethernet68:3"]
	countersEthernet68PfcwdJsonUpdate["Ethernet68:3"].(map[string]interface{})["PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED"] = "1"

	tmp4.(map[string]interface{})["Ethernet68:3"].(map[string]interface{})["PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED"] = "1"
	countersEthernet68PfcwdPollUpdate := tmp4

	// (use vendor alias) for Ethernet68/1 Pfcwd subscription
	fileName = "../testdata/COUNTERS:Ethernet68:Pfcwd_alias.txt"
	countersEthernet68PfcwdAliasByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersEthernet68PfcwdAliasJson interface{}
	json.Unmarshal(countersEthernet68PfcwdAliasByte, &countersEthernet68PfcwdAliasJson)

	var tmp5 interface{}
	json.Unmarshal(countersEthernet68PfcwdAliasByte, &tmp5)
	countersEthernet68PfcwdAliasJsonUpdate := map[string]interface{}{}
	countersEthernet68PfcwdAliasJsonUpdate["Ethernet68/1:3"] = tmp5.(map[string]interface{})["Ethernet68/1:3"]
	countersEthernet68PfcwdAliasJsonUpdate["Ethernet68/1:3"].(map[string]interface{})["PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED"] = "1"

	tmp5.(map[string]interface{})["Ethernet68/1:3"].(map[string]interface{})["PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED"] = "1"
	countersEthernet68PfcwdAliasPollUpdate := tmp5

	fileName = "../testdata/COUNTERS:Ethernet_wildcard_alias.txt"
	countersEthernetWildcardByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersEthernetWildcardJson interface{}
	json.Unmarshal(countersEthernetWildcardByte, &countersEthernetWildcardJson)
	// Will have "test_field" : "test_value" in Ethernet68,
	countersEtherneWildcardJsonUpdate := map[string]interface{}{"Ethernet68/1": countersEthernet68JsonUpdate}

	// all counters on all ports with change on one field of one port
	var countersFieldUpdate map[string]interface{}
	json.Unmarshal(countersEthernetWildcardByte, &countersFieldUpdate)
	countersFieldUpdate["Ethernet68/1"] = countersEthernet68JsonPfcUpdate

	fileName = "../testdata/COUNTERS:Ethernet_wildcard_PFC_7_RX_alias.txt"
	countersEthernetWildcardPfcByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersEthernetWildcardPfcJson interface{}
	json.Unmarshal(countersEthernetWildcardPfcByte, &countersEthernetWildcardPfcJson)
	//The update with new value of 4 (original value is 2)
	pfc7Map := map[string]interface{}{"SAI_PORT_STAT_PFC_7_RX_PKTS": "4"}
	singlePortPfcJsonUpdate := make(map[string]interface{})
	singlePortPfcJsonUpdate["Ethernet68/1"] = pfc7Map

	allPortPfcJsonUpdate := make(map[string]interface{})
	json.Unmarshal(countersEthernetWildcardPfcByte, &allPortPfcJsonUpdate)
	//allPortPfcJsonUpdate := countersEthernetWildcardPfcJson.(map[string]interface{})
	allPortPfcJsonUpdate["Ethernet68/1"] = pfc7Map

	// for Ethernet*/Pfcwd subscription
	fileName = "../testdata/COUNTERS:Ethernet_wildcard_Pfcwd_alias.txt"
	countersEthernetWildPfcwdByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}

	var countersEthernetWildPfcwdJson interface{}
	json.Unmarshal(countersEthernetWildPfcwdByte, &countersEthernetWildPfcwdJson)

	var tmp6 interface{}
	json.Unmarshal(countersEthernetWildPfcwdByte, &tmp6)
	tmp6.(map[string]interface{})["Ethernet68/1:3"].(map[string]interface{})["PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED"] = "1"
	countersEthernetWildPfcwdUpdate := tmp6

	fileName = "../testdata/COUNTERS:Ethernet_wildcard_Queues_alias.txt"
	countersEthernetWildQueuesByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersEthernetWildQueuesJson interface{}
	json.Unmarshal(countersEthernetWildQueuesByte, &countersEthernetWildQueuesJson)

	fileName = "../testdata/COUNTERS:Ethernet68:Queues.txt"
	countersEthernet68QueuesByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersEthernet68QueuesJson interface{}
	json.Unmarshal(countersEthernet68QueuesByte, &countersEthernet68QueuesJson)

	countersEthernet68QueuesJsonUpdate := make(map[string]interface{})
	json.Unmarshal(countersEthernet68QueuesByte, &countersEthernet68QueuesJsonUpdate)
	eth68_1 := map[string]interface{}{
		"SAI_QUEUE_STAT_BYTES":           "0",
		"SAI_QUEUE_STAT_DROPPED_BYTES":   "0",
		"SAI_QUEUE_STAT_DROPPED_PACKETS": "4",
		"SAI_QUEUE_STAT_PACKETS":         "0",
	}
	countersEthernet68QueuesJsonUpdate["Ethernet68:1"] = eth68_1

	// Alias translation for query Ethernet68/1:Queues
	fileName = "../testdata/COUNTERS:Ethernet68:Queues_alias.txt"
	countersEthernet68QueuesAliasByte, err := ioutil.ReadFile(fileName)
	if err != nil {
		t.Fatalf("read file %v err: %v", fileName, err)
	}
	var countersEthernet68QueuesAliasJson interface{}
	json.Unmarshal(countersEthernet68QueuesAliasByte, &countersEthernet68QueuesAliasJson)

	countersEthernet68QueuesAliasJsonUpdate := make(map[string]interface{})
	json.Unmarshal(countersEthernet68QueuesAliasByte, &countersEthernet68QueuesAliasJsonUpdate)
	countersEthernet68QueuesAliasJsonUpdate["Ethernet68/1:1"] = eth68_1

	tests := []struct {
		desc     string
		q        client.Query
		prepares []tablePathValue
		updates  []tablePathValue
		wantErr  bool
		wantNoti []client.Notification

		poll        int
		wantPollErr string
	}{{
		desc: "stream query for table COUNTERS_PORT_NAME_MAP with new test_field field",
		q: client.Query{
			Target:  "COUNTERS_DB",
			Type:    client.Stream,
			Queries: []client.Path{{"COUNTERS_PORT_NAME_MAP"}},
			TLS:     &tls.Config{InsecureSkipVerify: true},
		},
		updates: []tablePathValue{{
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS_PORT_NAME_MAP",
			field:     "test_field",
			value:     "test_value",
		}},
		wantNoti: []client.Notification{
			client.Connected{},
			client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJson},
			client.Sync{},
			client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJsonUpdate},
		},
	}, {
		desc: "stream query for table key Ethernet68 with new test_field field",
		q: client.Query{
			Target:  "COUNTERS_DB",
			Type:    client.Stream,
			Queries: []client.Path{{"COUNTERS", "Ethernet68"}},
			TLS:     &tls.Config{InsecureSkipVerify: true},
		},
		updates: []tablePathValue{{
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
			delimitor: ":",
			field:     "test_field",
			value:     "test_value",
		}, { //Same value set should not trigger multiple updates
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
			delimitor: ":",
			field:     "test_field",
			value:     "test_value",
		}},
		wantNoti: []client.Notification{
			client.Connected{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68"}, TS: time.Unix(0, 200), Val: countersEthernet68Json},
			client.Sync{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68"}, TS: time.Unix(0, 200), Val: countersEthernet68JsonUpdate},
		},
	}, {
		desc: "(use vendor alias) stream query for table key Ethernet68/1 with new test_field field",
		q: client.Query{
			Target:  "COUNTERS_DB",
			Type:    client.Stream,
			Queries: []client.Path{{"COUNTERS", "Ethernet68/1"}},
			TLS:     &tls.Config{InsecureSkipVerify: true},
		},
		updates: []tablePathValue{{
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
			delimitor: ":",
			field:     "test_field",
			value:     "test_value",
		}, { //Same value set should not trigger multiple updates
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
			delimitor: ":",
			field:     "test_field",
			value:     "test_value",
		}},
		wantNoti: []client.Notification{
			client.Connected{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68/1"}, TS: time.Unix(0, 200), Val: countersEthernet68Json},
			client.Sync{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68/1"}, TS: time.Unix(0, 200), Val: countersEthernet68JsonUpdate},
		},
	}, {
		desc: "stream query for COUNTERS/Ethernet68/SAI_PORT_STAT_PFC_7_RX_PKTS with update of field value",
		q: client.Query{
			Target:  "COUNTERS_DB",
			Type:    client.Stream,
			Queries: []client.Path{{"COUNTERS", "Ethernet68", "SAI_PORT_STAT_PFC_7_RX_PKTS"}},
			TLS:     &tls.Config{InsecureSkipVerify: true},
		},
		updates: []tablePathValue{{
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
			delimitor: ":",
			field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
			value:     "3", // be changed to 3 from 2
		}, { //Same value set should not trigger multiple updates
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
			delimitor: ":",
			field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
			value:     "3", // be changed to 3 from 2
		}},
		wantNoti: []client.Notification{
			client.Connected{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68", "SAI_PORT_STAT_PFC_7_RX_PKTS"}, TS: time.Unix(0, 200), Val: "2"},
			client.Sync{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68", "SAI_PORT_STAT_PFC_7_RX_PKTS"}, TS: time.Unix(0, 200), Val: "3"},
		},
	}, {
		desc: "(use vendor alias) stream query for COUNTERS/[Ethernet68/1]/SAI_PORT_STAT_PFC_7_RX_PKTS with update of field value",
		q: client.Query{
			Target:  "COUNTERS_DB",
			Type:    client.Stream,
			Queries: []client.Path{{"COUNTERS", "Ethernet68/1", "SAI_PORT_STAT_PFC_7_RX_PKTS"}},
			TLS:     &tls.Config{InsecureSkipVerify: true},
		},
		updates: []tablePathValue{{
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
			delimitor: ":",
			field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
			value:     "3", // be changed to 3 from 2
		}, { //Same value set should not trigger multiple updates
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
			delimitor: ":",
			field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
			value:     "3", // be changed to 3 from 2
		}},
		wantNoti: []client.Notification{
			client.Connected{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "SAI_PORT_STAT_PFC_7_RX_PKTS"}, TS: time.Unix(0, 200), Val: "2"},
			client.Sync{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "SAI_PORT_STAT_PFC_7_RX_PKTS"}, TS: time.Unix(0, 200), Val: "3"},
		},
	}, {
		desc: "stream query for COUNTERS/Ethernet68/Pfcwd with update of field value",
		q: client.Query{
			Target:  "COUNTERS_DB",
			Type:    client.Stream,
			Queries: []client.Path{{"COUNTERS", "Ethernet68", "Pfcwd"}},
			TLS:     &tls.Config{InsecureSkipVerify: true},
		},
		updates: []tablePathValue{{
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1500000000091e", // "Ethernet68:3": "oid:0x1500000000091e",
			delimitor: ":",
			field:     "PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
			value:     "1", // be changed to 1 from 0
		}, { //Same value set should not trigger multiple updates
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1500000000091e", // "Ethernet68:3": "oid:0x1500000000091e"
			delimitor: ":",
			field:     "PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
			value:     "1", // be changed to 1 from 1
		}},
		wantNoti: []client.Notification{
			client.Connected{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdJson},
			client.Sync{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdJsonUpdate},
		},
	}, {
		desc: "(use vendor alias) stream query for COUNTERS/[Ethernet68/1]/Pfcwd with update of field value",
		q: client.Query{
			Target:  "COUNTERS_DB",
			Type:    client.Stream,
			Queries: []client.Path{{"COUNTERS", "Ethernet68/1", "Pfcwd"}},
			TLS:     &tls.Config{InsecureSkipVerify: true},
		},
		updates: []tablePathValue{{
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1500000000091e", // "Ethernet68:3": "oid:0x1500000000091e",
			delimitor: ":",
			field:     "PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
			value:     "1", // be changed to 1 from 0
		}, { //Same value set should not trigger multiple updates
			dbName:    "COUNTERS_DB",
			tableName: "COUNTERS",
			tableKey:  "oid:0x1500000000091e", // "Ethernet68:3": "oid:0x1500000000091e"
			delimitor: ":",
			field:     "PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
			value:     "1", // be changed to 1 from 1
		}},
		wantNoti: []client.Notification{
			client.Connected{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdAliasJson},
			client.Sync{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdAliasJsonUpdate},
		},
	},
		{
			desc: "stream query for table key Ethernet* with new test_field field on Ethernet68",
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Stream,
				Queries: []client.Path{{"COUNTERS", "Ethernet*"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
				delimitor: ":",
				field:     "test_field",
				value:     "test_value",
			}, { //Same value set should not trigger multiple updates
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
				delimitor: ":",
				field:     "test_field",
				value:     "test_value",
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*"},
					TS: time.Unix(0, 200), Val: countersEthernetWildcardJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*"},
					TS: time.Unix(0, 200), Val: countersEtherneWildcardJsonUpdate},
			},
		}, {
			desc: "stream query for table key Ethernet*/SAI_PORT_STAT_PFC_7_RX_PKTS with field value update",
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Stream,
				Queries: []client.Path{{"COUNTERS", "Ethernet*", "SAI_PORT_STAT_PFC_7_RX_PKTS"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
				delimitor: ":",
				field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
				value:     "4", // being changed to 4 from 2
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: countersEthernetWildcardPfcJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: singlePortPfcJsonUpdate},
			},
		}, {
			desc: "stream query for table key Ethernet*/Pfcwd with field value update",
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Stream,
				Queries: []client.Path{{"COUNTERS", "Ethernet*", "Pfcwd"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1500000000091e", // "Ethernet68:3": "oid:0x1500000000091e",
				delimitor: ":",
				field:     "PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
				value:     "1", // being changed to 1 from 0
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernetWildPfcwdJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdAliasJsonUpdate},
			},
		}, {
			desc: "poll query for table COUNTERS_PORT_NAME_MAP with new field test_field",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS_PORT_NAME_MAP"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS_PORT_NAME_MAP",
				field:     "test_field",
				value:     "test_value",
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				// We are starting from the result data of "stream query for table with update of new field",
				client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJsonUpdate},
				client.Sync{},
			},
		}, {
			desc: "poll query for table COUNTERS_PORT_NAME_MAP with test_field delete",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS_PORT_NAME_MAP"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			prepares: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS_PORT_NAME_MAP",
				field:     "test_field",
				value:     "test_value",
			}},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS_PORT_NAME_MAP",
				field:     "test_field",
				op:        "hdel",
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				// We are starting from the result data of "stream query for table with update of new field",
				client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS_PORT_NAME_MAP"}, TS: time.Unix(0, 200), Val: countersPortNameMapJson},
				client.Sync{},
			},
		}, {
			desc: "poll query for COUNTERS/Ethernet68/SAI_PORT_STAT_PFC_7_RX_PKTS with field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet68", "SAI_PORT_STAT_PFC_7_RX_PKTS"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
				delimitor: ":",
				field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
				value:     "4", // being changed to 4 from 2
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: "2"},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: "4"},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: "4"},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: "4"},
				client.Sync{},
			},
		}, {
			desc: "(use vendor alias) poll query for COUNTERS/[Ethernet68/1]/SAI_PORT_STAT_PFC_7_RX_PKTS with field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet68/1", "SAI_PORT_STAT_PFC_7_RX_PKTS"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
				delimitor: ":",
				field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
				value:     "4", // being changed to 4 from 2
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: "2"},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: "4"},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: "4"},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: "4"},
				client.Sync{},
			},
		}, {
			desc: "poll query for COUNTERS/Ethernet68/Pfcwd with field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet68", "Pfcwd"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1500000000091e", // "Ethernet68:3": "oid:0x1500000000091e",
				delimitor: ":",
				field:     "PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
				value:     "1", // be changed to 1 from 0
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdPollUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdPollUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdPollUpdate},
				client.Sync{},
			},
		}, {
			desc: "(use vendor alias) poll query for COUNTERS/[Ethernet68/1]/Pfcwd with field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet68/1", "Pfcwd"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1500000000091e", // "Ethernet68:3": "oid:0x1500000000091e",
				delimitor: ":",
				field:     "PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
				value:     "1", // be changed to 1 from 0
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdAliasJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdAliasPollUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdAliasPollUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernet68PfcwdAliasPollUpdate},
				client.Sync{},
			},
		},
		{
			desc: "poll query for table key Ethernet* with Ethernet68/SAI_PORT_STAT_PFC_7_RX_PKTS field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet*"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
				delimitor: ":",
				field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
				value:     "4", // being changed to 4 from 2
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*"},
					TS: time.Unix(0, 200), Val: countersEthernetWildcardJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*"},
					TS: time.Unix(0, 200), Val: countersFieldUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*"},
					TS: time.Unix(0, 200), Val: countersFieldUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*"},
					TS: time.Unix(0, 200), Val: countersFieldUpdate},
				client.Sync{},
			},
		}, {
			desc: "poll query for table key field Ethernet*/SAI_PORT_STAT_PFC_7_RX_PKTS with Ethernet68/SAI_PORT_STAT_PFC_7_RX_PKTS field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet*", "SAI_PORT_STAT_PFC_7_RX_PKTS"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1000000000039", // "Ethernet68": "oid:0x1000000000039",
				delimitor: ":",
				field:     "SAI_PORT_STAT_PFC_7_RX_PKTS",
				value:     "4", // being changed to 4 from 2
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: countersEthernetWildcardPfcJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: allPortPfcJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: allPortPfcJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "SAI_PORT_STAT_PFC_7_RX_PKTS"},
					TS: time.Unix(0, 200), Val: allPortPfcJsonUpdate},
				client.Sync{},
			},
		}, {
			desc: "poll query for table key field Etherenet*/Pfcwd with Ethernet68:3/PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet*", "Pfcwd"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1500000000091e", // "Ethernet68:3": "oid:0x1500000000091e",
				delimitor: ":",
				field:     "PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
				value:     "1", // being changed to 1 from 0
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernetWildPfcwdJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernetWildPfcwdUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernetWildPfcwdUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "Pfcwd"}, TS: time.Unix(0, 200), Val: countersEthernetWildPfcwdUpdate},
				client.Sync{},
			},
		},
		{
			desc: "poll query for COUNTERS/Ethernet*/Queues",
			poll: 1,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet*", "Queues"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernetWildQueuesJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet*", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernetWildQueuesJson},
				client.Sync{},
			},
		}, {
			desc: "poll query for COUNTERS/Ethernet68/Queues with field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet68", "Queues"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1500000000091c", // "Ethernet68:1": "oid:0x1500000000091c",
				delimitor: ":",
				field:     "SAI_QUEUE_STAT_DROPPED_PACKETS",
				value:     "4", // being changed to 0 from 4
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernet68QueuesJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernet68QueuesJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernet68QueuesJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernet68QueuesJsonUpdate},
				client.Sync{},
			},
		}, {
			desc: "(use vendor alias) poll query for COUNTERS/Ethernet68/Queues with field value change",
			poll: 3,
			q: client.Query{
				Target:  "COUNTERS_DB",
				Type:    client.Poll,
				Queries: []client.Path{{"COUNTERS", "Ethernet68/1", "Queues"}},
				TLS:     &tls.Config{InsecureSkipVerify: true},
			},
			updates: []tablePathValue{{
				dbName:    "COUNTERS_DB",
				tableName: "COUNTERS",
				tableKey:  "oid:0x1500000000091c", // "Ethernet68:1": "oid:0x1500000000091c",
				delimitor: ":",
				field:     "SAI_QUEUE_STAT_DROPPED_PACKETS",
				value:     "4", // being changed to 0 from 4
			}},
			wantNoti: []client.Notification{
				client.Connected{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernet68QueuesAliasJson},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernet68QueuesAliasJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernet68QueuesAliasJsonUpdate},
				client.Sync{},
				client.Update{Path: []string{"COUNTERS", "Ethernet68/1", "Queues"},
					TS: time.Unix(0, 200), Val: countersEthernet68QueuesAliasJsonUpdate},
				client.Sync{},
			},
		}}

	rclient := getRedisClient(t)
	defer rclient.Close()
	for _, tt := range tests {
		prepareDb(t)
		// Extra db preparation for this test case
		for _, prepare := range tt.prepares {
			switch prepare.op {
			case "hdel":
				rclient.HDel(prepare.tableName+prepare.delimitor+prepare.tableKey, prepare.field)
			default:
				rclient.HSet(prepare.tableName+prepare.delimitor+prepare.tableKey, prepare.field, prepare.value)
			}
		}
		time.Sleep(time.Millisecond * 1000)
		t.Run(tt.desc, func(t *testing.T) {
			q := tt.q
			q.Addrs = []string{"127.0.0.1:8081"}
			c := client.New()
			defer c.Close()
			var gotNoti []client.Notification
			q.NotificationHandler = func(n client.Notification) error {
				//t.Logf("reflect.TypeOf(n) %v :  %v", reflect.TypeOf(n), n)
				if nn, ok := n.(client.Update); ok {
					nn.TS = time.Unix(0, 200)
					gotNoti = append(gotNoti, nn)
				} else {
					gotNoti = append(gotNoti, n)
				}

				return nil
			}
			go func() {
				c.Subscribe(context.Background(), q)
				/*
					err := c.Subscribe(context.Background(), q)
					t.Log("c.Subscribe err:", err)
					switch {
					case tt.wantErr && err != nil:
						return
					case tt.wantErr && err == nil:
						t.Fatalf("c.Subscribe(): got nil error, expected non-nil")
					case !tt.wantErr && err != nil:
						t.Fatalf("c.Subscribe(): got error %v, expected nil", err)
					}
				*/
			}()
			// wait for half second for subscribeRequest to sync
			time.Sleep(time.Millisecond * 500)
			for _, update := range tt.updates {
				switch update.op {
				case "hdel":
					rclient.HDel(update.tableName+update.delimitor+update.tableKey, update.field)
				default:
					rclient.HSet(update.tableName+update.delimitor+update.tableKey, update.field, update.value)
				}
				time.Sleep(time.Millisecond * 1000)
			}
			// wait for half second for change to sync
			time.Sleep(time.Millisecond * 500)

			for i := 0; i < tt.poll; i++ {
				err := c.Poll()
				switch {
				case err == nil && tt.wantPollErr != "":
					t.Errorf("c.Poll(): got nil error, expected non-nil %v", tt.wantPollErr)
				case err != nil && tt.wantPollErr == "":
					t.Errorf("c.Poll(): got error %v, expected nil", err)
				case err != nil && err.Error() != tt.wantPollErr:
					t.Errorf("c.Poll(): got error %v, expected error %v", err, tt.wantPollErr)
				}
			}
			// t.Log("\n Want: \n", tt.wantNoti)
			// t.Log("\n Got : \n", gotNoti)
			if diff := pretty.Compare(tt.wantNoti, gotNoti); diff != "" {
				t.Log("\n Want: \n", tt.wantNoti)
				t.Log("\n Got : \n", gotNoti)
				t.Errorf("unexpected updates:\n%s", diff)
			}
		})
	}
}

func TestGnmiSubscribe(t *testing.T) {
	s := createServer(t)
	go runServer(t, s)

	runTestSubscribe(t)

	s.s.Stop()
}

func TestCapabilities(t *testing.T) {
	//t.Log("Start server")
	s := createServer(t)
	go runServer(t, s)

	// prepareDb(t)

	//t.Log("Start gNMI client")
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))}

	//targetAddr := "30.57.185.38:8080"
	targetAddr := "127.0.0.1:8081"
	conn, err := grpc.Dial(targetAddr, opts...)
	if err != nil {
		t.Fatalf("Dialing to %q failed: %v", targetAddr, err)
	}
	defer conn.Close()

	gClient := pb.NewGNMIClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var req pb.CapabilityRequest
	resp, err := gClient.Capabilities(ctx, &req)
	if err != nil {
		t.Fatalf("Failed to get Capabilities")
	}
	if len(resp.SupportedModels) == 0 {
		t.Fatalf("No Supported Models found!")
	}

}

func init() {
	// Inform gNMI server to use redis tcp localhost connection
	sdc.UseRedisLocalTcpPort = true
}
