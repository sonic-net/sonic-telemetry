package client

import (
//"fmt"
)

type PathNode struct {
	val      string
	depth    int
	term     bool
	children map[string]*PathNode
	parent   *PathNode
	meta     interface{}
}

type PathTrie struct {
	root *PathNode
	size int
}

func NewPathTrie() *PathTrie {
	return &PathTrie{
		root: &PathNode{children: make(map[string]*PathNode), depth: 0},
		size: 0,
	}
}

// Adds an entry to the Trie, including meta data. Meta data
// is stored as 'interface{}' and must be type cast by the caller
func (t *PathTrie) Add(keys []string, meta interface{}) *PathNode {
	node := t.root
	for _, key := range keys {
		if _, ok := node.children[key]; !ok {
			// A new PathNode
			newNode := PathNode{
				val:      key,
				depth:    node.depth + 1,
				term:     false,
				children: make(map[string]*PathNode),
				parent:   node,
				meta:     meta,
			}
			node.children[key] = &newNode
		}
		node = node.children[key]
	}
	node.meta = meta
	node.term = true
	return node
}

// Returns the parent of this node
func (n PathNode) Parent() *PathNode {
	return n.parent
}

func (n PathNode) Meta() interface{} {
	return n.meta
}

func (n PathNode) Terminating() bool {
	return n.term
}

func (n PathNode) Val() string {
	return n.val
}

func (n PathNode) Children() map[string]*PathNode {
	return n.children
}

func findPathNode(node *PathNode, keys []string, result *[]*PathNode) {
	// gNMI path convention:
	// '...' is multi-level wildcard, and '*' is single-level wildcard

	if len(keys) == 0 {
		if node.Terminating() == true {
			*result = append(*result, node)
		}
		return
	}

	key := keys[0]
	if node.Terminating() == true {
		// Leaf node
		if (len(keys) == 1) && (key == "...") {
			*result = append(*result, node)
		}
		return
	}

	children := node.Children()
	if key == "..." {
		if len(keys) > 1 {
			if child, ok := children[keys[1]]; ok {
				findPathNode(child, keys[2:], result)
			}
		}
		for _, child := range children {
			findPathNode(child, keys, result)
		}
	} else if key == "*" {
		for _, child := range children {
			findPathNode(child, keys[1:], result)
		}
	} else {
		if child, ok := children[key]; ok {
			findPathNode(child, keys[1:], result)
		}
	}
	return
}
