package checker

import (
	"context"
	"fmt"
	"time"

	"speedify-check/internal/model"
	"speedify-check/internal/probe"
)

// attempt performs a single connect + hold cycle for s.
func (e *Engine) attempt(ctx context.Context, s model.Server, emit func(model.Event)) model.Result {
	r := model.Result{Server: s.Name(), Country: s.Country, City: s.City, Num: s.Num, TestedAt: time.Now()}

	emit(model.PhaseEvent{Phase: "connecting"})
	e.logf("DEBUG", "%s: connecting", r.Server)
	_ = e.cli.Connect(ctx, s)

	start := time.Now()
	last := ""
	connected := false
	for time.Since(start) < e.cfg.ConnectTimeout {
		if ctx.Err() != nil {
			r.Category, r.Reason = "cancelled", "cancelled"
			return r
		}
		last = e.cli.State(ctx)
		if last == "CONNECTED" {
			connected = true
			break
		}
		sleepCtx(ctx, e.cfg.PollInterval)
	}
	if !connected {
		r.Category = "connect_timeout"
		r.Reason = fmt.Sprintf("connect timeout (last state: %s)", last)
		e.logf("FAIL", "%s: %s", r.Server, r.Reason)
		return r
	}
	r.ConnectTime = time.Since(start).Seconds()
	r.ExitIP = e.cli.CurrentServer(ctx)
	e.logf("DEBUG", "%s: connected in %.1fs, exit ip %s", r.Server, r.ConnectTime, r.ExitIP)
	emit(model.PhaseEvent{Phase: "holding", ConnectTime: r.ConnectTime, IP: r.ExitIP})

	var rtts []float64
	holdStart := time.Now()
	for time.Since(holdStart) < e.cfg.Hold {
		if !sleepCtx(ctx, e.cfg.Interval) {
			r.Category, r.Reason = "cancelled", "cancelled"
			return r
		}
		r.Samples++
		el := time.Since(holdStart).Seconds()

		st := e.cli.State(ctx)
		if st != "CONNECTED" {
			e.logf("DEBUG", "%s: t=%.1fs state=%s (dropped)", r.Server, el, st)
			emit(model.SampleEvent{N: r.Samples, StateOK: false, State: st})
			r.Category = "dropped"
			r.Reason = fmt.Sprintf("dropped to %s at %.1fs", st, el)
			break
		}
		d, ok := e.prober.Probe(ctx)
		if ok {
			ms := float64(d.Microseconds()) / 1000
			rtts = append(rtts, ms)
			e.logf("DEBUG", "%s: t=%.1fs state=CONNECTED rtt=%.1fms", r.Server, el, ms)
			emit(model.SampleEvent{N: r.Samples, StateOK: true, ProbeOK: true, RTT: ms})
		} else {
			r.ProbeLoss++
			e.logf("DEBUG", "%s: t=%.1fs state=CONNECTED probe FAILED", r.Server, el)
			emit(model.SampleEvent{N: r.Samples, StateOK: true, ProbeOK: false})
		}
	}

	r.MinMs, r.AvgMs, r.MaxMs, r.JitterMs = probe.Stats(rtts)

	if r.Reason == "" && r.ProbeLoss > e.cfg.MaxLoss {
		r.Category = "loss"
		r.Reason = fmt.Sprintf("traffic loss: %d/%d probes failed", r.ProbeLoss, r.Samples)
	}
	if r.Reason == "" {
		r.Stable = true
		r.Category = "ok"
		r.Score = r.AvgMs + 2*r.JitterMs + 20*r.ConnectTime
	}
	return r
}

// logf writes a diagnostics line when a logger is configured.
func (e *Engine) logf(level, format string, a ...any) {
	if e.log == nil {
		return
	}
	e.log.Logf(level, format, a...)
}

// sleepCtx waits for d or until ctx is cancelled; it reports whether
// the full wait happened.
func sleepCtx(ctx context.Context, d time.Duration) bool {
	select {
	case <-time.After(d):
		return true
	case <-ctx.Done():
		return false
	}
}
