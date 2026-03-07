package ui

import (
	"testing"

	"github.com/pgavlin/crtea/model"
)

func TestBuildFileTree(t *testing.T) {
	files := []model.DiffFile{
		{NewPath: "cmd/main.go"},
		{NewPath: "cmd/util.go"},
		{NewPath: "pkg/model/types.go"},
		{NewPath: "README.md"},
	}

	root := buildFileTree(files)

	if len(root.Children) == 0 {
		t.Fatal("root should have children")
	}

	// Should have directories first, then files
	var names []string
	for _, c := range root.Children {
		names = append(names, c.Name)
	}

	// README.md should be last (file after dirs)
	last := root.Children[len(root.Children)-1]
	if last.Name != "README.md" {
		t.Errorf("last child = %q, want README.md", last.Name)
	}
}

func TestBuildFileTreeCollapsesChains(t *testing.T) {
	files := []model.DiffFile{
		{NewPath: "a/b/c/file.go"},
	}

	root := buildFileTree(files)

	// Single-child chain a -> b -> c should be collapsed into "a/b/c"
	if len(root.Children) != 1 {
		t.Fatalf("root should have 1 child, got %d", len(root.Children))
	}
	dir := root.Children[0]
	if dir.Name != "a/b/c" {
		t.Errorf("collapsed dir name = %q, want a/b/c", dir.Name)
	}
	if !dir.IsDir {
		t.Error("collapsed node should be a directory")
	}
}

func TestFlattenTree(t *testing.T) {
	files := []model.DiffFile{
		{NewPath: "dir/a.go"},
		{NewPath: "dir/b.go"},
		{NewPath: "root.go"},
	}

	root := buildFileTree(files)
	rows := flattenTree(root, map[string]bool{})

	if len(rows) == 0 {
		t.Fatal("flattenTree should produce rows")
	}

	// First row should be the directory
	if !rows[0].IsDir {
		t.Error("first row should be a directory")
	}

	// Files inside dir should have depth > 0
	for _, r := range rows {
		if !r.IsDir && r.Depth > 0 {
			return // found a nested file, good
		}
	}
	t.Error("expected at least one nested file row")
}

func TestFlattenTreeCollapsed(t *testing.T) {
	files := []model.DiffFile{
		{NewPath: "dir/a.go"},
		{NewPath: "dir/b.go"},
	}

	root := buildFileTree(files)
	collapsed := map[string]bool{"dir": true}
	rows := flattenTree(root, collapsed)

	// Should only show the directory, not its children
	if len(rows) != 1 {
		t.Errorf("collapsed tree should have 1 row, got %d", len(rows))
	}
	if !rows[0].IsDir {
		t.Error("only row should be a directory")
	}
}
