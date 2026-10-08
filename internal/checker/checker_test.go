package checker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hvmidrezv/Speedify-Server-Checker/internal/model"
)

// fakeCLI answers State from a per-Connect script; the last entry of a
// script repeats. Multiple scripts model multiple attempts.
type fakeCLI struct {
	mu          sync.Mutex
	scripts     [][]string
	script      int
	idx         int
	stateHook   func() // called on every State call
	disconnects int
}

func (f *fakeCLI) Connect(ctx context.Context, s model.Server) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.idx > 0 && f.script+1 < len(f.scripts) {
		f.script++
		f.idx = 0
	}
	return nil
}

func (f *fakeCLI) State(ctx context.Context) string {
	f.mu.Lock()
	if f.script >= len(f.scripts) || len(f.scripts[f.script]) == 0 {
		f.mu.Unlock()
		return "DISCONNECTED"
	}
	script := f.scripts[f.script]
	if f.idx < len(script) {
		f.idx++
	}
	cur := script[f.idx-1]
	hook := f.stateHook
	f.mu.Unlock()
	if hook != nil {
		hook()
	}
	return cur
}

func (f *fakeCLI) CurrentServer(ctx context.Context) string { return "9.9.9.9" }

func (f *fakeCLI) Disconnect(ctx context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.disconnects++
	return nil
}

type fakeProber struct {
	ok bool
	d  time.Duration
}

func (f fakeProber) Probe(ctx context.Context) (time.Duration, bool) { return f.d, f.ok }

type fakeRecorder struct {
	recorded  []model.Result
	finalized [][]model.Result
}

func (f *fakeRecorder) Record(r model.Result) { f.recorded = append(f.recorded, r) }

func (f *fakeRecorder) Finalize(all []model.Result) { f.finalized = append(f.finalized, all) }

// testCfg returns a Config with tiny durations so tests run in
// milliseconds while exercising the same code paths as production.
func testCfg() Config {
	return Config{
		ConnectTimeout: 30 * time.Millisecond,
		Hold:           25 * time.Millisecond,
		Interval:       5 * time.Millisecond,
		MaxLoss:        0,
		Retries:        0,
		Pause:          0,
		PollInterval:   2 * time.Millisecond,
		RetryDelay:     time.Millisecond,
		ProbeAddr:      "test:443",
	}
}

// run executes the engine synchronously and captures events.
func run(t *testing.T, e *Engine, todo, prev []model.Result) ([]model.Result, []model.Event) {
	t.Helper()
	servers := make([]model.Server, len(todo))
	for i, r := range todo {
		servers[i] = model.Server{Country: r.Country, City: r.City, Num: r.Num}
	}
	var events []model.Event
	all := e.Run(context.Background(), servers, prev, func(ev model.Event) { events = append(events, ev) })
	return all, events
}

func countEvents(events []model.Event) map[string]int {
	m := map[string]int{}
	for _, ev := range events {
		switch ev.(type) {
		case model.LogEvent:
			m["log"]++
		case model.ServerStartEvent:
			m["start"]++
		case model.PhaseEvent:
			m["phase"]++
		case model.SampleEvent:
			m["sample"]++
		case model.ServerDoneEvent:
			m["done"]++
		case model.FinishedEvent:
			m["finished"]++
		}
	}
	return m
}

func TestStableServer(t *testing.T) {
	cli := &fakeCLI{scripts: [][]string{{"CONNECTED"}}}
	rec := &fakeRecorder{}
	e := New(testCfg(), cli, fakeProber{ok: true, d: 10 * time.Millisecond}, rec, nil)

	all, events := run(t, e, []model.Result{{Country: "de", City: "frankfurt", Num: 3}}, nil)
	if len(all) != 1 || !all[0].Stable || all[0].Category != "ok" {
		t.Fatalf("results = %+v, want one stable ok result", all)
	}
	r := all[0]
	if r.Server != "de-frankfurt-3" || r.Attempts != 1 || r.ExitIP != "9.9.9.9" {
		t.Errorf("unexpected identity fields: %+v", r)
	}
	if r.Samples < 2 {
		t.Errorf("Samples = %d, want at least 2", r.Samples)
	}
	if r.ProbeLoss != 0 || r.AvgMs != 10 || r.MinMs != 10 || r.MaxMs != 10 || r.JitterMs != 0 {
		t.Errorf("unexpected probe stats: %+v", r)
	}
	if want := r.AvgMs + 2*r.JitterMs + 20*r.ConnectTime; r.Score != want {
		t.Errorf("Score = %v, want %v", r.Score, want)
	}
	if len(rec.recorded) != 1 {
		t.Errorf("recorded %d results, want 1", len(rec.recorded))
	}
	if len(rec.finalized) != 1 || len(rec.finalized[0]) != 1 {
		t.Errorf("finalize got %+v, want one result", rec.finalized)
	}
	counts := countEvents(events)
	for _, k := range []string{"start", "phase", "sample", "done", "finished"} {
		if counts[k] == 0 {
			t.Errorf("missing %s events (got %v)", k, counts)
		}
	}
	if counts["finished"] != 1 {
		t.Errorf("finished events = %d, want 1", counts["finished"])
	}
}

func TestConnectTimeout(t *testing.T) {
	cli := &fakeCLI{scripts: [][]string{{"DISCONNECTED"}}}
	e := New(testCfg(), cli, fakeProber{ok: true, d: time.Millisecond}, &fakeRecorder{}, nil)

	all, _ := run(t, e, []model.Result{{Country: "de", City: "berlin", Num: 1}}, nil)
	if len(all) != 1 || all[0].Stable {
		t.Fatalf("results = %+v, want one unstable result", all)
	}
	r := all[0]
	if r.Category != "connect_timeout" {
		t.Errorf("Category = %q, want connect_timeout", r.Category)
	}
	if r.Reason != "connect timeout (last state: DISCONNECTED)" {
		t.Errorf("Reason = %q", r.Reason)
	}
}

