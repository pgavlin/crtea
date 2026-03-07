package ui

import (
	"sort"
	"strings"

	"github.com/pgavlin/crtea/model"
)

// fileTreeNode represents a node in the file tree (either a directory or a file).
type fileTreeNode struct {
	Name     string          // base name of this segment
	Path     string          // full path up to this node
	FileIdx  int             // index into diffFiles, -1 for directories
	Children []*fileTreeNode // sorted children
	IsDir    bool
}

// fileTreeRow is a flattened row in the visible file tree.
type fileTreeRow struct {
	Node   *fileTreeNode
	Depth  int
	IsDir  bool
	IsLast []bool // whether each ancestor (and self) is the last child at its level
}

// buildFileTree constructs a tree from the diff files.
func buildFileTree(files []model.DiffFile) *fileTreeNode {
	root := &fileTreeNode{Name: "", Path: "", FileIdx: -1, IsDir: true}

	for i, file := range files {
		parts := strings.Split(file.DisplayPath(), "/")
		node := root
		for j, part := range parts {
			isFile := j == len(parts)-1
			fullPath := strings.Join(parts[:j+1], "/")

			// Find or create child
			var child *fileTreeNode
			for _, c := range node.Children {
				if c.Name == part {
					child = c
					break
				}
			}
			if child == nil {
				child = &fileTreeNode{
					Name:    part,
					Path:    fullPath,
					FileIdx: -1,
					IsDir:   !isFile,
				}
				if isFile {
					child.FileIdx = i
				}
				node.Children = append(node.Children, child)
			}
			node = child
		}
	}

	// Sort children: directories first, then alphabetical
	sortTree(root)

	// Collapse single-child directory chains
	collapseChains(root)

	return root
}

func sortTree(node *fileTreeNode) {
	sort.Slice(node.Children, func(i, j int) bool {
		a, b := node.Children[i], node.Children[j]
		if a.IsDir != b.IsDir {
			return a.IsDir // directories first
		}
		return a.Name < b.Name
	})
	for _, child := range node.Children {
		sortTree(child)
	}
}

// collapseChains merges single-child directories into their parent.
// e.g. "ui" -> "components" -> file.go becomes "ui/components" -> file.go
func collapseChains(node *fileTreeNode) {
	for i, child := range node.Children {
		if child.IsDir && len(child.Children) == 1 && child.Children[0].IsDir {
			// Merge child into grandchild
			grandchild := child.Children[0]
			grandchild.Name = child.Name + "/" + grandchild.Name
			node.Children[i] = grandchild
			collapseChains(node) // re-check since we modified in place
			return
		}
		collapseChains(child)
	}
}

// flattenTree produces the visible rows from the tree, respecting collapsed state.
func flattenTree(root *fileTreeNode, collapsed map[string]bool) []fileTreeRow {
	var rows []fileTreeRow
	for i, child := range root.Children {
		isLast := []bool{i == len(root.Children)-1}
		flattenNode(child, 0, isLast, collapsed, &rows)
	}
	return rows
}

func flattenNode(node *fileTreeNode, depth int, isLast []bool, collapsed map[string]bool, rows *[]fileTreeRow) {
	*rows = append(*rows, fileTreeRow{
		Node:   node,
		Depth:  depth,
		IsDir:  node.IsDir,
		IsLast: append([]bool{}, isLast...),
	})

	if node.IsDir && !collapsed[node.Path] {
		for i, child := range node.Children {
			childIsLast := append(append([]bool{}, isLast...), i == len(node.Children)-1)
			flattenNode(child, depth+1, childIsLast, collapsed, rows)
		}
	}
}
