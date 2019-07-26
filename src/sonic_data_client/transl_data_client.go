//  Package client provides a generic access layer for data available in system
package client

import (
	spb "proto"
	transutil "transl_utils"
	log "github.com/golang/glog"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
	"github.com/Workiva/go-datastructures/queue"
	"sync"
	"time"
	"fmt"
	"reflect"
)

const (
	DELETE  int = 0
	REPLACE int = 1
	UPDATE  int = 2
)

type TranslClient struct {
	prefix *gnmipb.Path
	/* GNMI Path to REST URL Mapping */
	path2URI map[*gnmipb.Path]string
	channel  chan struct{}
	q        *queue.PriorityQueue

	synced sync.WaitGroup  // Control when to send gNMI sync_response
	w      *sync.WaitGroup // wait for all sub go routines to finish
	mu     sync.RWMutex    // Mutex for data protection among routines for transl_client

}

func NewTranslClient(prefix *gnmipb.Path, getpaths []*gnmipb.Path) (Client, error) {
	var client TranslClient
	var err error

	client.prefix = prefix
	if getpaths != nil {
		client.path2URI = make(map[*gnmipb.Path]string)
		/* Populate GNMI path to REST URL map. */
		err = transutil.PopulateClientPaths(prefix, getpaths, &client.path2URI)
	}

	if err != nil {
		return nil, err
	} else {
		return &client, nil
	}
}

func (c *TranslClient) Get(w *sync.WaitGroup) ([]*spb.Value, error) {

	var values []*spb.Value
	ts := time.Now()

	/* Iterate through all GNMI paths. */
	for gnmiPath, URIPath := range c.path2URI {
		/* Fill values for each GNMI path. */
		val, err := transutil.TranslProcessGet(URIPath, nil)

		if err != nil {
			return nil, err
		}

		/* Value of each path is added to spb value structure. */
		values = append(values, &spb.Value{
			Prefix:    c.prefix,
			Path:      gnmiPath,
			Timestamp: ts.UnixNano(),
			Val:       val,
		})
	}

	/* The values structure at the end is returned and then updates in notitications as
	specified in the proto file in the server.go */

	log.V(6).Infof("TranslClient : Getting #%v", values)
	log.V(4).Infof("TranslClient :Get done, total time taken: %v ms", int64(time.Since(ts)/time.Millisecond))

	return values, nil
}

func (c *TranslClient) Set(path *gnmipb.Path, val *gnmipb.TypedValue, flagop int) error {
	var uri string
	var err error

	/* Convert the GNMI Path to URI. */
	transutil.ConvertToURI(c.prefix, path, &uri)

	if flagop == DELETE {
		err = transutil.TranslProcessDelete(uri)
	} else if flagop == REPLACE {
		err = transutil.TranslProcessReplace(uri, val)
	} else if flagop == UPDATE {
		err = transutil.TranslProcessUpdate(uri, val)
	}

	return err
}

