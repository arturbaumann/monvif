# monvif

A Linux CLI tool for ONVIF camera discovery and querying.

## Installation

### From a GitHub release (recommended)

```bash
curl -fsSL https://raw.githubusercontent.com/artur/monvif/main/scripts/install.sh | bash
```

The script detects your OS and architecture, downloads the correct binary from
the [latest release](https://github.com/artur/monvif/releases/latest), and
installs it to `~/.local/bin`. Override the target directory:

```bash
INSTALL_DIR=/usr/local/bin curl -fsSL https://raw.githubusercontent.com/artur/monvif/main/scripts/install.sh | bash
```

A specific version can be pinned with `VERSION=v0.5.0`.

### With `go install`

```bash
go install github.com/artur/monvif@latest
```

### Build from source

```bash
git clone https://github.com/artur/monvif
cd monvif
make build   # embeds version, commit, and date
# or: go build -o monvif .
```

Requires Go 1.22+.

## How discovery works

`monvif discover` uses WS-Discovery: it sends a multicast UDP probe to
239.255.255.250 on **port 3702** and collects responses from cameras that
announce themselves on the local network.

**Discovery only finds cameras that respond to multicast.** A camera may
support ONVIF queries but stay silent during discovery if:

- WS-Discovery is disabled in the camera's firmware settings.
- The camera is on a different subnet and multicast is not routed across it.
- A managed switch or firewall drops UDP multicast traffic.

If a camera does not appear in `discover` output, it may still be fully
queryable by IP address using `monvif info`, `monvif capabilities`, or
`monvif check`.

## Typical workflow

**Step 1 — try automatic discovery**

```bash
monvif discover --interface ens18 --timeout 10s
```

Cameras that support WS-Discovery will appear here. Note the XAddrs column
for their ONVIF service URLs.

**Step 2 — find cameras that did not respond to discovery**

Option A: scan with nmap to find hosts with port 80 (or 8080) open:

```bash
nmap -p 80,8080 --open 192.168.1.0/24
```

Option B: maintain an inventory file `cameras.tsv` and use `monvif check`.

The TSV format is `name<TAB>ip<TAB>port` — one camera per line.
**Do not store passwords in `cameras.tsv`.** Use `MONVIF_PASSWORD` instead.

```
# cameras.tsv
# name          ip              port
front-door      192.168.1.10    80
garage          192.168.1.11    80
lobby           192.168.1.20    8080
```

```bash
# Basic reachability and device info for all cameras:
MONVIF_PASSWORD=secret monvif check --file cameras.tsv --user admin

# Also fetch capability service URLs:
MONVIF_PASSWORD=secret monvif check --file cameras.tsv --user admin --capabilities

# Machine-readable JSON output, piped through jq:
MONVIF_PASSWORD=secret monvif check --file cameras.tsv --user admin --capabilities --format json | jq .
```

**Step 3 — query individual cameras by IP**

```bash
export MONVIF_PASSWORD=secret

monvif info          --ip 192.168.1.10 --user admin
monvif capabilities  --ip 192.168.1.10 --user admin
monvif profiles      --ip 192.168.1.10 --user admin
monvif stream-uri    --ip 192.168.1.10 --user admin
monvif snapshot-uri  --ip 192.168.1.10 --user admin
monvif imaging get   --ip 192.168.1.10 --user admin
```

## Command reference

### discover

```bash
monvif discover [--timeout <duration>] [--interface <iface>]
```

Probes the LAN via WS-Discovery multicast (UDP 3702). Prints a table of
responding cameras with their XAddrs, types, and scopes. Default timeout
is 5 s. Some cameras support ONVIF but do not respond to discovery — use
`monvif check` for those.

### check

```bash
monvif check --file cameras.tsv --user <user> [--capabilities] [--format table|json]
```

Reads a TSV inventory (`name<TAB>ip<TAB>port`) and checks each camera in
sequence. Reports: reachable, authenticated, manufacturer, model, firmware,
serial number, hardware ID, and optionally capability URLs. Continues on
failure. Password from `MONVIF_PASSWORD` (never stored in the TSV file).

JSON output example (`--format json --capabilities`):

```json
[
  {
    "name": "front-door",
    "ip": "192.168.1.10",
    "port": 80,
    "reachable": true,
    "authenticated": true,
    "manufacturer": "Acme",
    "model": "X200",
    "firmware": "2.1.0",
    "serial_number": "SN001",
    "hardware_id": "HW1",
    "capabilities": {
      "device_url": "http://192.168.1.10/onvif/device_service",
      "media_url": "http://192.168.1.10/onvif/media"
    }
  }
]
```

### info

```bash
monvif info --ip <ip> --user <user> [--port <port>] [--format table|json]
```

Returns manufacturer, model, firmware version, serial number, and hardware
ID. Default output is JSON.

### capabilities

```bash
monvif capabilities --ip <ip> --user <user> [--port <port>] [--format table|json]
```

Returns the ONVIF service endpoint URLs (Device, Media, Imaging, Events, PTZ).

### profiles

```bash
monvif profiles --ip <ip> --user <user> [--port <port>] [--format table|json]
```

Lists media profiles (token and name).

### stream-uri

```bash
monvif stream-uri --ip <ip> --user <user> [--port <port>] \
  [--profile-token <token>] [--transport rtsp|tcp|http|udp] [--format table|json]
```

Returns the stream URI for a profile. Uses the first profile if
`--profile-token` is omitted. Default transport is `rtsp`. Credentials
embedded in the returned URI are automatically redacted.

### snapshot-uri

```bash
monvif snapshot-uri --ip <ip> --user <user> [--port <port>] \
  [--profile-token <token>] [--format table|json]
```

Returns the snapshot (JPEG) URI for a profile. Credentials embedded in the
returned URI are redacted.

### imaging get

```bash
monvif imaging get --ip <ip> --user <user> [--port <port>] [--format table|json]
```

Reads current imaging settings: brightness, contrast, saturation, sharpness,
backlight mode, exposure mode, white balance mode, IR cut filter, and WDR mode.

Table output example:

```
FIELD               VALUE
brightness          50
contrast            50
saturation          50
sharpness           50
exposure_mode       Auto
white_balance_mode  Auto
ir_cut_filter       Auto
```

JSON output example:

```json
{
  "brightness": 50,
  "contrast": 50,
  "saturation": 50,
  "sharpness": 50,
  "exposure_mode": "Auto",
  "white_balance_mode": "Auto",
  "ir_cut_filter": "Auto"
}
```

### imaging set

```bash
monvif imaging set --ip <ip> --user <user> [--port <port>] \
  [--brightness N] [--contrast N] [--saturation N] [--sharpness N] \
  (--yes | --dry-run) [--format table|json]
```

Updates one or more imaging settings. **Requires `--yes` to apply changes.**
Use `--dry-run` to preview the before/after values without sending them to
the camera.

```bash
# Preview what would change:
monvif imaging set --ip 192.168.1.10 --user admin --brightness 60 --dry-run

# Apply changes:
monvif imaging set --ip 192.168.1.10 --user admin --brightness 60 --yes
```

Table output example:

```
FIELD       BEFORE  AFTER (applied)
brightness  50      60
```

Only the fields you specify are changed — all other settings are preserved.

### network

Read-only network inspection commands:

```bash
export MONVIF_PASSWORD=secret

# Full network summary
./monvif network get --ip 172.17.17.27 --port 81 --user ha
./monvif network get --ip 172.17.17.27 --port 81 --user ha --format json | jq .

# Individual queries
./monvif network interfaces --ip 172.17.17.27 --port 81 --user ha
./monvif network protocols  --ip 172.17.17.27 --port 81 --user ha
./monvif network dns        --ip 172.17.17.27 --port 81 --user ha
./monvif network ntp        --ip 172.17.17.27 --port 81 --user ha
./monvif network hostname   --ip 172.17.17.27 --port 81 --user ha
```

Always run `network interfaces` first to find the interface token before using `set-ip`.

Network write commands (require `--yes` to apply, support `--dry-run` to preview):

```bash
# Preview IP change (no modification)
./monvif network set-ip --ip 172.17.17.27 --port 81 --user ha \
  --interface <token> --dhcp --dry-run

./monvif network set-ip --ip 172.17.17.27 --port 81 --user ha \
  --interface <token> --address 172.17.17.27 --prefix-length 24 \
  --gateway 172.17.17.1 --dry-run

# Preview DNS/NTP/hostname changes
./monvif network set-dns      --ip 172.17.17.27 --port 81 --user ha --server 172.17.17.1 --dry-run
./monvif network set-ntp      --ip 172.17.17.27 --port 81 --user ha --server se.pool.ntp.org --dry-run
./monvif network set-hostname --ip 172.17.17.27 --port 81 --user ha --name front --dry-run

# Apply (use only when physically able to recover the camera):
./monvif network set-ip --ip 172.17.17.27 --port 81 --user ha \
  --interface <token> --address 172.17.17.27 --prefix-length 24 \
  --gateway 172.17.17.1 --yes
```

**Network safety rules:**

- Network write commands can make cameras permanently unreachable.
- Always run the read-only commands first to understand the current state.
- Always use `--dry-run` before applying any change.
- Use `--yes` only when you are physically or operationally able to recover the camera.
- Some cameras require a reboot after network changes (reported in the output).
- Some ONVIF cameras expose read-only network APIs or have incomplete `Set*` support.
- `--dhcp` and `--yes`/`--dry-run` are mutually exclusive with their counterparts.
- Due to a library limitation, `set-dns` and `set-ntp` only send the first `--server` value to the camera.

## Security

- **Use `MONVIF_PASSWORD`** instead of `--password` to keep credentials out
  of shell history and process listings.
- **Do not put passwords in `cameras.tsv`** — the file only holds name, IP,
  and port.
- When `MONVIF_PASSWORD` is unset and `--password` is omitted, monvif
  prompts interactively (requires a terminal).
- Passwords are never logged, even with `--debug`.

## Global flags

| Flag | Description |
|---|---|
| `--debug` | Show ONVIF RPC debug logs (suppressed by default) |

## Non-standard port

```bash
monvif info --ip 192.168.1.42 --port 8080 --user admin
```

Default ONVIF port is 80.

### diagnose

```bash
monvif diagnose --file cameras.tsv --user <user> \
  [--format table|json|markdown] [--output <file>] \
  [--only <name-or-ip>] \
  [--timeout <duration>] [--camera-timeout <duration>] \
  [--concurrency <n>] [--rtsp-port <port>]
```

Runs a comprehensive health check on every camera in the inventory file.
Per-camera checks:

1. TCP connect to ONVIF port
2. TCP connect to RTSP port (default 554)
3. ONVIF authentication and device info (manufacturer, model, firmware)
4. Capabilities (service endpoint URLs)
5. Media profiles (count and tokens)
6. Stream URI (RTSP, first profile; credentials redacted)
7. Snapshot URI (credentials redacted)
8. Imaging settings read
9. Network configuration read

Per-camera status:

| Status | Meaning |
|--------|---------|
| `OK`   | All core checks passed |
| `WARN` | Core OK; optional checks (snapshot/imaging/network) had issues |
| `FAIL` | TCP unreachable, auth failed, no profiles, or no stream URI |

WS-Discovery check is skipped by default (pass `--skip-discovery=false` once
implemented; many ONVIF cameras do not respond to multicast discovery even when
fully functional).

**Recommended first run:**

```bash
./monvif diagnose --file cameras.tsv --user ha --quick
```

`--quick` skips snapshot URI, imaging, network, and RTSP checks, leaving only
the core health checks (TCP, auth, capabilities, profiles, stream URI).
With default concurrency of 4 and 6 local cameras, `--quick` typically
completes in under 20 seconds.

```bash
export MONVIF_PASSWORD=secret

# Quick check — recommended starting point
./monvif diagnose --file cameras.tsv --user ha --quick

# All cameras at once (fastest for small inventories)
./monvif diagnose --file cameras.tsv --user ha --quick --concurrency 6

# Debug one camera by name
./monvif diagnose --file cameras.tsv --user ha --quick --only veranda
./monvif --debug diagnose --file cameras.tsv --user ha --quick --only veranda

# Serial execution (useful for debugging)
./monvif diagnose --file cameras.tsv --user ha --quick --concurrency 1

# Full diagnostics with 3s TCP timeout and 60s per-camera cap
./monvif diagnose --file cameras.tsv --user ha --timeout 3s --camera-timeout 60s

# JSON output for scripting
./monvif diagnose --file cameras.tsv --user ha --quick --format json | jq .

# Markdown report saved to file
./monvif diagnose --file cameras.tsv --user ha --format markdown --output diagnostics.md

# Skip individual expensive checks
./monvif diagnose --file cameras.tsv --user ha --skip-imaging --skip-network

# Debug ONVIF calls
./monvif --debug diagnose --file cameras.tsv --user ha --quick
```

**Timeout and concurrency flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--timeout` | `5s` | TCP dial timeout per low-level check |
| `--camera-timeout` | `60s` | Max total time per active camera |
| `--concurrency` | `4` | Number of cameras diagnosed simultaneously |

The per-camera timeout starts when a worker slot is acquired, not when the
camera is queued. A camera waiting for a free slot can never time out while
idle. If a camera does exceed its budget, any checks that completed before the
timeout are preserved in the output row.

**Skip flags:**

| Flag | Skips |
|------|-------|
| `--quick` | snapshot, imaging, network, RTSP TCP check |
| `--skip-snapshot` | snapshot URI check |
| `--skip-imaging` | imaging settings check |
| `--skip-network` | network configuration check |
| `--skip-rtsp` | TCP RTSP port check |
| `--skip-discovery` | WS-Discovery (default: always skipped) |

Skipped checks show `skip` in table output and do not count as failures.

Diagnostics are **read-only** — no camera settings are changed.

### version

```bash
monvif version
```

Prints the version, commit hash, build date, and Go runtime version. Example
output: `monvif v0.5.0 (commit abc1234, built 2026-05-20T12:00:00Z, go1.22.4)`.

## Release

Releases are built automatically when a version tag is pushed:

```bash
git tag v0.5.0
git push origin v0.5.0
```

The GitHub Actions release workflow builds binaries for Linux and macOS on
amd64 and arm64, packages each as a `.tar.gz` archive containing the binary,
`README.md`, and `LICENSE`, then publishes a GitHub Release with auto-generated
release notes.

## Run tests

```bash
go test ./...
# or via Make:
make test
```
