package client

import (
	"fmt"
	log "github.com/golang/glog"
	"strings"
)

// virtual db is to Handle
// <1> different set of redis db data aggreggation
// <2> or non default TARGET_DEFINED stream subscription

// For virtual db path
const (
	DbIdx    uint = iota // DB name is the first element (no. 0) in path slice.
	TblIdx               // Table name is the second element (no. 1) in path slice.
	KeyIdx               // Key name is the first element (no. 2) in path slice.
	FieldIdx             // Field name is the first element (no. 3) in path slice.
)

type v2rTranslate func([]string) ([]tablePath, error)

type pathTransFunc struct {
	path      []string
	transFunc v2rTranslate
}

var (
	v2rTrie *Trie

	supportedCounterFields = map[string][]string{}

	countersNameOidTbls = map[string]map[string]string{
		// Port name to oid map in COUNTERS table of COUNTERS_DB
		"COUNTERS_PORT_NAME_MAP": make(map[string]string),
		// Queue name to oid map in COUNTERS table of COUNTERS_DB
		"COUNTERS_QUEUE_NAME_MAP": make(map[string]string),
		// PG name to oid map in COUNTERS table of COUNTERS_DB
		"COUNTERS_INGRESS_PRIORITY_GROUP_NAME_MAP": make(map[string]string),
		// Buffer pool name to oid map in COUNTERS table of COUNTERS_DB
		"COUNTERS_BUFFER_POOL_NAME_MAP": make(map[string]string),
		// Cput Trap Group to oid map in COUNTERS table of COUNTERS_DB
		"COUNTERS_TRAPGROUP_POLICER_MAP": make(map[string]string),
	}

	// path2TFuncTbl is used to populate trie tree which is reponsible
	// for virtual path to real data path translation
	pathTransFuncTbl = []pathTransFunc{
		{ // stats for one or all Ethernet ports
			path:      []string{"COUNTERS_DB", "COUNTERS", "Ethernet*"},
			transFunc: v2rTranslate(v2rEthPortStats),
		}, { // specific field stats for one or all Ethernet ports
			path:      []string{"COUNTERS_DB", "COUNTERS", "Ethernet*", "*"},
			transFunc: v2rTranslate(v2rEthPortStats),
		}, { // queue stats for one or all Ethernet ports
			path:      []string{"COUNTERS_DB", "COUNTERS", "Ethernet*", "Queues"},
			transFunc: v2rTranslate(v2rEthPortQueStats),
		}, { // specific queue stats for one or all Ethernet ports
			path:      []string{"COUNTERS_DB", "COUNTERS", "Ethernet*", "Queues", "*"},
			transFunc: v2rTranslate(v2rEthPortQueStats),
		},
	}
)

func (t *Trie) v2rTriePopulate() {
	for _, pt := range pathTransFuncTbl {
		n := t.Add(pt.path, pt.transFunc)
		if n.meta.(v2rTranslate) == nil {
			log.V(1).Infof("Failed to add trie node for %v with %v", pt.path, pt.transFunc)
		} else {
			log.V(2).Infof("Add trie node for %v with %v", pt.path, pt.transFunc)
		}

	}
}