func TestStateDropMidHold(t *testing.T) {
	// First State answers CONNECTED (connect phase); the next ones
	// report a drop.
	cli := &fakeCLI{scripts: [][]string{{"CONNECTED", "DISCONNECTED"}}}
	e := New(testCfg(), cli, fakeProber{ok: true, d: time.Millisecond}, &fakeRecorder{}, nil)

	all, events := run(t, e, []model.Result{{Country: "de", City: "berlin", Num: 1}}, nil)
	r := all[0]
	if r.Stable || r.Category != "dropped" {
		t.Fatalf("result = %+v, want dropped", r)
	}
	if !strings.HasPrefix(r.Reason, "dropped to DISCONNECTED at ") {
		t.Errorf("Reason = %q", r.Reason)
	}
	if r.Samples != 1 {
		t.Errorf("Samples = %d, want 1", r.Samples)
	}
	bad := false
	for _, ev := range events {
		if s, ok := ev.(model.SampleEvent); ok && !s.StateOK && s.State == "DISCONNECTED" {
			bad = true
		}
	}
	if !bad {
		t.Error("no SampleEvent with StateOK=false emitted")
	}
}

func TestProbeLossOverMaxLoss(t *testing.T) {
	cli := &fakeCLI{scripts: [][]string{{"CONNECTED"}}}
	cfg := testCfg()
	cfg.MaxLoss = 0
	e := New(cfg, cli, fakeProber{ok: false}, &fakeRecorder{}, nil)

	all, _ := run(t, e, []model.Result{{Country: "de", City: "berlin", Num: 1}}, nil)
	r := all[0]
	if r.Stable || r.Category != "loss" {
		t.Fatalf("result = %+v, want loss", r)
	}
	if r.ProbeLoss == 0 || r.ProbeLoss != r.Samples {
		t.Errorf("ProbeLoss = %d, Samples = %d, want equal and > 0", r.ProbeLoss, r.Samples)
	}
	if want := fmt.Sprintf("traffic loss: %d/%d probes failed", r.ProbeLoss, r.Samples); r.Reason != want {
		t.Errorf("Reason = %q, want %q", r.Reason, want)
	}
}

func TestProbeLossUnderMaxLoss(t *testing.T) {
	cli := &fakeCLI{scripts: [][]string{{"CONNECTED"}}}
	cfg := testCfg()
	cfg.MaxLoss = 1000
	e := New(cfg, cli, fakeProber{ok: false}, &fakeRecorder{}, nil)

	all, _ := run(t, e, []model.Result{{Country: "de", City: "berlin", Num: 1}}, nil)
	r := all[0]
	if !r.Stable || r.Category != "ok" {
		t.Fatalf("result = %+v, want stable despite probe loss", r)
	}
	if r.ProbeLoss == 0 {
		t.Error("expected some probe loss")
	}
}

func TestRetriesThenStable(t *testing.T) {
	// First attempt never connects (timeout); the second one does.
	cli := &fakeCLI{scripts: [][]string{{"DISCONNECTED"}, {"CONNECTED"}}}
	cfg := testCfg()
	cfg.Retries = 1
	e := New(cfg, cli, fakeProber{ok: true, d: time.Millisecond}, &fakeRecorder{}, nil)

	all, events := run(t, e, []model.Result{{Country: "de", City: "berlin", Num: 1}}, nil)
	r := all[0]
	if !r.Stable || r.Attempts != 2 {
		t.Fatalf("result = %+v, want stable on attempt 2", r)
	}
	warn := ""
	for _, ev := range events {
		if l, ok := ev.(model.LogEvent); ok && l.Kind == "warn" {
			warn = l.Text
		}
	}
	if !strings.Contains(warn, "retry 1/1") {
		t.Errorf("no retry log event, got %q", warn)
	}
	if got := countEvents(events)["done"]; got != 1 {
		t.Errorf("server-done events = %d, want 1", got)
	}
}

func TestCancelledAttemptIsNotRecorded(t *testing.T) {
	prev := []model.Result{{Server: "old-1", Stable: true}}
	rec := &fakeRecorder{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cli := &fakeCLI{scripts: [][]string{{"DISCONNECTED"}}}
	cli.stateHook = func() { cancel() } // cancel as soon as the engine asks for state
	cfg := testCfg()
	cfg.ConnectTimeout = time.Second // would loop forever without cancellation
	e := New(cfg, cli, fakeProber{ok: true, d: time.Millisecond}, rec, nil)

	servers := []model.Server{{Country: "de", City: "berlin", Num: 1}}
	var events []model.Event
	all := e.Run(ctx, servers, prev, func(ev model.Event) { events = append(events, ev) })

	if len(rec.recorded) != 0 {
		t.Errorf("recorded %d results, want none after cancellation", len(rec.recorded))
	}
	if len(all) != 1 || all[0].Server != "old-1" {
		t.Errorf("all = %+v, want only the previous result", all)
	}
	if len(rec.finalized) != 1 || len(rec.finalized[0]) != 1 {
		t.Errorf("finalize got %+v, want only previous results", rec.finalized)
	}
	if got := countEvents(events)["done"]; got != 0 {
		t.Errorf("server-done events = %d, want 0", got)
	}
	if got := countEvents(events)["finished"]; got != 1 {
		t.Errorf("finished events = %d, want 1 (run still reports completion)", got)
	}
}
