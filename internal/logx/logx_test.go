package logx

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogfWritesLeveledLine(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf)
	l.Logf("INFO", "hello %d", 42)
	got := buf.String()
	if !strings.Contains(got, "[INFO ] hello 42") {
		t.Errorf("log line = %q, want it to contain [INFO ] hello 42", got)
	}
}

func TestNilLoggerIsNoOp(t *testing.T) {
	var l *Logger
	l.Logf("INFO", "should not panic")
	if err := l.Close(); err != nil {
		t.Errorf("Close on nil logger = %v, want nil", err)
	}
}

func TestOpenAppendsAndClose(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")

	l, err := Open(path)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	l.Logf("OK", "first")
	if err := l.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	l2, err := Open(path)
	if err != nil {
		t.Fatalf("Open again: %v", err)
	}
	l2.Logf("FAIL", "second")
	_ = l2.Close()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "[OK   ] first") || !strings.Contains(out, "[FAIL ] second") {
		t.Errorf("log file = %q, want both lines appended", out)
	}
}

func TestOpenFailureReturnsError(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "missing", "deep", "x.log")); err == nil {
		t.Fatal("expected error opening log under missing directory")
	}
}
