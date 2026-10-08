# Architecture

## Layers

`speedify-check` is a Bubble Tea TUI whose test engine depends only on
small interfaces. Dependency direction (strict, no cycles):

```
cmd ──▶ tui ──▶ checker ──▶ speedify / probe ──▶ model
 │        │          │             │
 │        │          └─────────────┼── report (persistence, via checker.Recorder)
 │        │                        │
 │        ▼                        ▼
 │      config                   logx (diagnostics, via checker.Logger)
 │        │
 └────────┴──▶ model, geo (leaves: import nothing internal)
```

ASCII package map:

```
speedify-check/
├── cmd/speedify-check   flag parsing + wiring only, no logic
└── internal/
    ├── config    Config struct, BindFlags, Defaults, Validate
    ├── speedify  Runner interface + exec runner; typed client
    │             (Servers/State/CurrentServer/Connect/Disconnect);
    │             ALL speedify_cli JSON parsing lives here
    ├── probe     TCP prober + pure RTT statistics (min/avg/max/jitter)
    ├── checker   test engine: attempt/retry/hold loop, scoring
    ├── model     Server, Result, Country, sealed event types
    ├── geo       display names + flag emoji (string-only, leaf)
    ├── report    jsonl append/load, ranking, txt/names/csv/json writers
    ├── logx      nil-safe, mutex-protected leveled file logger
    └── tui       Bubble Tea model/update/view; adapts events to tea.Msg
```

## Dependency rules

- `model` and `geo` import nothing internal. They never depend on UI,
  engine or CLI details.
- `tui` never calls `os/exec` (directly or transitively outside
  `speedify`). All CLI access goes through `speedify.Client`.
- `speedify.Runner` is the seam for faking the CLI: `Run(ctx,
  args...) (string, error)`. Timeouts are set by the typed client via
  contexts, not by the runner.
- The checker declares what it needs **where it consumes it**:
  `checker.CLI`, `checker.Prober`, `checker.Recorder`,
  `checker.Logger`. `*speedify.Client` and `*probe.TCP` satisfy them
  structurally.
- The checker has no knowledge of Bubble Tea. It sends domain events
  over an `emit func(model.Event)`; `tui` wraps them in `eventMsg` and
  switches on the sealed `model.Event` set.
- Context-first signatures; config is passed explicitly (no flag
  globals); wrap errors with `%w` (where surfaced); no panics; minimal
  exported API with doc comments on every exported identifier.
- `gofmt`/`goimports` clean; golangci-lint: govet, staticcheck,
  errcheck, ineffassign, unused, gofmt, revive (with exported-comment
  rule).

## Data flow: one server, one attempt

```
cmd/main.go ── config.Parse/Validate, logx.Open, report.LoadJSONL
     │
     ▼ tui.New(cfg, prev, log) ── tea.NewProgram(...)
     │
     ▼ startRun: checker.New(cfg, client, prober, fileRecorder, log)
     │
     ▼ eng.Run(ctx, todo, prev, emit)        (goroutine)
        attempt: Connect(ctx, server)
          poll State(ctx) until CONNECTED / timeout ─▶ PhaseEvent
          CurrentServer(ctx) ─▶ PhaseEvent{holding}
          loop: State(ctx) + Probe(ctx) ─▶ SampleEvent per tick
        testServer: disconnect, retry/log on failure
        rec.Record(result)  → report.AppendJSONL (results.jsonl)
        rec.Finalize(all)   → report.WriteReports (txt/names/csv/json)
     │
     ▼ channel: checker goroutine ─▶ chan model.Event ─▶ tui waitEvent
        → tea.Msg eventMsg{ev} → Model.Update → onEvent(ev)
```

## Event model

`model.Event` is a sealed interface (only types in `model` implement
it): `LogEvent`, `ServerStartEvent`, `PhaseEvent`, `SampleEvent`,
`ServerDoneEvent`, `FinishedEvent`. The engine always emits
`FinishedEvent` (even after cancellation, with partial results), so
the UI never needs a second completion signal. Cancellation semantics:
an attempt returning after `ctx.Err() != nil` is never recorded.
