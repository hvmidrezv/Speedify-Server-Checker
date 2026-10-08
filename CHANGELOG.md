# Changelog

All notable changes to this project are documented here. The format is
based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/).

## [0.1.0] - 2026-10-08

### Added
- Terminal UI that tests Speedify VPN servers one by one and ranks the
  stable ones (Bubble Tea + Lip Gloss): notice, country menu, confirm
  popup, live run screen, ranked results.
- Test engine with connect timeout, hold loop, per-interval TCP probe,
  retries, max-loss tolerance, stable scoring
  (`score = avg + 2*jitter + 20*connectSeconds`) and resumable runs.
- Artifacts: `stable_servers.txt`, `stable_names.txt`, `results.csv`,
  `results.json`, append-only `results.jsonl`, `speedify_check.log`.
- Documentation: README, `docs/ARCHITECTURE.md`, `docs/USAGE.md`,
  `docs/TROUBLESHOOTING.md`.
- Project hygiene: Makefile, golangci-lint config, GitHub Actions CI
  (gofmt, `go vet`, lint, `go test -race`).

### Notes
- The tool was developed without access to a real Speedify install;
  `speedify_cli` output parsing is isolated in `internal/speedify` and
  covered by format tests, but the real formats are unverified.
