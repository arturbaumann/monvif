# Release Notes — monvif v0.6.0

## What's new

### `monvif diagnose` command

Runs a comprehensive read-only health check on every camera in a TSV inventory
file and produces a structured report.

**Per-camera checks:**

1. TCP connect to ONVIF port
2. TCP connect to RTSP port (default 554; override with `--rtsp-port`)
3. ONVIF authentication and device info (manufacturer, model, firmware, serial)
4. Capabilities (service endpoint URLs)
5. Media profiles (count and tokens)
6. Stream URI (RTSP, first profile; credentials redacted)
7. Snapshot URI (credentials redacted)
8. Imaging settings read (brightness/contrast/saturation/sharpness)
9. Network configuration read (hostname, IP, MAC, gateway, DNS, NTP)

**Per-camera status:**

- `OK` — all core checks passed
- `WARN` — core OK, but optional checks had issues
- `FAIL` — TCP unreachable, auth failed, no media profiles, or no stream URI

**Output formats:**

- `--format table` (default) — compact summary table with per-column status
- `--format json` — full structured JSON; `| jq .` friendly
- `--format markdown` — human-readable report with device info, check table,
  network details, warnings, and errors per camera

**Output file:**

```bash
./monvif diagnose --file cameras.tsv --user ha --format markdown --output report.md
# Prints: Wrote diagnostics report to report.md
```

**Progress:** reuses existing `--progress auto|always|never` (default `auto`).
JSON stdout is never corrupted by progress output.

**Other flags:**

- `--timeout 5s` — TCP dial timeout per low-level check (default 5s)
- `--camera-timeout 60s` — max wall time per active camera (default 60s)
- `--concurrency 4` — cameras diagnosed simultaneously (default 4)
- `--only <name|ip>` — diagnose a single camera from the inventory by name or IP
- `--rtsp-port 554` — RTSP port to test (default 554)
- `--skip-discovery` — WS-Discovery check is skipped (default `true`; pass
  `--skip-discovery=false` to enable once implemented)

## Performance and concurrency

Cameras are now diagnosed concurrently. The default `--concurrency 4` runs up
to 4 cameras at the same time, reducing total runtime roughly proportionally.

| Flags | ~Runtime (6 local cameras, auth 5–7s each) |
|-------|---------------------------------------------|
| `--quick --concurrency 1` | ~40–50s (serial) |
| `--quick --concurrency 4` | ~15–20s (default) |
| `--quick --concurrency 6` | ~10–15s (all at once) |

**Timeout semantics:**

- `--timeout 5s` — TCP dial timeout per low-level check (RTSP port test, etc.)
- `--camera-timeout 60s` — max wall time per camera, measured from when that
  camera acquires a worker slot. A camera waiting in queue for a free slot
  cannot time out while idle — only active work counts.
- If a camera does exceed its budget, checks completed before the timeout are
  preserved in the output row (TCP shows `yes` if it completed).
- The ONVIF library does not use Go context for HTTP, so ONVIF call timeouts
  are not bounded per-call. The `--camera-timeout` provides a safe outer bound.

```bash
# Recommended first run:
./monvif diagnose --file cameras.tsv --user ha --quick

# All cameras at once:
./monvif diagnose --file cameras.tsv --user ha --quick --concurrency 6

# Tuned full run:
./monvif diagnose --file cameras.tsv --user ha --timeout 3s --camera-timeout 30s --concurrency 4
```

## Known limitations

- WS-Discovery check is always skipped; the discovery check result shows
  `skipped` in the output. This is planned for a future release.
- `set-dns` and `set-ntp` still only send the first `--server` value due to a
  library limitation.
- Some cameras return zero imaging values; this is treated as `ok` with a
  note, not a failure.

## Upgrade

```bash
go install github.com/artur/monvif@v0.6.0
# or use the install script:
VERSION=v0.6.0 curl -fsSL https://raw.githubusercontent.com/artur/monvif/main/scripts/install.sh | bash
```
