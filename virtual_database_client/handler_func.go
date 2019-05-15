package client

import (
	//"fmt"
	log "github.com/golang/glog"
	gnmipb "github.com/openconfig/gnmi/proto/gnmi"
)

type handlerFunc func(*gnmipb.Path, *map[*gnmipb.Path][]tablePath) error

type pathHdlrFunc struct {
	path    []string
	handler handlerFunc
}

var (
	pathTrie *PathTrie

	// path2HdlrFuncTbl is used to populate trie tree which is used to
	// map gNMI path to real database paths.
	path2HdlrFuncTbl = []pathHdlrFunc{
		{
			// new virtual path for PFC WD stats
			path:    []string{"SONiC_DB", "Interfaces", "Port", "Queue", "Pfcwd"},
			handler: handlerFunc(v2rPortQueuePfcwdStats),
		}, {
			// new virtual path for Queue counters
			path:    []string{"SONiC_DB", "Interfaces", "Port", "Queue", "QueueCounter"},
			handler: handlerFunc(v2rPortQueueCounterStats),
		}, {
			// new virtual path for Port PFC counters
			path:    []string{"SONiC_DB", "Interfaces", "Port", "PfcCounter"},
			handler: handlerFunc(v2rPortPfcCounterStats),
		}, {
			// new virtual path for Port Base Counters
			path:    []string{"SONiC_DB", "Interfaces", "Port", "BaseCounter"},
			handler: handlerFunc(v2rPortBaseCounterStats),
		},
	}
)

func (t *PathTrie) TriePopulate() {
	for _, pf := range path2HdlrFuncTbl {
		n := t.Add(pf.path, pf.handler)
		if n.meta.(handlerFunc) == nil {
			log.V(1).Infof("Failed to add trie node for path %v with handler func %v", pf.path, pf.handler)
		} else {
			log.V(2).Infof("Add trie node for path %v with handler func %v", pf.path, pf.handler)
		}
	}
}

func searchPathTrie(keys []string, path *gnmipb.Path, pathG2S *map[*gnmipb.Path][]tablePath) error {
	var nodes = []*PathNode{}
	root := pathTrie.root
	findPathNode(root, keys, &nodes)

	for _, node := range nodes {
		handler := node.meta.(handlerFunc)
		err := handler(path, pathG2S)
		if err != nil {
			return err
		}
	}
	return nil
}

func init() {
	pathTrie = NewPathTrie()
	pathTrie.TriePopulate()
}
