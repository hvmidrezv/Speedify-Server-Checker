// Package speedify wraps the speedify_cli command-line tool behind a
// small typed client. All knowledge of the CLI invocation and of its
// (unverified) output formats lives here, so format fixes only ever
// touch this package.
package speedify

import (
	"context"
	"encoding/json"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"speedify-check/internal/model"
)

// Timeouts applied by the typed client methods, matching the ones the
// original single-file program used per invocation.
const (
	connectTimeout    = 60 * time.Second
	stateTimeout      = 15 * time.Second
	currentSrvTimeout = 15 * time.Second
	serversTimeout    = 30 * time.Second
	disconnectTimeout = 15 * time.Second
)

// Runner executes the speedify_cli binary with the given arguments and
// returns trimmed standard output. It is the seam that lets tests
// substitute a fake CLI; timeouts are the caller's concern and are
// carried by ctx.
type Runner interface {
	Run(ctx context.Context, args ...string) (string, error)
}

// ExecRunner runs a real speedify_cli binary via os/exec.
type ExecRunner struct {
	// Path is the binary to execute, usually "speedify_cli" or an
	// absolute path to it.
	Path string
}

// NewExecRunner returns a runner that executes the binary at path.
func NewExecRunner(path string) *ExecRunner {
	return &ExecRunner{Path: path}
}

// Run executes the binary with args and returns its trimmed standard
// output. The context bounds the execution.
func (r *ExecRunner) Run(ctx context.Context, args ...string) (string, error) {
	out, err := exec.CommandContext(ctx, r.Path, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// Available reports whether the CLI binary can be found on PATH or at
// the given path.
func Available(path string) error {
	_, err := exec.LookPath(path)
	return err
}

// Client talks to speedify_cli through a Runner and exposes typed
// operations for the checker and the UI.
type Client struct {
	runner Runner
}

// NewClient returns a Client that executes the CLI through runner.
func NewClient(runner Runner) *Client {
	return &Client{runner: runner}
}

// Servers returns the full server list from "show servers", merging the
// public, private and team groups.
func (c *Client) Servers(ctx context.Context) ([]model.Server, error) {
	ctx, cancel := context.WithTimeout(ctx, serversTimeout)
	defer cancel()
	out, err := c.runner.Run(ctx, "show", "servers")
	if err != nil {
		return nil, err
	}
	return parseServers(out)
}

// State returns the current connection state (e.g. "CONNECTED")
// upper-cased. CLI failures degrade to the empty string, as before.
func (c *Client) State(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, stateTimeout)
	defer cancel()
	out, _ := c.runner.Run(ctx, "state")
	return parseState(out)
}

// CurrentServer returns the exit IP reported by "show currentserver",
// or "" when unavailable.
func (c *Client) CurrentServer(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, currentSrvTimeout)
	defer cancel()
	out, _ := c.runner.Run(ctx, "show", "currentserver")
	return parseCurrentServer(out)
}

// Connect asks Speedify to connect to the given server.
func (c *Client) Connect(ctx context.Context, s model.Server) error {
	ctx, cancel := context.WithTimeout(ctx, connectTimeout)
	defer cancel()
	_, err := c.runner.Run(ctx, "connect", s.Country, s.City, strconv.Itoa(s.Num))
	return err
}

// Disconnect tears the current connection down.
func (c *Client) Disconnect(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, disconnectTimeout)
	defer cancel()
	_, err := c.runner.Run(ctx, "disconnect")
	return err
}

// ForceDisconnect is the best-effort disconnect used when the user
// forces the TUI to quit mid-run: it has no timeout and ignores errors.
func (c *Client) ForceDisconnect() {
	_, _ = c.runner.Run(context.Background(), "disconnect")
}

// parseServers understands the output of "show servers": an object
// with public, private and team arrays. Missing keys are tolerated;
// malformed JSON is reported as an error.
func parseServers(out string) ([]model.Server, error) {
	var data struct {
		Public  []model.Server `json:"public"`
		Private []model.Server `json:"private"`
		Team    []model.Server `json:"team"`
	}
	if err := json.Unmarshal([]byte(out), &data); err != nil {
		return nil, err
	}
	all := append([]model.Server{}, data.Public...)
	all = append(all, data.Private...)
	all = append(all, data.Team...)
	return all, nil
}

// parseState accepts three shapes seen in the wild: a bare JSON
// string, an object carrying a "state" string, or plain text. The
// result is always upper-cased.
func parseState(out string) string {
	var s string
	if json.Unmarshal([]byte(out), &s) == nil {
		return strings.ToUpper(s)
	}
	var m map[string]any
	if json.Unmarshal([]byte(out), &m) == nil {
		if v, ok := m["state"].(string); ok {
			return strings.ToUpper(v)
		}
	}
	return strings.ToUpper(strings.Trim(out, "\" \r\n"))
}

// parseCurrentServer extracts the exit IP from "show currentserver";
// anything unexpected yields "".
func parseCurrentServer(out string) string {
	var m map[string]any
	if json.Unmarshal([]byte(out), &m) != nil {
		return ""
	}
	if v, ok := m["ip"].(string); ok {
		return v
	}
	return ""
}
