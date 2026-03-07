package vcs

import (
	"fmt"
	"testing"

	"github.com/pgavlin/crtea/model"
)

// FuzzParseDiff feeds arbitrary input to parseDiff and verifies it never
// panics and always returns structurally valid output.
func FuzzParseDiff(f *testing.F) {
	// Seed with known-good diffs from the unit tests.
	f.Add(simpleDiff)
	f.Add(newFileDiff)
	f.Add(deletedFileDiff)
	f.Add(renameDiff)
	f.Add(binaryDiff)
	f.Add(multiFileDiff)
	f.Add(multiHunkDiff)
	// Edge cases
	f.Add("")
	f.Add("diff --git a/x b/x\n")
	f.Add("diff --git a/x b/x\n@@ -0,0 +1 @@\n+hello\n")
	f.Add("diff --git a/x b/x\n@@ -1 +0,0 @@\n-removed\n")
	f.Add("diff --git a/x b/x\nnot a real line\n")
	f.Add("@@ -1,1 +1,1 @@\n content\n")
	f.Add("diff --git a/a b/a\ndiff --git a/b b/b\n")
	f.Add("diff --git a/x b/x\nBinary files a/x and b/x differ\n")
	f.Add("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n context\n-old\n+new\n")

	f.Fuzz(func(t *testing.T, input string) {
		files := parseDiff(input)

		for i, file := range files {
			// Status must be a valid enum value
			switch file.Status {
			case model.FileModified, model.FileAdded, model.FileDeleted, model.FileRenamed, model.FileCopied:
			default:
				t.Errorf("file %d: invalid status %d", i, file.Status)
			}

			for j, hunk := range file.Hunks {
				// Line numbers must not be negative
				if hunk.OldStart < 0 {
					t.Errorf("file %d hunk %d: OldStart = %d (negative)", i, j, hunk.OldStart)
				}
				if hunk.NewStart < 0 {
					t.Errorf("file %d hunk %d: NewStart = %d (negative)", i, j, hunk.NewStart)
				}
				if hunk.OldCount < 0 {
					t.Errorf("file %d hunk %d: OldCount = %d (negative)", i, j, hunk.OldCount)
				}
				if hunk.NewCount < 0 {
					t.Errorf("file %d hunk %d: NewCount = %d (negative)", i, j, hunk.NewCount)
				}

				for k, line := range hunk.Lines {
					// Origin must be valid
					switch line.Origin {
					case model.OriginContext, model.OriginAddition, model.OriginDeletion:
					default:
						t.Errorf("file %d hunk %d line %d: invalid origin %d", i, j, k, line.Origin)
					}

					// Line numbers must not be negative
					if line.OldLineNo < 0 {
						t.Errorf("file %d hunk %d line %d: OldLineNo = %d (negative)", i, j, k, line.OldLineNo)
					}
					if line.NewLineNo < 0 {
						t.Errorf("file %d hunk %d line %d: NewLineNo = %d (negative)", i, j, k, line.NewLineNo)
					}

					// Addition lines should have NewLineNo > 0 (if hunk has positive NewStart)
					if line.Origin == model.OriginAddition && hunk.NewStart > 0 && line.NewLineNo == 0 {
						t.Errorf("file %d hunk %d line %d: addition line has NewLineNo=0", i, j, k)
					}

					// Deletion lines should have OldLineNo > 0 (if hunk has positive OldStart)
					if line.Origin == model.OriginDeletion && hunk.OldStart > 0 && line.OldLineNo == 0 {
						t.Errorf("file %d hunk %d line %d: deletion line has OldLineNo=0", i, j, k)
					}
				}
			}
		}
	})
}

