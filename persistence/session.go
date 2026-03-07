// Package persistence provides storage for review sessions.
package persistence

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pgavlin/crtea/model"
)

// Store defines the interface for session persistence.
type Store interface {
	Save(session *model.ReviewSession) (string, error)
	LoadLatest(repoPath, branchName string, diffSource model.DiffSource) (*model.ReviewSession, error)
}

// FileStore implements Store by persisting sessions as JSON files on disk.
type FileStore struct {
	dir string
}

// NewFileStore creates a FileStore using the default session directory.
func NewFileStore() (*FileStore, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(home, ".local", "share", "crtea", "reviews")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &FileStore{dir: dir}, nil
}

// Save writes the session to disk.
func (fs *FileStore) Save(session *model.ReviewSession) (string, error) {
	session.UpdatedAt = time.Now().UTC()

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return "", err
	}

	fingerprint := repoFingerprint(session.RepoPath)
	filename := fmt.Sprintf("%s_%s.json", fingerprint, session.ID)
	path := filepath.Join(fs.dir, filename)

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return "", err
	}

	return path, nil
}

// LoadLatest finds and loads the most recent session matching the given context.
func (fs *FileStore) LoadLatest(repoPath, branchName string, diffSource model.DiffSource) (*model.ReviewSession, error) {
	fingerprint := repoFingerprint(repoPath)

	entries, err := os.ReadDir(fs.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	type candidate struct {
		path    string
		modTime time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		if !strings.HasPrefix(entry.Name(), fingerprint+"_") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		candidates = append(candidates, candidate{
			path:    filepath.Join(fs.dir, entry.Name()),
			modTime: info.ModTime(),
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].modTime.After(candidates[j].modTime)
	})

	for _, c := range candidates {
		data, err := os.ReadFile(c.path)
		if err != nil {
			continue
		}
		var session model.ReviewSession
		if err := json.Unmarshal(data, &session); err != nil {
			continue
		}
		if session.BranchName == branchName && session.DiffSource == diffSource {
			return &session, nil
		}
	}

	return nil, nil
}

func repoFingerprint(repoPath string) string {
	h := fnv.New32a()
	h.Write([]byte(repoPath))
	return fmt.Sprintf("%08x", h.Sum32())
}
