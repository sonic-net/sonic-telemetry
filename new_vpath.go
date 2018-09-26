package client

import (
	"fmt"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)


func deepCopyPath(path, copyPath *gnmipb.Path) error {
	elems := path.GetElem()
	for _, elem := range elems {
		copyElem := gnmipb.PathElem{}
		copyElem.Name = elem.GetName()
		copyElem.Key = make(map[string]string)
		for k, v := range elem.GetKey() {
			copyElem.Key[k] = v
		}
		copyPath.Elem = append(copyPath.Elem, &copyElem)
	}
	return nil
}


// Populate real data paths from virtual paths like
//[Interfaces Port Queue Pfcwd]
func v2rPortQueuePfcwdStats(path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]newtablePath) error {
	fmt.Printf("path: %v\n", path)

	elems := path.GetElem()
	// If port name is wildcard
	idx_port := 1
	keys := elems[idx_port].GetKey()
	if portName, ok := keys["name"]; !ok{
		return fmt.Errorf("Missing port name in request %v", path)
	}

	if portName == "*" {
		// All Ethernet ports
		for port, _  := range countersPortNameMap {
			var oport string
			if alias, ok := name2aliasMap[port]; ok {
				oport = alias
			} else {
				log.V(2).Infof("%v does not have a vendor alias", port)
				oport = port
			}

			// Make a deep copy of the elems
			copyPath := gnmipb.Path{}
			deepCopyPath(path, &copyPath)
			copyElems := copyPath.GetElem()
			keys = copyElems[idx_port].GetKey()
			keys["name"] = oport
			err := v2rPortQueuePfcwdstats(copyPath, pathG2S)
			if err != nil {
				return err
			}
		}
	}
	

	// If queue is wildcard
	//idx_que := 2
	//keys = elems[idx_que].GetKey()
	//if queName, ok := keys["name"]; !ok{
	//	return nil, fmt.Errorf("Missing queue name in request %v", path)
	//}
}
