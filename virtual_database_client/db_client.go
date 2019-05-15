// Data client for new Virtual Path

package client

import (
	"fmt"
	"sync"
	"time"

	log "github.com/golang/glog"

	spb "github.com/Azure/sonic-telemetry/proto"
	"github.com/go-redis/redis"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/workiva/go-datastructures/queue"
)

const (
	// indentString represents the default indentation string used for JSON.
	// Two spaces are used here.
	indentString                 string = "  "
	Default_REDIS_UNIXSOCKET     string = "/var/run/redis/redis.sock"
	Default_REDIS_LOCAL_TCP_PORT string = "localhost:6379"
)

// Client defines a set of methods which every client must implement.
// This package provides one implmentation for now: the DbClient
//
type Client interface {
	// StreamRun will start watching service on data source
	// and enqueue data change to the priority queue.
	// It stops all activities upon receiving signal on stop channel
	// It should run as a go routine
	StreamRun(q *queue.PriorityQueue, stop chan struct{}, w *sync.WaitGroup)
	// Poll will  start service to respond poll signal received on poll channel.
	// data read from data source will be enqueued on to the priority queue
	// The service will stop upon detection of poll channel closing.
	// It should run as a go routine
	PollRun(q *queue.PriorityQueue, poll chan struct{}, w *sync.WaitGroup)
	// Get return data from the data source in format of *spb.Value
	Get(w *sync.WaitGroup) ([]*spb.Value, error)
	// Close provides implemenation for explicit cleanup of Client
	Close() error
}

var (
	// Let it be variable visible to other packages for now.
	// May add an interface function for it.
	UseRedisLocalTcpPort bool = false

	// Redis client connected to each DB
	Target2RedisDb = make(map[string]*redis.Client)
)

type tablePath struct {
	dbName    string
	keyName   string
	delimitor string
	fields    []string // fields listed in list are returned
	patterns  []string // fields matched with patterns are returned
}

type Value struct {
	*spb.Value
}

// Implement Compare method for priority queue
func (val Value) Compare(other queue.Item) int {
	oval := other.(Value)
	if val.GetTimestamp() > oval.GetTimestamp() {
		return 1
	} else if val.GetTimestamp() == oval.GetTimestamp() {
		return 0
	}
	return -1
}

type DbClient struct {
	prefix *gnmipb.Path
	// Used by Get server
	paths []*gnmipb.Path
	//pathG2S map[*gnmipb.Path][]tablePath

	q       *queue.PriorityQueue
	channel chan struct{}

	synced sync.WaitGroup  // Control when to send gNMI sync_response
	w      *sync.WaitGroup // wait for all sub go routines to finish
	mu     sync.RWMutex    // Mutex for data protection among routines for DbClient

	sendMsg int64
	recvMsg int64
	errors  int64
}

func NewDbClient(paths []*gnmipb.Path, prefix *gnmipb.Path) (Client, error) {
	var client DbClient
	var err error
	// Testing program may ask to use redis local tcp connection
	if UseRedisLocalTcpPort {
		useRedisTcpClient()
	}

	err = initCountersPortNameMap()
	if err != nil {
		return nil, err
	}
	err = initCountersQueueNameMap()
	if err != nil {
		return nil, err
	}
	err = initAliasMap()
	if err != nil {
		return nil, err
	}
	err = initCountersPfcwdNameMap()
	if err != nil {
		return nil, err
	}

	client.prefix = prefix
	client.paths = paths
	return &client, nil
}

func (c *DbClient) StreamRun(q *queue.PriorityQueue, stop chan struct{}, w *sync.WaitGroup) {
	c.w = w
	defer c.w.Done()
	c.q = q
	c.channel = stop

	pathG2S := make(map[*gnmipb.Path][]tablePath)
	err := populateAlltablePaths(c.prefix, c.paths, &pathG2S)
	if err != nil {
		enqueFatalMsg(c, err.Error())
		return
	}

	if len(pathG2S) == 0 {
		enqueFatalMsg(c, fmt.Sprintf("Prefix:%v, path: %v not valid paths", c.prefix, c.paths))
		return
	}

	// Assume all ON_CHANGE mode
	for gnmiPath, tblPaths := range pathG2S {
		c.w.Add(1)
		c.synced.Add(1)
		go dbPathSubscribe(gnmiPath, tblPaths, c)
	}

	// Wait until all data values corresponding to the paths specified
	// in the SubscriptionList has been transmitted at least once
	c.synced.Wait()
	// Inject sync message
	c.q.Put(Value{
		&spb.Value{
			Timestamp:    time.Now().UnixNano(),
			SyncResponse: true,
		},
	})
	log.V(2).Infof("%v Synced", pathG2S)
	return
}

