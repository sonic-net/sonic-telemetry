package client

import (
	"fmt"
	"strings"

	log "github.com/golang/glog"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

func deepcopy(path, copyPath *gnmipb.Path) {
	copyPath.Elem = []*gnmipb.PathElem{}

	for _, elem := range path.GetElem() {
		var copyElem = gnmipb.PathElem{}
		copyElem.Name = elem.GetName()
		copyElem.Key = map[string]string{}
		for k, v := range elem.Key {
			copyElem.Key[k] = v
		}
		copyPath.Elem = append(copyPath.Elem, &copyElem)
	}
}

// Contains tell whether array contains X
func contains(a []string, x string) bool {
	for _, n := range a {
		if x == n {
			return true
		}
	}
	return false
}

type leafConfig struct {
	idx  int
	name string
}

func updatePath(path, tmpl *gnmipb.Path, parentConfig map[int]string, leaf leafConfig, target_fields *[]string) {
	// Update parent node
	for idx, name := range parentConfig {
		for _, elem := range path.GetElem() {
			if elem.GetName() == name {
				(*tmpl).Elem[idx].Key["name"] = elem.Key["name"]
				break
			}
		}
	}

	// Update fields: if subscribe to particular fields
	for _, elem := range path.GetElem() {
		if elem.GetName() == leaf.name {
			if fieldStr, ok := elem.GetKey()["field"]; ok {
				(*target_fields) = strings.Split(fieldStr, ",")
			}
		}
	}
}

// Return template path
func GetTmpl_PortQueuePfcwdStats(path *gnmipb.Path) {
	path.Elem = []*gnmipb.PathElem{}

	var name string
	name = "Interfaces"
	path.Elem = append(path.Elem, &gnmipb.PathElem{Name: name})

	name = "Port"
	path.Elem = append(path.Elem, &gnmipb.PathElem{
		Name: name,
		Key:  map[string]string{"name": "*"},
	})

	name = "Queue"
	path.Elem = append(path.Elem, &gnmipb.PathElem{
		Name: name,
		Key:  map[string]string{"name": "*"},
	})

	name = "Pfcwd"
	path.Elem = append(path.Elem, &gnmipb.PathElem{Name: name})
}

// Translate gNMI path into a list of unique root-to-leaf vpaths
// then map each unique vpath to a list of real DB tablePaths
// gNMI paths are like
// [Inerfaces Port[name=<port name>] Queue[name=<queue name>] Pfcwd]
func v2rPortQueuePfcwdStats(path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath) error {
	var tmpl = gnmipb.Path{}
	GetTmpl_PortQueuePfcwdStats(&tmpl)
	//fmt.Printf("tmpl: %v\n", &tmpl)

	parentConfig := map[int]string{1: "Port", 2: "Queue"}

	leaf := leafConfig{
		idx:  3,
		name: "Pfcwd",
	}

	targetFields := []string{}
	updatePath(path, &tmpl, parentConfig, leaf, &targetFields)

	// Populate tablePaths
	//fmt.Printf("path passed in populate: %v\n", &tmpl)
	err := pop_PortQueuePfcwdStats(&tmpl, pathG2S, targetFields)
	if err != nil {
		return err
	}
	return nil
}

// Populate
func pop_PortQueuePfcwdStats(path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath, target_fields []string) error {
	dbName := "COUNTERS_DB"
	separator, _ := GetTableKeySeparator(dbName)

	elems := path.GetElem()
	log.V(5).Infof("path: %v\n\n", path)

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
			err := pop_PortQueuePfcwdStats(&copyPath, pathG2S, target_fields)
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

	oid_port, ok := countersPortNameMap[_name]
	if !ok {
		return fmt.Errorf("%v not a valid sonic interface. Vendor alias is %v", _name, alias)
	}

	// Populate queue level
	var idx_que = 2
	queName := elems[idx_que].GetKey()["name"]
	if queName == "*" {
		// wildcard queue name
		for pfcque, _ := range countersPfcwdNameMap[_name] {
			// pfcque is in format of "Interface:12"
			// Alias translation
			stringSlice := strings.Split(pfcque, separator)
			//new_queName := strings.Join([]string{"Queue", stringSlice[1]}, separator)
			new_queName := "Queue" + stringSlice[1]
			// Create a gNMI path for each PFC WD enabled queue
			var copyPath = gnmipb.Path{}
			deepcopy(path, &copyPath)
			copyPath.Elem[idx_que].Key["name"] = new_queName
			err := pop_PortQueuePfcwdStats(&copyPath, pathG2S, target_fields)
			if err != nil {
				return err
			}
		}
		return nil
	}

	// Alias translation
	if !strings.HasPrefix(queName, "Queue") {
		return fmt.Errorf("%v not a vaild queue name. Use format 'Queue<Num>'", queName)
	}
	queNum := strings.TrimPrefix(queName, "Queue")
	pfcque := strings.Join([]string{_name, queNum}, separator)
	if _, ok := countersPfcwdNameMap[_name]; ok {
		if oid_que, ok := countersPfcwdNameMap[_name][pfcque]; ok {
			// PFC WD is enabled for port:queue
			out_tblPaths := []tablePath{}
			// Fields under the queue oid
			full_fields := []string{
				"PFC_WD_QUEUE_STATS_DEADLOCK_DETECTED",
				"PFC_WD_QUEUE_STATS_TX_DROPPED_PACKETS",
				"PFC_WD_QUEUE_STATS_RX_DROPPED_PACKETS",
				"PFC_WD_QUEUE_STATS_DEADLOCK_RESTORED",
				"PFC_wD_QUEUE_STATS_TX_PACKETS",
				"PFC_WD_QUEUE_STATS_RX_PACKETS",
				"PFC_WD_STATUS",
			}

			if len(target_fields) > 0 {
				// Subscirbe to only particular fields
				key_target_fields := []string{}
				for _, targetField := range target_fields {
					if contains(full_fields, targetField) {
						key_target_fields = append(key_target_fields, targetField)
					}
				}
				full_fields = key_target_fields
			}

			if len(full_fields) > 0 {
				tblPath_que := tablePath{
					dbName:    dbName,
					keyName:   strings.Join([]string{"COUNTERS", oid_que}, separator),
					delimitor: separator,
					fields:    full_fields,
				}
				out_tblPaths = append(out_tblPaths, tblPath_que)
			}

			// Fields under the port oid
			full_fields = []string{fmt.Sprintf("SAI_PORT_STAT_PFC_%v_RX_PKTS", queNum)}

			if len(target_fields) > 0 {
				// Subscirbe to only particular fields
				key_target_fields := []string{}
				for _, targetField := range target_fields {
					if contains(full_fields, targetField) {
						key_target_fields = append(key_target_fields, targetField)
					}
				}
				full_fields = key_target_fields
			}

			if len(full_fields) > 0 {
				tblPath_port := tablePath{
					dbName:    dbName,
					keyName:   strings.Join([]string{"COUNTERS", oid_port}, separator),
					delimitor: separator,
					fields:    full_fields,
				}
				out_tblPaths = append(out_tblPaths, tblPath_port)
			}

			//fmt.Printf("tablePath: %v\n", &tblPath_que)
			//fmt.Printf("tablePath: %v\n", &tblPath_port)
			if len(out_tblPaths) > 0 {
				(*pathG2S)[path] = out_tblPaths
			}
		}
	}
	return nil
}
