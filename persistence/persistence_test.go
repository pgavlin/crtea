package persistence

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/pgavlin/crtea/model"
)

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	dir := t.TempDir()
	return &FileStore{dir: dir}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	store := newTestStore(t)

	session := model.NewSession("/repo/path", "main", "abc123", model.DiffWorkingTree)
	fr := session.GetOrCreateFileReview("test.go", model.FileModified)
	fr.AddFileComment(model.NewComment("test", model.CommentNote, model.SideNew))
	fr.Reviewed = true

	path, err := store.Save(session)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	if path == "" {
		t.Error("Save() returned empty path")
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("saved file not found: %v", err)
	}

	// Load it back
	loaded, err := store.LoadLatest("/repo/path", "main", model.DiffWorkingTree)
	if err != nil {
		t.Fatalf("LoadLatest() error: %v", err)
	}
	if loaded == nil {
		t.Fatal("LoadLatest() returned nil")
	}

	if loaded.ID != session.ID {
		t.Errorf("ID = %q, want %q", loaded.ID, session.ID)
	}
	if loaded.RepoPath != "/repo/path" {
		t.Errorf("RepoPath = %q, want /repo/path", loaded.RepoPath)
	}
	if loaded.BranchName != "main" {
		t.Errorf("BranchName = %q, want main", loaded.BranchName)
	}

	lfr := loaded.GetFileReview("test.go")
	if lfr == nil {
		t.Fatal("loaded session missing file review")
	}
	if !lfr.Reviewed {
		t.Error("Reviewed should be true")
	}
	if len(lfr.FileComments) != 1 {
		t.Errorf("FileComments len = %d, want 1", len(lfr.FileComments))
	}
}

func TestLoadLatestNoMatch(t *testing.T) {
	store := newTestStore(t)

	session := model.NewSession("/repo", "main", "abc", model.DiffWorkingTree)
	store.Save(session)

	// Different branch
	_, err := store.LoadLatest("/repo", "feature", model.DiffWorkingTree)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("LoadLatest() error = %v, want ErrSessionNotFound", err)
	}

	// Different diff source
	_, err = store.LoadLatest("/repo", "main", model.DiffCommitRange)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("LoadLatest() error = %v, want ErrSessionNotFound", err)
	}
}

func TestLoadLatestEmptyDir(t *testing.T) {
	store := newTestStore(t)

	_, err := store.LoadLatest("/repo", "main", model.DiffWorkingTree)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("LoadLatest() error = %v, want ErrSessionNotFound", err)
	}
}

func TestLoadLatestNonexistentDir(t *testing.T) {
	store := &FileStore{dir: filepath.Join(t.TempDir(), "nonexistent")}

	_, err := store.LoadLatest("/repo", "main", model.DiffWorkingTree)
	if !errors.Is(err, ErrSessionNotFound) {
		t.Fatalf("LoadLatest() error = %v, want ErrSessionNotFound", err)
	}
}

func TestRepoFingerprint(t *testing.T) {
	fp1 := repoFingerprint("/repo/a")
	fp2 := repoFingerprint("/repo/b")
	if fp1 == fp2 {
		t.Error("different repos should have different fingerprints")
	}

	// Deterministic
	fp3 := repoFingerprint("/repo/a")
	if fp1 != fp3 {
		t.Error("same repo should produce same fingerprint")
	}
}
