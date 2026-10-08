// Package checker runs the server-testing loop: connect, hold, sample,
// score, retry. It knows nothing about Bubble Tea or the filesystem;
// progress is reported as model.Event values handed to an emit
// function, persistence through the Recorder interface and diagnostics
// through the Logger interface.
package checker

import (
	"context"
	"fmt"
	"time"

	"speedify-check/internal/model"
)

// DefaultPollInterval and DefaultRetryDelay mirror the hard-coded
// timings of the original single-file program (500ms state polling
// while connecting, 2s between retries).
const (
	DefaultPollInterval = 500 * time.Millisecond
	DefaultRetryDelay   = 2 * time.Second
)

// CLI is what the engine needs from the Speedify client.
type CLI interface {
	Connect(ctx context.Context, s model.Server) error
	State(ctx context.Context) string
	CurrentServer(ctx context.Context) string
	Disconnect(ctx context.Context) error
}

// Prober is what the engine needs from the TCP probe.
type Prober interface {
	Probe(ctx context.Context) (time.Duration, bool)
}

// Recorder persists results while a run is in progress.
type Recorder interface {
	// Record is called once per finished server (stable or not).
	Record(r model.Result)
	// Finalize is called exactly once when the run ends, with every
	// result of the run (previous runs first).
	Finalize(all []model.Result)
}

// Logger is the leveled diagnostics sink the engine writes to.
type Logger interface {
	Logf(level, format string, a ...any)
}

// Config holds the engine timing knobs. ProbeAddr is only used for the
// run-start log line.
type Config struct {
	ConnectTimeout time.Duration // max time to wait for CONNECTED
	Hold           time.Duration // how long the connection must stay stable
	Interval       time.Duration // sampling interval during hold
	MaxLoss        int           // failed probes allowed for a server to count as stable
	Retries        int           // extra attempts for a failing server
	Pause          time.Duration // pause between servers
	PollInterval   time.Duration // state polling interval while connecting
	RetryDelay     time.Duration // wait between retries
	ProbeAddr      string        // logged at run start
}

// Engine tests servers one by one through the injected dependencies.
type Engine struct {
	cfg    Config
	cli    CLI
	prober Prober
	rec    Recorder
	log    Logger
}

// New returns an engine for cfg. rec may be nil (results are then only
// emitted as events); log may be nil (diagnostics are dropped).
func New(cfg Config, cli CLI, prober Prober, rec Recorder, log Logger) *Engine {
	if rec == nil {
		rec = discardRecorder{}
	}
	return &Engine{cfg: cfg, cli: cli, prober: prober, rec: rec, log: log}
}

// discardRecorder is the no-op Recorder used when none is supplied.
type discardRecorder struct{}

func (discardRecorder) Record(model.Result)     {}
func (discardRecorder) Finalize([]model.Result) {}

// Run tests every server in todo, keeping the results of previous runs
// (prev) so reports always contain everything. Progress events are
// handed to emit, which must not block for long. Run always
// disconnects and finalizes, even when ctx is cancelled; a cancelled
// attempt is never recorded.
func (e *Engine) Run(ctx context.Context, todo []model.Server, prev []model.Result, emit func(model.Event)) []model.Result {
	all := append([]model.Result{}, prev...)

	e.logf("INFO", "==== run started: %d servers, hold=%s, probe=%s, max-loss=%d, retries=%d ====",
		len(todo), e.cfg.Hold, e.cfg.ProbeAddr, e.cfg.MaxLoss, e.cfg.Retries)

	for i, s := range todo {
		if ctx.Err() != nil {
			break
		}
		emit(model.ServerStartEvent{Idx: i + 1, Total: len(todo), Name: s.Name()})
		r := e.testServer(ctx, s, emit)
		if ctx.Err() != nil {
			break
		}
		all = append(all, r)
		e.rec.Record(r)
		if r.Stable {
			e.logf("OK", "%s STABLE avg=%.1fms jitter=%.1fms loss=%d/%d connect=%.1fs ip=%s",
				r.Server, r.AvgMs, r.JitterMs, r.ProbeLoss, r.Samples, r.ConnectTime, r.ExitIP)
		} else {
			e.logf("FAIL", "%s UNSTABLE: %s", r.Server, r.Reason)
		}
		emit(model.ServerDoneEvent{Result: r})
		sleepCtx(ctx, e.cfg.Pause)
	}

	_ = e.cli.Disconnect(context.Background())
	e.rec.Finalize(all)
	e.logf("INFO", "==== run finished: %d results ====", len(all))
	emit(model.FinishedEvent{All: all})
	return all
}

// testServer runs one or more attempts for s, honouring the retry
// configuration.
func (e *Engine) testServer(ctx context.Context, s model.Server, emit func(model.Event)) model.Result {
	var r model.Result
	for n := 1; n <= 1+e.cfg.Retries; n++ {
		r = e.attempt(ctx, s, emit)
		r.Attempts = n
		_ = e.cli.Disconnect(context.Background())
		if r.Stable || ctx.Err() != nil {
			return r
		}
		if n <= e.cfg.Retries {
			emit(model.LogEvent{Kind: "warn", Text: fmt.Sprintf("%s  retry %d/%d · %s", s.Name(), n, e.cfg.Retries, r.Reason)})
			sleepCtx(ctx, e.cfg.RetryDelay)
		}
	}
	return r
}
