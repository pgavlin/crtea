package vcs

import (
	"testing"

	"github.com/pgavlin/crtea/model"
)

const simpleDiff = `diff --git a/hello.go b/hello.go
index 1234567..abcdefg 100644
--- a/hello.go
+++ b/hello.go
@@ -1,5 +1,6 @@
 package main

 func main() {
-	fmt.Println("hello")
+	fmt.Println("hello, world")
+	fmt.Println("goodbye")
 }
`

func TestParseDiffSimple(t *testing.T) {
	files := parseDiff(simpleDiff)
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}

	f := files[0]
	if f.OldPath != "hello.go" {
		t.Errorf("OldPath = %q, want hello.go", f.OldPath)
	}
	if f.NewPath != "hello.go" {
		t.Errorf("NewPath = %q, want hello.go", f.NewPath)
	}
	if f.Status != model.FileModified {
		t.Errorf("Status = %d, want FileModified", f.Status)
	}
	if len(f.Hunks) != 1 {
		t.Fatalf("got %d hunks, want 1", len(f.Hunks))
	}

	hunk := f.Hunks[0]
	if hunk.OldStart != 1 || hunk.OldCount != 5 {
		t.Errorf("old range = %d,%d, want 1,5", hunk.OldStart, hunk.OldCount)
	}
	if hunk.NewStart != 1 || hunk.NewCount != 6 {
		t.Errorf("new range = %d,%d, want 1,6", hunk.NewStart, hunk.NewCount)
	}

	// Count line types
	var ctx, add, del int
	for _, l := range hunk.Lines {
		switch l.Origin {
		case model.OriginContext:
			ctx++
		case model.OriginAddition:
			add++
		case model.OriginDeletion:
			del++
		}
	}
	if ctx != 4 {
		t.Errorf("context lines = %d, want 4", ctx)
	}
	if add != 2 {
		t.Errorf("addition lines = %d, want 2", add)
	}
	if del != 1 {
		t.Errorf("deletion lines = %d, want 1", del)
	}
}

func TestParseDiffLineNumbers(t *testing.T) {
	files := parseDiff(simpleDiff)
	hunk := files[0].Hunks[0]

	// First line: context " package main" -> old=1, new=1
	if l := hunk.Lines[0]; l.OldLineNo != 1 || l.NewLineNo != 1 {
		t.Errorf("line 0: old=%d new=%d, want old=1 new=1", l.OldLineNo, l.NewLineNo)
	}
	// Deletion line "-	fmt.Println..." -> old=4, new=0
	if l := hunk.Lines[3]; l.Origin != model.OriginDeletion || l.OldLineNo != 4 {
		t.Errorf("line 3: origin=%d old=%d, want deletion old=4", l.Origin, l.OldLineNo)
	}
	// First addition -> old=0, new=4
	if l := hunk.Lines[4]; l.Origin != model.OriginAddition || l.NewLineNo != 4 {
		t.Errorf("line 4: origin=%d new=%d, want addition new=4", l.Origin, l.NewLineNo)
	}
}

const newFileDiff = `diff --git a/new.go b/new.go
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/new.go
@@ -0,0 +1,3 @@
+package new
+
+func New() {}
`

func TestParseDiffNewFile(t *testing.T) {
	files := parseDiff(newFileDiff)
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	f := files[0]
	if f.Status != model.FileAdded {
		t.Errorf("Status = %d, want FileAdded", f.Status)
	}
	if f.NewPath != "new.go" {
		t.Errorf("NewPath = %q, want new.go", f.NewPath)
	}
}

const deletedFileDiff = `diff --git a/old.go b/old.go
deleted file mode 100644
index 1234567..0000000
--- a/old.go
+++ /dev/null
@@ -1,3 +0,0 @@
-package old
-
-func Old() {}
`

func TestParseDiffDeletedFile(t *testing.T) {
	files := parseDiff(deletedFileDiff)
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if files[0].Status != model.FileDeleted {
		t.Errorf("Status = %d, want FileDeleted", files[0].Status)
	}
}

const renameDiff = `diff --git a/old.go b/new.go
similarity index 100%
rename from old.go
rename to new.go
`