// FuzzParseHunkHeader feeds arbitrary input to parseHunkHeader.
func FuzzParseHunkHeader(f *testing.F) {
	f.Add("@@ -1,5 +1,6 @@")
	f.Add("@@ -10,3 +10,4 @@ func foo()")
	f.Add("@@ -1 +1 @@")
	f.Add("@@ -0,0 +1,3 @@")
	f.Add("@@@@")
	f.Add("@@ @@")
	f.Add("@@ -0 +0 @@")
	f.Add("@@ -999999,999999 +999999,999999 @@")
	f.Add("")
	f.Add("@@ garbage @@")
	f.Add("@@ -abc +def @@")

	f.Fuzz(func(t *testing.T, input string) {
		hunk := parseHunkHeader(input)
		if hunk == nil {
			t.Fatal("parseHunkHeader returned nil")
		}

		// Counts must not be negative (Atoi returns 0 on failure, not negative)
		if hunk.OldStart < 0 {
			t.Errorf("OldStart = %d (negative)", hunk.OldStart)
		}
		if hunk.NewStart < 0 {
			t.Errorf("NewStart = %d (negative)", hunk.NewStart)
		}
		if hunk.OldCount < 0 {
			t.Errorf("OldCount = %d (negative)", hunk.OldCount)
		}
		if hunk.NewCount < 0 {
			t.Errorf("NewCount = %d (negative)", hunk.NewCount)
		}

		// Header should be preserved
		if hunk.Header != input {
			t.Errorf("Header = %q, want %q", hunk.Header, input)
		}
	})
}

// FuzzParseDiffRoundTrip verifies that parseDiff produces consistent line
// number sequences (monotonically increasing within each origin type).
func FuzzParseDiffRoundTrip(f *testing.F) {
	f.Add(simpleDiff)
	f.Add(multiHunkDiff)
	f.Add("diff --git a/x b/x\n--- a/x\n+++ b/x\n@@ -1,3 +1,3 @@\n ctx\n-old\n+new\n ctx\n")

	f.Fuzz(func(t *testing.T, input string) {
		files := parseDiff(input)
		for fi, file := range files {
			for hi, hunk := range file.Hunks {
				var lastOld, lastNew int
				for li, line := range hunk.Lines {
					switch line.Origin {
					case model.OriginContext:
						if line.OldLineNo > 0 && line.OldLineNo < lastOld {
							t.Errorf("file %d hunk %d line %d: OldLineNo %d < prev %d",
								fi, hi, li, line.OldLineNo, lastOld)
						}
						if line.NewLineNo > 0 && line.NewLineNo < lastNew {
							t.Errorf("file %d hunk %d line %d: NewLineNo %d < prev %d",
								fi, hi, li, line.NewLineNo, lastNew)
						}
						if line.OldLineNo > 0 {
							lastOld = line.OldLineNo
						}
						if line.NewLineNo > 0 {
							lastNew = line.NewLineNo
						}
					case model.OriginDeletion:
						if line.OldLineNo > 0 && line.OldLineNo < lastOld {
							t.Errorf("file %d hunk %d line %d: OldLineNo %d < prev %d (deletion)",
								fi, hi, li, line.OldLineNo, lastOld)
						}
						if line.OldLineNo > 0 {
							lastOld = line.OldLineNo
						}
					case model.OriginAddition:
						if line.NewLineNo > 0 && line.NewLineNo < lastNew {
							t.Errorf("file %d hunk %d line %d: NewLineNo %d < prev %d (addition)",
								fi, hi, li, line.NewLineNo, lastNew)
						}
						if line.NewLineNo > 0 {
							lastNew = line.NewLineNo
						}
					}
				}
			}
		}
	})
}

// TestParseDiffDoesNotPanic runs parseDiff against a set of adversarial inputs.
func TestParseDiffDoesNotPanic(t *testing.T) {
	adversarial := []string{
		"",
		"\n",
		"\x00\x00\x00",
		"diff --git",
		"diff --git \n",
		"diff --git a/ b/\n",
		"diff --git a/x b/x\n@@\n",
		"diff --git a/x b/x\n@@ @@ @@\n",
		"diff --git a/x b/x\n@@ -a,b +c,d @@\n",
		"diff --git a/x b/x\n--- \n+++ \n@@ -1,1 +1,1 @@\n \n",
		"diff --git a/x b/x\n" + fmt.Sprintf("@@ -%d,%d +%d,%d @@\n", 1<<30, 1<<30, 1<<30, 1<<30),
		"diff --git a/x b/x\n@@ -1,1 +1,1 @@\n" + string(make([]byte, 10000)),
		"diff --git a/x b/x\nrename from \nrename to \n",
		"diff --git a/x b/x\nnew file mode 100644\ndeleted file mode 100644\n",
		"diff --git a/x b/x\nBinary files differ\n",
		"+not inside a hunk\n-also not inside a hunk\n",
	}
	for i, input := range adversarial {
		t.Run(fmt.Sprintf("adversarial_%d", i), func(t *testing.T) {
			// Just verify no panic
			_ = parseDiff(input)
		})
	}
}
