package client_test

// Prerequisite: redis-server should be running.

import (
	"context"
	tls "crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"

	sdc "github.com/Azure/sonic-telemetry/sonic_data_client"
	"github.com/kylelemons/godebug/pretty"

	gnmi "github.com/Azure/sonic-telemetry/gnmi_server"

	xpath "github.com/jipanyang/gnxi/utils/xpath"
	"github.com/openconfig/gnmi/client"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	value "github.com/openconfig/gnmi/value"

	spb "github.com/Azure/sonic-telemetry/proto"
	testcert "github.com/Azure/sonic-telemetry/testdata/tls"
	redis "github.com/go-redis/redis"
	"google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	status "google.golang.org/grpc/status"

	gclient "github.com/jipanyang/gnmi/client/gnmi"
)

var clientTypes = []string{gclient.Type}

func getRedisClient(t *testing.T, dbName string) *redis.Client {
	dbn := spb.Target_value[dbName]
	rclient := redis.NewClient(&redis.Options{
		Network:     "tcp",
		Addr:        "localhost:6379",
		Password:    "", // no password set
		DB:          int(dbn),
		DialTimeout: 0,
	})
	_, err := rclient.Ping().Result()
	if err != nil {
		t.Fatal("failed to connect to redis server ", err)
	}
	return rclient
}

