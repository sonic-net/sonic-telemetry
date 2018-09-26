package client

import (
	"fmt"
	"strings"

	log "github.com/golang/glog"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// Return template path
func GetTmpl_PortQueueCounterStats(path *gnmipb.Path) {
	path.Elem = []*gnmipb.PathElem{}

	var name string
	name = "Interfaces"
	path.Elem = append(path.Elem, &gnmipb.PathElem{Name: name})

	name = "Port"
	path.Elem = append(path.Elem, &gnmipb.PathElem{
		Name:	name,
		Key:	map[string]string{"name": "*"},
	})

	name = "Queue"
	path.Elem = append(path.Elem, &gnmipb.PathElem{
		Name:	name,
		Key:	map[string]string{"name": "*"},
	})

	name = "QueueCounter"
	path.Elem = append(path.Elem, &gnmipb.PathElem{Name: name})
}

// gNMI paths are like
// [Interfaces Port[name=<port name> Queue[name=<queue name>] QueueCounter]
func v2rPortQueueCounterStats(path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath) error {
	var tmpl = gnmipb.Path{}
	GetTmpl_PortQueueCounterStats(&tmpl)

	parentConfig := map[int]string{1: "Port", 2: "Queue"}

	leaf := leafConfig {
		idx:	3,
		name:	"QueueCounter",
	}

	target_fields := []string{}
	updatePath(path, &tmpl, parentConfig, leaf, &target_fields)

	// Populate tablePaths
	err := pop_PortQueueCounterStats(&tmpl, pathG2S, target_fields)
	if err != nil {
		return err
	}
	return nil
}

// Populate redis key and fields
func pop_PortQueueCounterStats(path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath, target_fields []string) error {
	dbName := "COUNTERS_DB"
	separator, _ := GetTableKeySeparator(dbName)

	elems := path.GetElem()

	// Populate port level
	var idx_port = 1
	portName := elems[idx_port].GetKey()["name"]
	if portName == "*" {
		// wildcard port name
		for port, _ := range countersPortNameMap {
			// Alias translation
			var oport string
			if alias, ok := name2aliasMap[port]; ok {
				oport = alias
			} else {
				log.V(2).Infof("%v does not have a vendor alias", port)
				oport = port
			}
			// Create a gNMI path for each port
			var copyPath = gnmipb.Path{}
			deepcopy(path, &copyPath)
			copyPath.Elem[idx_port].Key["name"] = oport
			err := pop_PortQueueCounterStats(&copyPath, pathG2S, target_fields)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Alias translation
	var alias, _name string
	alias = portName
	_name = alias
	if val, ok := alias2nameMap[alias]; ok {
		_name = val
	}

	_, ok := countersQueueNameMap[_name]
	if !ok {
		return fmt.Errorf("%v not a valid sonic interface. Vendor alias is %v", _name, alias)
	}

	// Populate queue level
	var idx_que = 2
	queName := elems[idx_que].GetKey()["name"]
	if queName == "*" {
		// wildcard queue name
		for que, _ := range countersQueueNameMap[_name] {
			// que is in format of "Ethernet68:12"
			stringSlice := strings.Split(que, separator)
			new_queName := "Queue" + stringSlice[1]
			// Create a gNMI path for each queue
			var copyPath = gnmipb.Path{}
			deepcopy(path, &copyPath)
			copyPath.Elem[idx_que].Key["name"] = new_queName
			err := pop_PortQueueCounterStats(&copyPath, pathG2S, target_fields)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Alias translation
	//stringSlice := strings.Split(queName, separator)
	if !strings.HasPrefix(queName, "Queue") {
		return fmt.Errorf("%v not a vaild queue name in request. Use format 'Queue<Num>'", queName)
	}
	queNum := strings.TrimPrefix(queName, "Queue")
	que := strings.Join([]string{_name, queNum}, separator)
	oid_que, ok := countersQueueNameMap[_name][que]
	if !ok {
		return fmt.Errorf("%v not a valid queue name in redis db.", que)
	}

	// TODO: subscribe to only particular fields
	if len(target_fields) > 0 {
		return fmt.Errorf("Subscribe to particular field of path %v not supported", path)
	}

	tblPath_que := tablePath{
		dbName:		dbName,
		keyName:	strings.Join([]string{"COUNTERS", oid_que}, separator),
		delimitor:	separator,
		fields:		[]string{
					"SAI_QUEUE_STAT_PACKETS",
					"SAI_QUEUE_STAT_BYTES",
					"SAI_QUEUE_STAT_DROPPED_PACKETS",
					"SAI_QUEUE_STAT_DROPPED_BYTES"},
	}
	(*pathG2S)[path] = []tablePath{tblPath_que}
	return nil
}

