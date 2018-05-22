package client

import (
	"fmt"
	spb "github.com/Azure/sonic-telemetry/proto"
	log "github.com/golang/glog"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/workiva/go-datastructures/queue"
	"sync"
	"time"
)

// Non db client is to Handle
// <1> data not in SONiC redis db

type dataGetFunc func() ([]byte, error)

type path2DataFunc struct {
	path    []string
	getFunc dataGetFunc
}

var (
	clientTrie *Trie

	// path2DataFuncTbl is used to populate trie tree which is reponsible
	// for getting data at the path specified
	path2DataFuncTbl = []path2DataFunc{
		{ // Get system cpu usage
			path:    []string{"OTHERS", "system", "cpu"},
			getFunc: dataGetFunc(GetCpuUtil),
		},
		{ // Get system vmstat
			path:    []string{"OTHERS", "system", "memory"},
			getFunc: dataGetFunc(GetMemInfo),
		},
		{ // Get system disk stat
			path:    []string{"OTHERS", "system", "disk"},
			getFunc: dataGetFunc(GetDiskUsage),
		},
		{ // Get system version
			path:    []string{"OTHERS", "system", "version"},
			getFunc: dataGetFunc(GetVersion),
		},
		{ // Get system ntpstat
			path:    []string{"OTHERS", "system", "ntp"},
			getFunc: dataGetFunc(GetNtpStat),
		},
		{ // Get system shutdown reason
			path:    []string{"OTHERS", "system", "shutdown"},
			getFunc: dataGetFunc(GetShutdownReason),
		},
	}
)

func (t *Trie) clientTriePopulate() {
	for _, pt := range path2DataFuncTbl {
		n := t.Add(pt.path, pt.getFunc)
		if n.meta.(dataGetFunc) == nil {
			log.V(1).Infof("Failed to add trie node for %v with %v", pt.path, pt.getFunc)
		} else {
			log.V(2).Infof("Add trie node for %v with %v", pt.path, pt.getFunc)
		}
	}
}

func init() {
	clientTrie = NewTrie()
	clientTrie.clientTriePopulate()
}

type NonDbClient struct {
	prefix      *gnmipb.Path
	path2Getter map[*gnmipb.Path]dataGetFunc

	q       *queue.PriorityQueue
	channel chan struct{}

	synced sync.WaitGroup  // Control when to send gNMI sync_response
	w      *sync.WaitGroup // wait for all sub go routines to finish
	mu     sync.RWMutex    // Mutex for data protection among routines for DbClient

	sendMsg int64
	recvMsg int64
	errors  int64
}

func lookupGetFunc(prefix, path *gnmipb.Path) (dataGetFunc, error) {
	stringSlice := []string{prefix.GetTarget()}
	fullPath := gnmiFullPath(prefix, path)

	elems := fullPath.GetElem()
	if elems != nil {
		for i, elem := range elems {
			// TODO: Usage of key field
			log.V(6).Infof("index %d elem : %#v %#v", i, elem.GetName(), elem.GetKey())
			stringSlice = append(stringSlice, elem.GetName())
		}
	}
	n, ok := clientTrie.Find(stringSlice)
	if ok {
		getter := n.meta.(dataGetFunc)
		return getter, nil
	}
	return nil, fmt.Errorf("%v not found in clientTrie tree", stringSlice)
}

func NewNonDbClient(paths []*gnmipb.Path, prefix *gnmipb.Path) (Client, error) {
	var ndc NonDbClient

	ndc.path2Getter = make(map[*gnmipb.Path]dataGetFunc)
	ndc.prefix = prefix
	for _, path := range paths {
		getter, err := lookupGetFunc(prefix, path)
		if err != nil {
			return nil, err
		}
		ndc.path2Getter[path] = getter
	}

	return &ndc, nil
}

// String returns the target the client is querying.
func (c *NonDbClient) String() string {
	// TODO: print gnmiPaths of this NonDbClient
	return fmt.Sprintf("NonDbClient Prefix %v  sendMsg %v, recvMsg %v",
		c.prefix.GetTarget(), c.sendMsg, c.recvMsg)
}

// To be implemented
func (c *NonDbClient) StreamRun(q *queue.PriorityQueue, stop chan struct{}, w *sync.WaitGroup) {
	return
}

func (c *NonDbClient) PollRun(q *queue.PriorityQueue, poll chan struct{}, w *sync.WaitGroup) {
	c.w = w
	defer c.w.Done()
	c.q = q
	c.channel = poll

	for {
		_, more := <-c.channel
		if !more {
			log.V(1).Infof("%v poll channel closed, exiting pollDb routine", c)
			return
		}
		t1 := time.Now()
		for gnmiPath, getter := range c.path2Getter {
			v, err := getter()
			if err != nil {
				log.V(3).Infof("PollRun getter error %v for %v", err, v)
			}
			spbv := &spb.Value{
				Prefix:       c.prefix,
				Path:         gnmiPath,
				Timestamp:    time.Now().UnixNano(),
				SyncResponse: false,
				Val: &gnmipb.TypedValue{
					Value: &gnmipb.TypedValue_JsonIetfVal{
						JsonIetfVal: v,
					}},
			}

			c.q.Put(Value{spbv})
			log.V(6).Infof("Added spbv #%v", spbv)
		}

		c.q.Put(Value{
			&spb.Value{
				Timestamp:    time.Now().UnixNano(),
				SyncResponse: true,
			},
		})
		log.V(4).Infof("Sync done, poll time taken: %v ms", int64(time.Since(t1)/time.Millisecond))
	}
}

func (c *NonDbClient) Get(w *sync.WaitGroup) ([]*spb.Value, error) {
	// wait sync for Get, not used for now
	c.w = w

	var values []*spb.Value
	ts := time.Now()
	for gnmiPath, getter := range c.path2Getter {
		v, err := getter()
		if err != nil {
			log.V(3).Infof("PollRun getter error %v for %v", err, v)
		}
		values = append(values, &spb.Value{
			Prefix:    c.prefix,
			Path:      gnmiPath,
			Timestamp: ts.UnixNano(),
			Val: &gnmipb.TypedValue{
				Value: &gnmipb.TypedValue_JsonIetfVal{
					JsonIetfVal: v,
				}},
		})
	}
	log.V(6).Infof("Getting #%v", values)
	log.V(4).Infof("Get done, total time taken: %v ms", int64(time.Since(ts)/time.Millisecond))
	return values, nil
}

func (c *NonDbClient) Set(path *gnmipb.Path, val interface{}) error {
	return nil
}

// TODO: Log data related to this session
func (c *NonDbClient) Close() error {
	return nil
}