func (c *DbClient) PollRun(q *queue.PriorityQueue, poll chan struct{}, w *sync.WaitGroup) {
	return
}

func (c *DbClient) Get(w *sync.WaitGroup) ([]*spb.Value, error) {
	// wait sync for Get, not used for now
	c.w = w

	pathG2S := make(map[*gnmipb.Path][]tablePath)
	err := populateAlltablePaths(c.prefix, c.paths, &pathG2S)
	if err != nil {
		return nil, err
	}

	if len(pathG2S) == 0 {
		return nil, fmt.Errorf("Failed to map to real db paths. Prefix: %v, paths: %v not valid paths", c.prefix, c.paths)
	}

	var values []*spb.Value
	ts := time.Now()
	for gnmiPath, tblPaths := range pathG2S {
		val, err := tableData2TypedValue(tblPaths)
		if err != nil {
			return nil, err
		}

		values = append(values, &spb.Value{
			Prefix:    c.prefix,
			Path:      gnmiPath,
			Timestamp: ts.UnixNano(),
			Val:       val,
		})
	}
	log.V(5).Infof("Getting #%v", values)
	log.V(4).Infof("Get done, total time taken: %v ms", int64(time.Since(ts)/time.Millisecond))
	return values, nil
}

// TODO: Log data related to this session
func (c *DbClient) Close() error {
	return nil
}

func GetTableKeySeparator(target string) (string, error) {
	_, ok := spb.Target_value[target]
	if !ok {
		log.V(1).Infof(" %v not a valid path target", target)
		return "", fmt.Errorf("%v not a valid path target", target)
	}

	var separator string
	switch target {
	case "CONFIG_DB":
		separator = "|"
	case "STATE_DB":
		separator = "|"
	default:
		separator = ":"
	}
	return separator, nil
}

// For testing only
func useRedisTcpClient() {
	for dbName, dbn := range spb.Target_value {
		if dbName != "OTHERS" {
			// DB connector for direct redis operation
			var redisDb *redis.Client
			if UseRedisLocalTcpPort {
				redisDb = redis.NewClient(&redis.Options{
					Network:     "tcp",
					Addr:        Default_REDIS_LOCAL_TCP_PORT,
					Password:    "", // no password set
					DB:          int(dbn),
					DialTimeout: 0,
				})
			}
			Target2RedisDb[dbName] = redisDb
		}
	}
}

// Client package prepare redis clients to all DBs automatically
func init() {
	for dbName, dbn := range spb.Target_value {
		if dbName != "OTHERS" {
			// DB connector for direct redis operation
			var redisDb *redis.Client

			redisDb = redis.NewClient(&redis.Options{
				Network:     "unix",
				Addr:        Default_REDIS_UNIXSOCKET,
				Password:    "", // no password set
				DB:          int(dbn),
				DialTimeout: 0,
			})
			Target2RedisDb[dbName] = redisDb
		}
	}
}

// Convert from SONiC Value to its corresponding gNMI proto stream
// response type
func ValToResp(val Value) (*gnmipb.SubscribeResponse, error) {
	switch val.GetSyncResponse() {
	case true:
		return &gnmipb.SubscribeResponse{
			Response: &gnmipb.SubscribeResponse_SyncResponse{
				SyncResponse: true,
			},
		}, nil
	default:
		// In case the subscribe/poll routines encountered fatal error
		if fatal := val.GetFatal(); fatal != "" {
			return nil, fmt.Errorf("%s", fatal)
		}

		return &gnmipb.SubscribeResponse{
			Response: &gnmipb.SubscribeResponse_Update{
				Update: &gnmipb.Notification{
					Timestamp: val.GetTimestamp(),
					Prefix:    val.GetPrefix(),
					Update: []*gnmipb.Update{
						{
							Path: val.GetPath(),
							Val:  val.GetVal(),
						},
					},
				},
			},
		}, nil
	}
}
