package client

import (
	"fmt"
	spb "github.com/Azure/sonic-telemetry/proto"
	log "github.com/golang/glog"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

var (
	cfgPermit = [][]string{
		[]string{"CONFIG_DB", "TELEMETRY_CLIENT"},
		[]string{"CONFIG_DB", "VLAN"},
		[]string{"CONFIG_DB", "VLAN_MEMBER"},
		[]string{"CONFIG_DB", "VLAN_INTERFACE"},
		[]string{"CONFIG_DB", "BGP_NETWORK"},
		[]string{"CONFIG_DB", "PORT", "*", "admin_status"},
	}
)

type tablePath struct {
	dbName    string
	tableName string
	tableKey  string
	delimitor string
	fields    string
	// path name to be used in json data which may be different
	// from the real data path. Ex. in Counters table, real tableKey
	// is oid:0x####, while key name like Ethernet## may be put
	// in json data.
	jsonTableName string
	jsonTableKey  string
	jsonFields    string
}

type GSPath struct {
	gpath []string    // path string from gNMI path
	tpath []tablePath // table path for SONiC DB
}

// newGSPath construct new GSPath by gNMI path
func newGSPath(path *gnmipb.Path) (*GSPath, error) {
	elems := path.GetElem()
	if elems == nil {
		log.V(2).Infof("empty path: %v", elems)
		return nil, fmt.Errorf("empty path")
	}

	if len(path.GetTarget()) == 0 {
		return nil, fmt.Errorf("empty target")
	}
	gp := []string{path.GetTarget()}
	for _, elem := range elems {
		// TODO: Usage of key field
		gp = append(gp, elem.GetName())
	}
	log.V(6).Infof("path []string: %v", gp)

	return &GSPath{gpath: gp}, nil
}

// GetDbPath return tablePath to get DB data
func (p *GSPath) GetDbPath(allowNotFound bool) error {
	target := p.gpath[0]
	if !isValidDbTarget(target) {
		return fmt.Errorf("invaild db target: %v", target)
	}

	if target == "COUNTERS_DB" {
		tp, err := getv2rPath(p.gpath)
		if err == nil {
			p.tpath = tp
			return nil
		}
	}

	rp, err := getTblPath(p.gpath, allowNotFound)
	if err != nil {
		return err
	}
	p.tpath = append(p.tpath, rp)
	return nil
}

// GetCfgpath check if path permit, return DB path to set value
func (p *GSPath) GetCfgPath() error {
	target := p.gpath[0]
	if !isValidDbTarget(target) {
		return fmt.Errorf("invaild db target: %v", target)
	}

	if target != "CONFIG_DB" {
		return fmt.Errorf("config %s not supported", target)
	}

	// Check if path permit
	for _, s := range cfgPermit {
		if pathPermit(p.gpath, s) {
			rp, err := getTblPath(p.gpath, true)
			if err != nil {
				return err
			}
			p.tpath = append(p.tpath, rp)
			return nil
		}
	}

	return fmt.Errorf("config %s not supported", p.gpath)
}

func isValidDbTarget(t string) bool {
	_, ok := spb.Target_value[t]
	if t == "OTHERS" {
		return false
	}
	return ok
}

// getTblPath convert path string slice to real DB table path
func getTblPath(gp []string, allowNotFound bool) (tablePath, error) {
	// not support only DB
	if len(gp) < 2 {
		return tablePath{}, fmt.Errorf("not support")
	}

	target := gp[0]
	separator, _ := GetTableKeySeparator(target)
	tp := tablePath{
		dbName:    target,
		tableName: gp[1],
		delimitor: separator,
	}

	redisDb := Target2RedisDb[target]

	log.V(2).Infof("redisDb: %v tp: %v", redisDb, tp)
	// The expect real db path could be in one of the formats:
	//   DB Table
	//   DB Table Key
	//   DB Table Key Key
	//   DB Table Key Field
	//   DB Table Key Key Field
	var retError error
	switch len(gp) {
	case 2: // only table name provided
		res, err := redisDb.Keys(tp.tableName + "*").Result()
		if err != nil || len(res) < 1 {
			log.V(2).Infof("Invalid db table Path %v %v", target, gp)
			retError = fmt.Errorf("failed to find %v %v %v %v", target, gp, err, res)
		}
		tp.tableKey = ""
	case 3: // Third element could be table key
		_, err := redisDb.Exists(tp.tableName + tp.delimitor + gp[2]).Result()
		if err != nil {
			retError = fmt.Errorf("redis Exists op failed for %v", gp)
		}
		tp.tableKey = gp[2]
	case 4: // Fourth element could part of the table key or field name
		tp.tableKey = gp[2] + tp.delimitor + gp[3]
		// verify whether this key exists
		key := tp.tableName + tp.delimitor + tp.tableKey
		n, err := redisDb.Exists(key).Result()
		if err != nil {
			retError = fmt.Errorf("redis Exists op failed for %v", gp)
		} else if n != 1 { // Looks like the Fourth slice is not part of the key
			tp.tableKey = gp[2]
			tp.fields = gp[3]
		}
	case 5: // both third and fourth element are part of table key, fourth element must be field name
		tp.tableKey = gp[2] + tp.delimitor + gp[3]
		tp.fields = gp[4]
	default:
		log.V(2).Infof("Invalid db table Path %v", gp)
		retError = fmt.Errorf("invalid db table Path %v", gp)
	}

	if allowNotFound {
		if nil != retError {
			return tp, nil
		}
	} else {
		if nil != retError {
			return tablePath{}, retError
		}

		var key string
		if tp.tableKey != "" {
			key = tp.tableName + tp.delimitor + tp.tableKey
			n, _ := redisDb.Exists(key).Result()
			if n != 1 {
				log.V(2).Infof("No valid entry found on %v with key %v", gp, key)
				return tablePath{}, fmt.Errorf("no valid entry found on %v with key %v", gp, key)
			}
		}
	}
	log.V(6).Infof("get tablePath: %v", tp)

	return tp, nil
}

// pathPermit check if path is a subset of permit path
func pathPermit(a, permit []string) bool {
	if (a == nil) || (permit == nil) {
		return false
	}

	if len(a) < len(permit) {
		return false
	}

	b := a[:len(permit)]
	for i, v := range b {
		if permit[i] == "*" {
			continue
		}
		if v != permit[i] {
			return false
		}
	}

	return true
}
