// Package releasetree provides an N-ary tree for managing software release
// versions and computing change diffs via LCA (Least Common Ancestor).
// Vendored from github.com/gaozhe8848/lca/releaseTree.
package releasetree

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"sync"
)

// Chg represents a single change item.
type Chg struct {
	ID string
}

// ReleaseInput represents the raw data for a release node.
type ReleaseInput struct {
	Ver     string
	FromVer string
	Changes []Chg
}

// node represents a node in the N-ary release tree.
type node struct {
	version  string
	changes  []Chg
	parent   *node
	children []*node
}

// ReleaseTree holds the entire tree structure.
type ReleaseTree struct {
	nodes map[string]*node
	root  *node
	mu    sync.RWMutex
}

// NewReleaseTree builds the n-ary tree from a slice of input release data.
func NewReleaseTree(inputs []ReleaseInput) (*ReleaseTree, error) {
	tree := &ReleaseTree{
		nodes: make(map[string]*node),
	}

	// Pass 1: Create nodes
	for _, input := range inputs {
		if _, exists := tree.nodes[input.Ver]; exists {
			return nil, fmt.Errorf("NewReleaseTree: duplicate version detected: %s", input.Ver)
		}
		changesCopy := make([]Chg, len(input.Changes))
		copy(changesCopy, input.Changes)
		newNode := &node{
			version:  input.Ver,
			changes:  changesCopy,
			children: []*node{},
		}
		tree.nodes[input.Ver] = newNode
	}

	// Pass 2: Link nodes
	foundRoots := 0
	for _, input := range inputs {
		newNode := tree.nodes[input.Ver]
		if input.FromVer == "" {
			if tree.root == nil {
				tree.root = newNode
			}
			foundRoots++
			continue
		}
		parent, exists := tree.nodes[input.FromVer]
		if !exists {
			return nil, fmt.Errorf("NewReleaseTree: parent version '%s' for node '%s' not found in input data", input.FromVer, input.Ver)
		}
		newNode.parent = parent
		parent.children = append(parent.children, newNode)
	}

	if len(inputs) > 0 && tree.root == nil {
		if foundRoots == 0 {
			return nil, errors.New("NewReleaseTree: no root node detected (no node has empty FromVer)")
		}
		return nil, errors.New("NewReleaseTree: tree construction failed, root node is nil despite inputs existing")
	}
	return tree, nil
}

// InsertNode adds a single new release node to the tree concurrently safely.
func (tree *ReleaseTree) InsertNode(input ReleaseInput) error {
	tree.mu.Lock()
	defer tree.mu.Unlock()

	if _, exists := tree.nodes[input.Ver]; exists {
		return fmt.Errorf("InsertNode: node with version '%s' already exists", input.Ver)
	}

	var parent *node
	if input.FromVer == "" {
		if tree.root != nil {
			return fmt.Errorf("InsertNode: cannot insert node '%s' with empty FromVer; tree already has a root ('%s')", input.Ver, tree.root.version)
		}
	} else {
		p, exists := tree.nodes[input.FromVer]
		if !exists {
			return fmt.Errorf("InsertNode: parent version '%s' for node '%s' not found", input.FromVer, input.Ver)
		}
		parent = p
	}

	changesCopy := make([]Chg, len(input.Changes))
	copy(changesCopy, input.Changes)
	newNode := &node{
		version:  input.Ver,
		changes:  changesCopy,
		children: []*node{},
		parent:   parent,
	}

	tree.nodes[newNode.version] = newNode

	if parent != nil {
		parent.children = append(parent.children, newNode)
	} else {
		tree.root = newNode
	}
	return nil
}

// findLCA is the internal implementation without locking.
func (tree *ReleaseTree) findLCA(version1, version2 string) (*node, error) {
	node1, exists1 := tree.nodes[version1]
	if !exists1 {
		return nil, fmt.Errorf("findLCA internal: version '%s' not found in tree", version1)
	}
	node2, exists2 := tree.nodes[version2]
	if !exists2 {
		return nil, fmt.Errorf("findLCA internal: version '%s' not found in tree", version2)
	}

	if node1 == node2 {
		return node1, nil
	}

	ancestors := make(map[*node]bool)
	curr := node1
	for curr != nil {
		ancestors[curr] = true
		curr = curr.parent
	}
	curr = node2
	for curr != nil {
		if ancestors[curr] {
			return curr, nil
		}
		curr = curr.parent
	}
	return nil, fmt.Errorf("findLCA internal: logic error: no common ancestor for '%s' and '%s'", version1, version2)
}