func setTestDataToRedisDB(t *testing.T, rclient *redis.Client, mpi map[string]interface{}) {
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

func parseTestData(t *testing.T, key string, in []byte) map[string]interface{} {
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

func loadTestDataIntoRedis(t *testing.T, redisClient *redis.Client, dbKey string, testDataPath string) {
	data, err := ioutil.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("read file %v err: %v", testDataPath, err)
	}
	data_kv_map := parseTestData(t, dbKey, data)
	setTestDataToRedisDB(t, redisClient, data_kv_map)
}

func prepareConfigDB(t *testing.T) {
	configDB := getRedisClient(t, "CONFIG_DB")
	defer configDB.Close()
	configDB.FlushDB()

	loadTestDataIntoRedis(t, configDB, "", "../testdata/COUNTERS_PORT_ALIAS_MAP.txt")
	loadTestDataIntoRedis(t, configDB, "", "../testdata/CONFIG_PFCWD_PORTS.txt")
}

func creategNMIServer(t *testing.T) *gnmi.Server {
	certificate, err := testcert.NewCert()
	if err != nil {
		t.Errorf("could not load server key pair: %s", err)
	}
	tlsCfg := &tls.Config{
		ClientAuth:   tls.RequestClientCert,
		Certificates: []tls.Certificate{certificate},
	}

	opts := []grpc.ServerOption{grpc.Creds(credentials.NewTLS(tlsCfg))}
	cfg := &gnmi.Config{Port: 8080}
	s, err := gnmi.NewServer(cfg, opts)
	if err != nil {
		t.Errorf("Failed to create gNMI server: %v", err)
	}

	return s
}

func rungNMIServer(t *testing.T, s *gnmi.Server) {
	//t.Log("Starting RPC server on address:", s.Address())
	err := s.Serve() // blocks until close
	if err != nil {
		t.Fatalf("gRPC server err: %v", err)
	}
	//t.Log("Exiting RPC server on address", s.Address())
}

func sendGetRequest(t *testing.T, ctx context.Context, gnmiClient gnmipb.GNMIClient, xPath string,
	pathToTargetDB string, expectedReturnCode codes.Code) *gnmipb.GetResponse {
	// Issue Get RPC.
	pbPath, err := xpath.ToGNMIPath(xPath)
	if err != nil {
		t.Fatalf("error in parsing xpath %q to gnmi path", xPath)
	}

	prefix := gnmipb.Path{Target: pathToTargetDB}
	request := &gnmipb.GetRequest{
		Prefix:   &prefix,
		Path:     []*gnmipb.Path{pbPath},
		Encoding: gnmipb.Encoding_JSON_IETF,
	}

	response, err := gnmiClient.Get(ctx, request)

	// Check return value and gRPC status code.
	returnStatus, ok := status.FromError(err)
	if !ok {
		t.Fatal("got a non-grpc error from grpc call")
	}
	if returnStatus.Code() != expectedReturnCode {
		t.Log("err: ", err)
		t.Fatalf("got return code %v, expected %v", returnStatus.Code(), expectedReturnCode)
	}

	return response
}

var expectedValueEthernet68 = map[string]interface{}{
	"PFC_WD_QUEUE_STATS_DEADLOCK_RESTORED":                "0",
	"PFC_WD_STATUS":                                       "operational",
	"SAI_PORT_STAT_PFC_4_RX_PKTS":                         "0",
	"PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED":                "0",
	"SAI_PORT_STAT_ETHER_STATS_RX_NO_ERRORS":              "0",
	"SAI_PORT_STAT_IF_OUT_OCTETS":                         "0",
	"SAI_PORT_STAT_IPV6_IN_RECEIVES":                      "0",
	"SAI_PORT_STAT_IPV6_OUT_MCAST_PKTS":                   "0",
	"SAI_PORT_STAT_IP_IN_UCAST_PKTS":                      "0",
	"SAI_PORT_STAT_ETHER_STATS_OVERSIZE_PKTS":             "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_128_TO_255_OCTETS":       "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_256_TO_511_OCTETS":      "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_9217_TO_16383_OCTETS":   "0",
	"SAI_PORT_STAT_ETHER_STATS_UNDERSIZE_PKTS":            "0",
	"SAI_PORT_STAT_IF_IN_MULTICAST_PKTS":                  "0",
	"SAI_PORT_STAT_IF_OUT_UCAST_PKTS":                     "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_1024_TO_1518_OCTETS":     "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_64_OCTETS":               "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_2048_TO_4095_OCTETS":    "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_65_TO_127_OCTETS":       "0",
	"SAI_PORT_STAT_ETHER_RX_OVERSIZE_PKTS":                "0",
	"SAI_PORT_STAT_ETHER_STATS_JABBERS":                   "0",
	"SAI_PORT_STAT_IPV6_OUT_NON_UCAST_PKTS":               "0",
	"SAI_PORT_STAT_IP_IN_DISCARDS":                        "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_2048_TO_4095_OCTETS":     "0",
	"SAI_PORT_STAT_IF_IN_VLAN_DISCARDS":                   "0",
	"SAI_PORT_STAT_IP_OUT_OCTETS":                         "0",
	"SAI_PORT_STAT_IF_IN_UCAST_PKTS":                      "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_256_TO_511_OCTETS":    "0",
	"SAI_PORT_STAT_IF_IN_OCTETS":                          "0",
	"SAI_PORT_STAT_IF_OUT_ERRORS":                         "0",
	"SAI_PORT_STAT_IP_IN_NON_UCAST_PKTS":                  "0",
	"SAI_PORT_STAT_ETHER_STATS_BROADCAST_PKTS":            "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_128_TO_255_OCTETS":    "0",
	"SAI_PORT_STAT_IPV6_OUT_DISCARDS":                     "0",
	"SAI_PORT_STAT_IPV6_OUT_UCAST_PKTS":                   "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_1519_TO_2047_OCTETS":     "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_9217_TO_16383_OCTETS":    "0",
	"SAI_PORT_STAT_ETHER_STATS_MULTICAST_PKTS":            "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS":                      "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_1519_TO_2047_OCTETS":  "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_512_TO_1023_OCTETS":   "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_64_OCTETS":            "0",
	"SAI_PORT_STAT_IF_OUT_MULTICAST_PKTS":                 "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_65_TO_127_OCTETS":        "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_65_TO_127_OCTETS":     "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_9217_TO_16383_OCTETS": "0",
	"SAI_PORT_STAT_ETHER_TX_OVERSIZE_PKTS":                "0",
	"SAI_PORT_STAT_IF_OUT_DISCARDS":                       "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_4096_TO_9216_OCTETS":  "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_1024_TO_1518_OCTETS":  "0",
	"SAI_PORT_STAT_IPV6_IN_DISCARDS":                      "0",
	"SAI_PORT_STAT_IPV6_OUT_OCTETS":                       "0",
	"SAI_PORT_STAT_IP_IN_OCTETS":                          "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_4096_TO_9216_OCTETS":     "0",
	"SAI_PORT_STAT_IPV6_IN_OCTETS":                        "0",
	"SAI_PORT_STAT_ETHER_STATS_COLLISIONS":                "0",
	"SAI_PORT_STAT_IF_OUT_NON_UCAST_PKTS":                 "0",
	"SAI_PORT_STAT_IP_OUT_NON_UCAST_PKTS":                 "0",
	"SAI_PORT_STAT_IP_OUT_UCAST_PKTS":                     "0",
	"SAI_PORT_STAT_IF_IN_NON_UCAST_PKTS":                  "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_512_TO_1023_OCTETS":      "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_1024_TO_1518_OCTETS":    "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_128_TO_255_OCTETS":      "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_512_TO_1023_OCTETS":     "0",
	"SAI_PORT_STAT_ETHER_STATS_CRC_ALIGN_ERRORS":          "0",
	"SAI_PORT_STAT_ETHER_STATS_DROP_EVENTS":               "0",
	"SAI_PORT_STAT_ETHER_STATS_PKTS_2048_TO_4095_OCTETS":  "0",
	"SAI_PORT_STAT_ETHER_IN_PKTS_256_TO_511_OCTETS":       "0",
	"SAI_PORT_STAT_IPV6_IN_MCAST_PKTS":                    "0",
	"SAI_PORT_STAT_IP_IN_RECEIVES":                        "0",
	"SAI_PORT_STAT_IF_IN_BROADCAST_PKTS":                  "0",
	"SAI_PORT_STAT_IF_IN_DISCARDS":                        "0",
	"SAI_PORT_STAT_IF_IN_ERRORS":                          "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_4096_TO_9216_OCTETS":    "0",
	"SAI_PORT_STAT_ETHER_STATS_FRAGMENTS":                 "0",
	"SAI_PORT_STAT_ETHER_STATS_TX_NO_ERRORS":              "0",
	"SAI_PORT_STAT_IF_OUT_BROADCAST_PKTS":                 "0",
	"SAI_PORT_STAT_IPV6_IN_NON_UCAST_PKTS":                "0",
	"SAI_PORT_STAT_IP_OUT_DISCARDS":                       "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_1519_TO_2047_OCTETS":    "0",
	"SAI_PORT_STAT_ETHER_STATS_OCTETS":                    "0",
	"SAI_PORT_STAT_IF_IN_UNKNOWN_PROTOS":                  "0",
	"SAI_PORT_STAT_IF_OUT_QLEN":                           "0",
	"SAI_PORT_STAT_IPV6_IN_UCAST_PKTS":                    "0",
	"SAI_PORT_STAT_ETHER_OUT_PKTS_64_OCTETS":              "0",
	"SAI_QUEUE_STAT_DROPPED_PACKETS":                      "0",
	"SAI_QUEUE_STAT_PACKETS":                              "0",
	"SAI_QUEUE_STAT_BYTES":                                "0",
	"SAI_QUEUE_STAT_DROPPED_BYTES":                        "0",
	"SAI_PORT_STAT_PFC_3_RX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_7_RX_PKTS":                         "2",
	"SAI_PORT_STAT_PFC_4_TX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_6_RX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_5_RX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_7_TX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_1_TX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_2_RX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_5_TX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_6_TX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_0_RX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_2_TX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_3_TX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_0_TX_PKTS":                         "0",
	"SAI_PORT_STAT_PFC_1_RX_PKTS":                         "0",
}

var expectedValueEthernet68Pfcwd = map[string]interface{}{
	"PFC_WD_STATUS":                        "operational",
	"SAI_PORT_STAT_PFC_3_RX_PKTS":          "0",
	"SAI_PORT_STAT_PFC_4_RX_PKTS":          "0",
	"PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED": "0",
	"PFC_WD_QUEUE_STATS_DEADLOCK_RESTORED": "0",
}

func dumpMapFromResponse(response *gnmipb.GetResponse, name string) {
	fmt.Printf("\n\n>>>>>\n\n")
	notifs := response.GetNotification()
	var gotVal interface{}
	var gotMap = make(map[string]interface{})

	count := len(notifs)
	for i := 0; i < count; i++ {
		val := notifs[i].GetUpdate()[0].GetVal()
		json.Unmarshal(val.GetJsonIetfVal(), &gotVal)
		m := gotVal.(map[string]interface{})
		for k, v := range m {
			gotMap[k] = v
		}
	}

	fmt.Printf("var %s = map[string]interface{}{\n", name)
	for k, v := range gotMap {
		fmt.Printf("\t\"%v\": \"%v\",\n", k, v)
	}
	fmt.Printf("}\n")
	fmt.Printf("\n\n>>>>>\n\n")
}

func assertExpectedValueFromMap(t *testing.T, response *gnmipb.GetResponse, expected map[string]interface{}) {
	notifs := response.GetNotification()
	var gotVal interface{}
	var gotMap = make(map[string]interface{})

	// Load up all k, v pairs out of the JSON value into a go map.
	count := len(notifs)
	for i := 0; i < count; i++ {
		val := notifs[i].GetUpdate()[0].GetVal()
		json.Unmarshal(val.GetJsonIetfVal(), &gotVal)
		m := gotVal.(map[string]interface{})
		for k, v := range m {
			gotMap[k] = v
		}
	}

	// Assert matching k, v pairs from the `expected` map and data from the gnmi response.
	if len(expected) != len(gotMap) {
		t.Fatalf("Expected %v entries, got %v.", len(expected), len(gotMap))
	}
	for k, v := range gotMap {
		if val, ok := expected[k]; ok {
			if val != v {
				t.Fatalf("Expected key %v with value %v, but got the value %v instead.", k, val, v)
			}
		} else {
			t.Fatalf("Received unexpected key %v from output.", k)
		}
	}
}

func assertExpectedValue(t *testing.T, response *gnmipb.GetResponse, expectedResponseValue interface{}) {
	var gotVal interface{}
	if response != nil {
		notifs := response.GetNotification()
		if len(notifs) != 1 {
			t.Fatalf("got %d notifications, want 1", len(notifs))
		}
		updates := notifs[0].GetUpdate()
		if len(updates) != 1 {
			t.Fatalf("got %d updates in the notification, want 1", len(updates))
		}
		val := updates[0].GetVal()
		if val.GetJsonIetfVal() == nil {
			gotVal, err := value.ToScalar(val)
			if err != nil {
				t.Errorf("got: %v, want a scalar value", gotVal)
			}
		} else {
			// Unmarshal json data to gotVal container for comparison.
			if err := json.Unmarshal(val.GetJsonIetfVal(), &gotVal); err != nil {
				t.Fatalf("error in unmarshaling IETF JSON data to json container: %v", err)
			}
			var expectedJSONStruct interface{}
			if err := json.Unmarshal(expectedResponseValue.([]byte), &expectedJSONStruct); err != nil {
				t.Fatalf("error in unmarshaling IETF JSON data to json container: %v", err)
			}
			expectedResponseValue = expectedJSONStruct
		}
	}

	if !reflect.DeepEqual(gotVal, expectedResponseValue) {
		t.Errorf("got: %v (%T),\nwant %v (%T)", gotVal, gotVal, expectedResponseValue, expectedResponseValue)
	}
}

func loadExpectedResponseByteData(t *testing.T, path string) interface{} {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatalf("read file %v err: %v", path, err)
	}
	return data
}