func TestParseDiffRename(t *testing.T) {
	files := parseDiff(renameDiff)
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	f := files[0]
	if f.Status != model.FileRenamed {
		t.Errorf("Status = %d, want FileRenamed", f.Status)
	}
	if f.OldPath != "old.go" {
		t.Errorf("OldPath = %q, want old.go", f.OldPath)
	}
	if f.NewPath != "new.go" {
		t.Errorf("NewPath = %q, want new.go", f.NewPath)
	}
}

const binaryDiff = `diff --git a/image.png b/image.png
index 1234567..abcdefg 100644
Binary files a/image.png and b/image.png differ
`

func TestParseDiffBinary(t *testing.T) {
	files := parseDiff(binaryDiff)
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if !files[0].IsBinary {
		t.Error("expected IsBinary = true")
	}
}

const multiFileDiff = `diff --git a/a.go b/a.go
index 1111111..2222222 100644
--- a/a.go
+++ b/a.go
@@ -1,3 +1,3 @@
 package a

-var x = 1
+var x = 2
diff --git a/b.go b/b.go
new file mode 100644
index 0000000..3333333
--- /dev/null
+++ b/b.go
@@ -0,0 +1,3 @@
+package b
+
+var y = 1
`

func TestParseDiffMultipleFiles(t *testing.T) {
	files := parseDiff(multiFileDiff)
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].NewPath != "a.go" {
		t.Errorf("file 0 path = %q, want a.go", files[0].NewPath)
	}
	if files[1].NewPath != "b.go" {
		t.Errorf("file 1 path = %q, want b.go", files[1].NewPath)
	}
	if files[0].Status != model.FileModified {
		t.Errorf("file 0 status = %d, want FileModified", files[0].Status)
	}
	if files[1].Status != model.FileAdded {
		t.Errorf("file 1 status = %d, want FileAdded", files[1].Status)
	}
}

const multiHunkDiff = `diff --git a/multi.go b/multi.go
index 1234567..abcdefg 100644
--- a/multi.go
+++ b/multi.go
@@ -1,3 +1,3 @@
 package multi

-var a = 1
+var a = 2
@@ -10,3 +10,3 @@

-var b = 1
+var b = 2

`

func TestParseDiffMultipleHunks(t *testing.T) {
	files := parseDiff(multiHunkDiff)
	if len(files) != 1 {
		t.Fatalf("got %d files, want 1", len(files))
	}
	if len(files[0].Hunks) != 2 {
		t.Fatalf("got %d hunks, want 2", len(files[0].Hunks))
	}
	if files[0].Hunks[0].OldStart != 1 {
		t.Errorf("hunk 0 OldStart = %d, want 1", files[0].Hunks[0].OldStart)
	}
	if files[0].Hunks[1].OldStart != 10 {
		t.Errorf("hunk 1 OldStart = %d, want 10", files[0].Hunks[1].OldStart)
	}
}

func TestParseHunkHeader(t *testing.T) {
	tests := []struct {
		input                                  string
		oldStart, oldCount, newStart, newCount int
	}{
		{"@@ -1,5 +1,6 @@", 1, 5, 1, 6},
		{"@@ -10,3 +10,4 @@ func foo()", 10, 3, 10, 4},
		{"@@ -1 +1 @@", 1, 1, 1, 1},     // no comma = count of 1
		{"@@ -0,0 +1,3 @@", 0, 0, 1, 3}, // new file
	}
	for _, tt := range tests {
		hunk := parseHunkHeader(tt.input)
		if hunk.OldStart != tt.oldStart || hunk.OldCount != tt.oldCount {
			t.Errorf("parseHunkHeader(%q): old = %d,%d, want %d,%d",
				tt.input, hunk.OldStart, hunk.OldCount, tt.oldStart, tt.oldCount)
		}
		if hunk.NewStart != tt.newStart || hunk.NewCount != tt.newCount {
			t.Errorf("parseHunkHeader(%q): new = %d,%d, want %d,%d",
				tt.input, hunk.NewStart, hunk.NewCount, tt.newStart, tt.newCount)
		}
	}
}

func TestParseDiffEmpty(t *testing.T) {
	files := parseDiff("")
	if len(files) != 0 {
		t.Errorf("got %d files, want 0", len(files))
	}
}
