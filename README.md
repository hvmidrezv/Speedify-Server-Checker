# speedify-check

A terminal UI that tests Speedify VPN servers one by one and produces a
**ranked list of stable servers**.

<!-- Screenshot placeholder: TUI running on the country menu.
     Add `docs/screenshot-menu.png` here once captured against a real install. -->

## What it does

1. Notices you (a short intro screen) and fetches the server list from
   `speedify_cli show servers`.
2. Lets you pick countries in a menu (toggle, select all/none).
3. Connects to **each selected server in turn** with `speedify_cli
   connect <country> <city> <num>` and:
   - waits up to `-connect-timeout` for the `CONNECTED` state,
   - holds the connection for `-hold`, sampling `speedify_cli state`
     and a TCP probe (`-probe`, default `1.1.1.1:443`) every
     `-interval`,
   - disconnects and optionally retries (`-retries`) a failing server.
4. Ranks the stable servers by `score = avg + 2*jitter + 20*connectSeconds`
   (lower is better) and writes the artifacts below.

## Requirements

- Go 1.22+ to build (external deps: `bubbletea` v1.2.0,
  `lipgloss` v1.0.0).
- `speedify_cli` installed and logged in, runnable by the same user.

## Install

```sh
git clone <repo> && cd speedify-check
go build -o bin/speedify-check ./cmd/speedify-check
# or
go install ./cmd/speedify-check
```

## Quick start

```sh
speedify-check
speedify-check -hold 15s -retries 1 -max-loss 1
```

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

## Key bindings

### Notice screen
| Key | Action |
|---|---|
| `Enter` | continue |
| `r` | re-check the CLI / server list |
| `q` / `ctrl+c` | quit |

### Country menu
| Key | Action |
|---|---|
| `↑/↓` (or `k`/`j`, `PgUp`/`PgDn`, `Home`/`End`) | move (first row is "All countries") |
| `Space` / `x` | toggle country |
| `a` | select all |
| `n` | select none |
| `Enter` | continue (selection shown in the confirm popup) |
| `Esc` | back to notice |

### Confirm popup
| Key | Action |
|---|---|
| `Y` / `Enter` | start the run |
| `N` / `Esc` / `q` | go back to the menu |

### Run screen
| Key | Action |
|---|---|
| `q` (first press) | graceful stop, shows partial results |
| `q` again / `ctrl+c` | force quit + `disconnect` |

### Results screen
| Key | Action |
|---|---|
| `↑/↓` | move |
| `Enter` | connect `speedify_cli` to the selected stable server |
| `d` | disconnect |
| `r` | back to the menu for a new run |
| `q` | quit |

## Output files (in `-out`)

- `stable_servers.txt` — ranked table of stable servers
- `stable_names.txt` — one stable server name per line, ranked
- `results.csv` / `results.json` — every result, tested or not
- `results.jsonl` — append-only journal (enables `-resume`)
- `speedify_check.log` — full detail log

## How the stability score works

For a stable server, `score = avg_ms + 2*jitter_ms + 20*connect_seconds`
(lower is better). Latency dominates via the average RTT; jitter is
double-weighted; slow connects are penalized hard (20×). Servers that
never connect (`connect_timeout`), drop mid-hold (`dropped`) or lose
more probes than `-max-loss` (`loss`) never get a score.

## How the probe works (and its limits)

Each tick the tool dials `-probe` with `-probe-timeout` and records
the TCP handshake time as the RTT sample. It does **not** measure HTTP
throughput — a server can pass while browsing is slow. Speedify routes
the probe through the tunnel for the client under test, so the sample
reflects the tunnel's current path (including split-tunneling quirks:
if `speedify_cli` bypasses the probe target locally, samples may not
reflect tunnel quality).

## Known limitations / unverified against real Speedify

> The program was refactored without access to a real Speedify
> installation: parsing of `show servers`, `state` and `show
> currentserver` is isolated in `internal/speedify` and covered by
> format tests, but the *actual* formats may differ.

If your run shows empty states or no servers, please capture and share:
`speedify_cli show servers`, `speedify_cli state` and `speedify_cli
show currentserver` (redact IPs), then open an issue. See
[docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) and
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Contributing

`gofmt -l .`, `go vet ./...` and `go test -race ./...` must be clean.
`make lint` (golangci-lint) covers govet, staticcheck, errcheck,
ineffassign, unused, gofmt and revive. Conventional Commits, one
logical change per commit.

## License

MIT — see [LICENSE](LICENSE).