func TestVirtualDatabaseGNMIGet(t *testing.T) {
	// Open COUNTERS_DB redis client.
	countersDB := getRedisClient(t, "COUNTERS_DB")
	defer countersDB.Close()
	countersDB.FlushDB()

	// Enable keyspace notification.
	os.Setenv("PATH", "/usr/bin:/sbin:/bin:/usr/local/bin")
	cmd := exec.Command("redis-cli", "config", "set", "notify-keyspace-events", "KEA")
	_, err := cmd.Output()
	if err != nil {
		t.Fatal("failed to enable redis keyspace notification ", err)
	}

	// Load test data into COUNTERS_DB.
	loadTestDataIntoRedis(t, countersDB, "COUNTERS_PORT_NAME_MAP", "../testdata/COUNTERS_PORT_NAME_MAP.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS_QUEUE_NAME_MAP", "../testdata/COUNTERS_QUEUE_NAME_MAP.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1000000000039", "../testdata/COUNTERS:Ethernet68.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1000000000003", "../testdata/COUNTERS:Ethernet1.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000092a", "../testdata/COUNTERS:oid:0x1500000000092a.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000091c", "../testdata/COUNTERS:oid:0x1500000000091c.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000091e", "../testdata/COUNTERS:oid:0x1500000000091e.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000091f", "../testdata/COUNTERS:oid:0x1500000000091f.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000091f", "../testdata/COUNTERS:oid:0x1500000000091f.txt")

	// Load CONFIG_DB, flush old data, and load in test data.
	prepareConfigDB(t)

	// Start telementry service.
	gnmiServer := creategNMIServer(t)
	if gnmiServer == nil {
		t.Fatalf("Unable to bind gNMI server to local port 8080.")
	}
	go rungNMIServer(t, gnmiServer)
	defer gnmiServer.Stop()

	// Create a GNMI client used to invoke RPCs.
	tlsConfig := &tls.Config{InsecureSkipVerify: true}
	opts := []grpc.DialOption{grpc.WithTransportCredentials(credentials.NewTLS(tlsConfig))}

	targetAddr := "127.0.0.1:8080"
	conn, err := grpc.Dial(targetAddr, opts...)
	if err != nil {
		t.Fatalf("Dialing to %q failed: %v", targetAddr, err)
	}
	defer conn.Close()

	gnmiClient := gnmipb.NewGNMIClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Perform unit tests that closely resemble Jipan's original gnmi tests.
	t.Run("Get Interfaces/Port[name=Ethernet70], a valid path (with no corresponding data in the db); expected NotFound", func(t *testing.T) {
		expectedReturnCode := codes.NotFound
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet70]"
		sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
	})
	t.Run("Get Interfaces/Port[name=Ethernet400], invalid valid path; expected NotFound", func(t *testing.T) {
		expectedReturnCode := codes.NotFound
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet400]"
		sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
	})
	t.Run("Get Interfaces/Port[name=Ethernet68]/..., Everything under Ethernet68", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet68]/..."
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		assertExpectedValueFromMap(t, response, expectedValueEthernet68)
	})
	t.Run("Get Interfaces/Port[name=Ethernet68]/PfcCounter[field=SAI_PORT_STAT_PFC_7_RX_PKTS], valid path for specific leaf, but not implemented.", func(t *testing.T) {
		expectedReturnCode := codes.NotFound
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet68]/PfcCounter[field=SAI_PORT_STAT_PFC_7_RX_PKTS]"
		sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
	})
	t.Run("Get Interfaces/Port[name=Ethernet68]/Queue[name=*]/Pfcwd", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet68]/Queue[name=*]/Pfcwd"
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		assertExpectedValueFromMap(t, response, expectedValueEthernet68Pfcwd)
	})
	t.Run("Get Interfaces/Port[name=*]/PfcCounter[field=SAI_PORT_STAT_PFC_7_RX_PKTS], valid path for specific leaf for all nodes, but not implemented.", func(t *testing.T) {
		expectedReturnCode := codes.NotFound
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=*]/PfcCounter[field=SAI_PORT_STAT_PFC_7_RX_PKTS]"
		sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
	})
	t.Run("Get Interfaces/.../Pfcwd, valid path for specific PFC-related leaf for all nodes, but not implemented.", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/.../Pfcwd"
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		assertExpectedValueFromMap(t, response, expectedValueEthernet68Pfcwd)
	})

	// Perform some additional unit tests.
	t.Run("Get Interfaces/Port[name=Ethernet68/1]/BaseCounter", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet68/1]/BaseCounter"
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		expectedResponseValue := loadExpectedResponseByteData(t, "../testdata/Interfaces_Port_name_Ethernet68_1_BaseCounter.txt")
		assertExpectedValue(t, response, expectedResponseValue)
	})
	t.Run("Get Interfaces/Port[name=Ethernet68]/BaseCounter (no slash /1 after Ethernet68)", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet68]/BaseCounter"
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		expectedResponseValue := loadExpectedResponseByteData(t, "../testdata/Interfaces_Port_name_Ethernet68_1_BaseCounter.txt")
		assertExpectedValue(t, response, expectedResponseValue)
	})
	t.Run("Get Interfaces/Port[name=Ethernet68/1]/PfcCounter", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet68/1]/PfcCounter"
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		expectedResponseValue := loadExpectedResponseByteData(t, "../testdata/Interfaces_Port_name_Ethernet68_1_PfcCounter.txt")
		assertExpectedValue(t, response, expectedResponseValue)
	})
	t.Run("Get Interfaces/Port[name=Ethernet68/1]/Queue[name=Queue4]/Pfcwd", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet68/1]/Queue[name=Queue4]/Pfcwd"
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		expectedResponseValue := loadExpectedResponseByteData(t, "../testdata/Interfaces_Port_name_Ethernet68_1_Queue_name_Queue4_Pfcwd.txt")
		assertExpectedValue(t, response, expectedResponseValue)
	})
	t.Run("Get Interfaces/Port[name=Ethernet68/1]/Queue[name=Queue4]/QueueCounter", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet68/1]/Queue[name=Queue4]/QueueCounter"
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		expectedResponseValue := loadExpectedResponseByteData(t, "../testdata/Interfaces_Port_name_Ethernet68_1_Queue_name_Queue4_QueueCounter.txt")
		assertExpectedValue(t, response, expectedResponseValue)
	})
	t.Run("Get Interfaces/Port[name=Ethernet70], a valid path (with no corresponding data in the db); expected NotFound", func(t *testing.T) {
		expectedReturnCode := codes.NotFound
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=Ethernet70]"
		response := sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		var expectedResponseValue interface{} = nil
		assertExpectedValue(t, response, expectedResponseValue)
	})
	t.Run("Get Interfaces/Port[name=*]/..., Retrieve everything under all ports", func(t *testing.T) {
		expectedReturnCode := codes.OK
		pathToTargetDB := "SONiC_DB"
		xpath := "Interfaces/Port[name=*]/..."
		sendGetRequest(t, ctx, gnmiClient, xpath, pathToTargetDB, expectedReturnCode)
		// Only checking return code, since returned data will: A) require a large text file; B) change
		// whenever new kinds of test data is loaded into the DB (for example when modifying tests).
	})
}

