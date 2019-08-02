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
	"translib"
	"bytes"
	"encoding/json"
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
func enqueFatalMsgTranslib(c *TranslClient, msg string) {
	c.q.Put(Value{
		&spb.Value{
			Timestamp: time.Now().UnixNano(),
			Fatal:     msg,
		},
	})
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
	ticker_map := make(map[int]*ticker_info)
	var cases []reflect.SelectCase
	cases_map := make(map[int]int)
	var subscribe_mode gnmipb.SubscriptionMode
	stringPaths := make([]string, len(subscribe.Subscription))
	for i,sub := range subscribe.Subscription {
		stringPaths[i] = c.path2URI[sub.Path]
	}
	subSupport,_ := translib.IsSubscribeSupported(stringPaths)
	var onChangeSubsString []string
	var onChangeSubsgNMI []*gnmipb.Path
	onChangeMap := make(map[string]*gnmipb.Path)
	
	for i,sub := range subscribe.Subscription {
		fmt.Println(sub.Mode, sub.SampleInterval)
		switch sub.Mode {

		case gnmipb.SubscriptionMode_TARGET_DEFINED:

			if subSupport[i].IsSupported {
				if subSupport[i].PreferredType == translib.Sample {
					subscribe_mode = gnmipb.SubscriptionMode_SAMPLE
				} else if subSupport[i].PreferredType == translib.OnChange {
					subscribe_mode = gnmipb.SubscriptionMode_ON_CHANGE
				}
			} else {
				subscribe_mode = gnmipb.SubscriptionMode_SAMPLE
			}

		case gnmipb.SubscriptionMode_ON_CHANGE:
			if subSupport[i].IsSupported {	
				if (subSupport[i].MinInterval > 0) {
					subscribe_mode = gnmipb.SubscriptionMode_ON_CHANGE
				}else{
					enqueFatalMsgTranslib(c, fmt.Sprintf("Invalid subscribe path %v", stringPaths[i]))
					return
				}
			} else {
				enqueFatalMsgTranslib(c, fmt.Sprintf("ON_CHANGE Streaming mode invalid for %v", stringPaths[i]))
				return
			}
		case gnmipb.SubscriptionMode_SAMPLE:
			if (subSupport[i].MinInterval > 0) {
				subscribe_mode = gnmipb.SubscriptionMode_SAMPLE
			}else{
				enqueFatalMsgTranslib(c, fmt.Sprintf("Invalid subscribe path %v", stringPaths[i]))
				return
			}
		default:
			log.V(1).Infof("Bad Subscription Mode for client %s ", c)
			enqueFatalMsgTranslib(c, fmt.Sprintf("Invalid Subscription Mode %d", sub.Mode))
			return
		}
		fmt.Println("subscribe_mode:", subscribe_mode)
		fmt.Println("min int:", subSupport[i].MinInterval*int(time.Second))
		if subscribe_mode == gnmipb.SubscriptionMode_SAMPLE {
			interval := int(sub.SampleInterval)
			if interval == 0 {
				interval = subSupport[i].MinInterval * int(time.Second)
				fmt.Println("OK", interval, subSupport[i].MinInterval*int(time.Second))
			} else {
				if interval < (subSupport[i].MinInterval*int(time.Second)) {
					enqueFatalMsgTranslib(c, fmt.Sprintf("Invalid Sample Interval %ds, minimum interval is %ds", interval/int(time.Second), subSupport[i].MinInterval))
					return
				}
			}
			//Reuse ticker for same sample intervals, otherwise create a new one.
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

			onChangeSubsString = append(onChangeSubsString, c.path2URI[sub.Path])
			onChangeSubsgNMI = append(onChangeSubsgNMI, sub.Path)
			onChangeMap[c.path2URI[sub.Path]] = sub.Path
		}
	}
	
	if len(onChangeSubsString) > 0 {
		c.w.Add(1)
		// c.synced.Add(1)
		// go translib.Subscribe(onChangeSubs, q, stop)
		go TranslSubscribe(onChangeSubsgNMI, onChangeSubsString, onChangeMap, c)
		fmt.Println("OnChange:", onChangeSubsString)
		fmt.Println("OnChange:", onChangeSubsgNMI)

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

func TranslSubscribe(gnmiPaths []*gnmipb.Path, stringPaths []string, pathMap map[string]*gnmipb.Path, c *TranslClient) {
	defer c.w.Done()
	q := queue.NewPriorityQueue(1, false)
	fmt.Println(q)
	fmt.Println("HERE A")
	resp, err := translib.Subscribe(stringPaths, q, c.channel)
	fmt.Println("HERE B")
	for {
		items, err := q.Get(1)
		if err != nil {
			log.V(1).Infof("%v", err)
			return
		}
		switch v := items[0].(type) {
		case *translib.SubscribeResponse:
			var jv []byte
			dst := new(bytes.Buffer)
			json.Compact(dst, v.Payload)
			jv = dst.Bytes()


			/* Fill the values into GNMI data structures . */
			val := &gnmipb.TypedValue{
				Value: &gnmipb.TypedValue_JsonIetfVal{
				JsonIetfVal: jv,
				}}

			spbv := &spb.Value{
				Prefix:       c.prefix,
				Path:         pathMap[v.Path],
				Timestamp:    v.Timestamp,
				SyncResponse: false,
				Val:          val,
			}
			c.q.Put(Value{spbv})
			log.V(6).Infof("Added spbv #%v", spbv)
			
			if v.SyncComplete {
				c.q.Put(Value{
					&spb.Value{
						Timestamp:    time.Now().UnixNano(),
						SyncResponse: true,
					},
				})
			}
		default:
			log.V(1).Infof("Unknown data type %v for %s in queue", items[0], c)
			fmt.Println(v)
		}
		
	}
	fmt.Println(resp)
	fmt.Println(err)
	
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
