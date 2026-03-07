// Package bugreport creates zip archives containing bug report data.
package bugreport

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/pgavlin/crtea/model"
	"github.com/pgavlin/crtea/vcs"
)

// Report holds the user-provided bug report data.
type Report struct {
	Body        string
	Environment map[string]string
	CreatedAt   time.Time
}

type manifest struct {
	Report      string            `json:"report"`
	Environment map[string]string `json:"environment"`
	CreatedAt   time.Time         `json:"created_at"`
}

// Write creates a zip archive containing the bug report, session, and screen content.
// It returns the path to the temporary zip file.
func Write(report Report, session *model.ReviewSession, screenContent string) (string, error) {
	f, err := os.CreateTemp("", "crtea-bug-*.zip")
	if err != nil {
		return "", err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Write manifest.json
	m := manifest{
		Report:      report.Body,
		Environment: report.Environment,
		CreatedAt:   report.CreatedAt,
	}
	manifestData, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	w, err := zw.Create("manifest.json")
	if err != nil {
		return "", err
	}
	if _, err := w.Write(manifestData); err != nil {
		return "", err
	}

	// Write attachments/session.json
	sessionData, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return "", err
	}
	w, err = zw.Create("attachments/session.json")
	if err != nil {
		return "", err
	}
	if _, err := w.Write(sessionData); err != nil {
		return "", err
	}

	// Write attachments/screen.txt
	w, err = zw.Create("attachments/screen.txt")
	if err != nil {
		return "", err
	}
	if _, err := w.Write([]byte(screenContent)); err != nil {
		return "", err
	}

	return f.Name(), nil
}

// CollectEnvironment gathers environment metadata for the bug report.
func CollectEnvironment(vcsInfo vcs.VcsInfo, width, height int) map[string]string {
	env := map[string]string{
		"os":            runtime.GOOS,
		"arch":          runtime.GOARCH,
		"go_version":    runtime.Version(),
		"terminal":      os.Getenv("TERM"),
		"term_program":  os.Getenv("TERM_PROGRAM"),
		"shell":         os.Getenv("SHELL"),
		"lang":          os.Getenv("LANG"),
		"crtea_version": "1.0",
		"window_width":  fmt.Sprintf("%d", width),
		"window_height": fmt.Sprintf("%d", height),
		"vcs_type":      vcsInfo.VcsType,
		"branch":        vcsInfo.BranchName,
		"head_commit":   vcsInfo.HeadCommit,
	}
	return env
}