func (c *TranslClient) StreamRun(q *queue.PriorityQueue, stop chan struct{}, w *sync.WaitGroup, subscribe *gnmipb.SubscriptionList) {
	c.w = w
	defer c.w.Done()
	c.q = q
	c.channel = stop

	type ticker_info struct{
		t     *time.Ticker
		uris  []string
		paths []*gnmipb.Path
	}
	ticker_map := make(map[uint64]*ticker_info)
	var cases []reflect.SelectCase
	cases_map := make(map[int]uint64)
	var subscribe_mode gnmipb.SubscriptionMode

	for _,sub := range subscribe.Subscription {
		fmt.Println(sub.Mode, sub.SampleInterval)
		switch sub.Mode {

		case gnmipb.SubscriptionMode_TARGET_DEFINED:
			// Until we get event subscription mode discovery from translib api, default to sample mode.
			//Future API:
			//if !IsSubscribeSupported(c.path2URI[sub.Path]) {
			//	subscribe_mode = gnmipb.SubscriptionMode_SAMPLE
			//} else {
			//	subscribe_mode = gnmipb.SubscriptionMode_ON_CHANGE
			//}

			subscribe_mode = gnmipb.SubscriptionMode_SAMPLE

		case gnmipb.SubscriptionMode_ON_CHANGE:
			subscribe_mode = gnmipb.SubscriptionMode_ON_CHANGE
		case gnmipb.SubscriptionMode_SAMPLE:
			subscribe_mode = gnmipb.SubscriptionMode_SAMPLE
		default:
			log.V(1).Infof("Bad Subscription Mode for client %s ", c)
			continue
		}

		if subscribe_mode == gnmipb.SubscriptionMode_SAMPLE {
			interval := sub.SampleInterval
			if interval == 0 {
				//For now set default interval to 5 seoncds until we get API to discover minimum interval per path.
				interval = 5*1e9

			} else {
				interval = sub.SampleInterval
			}
			if ticker_map[interval] == nil {
				ticker_map[interval] = &ticker_info {
					t: time.NewTicker(time.Duration(interval) * time.Nanosecond),
					paths: []*gnmipb.Path{sub.Path},
					uris: []string{c.path2URI[sub.Path]},
				}
				cases_map[len(cases)] = interval
				cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ticker_map[interval].t.C)})

			} else {
				ticker_map[interval].paths = append(ticker_map[interval].paths, sub.Path)
				ticker_map[interval].uris = append(ticker_map[interval].uris, c.path2URI[sub.Path])
			}
		} else if subscribe_mode == gnmipb.SubscriptionMode_ON_CHANGE {

			//Dont support ON_CHANGE for now, until translib API support for subscribe is available
			//Future API:
			//c.w.Add(1)
			//c.synced.Add(1)
			//go Subscribe(c.path2URI[sub.Path], q, stop)
			return
		}

	}
	cases = append(cases, reflect.SelectCase{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(c.channel)})

	for {
		chosen, ttm, ok := reflect.Select(cases)


		if !ok {
			fmt.Println("Done")
			return
		}
		fmt.Println("tick", ttm, time.Now(), cases_map[chosen], ticker_map[cases_map[chosen]].uris)

		for ii, uri := range ticker_map[cases_map[chosen]].uris {
			val, err := transutil.TranslProcessGet(uri, nil)
			if err != nil {
				return
			}

			spbv := &spb.Value{
				Prefix:       c.prefix,
				Path:         ticker_map[cases_map[chosen]].paths[ii],
				Timestamp:    time.Now().UnixNano(),
				SyncResponse: false,
				Val:          val,
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
	}
}


func (c *TranslClient) PollRun(q *queue.PriorityQueue, poll chan struct{}, w *sync.WaitGroup) {
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
		for gnmiPath, URIPath := range c.path2URI {
			val, err := transutil.TranslProcessGet(URIPath, nil)
			if err != nil {
				return
			}

			spbv := &spb.Value{
				Prefix:       c.prefix,
				Path:         gnmiPath,
				Timestamp:    time.Now().UnixNano(),
				SyncResponse: false,
				Val:          val,
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
func (c *TranslClient) OnceRun(q *queue.PriorityQueue, once chan struct{}, w *sync.WaitGroup) {
	c.w = w
	defer c.w.Done()
	c.q = q
	c.channel = once

	
	_, more := <-c.channel
	if !more {
		log.V(1).Infof("%v once channel closed, exiting onceDb routine", c)
		return
	}
	t1 := time.Now()
	for gnmiPath, URIPath := range c.path2URI {
		val, err := transutil.TranslProcessGet(URIPath, nil)
		if err != nil {
			return
		}

		spbv := &spb.Value{
			Prefix:       c.prefix,
			Path:         gnmiPath,
			Timestamp:    time.Now().UnixNano(),
			SyncResponse: false,
			Val:          val,
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
	log.V(4).Infof("Sync done, once time taken: %v ms", int64(time.Since(t1)/time.Millisecond))
	
}

func (c *TranslClient) Capabilities() []gnmipb.ModelData {

	/* Fetch the supported models. */
	supportedModels := transutil.GetModels()
	return supportedModels
}

func (c *TranslClient) Close() error {
	return nil
}