func flushDBAndLoadTestData(t *testing.T, countersDB *redis.Client) {
	countersDB.FlushDB()

	loadTestDataIntoRedis(t, countersDB, "COUNTERS_PORT_NAME_MAP", "../testdata/COUNTERS_PORT_NAME_MAP.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS_QUEUE_NAME_MAP", "../testdata/COUNTERS_QUEUE_NAME_MAP.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1000000000039", "../testdata/COUNTERS:Ethernet68.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1000000000003", "../testdata/COUNTERS:Ethernet1.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000092a", "../testdata/COUNTERS:oid:0x1500000000092a.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000091c", "../testdata/COUNTERS:oid:0x1500000000091c.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000091e", "../testdata/COUNTERS:oid:0x1500000000091e.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000091f", "../testdata/COUNTERS:oid:0x1500000000091f.txt")
	loadTestDataIntoRedis(t, countersDB, "COUNTERS:oid:0x1500000000091f", "../testdata/COUNTERS:oid:0x1500000000091f.txt")

	prepareConfigDB(t)
}

func loadTestDataAsJSON(t *testing.T, testDataPath string) interface{} {
	data, err := ioutil.ReadFile(testDataPath)
	if err != nil {
		t.Fatalf("read file %v err: %v", testDataPath, err)
	}

	var dataJSON interface{}
	json.Unmarshal(data, &dataJSON)

	return dataJSON
}

