package client

import (
	"fmt"
	"strconv"

	"encoding/json"
	"net"
	"regexp"
	"time"

	spb "github.com/Azure/sonic-telemetry/proto"
	"github.com/go-redis/redis"
	log "github.com/golang/glog"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// gnmiFullPath builds the full path from the prefix and path.
func gnmiFullPath(prefix, path *gnmipb.Path) *gnmipb.Path {

	fullPath := &gnmipb.Path{Origin: path.Origin}
	if path.GetElement() != nil {
		fullPath.Element = append(prefix.GetElement(), path.GetElement()...)
	}
	if path.GetElem() != nil {
		fullPath.Elem = append(prefix.GetElem(), path.GetElem()...)
	}
	return fullPath
}

func populateAlltablePaths(prefix *gnmipb.Path, paths []*gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath) error {
	for _, path := range paths {
		err := populateNewtablePath(prefix, path, pathG2S)
		if err != nil {
			return err
		}
	}
	return nil
}

// First translate gNMI paths to a list of unique
// root-to-leaf virtual paths in the vpath tree.
// Then map each vpath to a list of redis DB path.
func populateNewtablePath(prefix, path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath) error {
	target := prefix.GetTarget()
	stringSlice := []string{target}
	elems := path.GetElem()
	for _, elem := range elems {
		stringSlice = append(stringSlice, elem.GetName())
	}

	err := searchPathTrie(stringSlice, path, pathG2S)
	if err != nil {
		return err
	}

	return nil
}

func msi2TypedValue(msi map[string]interface{}) (*gnmipb.TypedValue, error) {
	jv, err := emitJSON(&msi)
	if err != nil {
		log.V(2).Infof("emitJSON err %s for  %v", err, msi)
		return nil, fmt.Errorf("emitJSON err %s for  %v", err, msi)
	}
	return &gnmipb.TypedValue{
		Value: &gnmipb.TypedValue_JsonIetfVal{
			JsonIetfVal: jv,
		}}, nil
}

// emitJSON marshalls map[string]interface{} to JSON byte stream.
func emitJSON(v *map[string]interface{}) ([]byte, error) {
	//j, err := json.MarshalIndent(*v, "", indentString)
	j, err := json.Marshal(*v)
	if err != nil {
		return nil, fmt.Errorf("JSON marshalling error: %v", err)
	}

	return j, nil
}

// Render the redis DB data to map[string]interface{}
// which may be marshaled to JSON format
// tablePath includes [dbName, keyName, fields]
func tableData2TypedValue(tblPaths []tablePath) (*gnmipb.TypedValue, error) {
	msi := make(map[string]interface{})

	for _, tblPath := range tblPaths {
		err := tableData2Msi(&tblPath, &msi)
		if err != nil {
			return nil, err
		}
	}
	return msi2TypedValue(msi)
}

func tableData2Msi(tblPath *tablePath, msi *map[string]interface{}) error {
	dbName := tblPath.dbName
	keyName := tblPath.keyName

	redisDb := Target2RedisDb[dbName]
	val, err := redisDb.HGetAll(keyName).Result()
	if err != nil {
		log.V(3).Infof("redis HGetAll failed for %v, dbName = %v, keyName=%v", tblPath, dbName, keyName)
		return err
	}

	patterns := tblPath.patterns
	for field, value := range val {
		for _, pattern := range patterns {
			r := regexp.MustCompile(pattern)
			if r.MatchString(field) {
				(*msi)[field] = value
				break
			}
		}
	}

	fields := tblPath.fields
	for _, field := range fields {
		if value, ok := val[field]; !ok {
			log.V(1).Infof("Missing field: %v", field)
		} else {
			(*msi)[field] = value
		}
	}
	return nil
}

func enqueFatalMsg(c *DbClient, msg string) {
	c.q.Put(Value{
		&spb.Value{
			Timestamp: time.Now().UnixNano(),
			Fatal:     msg,
		},
	})
}

type redisSubData struct {
	tblPath tablePath
	pubsub  *redis.PubSub
}

func dbSingleTableKeySubscribe(rsd redisSubData, c *DbClient, msiInit *map[string]interface{}, msiOut *map[string]interface{}) {
	tblPath := rsd.tblPath
	pubsub := rsd.pubsub
	msi := make(map[string]interface{})
	// Initialize msi
	for k, v := range *msiInit {
		msi[k] = v
	}

	for {
		select {
		default:
			msgi, err := pubsub.ReceiveTimeout(time.Millisecond * 500)
			if err != nil {
				neterr, ok := err.(net.Error)
				if ok {
					if neterr.Timeout() == true {
						continue
					}
				}
				log.V(2).Infof("pubsub.ReceiveTimeout err %v", err)
				continue
			}
			newMsi := make(map[string]interface{})
			subscr := msgi.(*redis.Message)

			if subscr.Payload == "hset" || subscr.Payload == "hsetnx" || subscr.Payload == "hmset" {
				err = tableData2Msi(&tblPath, &newMsi)
				if err != nil {
					enqueFatalMsg(c, err.Error())
					return
				}

				c.mu.Lock()
				for k, v := range newMsi {
					_, ok := msi[k]
					if !ok {
						(*msiOut)[k] = v
						msi[k] = v
					} else {
						if v != msi[k] {
							(*msiOut)[k] = v
							msi[k] = v
						}
					}
				}
				c.mu.Unlock()
			}
		case <-c.channel:
			log.V(2).Infof("Stopping dbSingleTableKeySubscribe routine for %+v", tblPath)
			return
		}
	}
}

// Subscribe to a specific gNMI path
func dbPathSubscribe(gnmiPath *gnmipb.Path, tblPaths []tablePath, c *DbClient) {
	//tblPaths := c.pathG2S[gnmiPath]
	msi := make(map[string]interface{})

	for _, tblPath := range tblPaths {
		err := tableData2Msi(&tblPath, &msi)
		if err != nil {
			enqueFatalMsg(c, err.Error())
			return
		}
	}
	val, err := msi2TypedValue(msi)
	if err != nil {
		enqueFatalMsg(c, err.Error())
		return
	}

	var spbv *spb.Value
	spbv = &spb.Value{
		Prefix:    c.prefix,
		Path:      gnmiPath,
		Timestamp: time.Now().UnixNano(),
		Val:       val,
	}
	if err = c.q.Put(Value{spbv}); err != nil {
		log.V(1).Infof("Queue error: %v", err)
		return
	}

	// First sync for this key is done
	c.synced.Done()

	msiInit := make(map[string]interface{})
	for k, v := range msi {
		msiInit[k] = v
	}
	for k := range msi {
		delete(msi, k)
	}

	// Redis pubsub to monitor table keys
	for _, tblPath := range tblPaths {
		dbName := tblPath.dbName
		keyName := tblPath.keyName
		redisDb := Target2RedisDb[dbName]

		// Subscribe to keyspace notification
		pattern := "__keyspace@" + strconv.Itoa(int(spb.Target_value[dbName])) + "__:"
		pattern += keyName
		pubsub := redisDb.PSubscribe(pattern)
		defer pubsub.Close()

		msgi, err := pubsub.ReceiveTimeout(time.Second)
		if err != nil {
			log.V(1).Infof("psubscribe to %s failed for %v", pattern, tblPath)
			enqueFatalMsg(c, fmt.Sprintf("psubscribe to %s failed for %v", pattern, tblPath))
			return
		}

		subscr := msgi.(*redis.Subscription)
		if subscr.Channel != pattern {
			log.V(1).Infof("psubscribe to %s failed for %v", pattern, tblPath)
			enqueFatalMsg(c, fmt.Sprintf("psubscribe to %s failed for %v", pattern, tblPath))
			return
		}
		log.V(2).Infof("Psubscribe succeeded for %v: %v", tblPath, subscr)

		rsd := redisSubData{
			tblPath: tblPath,
			pubsub:  pubsub,
		}
		go dbSingleTableKeySubscribe(rsd, c, &msiInit, &msi)
	}

	for {
		select {
		default:
			val = nil
			err = nil
			c.mu.Lock()
			if len(msi) > 0 {
				val, err = msi2TypedValue(msi)
				for k := range msi {
					delete(msi, k)
				}
			}
			c.mu.Unlock()

			if err != nil {
				enqueFatalMsg(c, err.Error())
				return
			}

			if val != nil {
				spbv = &spb.Value{
					Path:      gnmiPath,
					Timestamp: time.Now().UnixNano(),
					Val:       val,
				}

				log.V(5).Infof("dbTableKeySubscribe enque: %v", spbv)
				if err = c.q.Put(Value{spbv}); err != nil {
					log.V(1).Infof("Queue error: %v", err)
					return
				}
			}

			// check possible value change every 100 millisecond
			// TODO: make all the instances of wait timer consistent
			time.Sleep(time.Millisecond * 100)
		case <-c.channel:
			log.V(1).Infof("Stopping dbTableKeySubscribe routine for %v ", gnmiPath)
			return
		}
	}
}
