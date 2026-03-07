package bugreport

import (
	"archive/zip"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/vcs"
)

func TestWrite(t *testing.T) {
	session := model.NewSession("/tmp/repo", "main", "abc1234", model.DiffWorkingTree)

	report := Report{
		Body:        "Something is broken",
		Environment: map[string]string{"os": "darwin", "arch": "arm64"},
		CreatedAt:   time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC),
	}

	path, err := Write(report, session, "screen content here")
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	defer os.Remove(path)

	// Open and verify zip contents
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("failed to open zip: %v", err)
	}
	defer r.Close()

	files := make(map[string]*zip.File)
	for _, f := range r.File {
		files[f.Name] = f
	}

	// Check manifest.json
	mf, ok := files["manifest.json"]
	if !ok {
		t.Fatal("manifest.json not found in zip")
	}
	rc, err := mf.Open()
	if err != nil {
		t.Fatalf("failed to open manifest.json: %v", err)
	}
	defer rc.Close()
	var m manifest
	if err := json.NewDecoder(rc).Decode(&m); err != nil {
		t.Fatalf("failed to decode manifest.json: %v", err)
	}
	if m.Report != "Something is broken" {
		t.Errorf("report = %q, want %q", m.Report, "Something is broken")
	}
	if m.Environment["os"] != "darwin" {
		t.Errorf("environment.os = %q, want %q", m.Environment["os"], "darwin")
	}

	// Check attachments/session.json
	sf, ok := files["attachments/session.json"]
	if !ok {
		t.Fatal("attachments/session.json not found in zip")
	}
	src, err := sf.Open()
	if err != nil {
		t.Fatalf("failed to open session.json: %v", err)
	}
	defer src.Close()
	var sess model.ReviewSession
	if err := json.NewDecoder(src).Decode(&sess); err != nil {
		t.Fatalf("failed to decode session.json: %v", err)
	}
	if sess.RepoPath != "/tmp/repo" {
		t.Errorf("session.RepoPath = %q, want %q", sess.RepoPath, "/tmp/repo")
	}

	// Check attachments/screen.txt
	scf, ok := files["attachments/screen.txt"]
	if !ok {
		t.Fatal("attachments/screen.txt not found in zip")
	}
	scrc, err := scf.Open()
	if err != nil {
		t.Fatalf("failed to open screen.txt: %v", err)
	}
	defer scrc.Close()
	buf := make([]byte, 1024)
	n, _ := scrc.Read(buf)
	if string(buf[:n]) != "screen content here" {
		t.Errorf("screen.txt = %q, want %q", string(buf[:n]), "screen content here")
	}
}

func TestCollectEnvironment(t *testing.T) {
	info := vcs.VcsInfo{
		RootPath:   "/tmp/repo",
		HeadCommit: "abc1234",
		BranchName: "main",
		VcsType:    "git",
	}

	env := CollectEnvironment(info, 120, 40)

	requiredKeys := []string{
		"os", "arch", "go_version", "terminal", "term_program",
		"shell", "lang", "crtea_version", "window_width", "window_height",
		"vcs_type", "branch", "head_commit",
	}
	for _, key := range requiredKeys {
		if _, ok := env[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
	if env["vcs_type"] != "git" {
		t.Errorf("vcs_type = %q, want %q", env["vcs_type"], "git")
	}
	if env["window_width"] != "120" {
		t.Errorf("window_width = %q, want %q", env["window_width"], "120")
	}
}