func initCountersNameMap() error {
	var err error
	for name, tbl := range countersNameOidTbls {
		if len(tbl) == 0 {
			countersNameOidTbls[name], err = getCountersMap(name)
			if err != nil {
				return err
			}
			supportedCounterFields[name], err = getSupportedFields(name)
			if err != nil {
				return err
			}
		}
	}
	return nil
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

// Get suppored fields for counters
func getSupportedFields(tableName string) ([]string, error) {
	redisDb, _ := Target2RedisDb["COUNTERS_DB"]
	for _, oid := range countersNameOidTbls[tableName] {
		statKey := "COUNTERS:" + oid
		keys, err := redisDb.HKeys(statKey).Result()
		if err != nil {
			log.V(2).Infof("redis HKeys failed for COUNTERS_DB, tableName: %s", tableName)
			return nil, err
		}
		if len(keys) <= 0 {
			log.V(2).Infof("supported fields empty, tableName: %s", tableName)
		}
		log.V(6).Infof("supported fields: %v", keys)
		return keys, nil
	}
	return nil, nil
}

// Get match fields from supported fields
func getMatchFields(field string, name string) (string, error) {
	// the field is prefixed with "SAI"
	if !strings.HasPrefix(field, "SAI") {
		return "", nil
	}
	// don't lookup field without wildcard
	if !strings.HasSuffix(field, "*") {
		return field, nil
	}

	var fs string
	matchFields := []string{}
	supportedFields := supportedCounterFields[name]

	if supportedFields != nil {
		fieldPrefix := strings.TrimSuffix(field, "*")
		for _, v := range supportedFields {
			if strings.HasPrefix(v, fieldPrefix) {
				matchFields = append(matchFields, v)
			}
		}

		if len(matchFields) <= 0 {
			return "", fmt.Errorf("%v has no match fields", field)
		}
		// multiple fields are separated with ","
		for _, f := range matchFields {
			if fs != "" {
				fs = fs + "," + f
			} else {
				fs = f
			}
		}
	}

	return fs, nil
}

// Supported cases:
// <1> port name paths without field
//     Ex. [COUNTER_DB COUNTERS Ethernet*]
//         [COUNTER_DB COUNTERS Ethernet68]
// <2> port name having suffix of "*" with specific field;
//     Ex. [COUNTER_DB COUNTERS Ethernet* SAI_PORT_STAT_PFC_0_RX_PKTS]
//         [COUNTER_DB COUNTERS Ethernet* SAI_PORT_STAT_PFC_*]
// <3> exact port name with specific field.
//     Ex. [COUNTER_DB COUNTERS Ethernet68 SAI_PORT_STAT_PFC_0_RX_PKTS]
//         [COUNTER_DB COUNTERS Ethernet68 SAI_PORT_STAT_PFC_*]
func v2rEthPortStats(paths []string) ([]tablePath, error) {
	separator, _ := GetTableKeySeparator(paths[DbIdx])
	countersNameMap := countersNameOidTbls["COUNTERS_PORT_NAME_MAP"]
	var tblPaths []tablePath
	var fields string

	fields, err := getMatchFields(paths[len(paths)-1], "COUNTERS_PORT_NAME_MAP")
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(paths[KeyIdx], "*") { //all ports
		for port, oid := range countersNameMap {
			tblPath := tablePath{
				dbName:       paths[DbIdx],
				tableName:    paths[TblIdx],
				tableKey:     oid,
				fields:       fields,
				delimitor:    separator,
				jsonTableKey: port,
				jsonFields:   fields,
			}
			tblPaths = append(tblPaths, tblPath)
		}
	} else { //single port
		oid, ok := countersNameMap[paths[KeyIdx]]
		if !ok {
			return nil, fmt.Errorf(" %v not a valid port ", paths[KeyIdx])
		}

		// When fields is a single field, jsonFields isn't filled up
		jf := ""
		if strings.Contains(fields, ",") {
			jf = fields
		}
		tblPaths = []tablePath{{
			dbName:     paths[DbIdx],
			tableName:  paths[TblIdx],
			tableKey:   oid,
			fields:     fields,
			delimitor:  separator,
			jsonFields: jf,
		}}
	}
	log.V(6).Infof("v2rEthPortStats: tblPaths %+v", tblPaths)
	return tblPaths, nil
}

func v2rCpuStats(paths []string) ([]tablePath, error) {
	separator, _ := GetTableKeySeparator(paths[DbIdx])
	countersNameMap := countersNameOidTbls["COUNTERS_TRAPGROUP_POLICER_MAP"]
	var tblPaths []tablePath
	var fields string

	fields, err := getMatchFields(paths[len(paths)-1], "COUNTERS_TRAPGROUP_POLICER_MAP")

	if err != nil {
		return nil, err
	}

	for trapname, oid := range countersNameMap {
		tblPath := tablePath{
			dbName:       paths[DbIdx],
			tableName:    paths[TblIdx],
			tableKey:     oid,
			fields:       fields,
			delimitor:    separator,
			jsonTableKey: trapname,
			jsonFields:   fields,
		}
		tblPaths = append(tblPaths, tblPath)
	}

	log.V(6).Infof("v2rCpuStats: tblPaths %+v", tblPaths)
	return tblPaths, nil
}

func v2rQStatsGeneric(paths []string, mapTblName string) ([]tablePath, error) {
	separator, _ := GetTableKeySeparator(paths[DbIdx])
	countersNameMap := countersNameOidTbls[mapTblName]
	var tblPaths []tablePath
	var fields string

	fields, err := getMatchFields(paths[len(paths)-1], mapTblName)
	if err != nil {
		return nil, err
	}

	if strings.HasSuffix(paths[KeyIdx], "*") { //all ports
		for q, oid := range countersNameMap {
			tblPath := tablePath{
				dbName:       paths[DbIdx],
				tableName:    paths[TblIdx],
				tableKey:     oid,
				fields:       fields,
				delimitor:    separator,
				jsonTableKey: q,
				jsonFields:   fields,
			}
			tblPaths = append(tblPaths, tblPath)
		}
	} else { //single port
		portName := paths[KeyIdx]
		for q, oid := range countersNameMap {
			//que is in formate of "Ethernet64:12"
			names := strings.Split(q, separator)
			if portName != names[0] {
				continue
			}

			tblPath := tablePath{
				dbName:       paths[DbIdx],
				tableName:    paths[TblIdx],
				tableKey:     oid,
				fields:       fields,
				delimitor:    separator,
				jsonTableKey: q,
				jsonFields:   fields,
			}
			tblPaths = append(tblPaths, tblPath)
		}
	}
	log.V(6).Infof("v2rGenericStats: mapTblName %s tblPaths %+v", mapTblName, tblPaths)
	return tblPaths, nil
}

func v2rEthPortQueStats(paths []string) ([]tablePath, error) {
	tblPaths, err := v2rQStatsGeneric(paths, "COUNTERS_QUEUE_NAME_MAP")
	return tblPaths, err
}

func getv2rPath(paths []string) ([]tablePath, error) {
	n, ok := v2rTrie.Find(paths)
	if ok {
		v2rTrans := n.meta.(v2rTranslate)
		return v2rTrans(paths)
	}
	return nil, fmt.Errorf("%v not found in virtual path tree", paths)
}

func init() {
	v2rTrie = NewTrie()
	v2rTrie.v2rTriePopulate()
}