// FindLCA finds the version string of the LCA concurrently safely.
func (tree *ReleaseTree) FindLCA(version1, version2 string) (string, error) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()

	lcaNode, err := tree.findLCA(version1, version2)
	if err != nil {
		return "", fmt.Errorf("FindLCA: failed for versions '%s' and '%s': %w", version1, version2, err)
	}
	return lcaNode.version, nil
}

// CalcChgs calculates the net changes concurrently safely.
func (tree *ReleaseTree) CalcChgs(endVersion, startVersion string) ([]Chg, error) {
	tree.mu.RLock()
	defer tree.mu.RUnlock()

	lcaNode, err := tree.findLCA(endVersion, startVersion)
	if err != nil {
		return nil, fmt.Errorf("CalcChgs: failed to find LCA for '%s' and '%s': %w", endVersion, startVersion, err)
	}

	endNode := tree.nodes[endVersion]
	startNode := tree.nodes[startVersion]

	netChanges := make(map[string]Chg)

	// Accumulate End Path Changes
	curr := endNode
	for curr != nil && curr != lcaNode {
		for _, change := range curr.changes {
			netChanges[change.ID] = change
		}
		curr = curr.parent
	}

	// Subtract Start Path Changes (with Subset Check)
	curr = startNode
	for curr != nil && curr != lcaNode {
		for _, change := range curr.changes {
			if _, exists := netChanges[change.ID]; !exists {
				return nil, fmt.Errorf("CalcChgs: change ID '%s' from start path (node '%s', version '%s') not found in end path changes (version '%s' to LCA)",
					change.ID, curr.version, startVersion, endVersion)
			}
			delete(netChanges, change.ID)
		}
		curr = curr.parent
	}

	result := make([]Chg, 0, len(netChanges))
	for _, change := range netChanges {
		result = append(result, change)
	}

	sort.Slice(result, func(i, j int) bool {
		idNumI, errI := strconv.Atoi(result[i].ID)
		idNumJ, errJ := strconv.Atoi(result[j].ID)
		if errI == nil && errJ == nil {
			return idNumI < idNumJ
		}
		return result[i].ID < result[j].ID
	})

	return result, nil
}

// NodeInfo represents a single node in the tree dump.
type NodeInfo struct {
	Version  string   `json:"version"`
	FromVer  string   `json:"from_ver"`
	Changes  []string `json:"changes"`
	Children []string `json:"children"`
}

// TreeDump represents the full tree structure for introspection.
type TreeDump struct {
	NodeCount int        `json:"node_count"`
	Root      string     `json:"root"`
	Nodes     []NodeInfo `json:"nodes"`
}

// Dump returns a serializable snapshot of the tree (read-locked).
func (tree *ReleaseTree) Dump() TreeDump {
	tree.mu.RLock()
	defer tree.mu.RUnlock()

	dump := TreeDump{
		NodeCount: len(tree.nodes),
	}
	if tree.root != nil {
		dump.Root = tree.root.version
	}

	// Collect all nodes sorted by version for deterministic output
	versions := make([]string, 0, len(tree.nodes))
	for v := range tree.nodes {
		versions = append(versions, v)
	}
	sort.Strings(versions)

	for _, v := range versions {
		n := tree.nodes[v]
		info := NodeInfo{
			Version: n.version,
		}
		if n.parent != nil {
			info.FromVer = n.parent.version
		}
		info.Changes = make([]string, len(n.changes))
		for i, c := range n.changes {
			info.Changes[i] = c.ID
		}
		info.Children = make([]string, len(n.children))
		for i, c := range n.children {
			info.Children[i] = c.version
		}
		dump.Nodes = append(dump.Nodes, info)
	}

	return dump
}
