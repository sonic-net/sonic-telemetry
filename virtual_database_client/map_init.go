package client

import (
	"fmt"
	"strings"

	log "github.com/golang/glog"
)

var (
	// Port name to oid map in COUNTERS table of COUNTERS_DB
	countersPortNameMap = make(map[string]string)

	// Queue name to oid map in COUNTERS table of COUNTERS_DB
	countersQueueNameMap = make(map[string]map[string]string)

	// Alias translation: from vendor port name to sonic interface name
	alias2nameMap = make(map[string]string)
	// Alias translation: from sonic interface name to vendor port name
	name2aliasMap = make(map[string]string)

	// SONiC interface name to their PFC-WD enabled queues, then to oid map
	countersPfcwdNameMap = make(map[string]map[string]string)
)

func initCountersQueueNameMap() error {
	dbName := "COUNTERS_DB"
	separator, _ := GetTableKeySeparator(dbName)

	if len(countersQueueNameMap) == 0 {
		queueMap, err := getCountersMap("COUNTERS_QUEUE_NAME_MAP")
		if err != nil {
			return err
		}
		for k, v := range queueMap {
			stringSlice := strings.Split(k, separator)
			port := stringSlice[0]
			if _, ok := countersQueueNameMap[port]; !ok {
				countersQueueNameMap[port] = make(map[string]string)
			}
			countersQueueNameMap[port][k] = v
		}
	}
	return nil
}

func initCountersPortNameMap() error {
	var err error
	if len(countersPortNameMap) == 0 {
		countersPortNameMap, err = getCountersMap("COUNTERS_PORT_NAME_MAP")
		if err != nil {
			return err
		}
	}
	return nil
}

func initAliasMap() error {
	var err error
	if len(alias2nameMap) == 0 {
		alias2nameMap, name2aliasMap, err = getAliasMap()
		if err != nil {
			return err
		}
	}
	return nil
}

func initCountersPfcwdNameMap() error {
	var err error
	if len(countersPfcwdNameMap) == 0 {
		countersPfcwdNameMap, err = getPfcwdMap()
		if err != nil {
			return err
		}
	}
	return nil
}

// Get the mapping between sonic interface name and oids of their PFC-WD enabled queues in COUNTERS_DB
func getPfcwdMap() (map[string]map[string]string, error) {
	var pfcwdName_map = make(map[string]map[string]string)

	dbName := "CONFIG_DB"
	separator, _ := GetTableKeySeparator(dbName)
	redisDb, _ := Target2RedisDb[dbName]
	_, err := redisDb.Ping().Result()
	if err != nil {
		log.V(1).Infof("Can not connect to %v, err: %v", dbName, err)
		return nil, err
	}

	keyName := fmt.Sprintf("PFC_WD_TABLE%v*", separator)
	resp, err := redisDb.Keys(keyName).Result()
	if err != nil {
		log.V(1).Infof("redis get keys failed for %v, key = %v, err: %v", dbName, keyName, err)
		return nil, err
	}

	if len(resp) == 0 {
		// PFC WD service not enabled on device
		log.V(1).Infof("PFC WD not enabled on device")
		return nil, nil
	}

	for _, key := range resp {
		name := key[13:]
		pfcwdName_map[name] = make(map[string]string)
	}

	// Get Queue indexes that are enabled with PFC-WD
	keyName = "PORT_QOS_MAP*"
	resp, err = redisDb.Keys(keyName).Result()
	if err != nil {
		log.V(1).Infof("redis get keys failed for %v, key = %v, err: %v", dbName, keyName, err)
		return nil, err
	}
	if len(resp) == 0 {
		log.V(1).Infof("PFC WD not enabled on device")
		return nil, nil
	}
	qos_key := resp[0]

	fieldName := "pfc_enable"
	priorities, err := redisDb.HGet(qos_key, fieldName).Result()
	if err != nil {
		log.V(1).Infof("redis get field failed for %v, key = %v, field = %v, err: %v", dbName, qos_key, fieldName, err)
		return nil, err
	}

	keyName = fmt.Sprintf("MAP_PFC_PRIORITY_TO_QUEUE%vAZURE", separator)
	pfc_queue_map, err := redisDb.HGetAll(keyName).Result()
	if err != nil {
		log.V(1).Infof("redis get fields failed for %v, key = %v, err: %v", dbName, keyName, err)
		return nil, err
	}

	var indices []string
	for _, p := range strings.Split(priorities, ",") {
		_, ok := pfc_queue_map[p]
		if !ok {
			log.V(1).Infof("Missing mapping between PFC priority %v to queue", p)
		} else {
			indices = append(indices, pfc_queue_map[p])
		}
	}

	if len(countersQueueNameMap) == 0 {
		log.V(1).Infof("COUNTERS_QUEUE_NAME_MAP is empty")
		return nil, nil
	}

	var queue_key string
	queue_separator, _ := GetTableKeySeparator("COUNTERS_DB")
	for port, _ := range pfcwdName_map {
		for _, indice := range indices {
			queue_key = port + queue_separator + indice
			oid, ok := countersQueueNameMap[port][queue_key]
			if !ok {
				return nil, fmt.Errorf("key %v not exists in COUNTERS_QUEUE_NAME_MAP", queue_key)
			}
			pfcwdName_map[port][queue_key] = oid
		}
	}

	log.V(6).Infof("countersPfcwdNameMap: %v", pfcwdName_map)
	return pfcwdName_map, nil
}

// Get the mapping between sonic interface name and vendor alias
func getAliasMap() (map[string]string, map[string]string, error) {
	var alias2name_map = make(map[string]string)
	var name2alias_map = make(map[string]string)

	dbName := "CONFIG_DB"
	separator, _ := GetTableKeySeparator(dbName)
	redisDb, _ := Target2RedisDb[dbName]
	_, err := redisDb.Ping().Result()
	if err != nil {
		log.V(1).Infof("Can not connect to %v, err: %v", dbName, err)
		return nil, nil, err
	}

	keyName := fmt.Sprintf("PORT%v*", separator)
	resp, err := redisDb.Keys(keyName).Result()
	if err != nil {
		log.V(1).Infof("redis get keys failed for %v, key = %v, err: %v", dbName, keyName, err)
		return nil, nil, err
	}
	for _, key := range resp {
		alias, err := redisDb.HGet(key, "alias").Result()
		if err != nil {
			log.V(1).Infof("redis get field alias failed for %v, key = %v, err: %v", dbName, key, err)
			// clear aliasMap
			alias2name_map = make(map[string]string)
			name2alias_map = make(map[string]string)
			return nil, nil, err
		}
		alias2name_map[alias] = key[5:]
		name2alias_map[key[5:]] = alias
	}
	log.V(6).Infof("alias2nameMap: %v", alias2name_map)
	log.V(6).Infof("name2aliasMap: %v", name2alias_map)
	return alias2name_map, name2alias_map, nil
}

// Get the mapping between objects in counters DB, Ex. port name to oid in "COUNTERS_PORT_NAME_MAP" table.
// Aussuming static port name to oid map in COUNTERS table
func getCountersMap(tableName string) (map[string]string, error) {
	redisDb, _ := Target2RedisDb["COUNTERS_DB"]
	fv, err := redisDb.HGetAll(tableName).Result()
	if err != nil {
		log.V(2).Infof("redis HGetAll failed for COUNTERS_DB, tableName: %s", tableName)
		return nil, err
	}
	log.V(6).Infof("tableName: %s, map %v", tableName, fv)
	return fv, nil
}
