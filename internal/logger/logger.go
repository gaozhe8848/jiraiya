package logger

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// New creates a dual-output JSON logger that writes to both stdout and a
// timestamped file under the runtime/ directory.
func New() *slog.Logger {
	runtimeDir := "runtime"
	if err := os.MkdirAll(runtimeDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "failed to create runtime dir: %v\n", err)
		os.Exit(1)
	}

	filename := fmt.Sprintf("app-%s.log", time.Now().Format("20060102-150405"))
	logFile, err := os.OpenFile(filepath.Join(runtimeDir, filename), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to open log file: %v\n", err)
		os.Exit(1)
	}

	w := io.MultiWriter(os.Stdout, logFile)
	return slog.New(slog.NewJSONHandler(w, nil))
}
