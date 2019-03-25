package client

import (
	"fmt"
	"strings"

	log "github.com/golang/glog"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

// Return template path
func GetTmpl_PortBaseCounterStats(path *gnmipb.Path) {
	path.Elem = []*gnmipb.PathElem{}

	var name string
	name = "Interfaces"
	path.Elem = append(path.Elem, &gnmipb.PathElem{Name: name})

	name = "Port"
	path.Elem = append(path.Elem, &gnmipb.PathElem{
		Name: name,
		Key:  map[string]string{"name": "*"},
	})

	name = "BaseCounter"
	path.Elem = append(path.Elem, &gnmipb.PathElem{Name: name})
}

// gNMI paths are like
// [Interfaces Port[name=<port name> PfcCounter]
func v2rPortBaseCounterStats(path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath) error {
	var tmpl = gnmipb.Path{}
	GetTmpl_PortBaseCounterStats(&tmpl)

	parentConfig := map[int]string{1: "Port"}

	leaf := leafConfig{
		idx:  2,
		name: "BaseCounter",
	}

	target_fields := []string{}
	updatePath(path, &tmpl, parentConfig, leaf, &target_fields)

	// Populate tabelPaths
	err := pop_PortBaseCounterStats(&tmpl, pathG2S, target_fields)
	if err != nil {
		return err
	}
	return nil
}

// Populate redis key and fields
func pop_PortBaseCounterStats(path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath, target_fields []string) error {
	dbName := "COUNTERS_DB"
	separator, _ := GetTableKeySeparator(dbName)

	elems := path.GetElem()

	// Populate port level
	var idx_port = 1
	portName := elems[idx_port].GetKey()["name"]
	if portName == "*" {
		// Wildcard port name
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
			err := pop_PortBaseCounterStats(&copyPath, pathG2S, target_fields)
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
		log.V(1).Infof("RANDY: 3")
		return fmt.Errorf("%v not a valid sonic interface. Vendor alias is %v", _name, alias)
	}

	// TODO: Subscribe to only particular fields
	if len(target_fields) > 0 {
		return fmt.Errorf("Subscribe to field of Path: %v not supported", path)
	}
	tblPath_port := tablePath{
		dbName:    dbName,
		keyName:   strings.Join([]string{"COUNTERS", oid_port}, separator),
		delimitor: separator,
		patterns:  []string{"SAI_PORT_STAT_I.*", "SAI_PORT_STAT_ETHER.*"},
	}

	(*pathG2S)[path] = []tablePath{tblPath_port}
	return nil
}
