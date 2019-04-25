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

	gnmi "github.com/Azure/sonic-telemetry/gnmi_server"

	xpath "github.com/jipanyang/gnxi/utils/xpath"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	value "github.com/openconfig/gnmi/value"

	spb "github.com/Azure/sonic-telemetry/proto"
	testcert "github.com/Azure/sonic-telemetry/testdata/tls"
	redis "github.com/go-redis/redis"
	"google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	status "google.golang.org/grpc/status"
)

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

	fmt.Printf("got: %v (%T),\nwant %v (%T)\n", gotVal, gotVal, expectedResponseValue, expectedResponseValue)
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

func TestVirtualPathClient(t *testing.T) {
	// Open COUNTERS_DB redis client.
	countersDB := getRedisClient(t, "COUNTERS_DB")
	defer countersDB.Close()
	countersDB.FlushDB()

	// Enable keyspace notification.
	// Note (@ragaul):
	//   Don't know exactly why this is needed -- likely for testing pub/sub (need investigate +
	//   confirmation). Copied from gnmi_server/server_test.go
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

	// Perform unit tests.
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
	t.Run("Get Interfaces/Port[name=Ethernet68/1]/Queue[name=Queue4]/Pfcwd", func(t *testing.T) {
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

func init() {
	// Inform gNMI server to use redis tcp localhost connection
	sdc.UseRedisLocalTcpPort = true
}
