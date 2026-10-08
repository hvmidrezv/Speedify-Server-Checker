# Usage

Flags, key bindings, output files and scoring. Screens flow:

`notice → menu → confirm → run → results`

## Flags

| Flag | Default | Meaning |
|---|---|---|
| `-cli` | `speedify_cli` | path to `speedify_cli` |
| `-connect-timeout` | `25s` | max time to wait for `CONNECTED` |
| `-hold` | `10s` | how long the connection must stay stable |
| `-interval` | `1s` | sampling interval during hold |
| `-probe` | `1.1.1.1:443` | host:port for the TCP probe through the tunnel |
| `-probe-timeout` | `2s` | TCP probe timeout |
| `-max-loss` | `0` | failed probes allowed for a server to count as stable |
| `-retries` | `0` | extra attempts for a failing server |
| `-resume` | `false` | skip servers already tested (resume from `results.jsonl`) |
| `-out` | `.` | output directory |
| `-pause` | `1s` | pause between servers |

Examples:

```sh
speedify-check
speedify-check -hold 15s -retries 1 -max-loss 1
speedify-check -out ./results -resume
```

Invalid values (non-positive durations, negative counts) fail fast with
`error: -flag ...`; the usage line matches the original (`flag.Parse`
semantics via an `ExitOnError` flag set).

## Key bindings

### Notice
`Enter` continue · `r` re-check · `q` quit

### Country menu
`↑/↓` (also `k`/`j`, `PgUp`/`PgDn`, `Home`/`End`) move; row 0 is "All
countries" · `Space`/`x` toggle · `a` all · `n` none · `Enter`
continue · `Esc` back

### Confirm popup
`Y`/`Enter` start · `N`/`Esc`/`q` back to the menu

### Run
`q` stop gracefully and show partial results; press `q` again or
`ctrl+c` to force quit (force quit runs `disconnect`)

### Results
`↑/↓` move · `Enter` connect to the selected stable server · `d`
disconnect · `r` new run (back to menu) · `q` quit

## Screens

1. **Notice** — intro + status while checking `speedify_cli` and
   loading `show servers`.
2. **Country menu** — toggle whole countries; counters show
   `countries · servers`; arrow-key navigation with a "move to selected
   servers" hint while resuming.
3. **Confirm** — chosen countries/servers, approximate duration
   (`n_servers × (hold + 8s)` estimate), `stable: N`, and the go-ahead.
4. **Run** — progress (`tested/total`, spinner), per-sample dots
   (`●` ok / red `✘` state drop / yellow `✘` probe fail), live log and
   the stable-servers-so-far list.
5. **Results** — ranked table (score, avg, jitter, max, connect time,
   exit IP), failure breakdown (`connect_timeout` / `dropped` /
   `loss`), saved-file locations, and interactive connect/disconnect.

## Output files (in `-out`)

- `stable_servers.txt` — ranked table of stable servers
- `stable_names.txt` — one stable server name per line, ranked
- `results.csv`, `results.json` — all results (stable and not)
- `results.jsonl` — append-only journal; with `-resume`, servers
  already present are reported as `skipped N` and excluded
- `speedify_check.log` — `timestamp [LEVEL] message` debug log

## The stability score

`score = avg_ms + 2*jitter_ms + 20*connect_seconds`, lower is better.
`avg/jitter/max` come from the probe RTTs; `connect_seconds` is the
time-to-CONNECTED. Only `ok` (stable) results get a score; `connect_timeout`,
`dropped` and over-limit `loss` results are ranked out.
