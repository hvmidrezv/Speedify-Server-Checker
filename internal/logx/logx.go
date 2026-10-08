// Package logx provides the leveled, mutex-protected file logger that
// backs speedify_check.log.
package logx

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

// Logger writes timestamped, level-tagged lines to a writer. It is
// safe for concurrent use. The zero value and a nil *Logger are
// no-ops, so callers never need nil checks.
type Logger struct {
	mu sync.Mutex
	w  io.Writer // nil disables logging
}

// New returns a logger writing to w.
func New(w io.Writer) *Logger {
	return &Logger{w: w}
}

// Open opens (or creates) the log file at path in append mode and
// returns a logger that owns it. On failure a usable no-op logger is
// returned together with the error, so logging setup can never abort
// the program.
func Open(path string) (*Logger, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	return &Logger{w: f}, nil
}

// Close closes the underlying log file when the logger owns one.
func (l *Logger) Close() error {
	if l == nil {
		return nil
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if c, ok := l.w.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// Logf writes one line in the form
// "2006-01-02 15:04:05 [LEVEL] message". Level is free-form (INFO,
// DEBUG, OK, FAIL, WARN, ...). Write failures are ignored: the log is
// best-effort diagnostics and must not affect the run.
func (l *Logger) Logf(level, format string, a ...any) {
	if l == nil || l.w == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	// Best-effort diagnostics; a failed write must not affect the run.
	_, _ = fmt.Fprintf(l.w, "%s [%-5s] %s\n", time.Now().Format("2006-01-02 15:04:05"), level, fmt.Sprintf(format, a...))
}
