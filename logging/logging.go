// Package logging provides structured JSON logging to a file using log/slog.
package logging

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
)

// Init creates a new slog.Logger that writes JSON-encoded logs to
// ~/.local/share/crtea/crtea.log. It returns the logger and the underlying
// file handle (so the caller can defer closing it).
func Init() (*slog.Logger, *os.File, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, nil, fmt.Errorf("getting home directory: %w", err)
	}

	dir := filepath.Join(home, ".local", "share", "crtea")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("creating log directory: %w", err)
	}

	logPath := filepath.Join(dir, "crtea.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, nil, fmt.Errorf("opening log file: %w", err)
	}

	handler := slog.NewJSONHandler(f, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	logger := slog.New(handler)

	return logger, f, nil
}
