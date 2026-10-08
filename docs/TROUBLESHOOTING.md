# Troubleshooting

## `speedify_cli` not found

The notice screen shows the `exec` error and offers `r` to re-check.
Fixes: install Speedify, ensure `speedify_cli` is on `PATH`, or pass
the binary explicitly:

```sh
speedify-check -cli /opt/speedify/speedify_cli
# or: speedify-check -cli ./speedify_cli
```

## Empty server list / empty states

**Most likely cause:** the real `speedify_cli` JSON differs from the
shapes this tool parses. The code was developed without a real
Speedify install, so all parsing assumptions are centralized and
untested against production output.

Expected shapes (see `internal/speedify/client.go`, the only file that
needs changing):

- `show servers` → object with `public`, `private`, `team` arrays of
  `{country, city, num}`.
- `state` → one of: a JSON string (`"CONNECTED"`), an object with a
  `"state"` string, or plain text. Anything else degrades to the
  upper-cased raw text.
- `show currentserver` → object with an `"ip"` string, else `""`.

To report a mismatch, run these and share the output (redact IPs):

```sh
speedify_cli show servers
speedify_cli state
speedify_cli show currentserver
```

Then add the new shape as a table case in
`internal/speedify/client_test.go` and adjust `parseServers`,
`parseState` or `parseCurrentServer`. Nothing else touches formats.

## Servers never connect (`connect_timeout`)

- Raise the budget: `-connect-timeout 40s`.
- Check the CLI works by hand: `speedify_cli connect <country> <city> <num>`.
- Verify you are logged in and the machine can reach the Speedify
  network at all.

## Stable servers found = 0

- Allow some loss: `-max-loss 1` (a single failed 2s probe no longer
  disqualifies).
- Give flaky servers another chance: `-retries 1`.
- Shorten the bar: `-hold 5s`.
- Read `speedify_check.log`: it has a per-sample trace (`state=…`,
  `rtt=…`, `probe FAILED`, `dropped`) for every server.

## Probe samples look wrong

The probe is a single TCP dial to `-probe` (default `1.1.1.1:443`),
timed with the handshake duration. Things that skew it:

- The probe target being reachable without the tunnel (split
  tunneling / local bypass) — samples measure the bypass, not the VPN.
- Hostile middleboxes that stall SYNs — raise `-probe-timeout`.
- The probe host being down from your location — pick `-probe
  example.com:443` and compare.

## Resume keeps everything skipped

`-resume` reads `results.jsonl` and skips every server recorded there.
Delete the journal (or drop the flag) to start over:

```sh
rm results.jsonl
```

## Invalid flag values

`error: -interval must be > 0, got 0s` etc. mean the config failed
validation; the program exits before touching the network. Fix the
flag and rerun.