func TestVirtualDatabaseGNMISubscribe(t *testing.T) {
	// Open COUNTERS_DB redis client.
	countersDB := getRedisClient(t, "COUNTERS_DB")
	defer countersDB.Close()

	// Enable keyspace notification.
	os.Setenv("PATH", "/usr/bin:/sbin:/bin:/usr/local/bin")
	cmd := exec.Command("redis-cli", "config", "set", "notify-keyspace-events", "KEA")
	_, err := cmd.Output()
	if err != nil {
		t.Fatal("failed to enable redis keyspace notification ", err)
	}

	// Start telementry service.
	gnmiServer := creategNMIServer(t)
	if gnmiServer == nil {
		t.Fatalf("Unable to bind gNMI server to local port 8080.")
	}
	go rungNMIServer(t, gnmiServer)
	defer gnmiServer.Stop()

	// One unit test.
	t.Run("Test description.", func(t *testing.T) {
		flushDBAndLoadTestData(t, countersDB)

		time.Sleep(time.Millisecond * 1000)

		c := client.New()
		defer c.Close()

		// Query
		var query client.Query
		query.Addrs = []string{"127.0.0.1:8080"}
		query.Target = "COUNTERS_DB"
		query.Type = client.Stream
		query.Queries = []client.Path{{"COUNTERS", "Ethernet68"}}
		query.TLS = &tls.Config{InsecureSkipVerify: true}

		logNotifications := false

		// Collate notifications with handler.
		var gotNotifications []client.Notification
		query.NotificationHandler = func(notification client.Notification) error {
			if logNotifications {
				t.Logf("reflect.TypeOf(notification) %v : %v", reflect.TypeOf(notification), notification)
			}

			if n, ok := notification.(client.Update); ok {
				n.TS = time.Unix(0, 200)
				gotNotifications = append(gotNotifications, n)
			} else {
				gotNotifications = append(gotNotifications, notification)
			}

			return nil
		}

		go c.Subscribe(context.Background(), query)
		defer c.Close()

		// Wait for subscription to sync.
		time.Sleep(time.Millisecond * 500)

		// Do updates.
		// rclient.HSet(update.tableName+update.delimitor+update.tableKey, update.field, update.value)
		countersDB.HSet("COUNTERS:oid:0x1000000000039", "test_field", "test_value")

		// Wait for updates to propogate notifications.
		time.Sleep(time.Millisecond * 1000)

		countersEthernet68 := loadTestDataAsJSON(t, "../testdata/COUNTERS:Ethernet68.txt")
		countersEthernet68Updated := loadTestDataAsJSON(t, "../testdata/COUNTERS:Ethernet68.txt")
		updateMap := countersEthernet68Updated.(map[string]interface{})
		updateMap["test_field"] = "test_value"

		// Expected notifications.
		expectedNotifications := []client.Notification{
			client.Connected{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68"}, TS: time.Unix(0, 200), Val: countersEthernet68},
			client.Sync{},
			client.Update{Path: []string{"COUNTERS", "Ethernet68"}, TS: time.Unix(0, 200), Val: countersEthernet68Updated},
		}

		// Compare results.
		if diff := pretty.Compare(expectedNotifications, gotNotifications); diff != "" {
			t.Log("\n Want: \n", expectedNotifications)
			t.Log("\n Got : \n", gotNotifications)
			t.Errorf("Unexpected updates:\n%s", diff)
		}
	})
}

func init() {
	// Inform gNMI server to use redis tcp localhost connection
	sdc.UseRedisLocalTcpPort = true
}
