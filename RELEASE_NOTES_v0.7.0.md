# Release Notes — monvif v0.7.0

## What's new

### `monvif stream profiles` — profile listing with stream analysis

A new command group that lists all ONVIF media profiles for a camera,
retrieves the RTSP stream URI for each, and optionally probes each stream
with `ffprobe` to verify it is reachable and report its codec, resolution,
FPS, and bitrate.

**Without `--probe`** — no external tools required:

```bash
export MONVIF_PASSWORD=secret

monvif stream profiles --ip 172.17.17.27 --port 81 --user ha
```

```
TOKEN            NAME            URI
protoken_ch0001  proname_ch0001  rtsp://172.17.17.27:554/1/1
protoken_ch0002  proname_ch0002  rtsp://172.17.17.27:554/1/2
protoken_ch0003  proname_ch0003  rtsp://172.17.17.27:554/1/3
```

**With `--probe`** — requires `ffprobe` (see below):

```bash
monvif stream profiles --ip 172.17.17.27 --port 81 --user ha --probe --transport tcp
```

```
TOKEN            NAME            CODEC  RESOLUTION  FPS  BITRATE  WORKS
protoken_ch0001  proname_ch0001  h264   2560x1440   25   -        yes
protoken_ch0002  proname_ch0002  h264   704x576     15   -        yes
protoken_ch0003  proname_ch0003  h264   352x288     10   -        yes
```

`BITRATE` shows `-` when the camera stream does not report bitrate data.
When reported, it is formatted compactly: `4.2M`, `512k`, `800b`.

The `ERROR` column is added only when at least one probe fails:

```
TOKEN            NAME            CODEC  RESOLUTION  FPS  BITRATE  WORKS  ERROR
protoken_ch0001  proname_ch0001  h264   2560x1440   25   -        yes
protoken_ch0002  proname_ch0002  -      -           -    -        no     RTSP auth failed
```

**JSON output** (`--format json | jq .`) — flat structure with a stable schema.
`bitrate` and `error` are always present so automation scripts can rely on
fixed field names without defensive null-checks:

```bash
monvif stream profiles --ip 172.17.17.27 --port 81 --user ha --probe --transport tcp --format json | jq .
```

```json
[
  {
    "token": "protoken_ch0001",
    "name": "proname_ch0001",
    "stream_uri": "rtsp://172.17.17.27:554/1/1",
    "codec": "h264",
    "width": 2560,
    "height": 1440,
    "resolution": "2560x1440",
    "fps": 25,
    "bitrate": "-",
    "works": true,
    "error": ""
  }
]
```

`bitrate` is `"-"` when the camera does not report it; `"4.2M"` / `"512k"` when it does.
`error` is `""` on success, or a human-readable message on failure.
`works` is omitted when `--probe` is not used.

Stream URIs are always redacted — credentials are replaced with `***`.
Passwords are never printed in output or error messages.

### Probe flags

| Flag | Default | Description |
|------|---------|-------------|
| `--probe` | off | Run ffprobe on each stream URI |
| `--transport` | `tcp` | RTSP transport (`rtsp`, `tcp`, `http`, `udp`); tcp is recommended for routed/WAN networks |
| `--probe-timeout` | `10s` | Per-stream ffprobe timeout |
| `--format` | `table` | Output format: `table` or `json` |

### Installing ffprobe

`ffprobe` is required only when `--probe` is used. Basic profile listing
works without it.

```bash
# Debian/Ubuntu
sudo apt update && sudo apt install -y ffmpeg

# macOS
brew install ffmpeg
```

If ffprobe is not installed and `--probe` is passed, the `WORKS` column
shows `no` and the `ERROR` column reports:
`ffprobe not found in $PATH: install ffprobe (https://ffmpeg.org/download)`

### Probe error classification

`--probe` maps common ffprobe failures to readable messages:

| Condition | Error shown |
|-----------|-------------|
| ffprobe binary missing | `ffprobe not found in $PATH: install ffprobe (...)` |
| RTSP 401 / auth failure | `RTSP auth failed` |
| Connection timed out | `RTSP connection timed out` |
| ffprobe killed by `--probe-timeout` | `ffprobe timed out` |
| No video stream in output | `no video stream found` |

## Bug fixes

### RTSP auth failed on all streams with `--probe` (fixed)

ONVIF cameras return stream URIs without embedded credentials
(e.g. `rtsp://172.17.17.27:554/1/1`). The initial v0.7 implementation passed
this bare URI to ffprobe, which then failed RTSP digest authentication.

**Fix:** Before invoking ffprobe, ONVIF credentials are injected into the
URI using `url.UserPassword` (which URL-encodes special characters, so
passwords containing `@`, `!`, and similar characters are handled correctly).
The credential-bearing URI is passed only to ffprobe as a process argument —
it never appears in output, error messages, or debug logs.

Sanitization chain:
- **Table / JSON output** — uses the original unauthenticated URI, redacted to `***@host/path`
- **Debug output** (`--debug`) — shows `rtsp://user:[REDACTED]@host/path`
- **Error messages** — credential URI is replaced with `<uri>` before display

### Redundant ONVIF calls eliminated

Previously, `stream profiles` called `GetProfiles` once at startup and then
once more per profile inside `GetStreamUri` (via `resolveProfile`). For a
camera with 3 profiles this meant 4 `GetProfiles` round-trips instead of 1.

Fixed by adding `GetStreamURIForToken` to the camera client — it calls
`GetStreamUri` directly with a known token, skipping the profile lookup.

### JSON schema stability

`bitrate` and `error` are always included in JSON output — even when the camera
does not report bitrate or the probe succeeds without error — so downstream
scripts and jq pipelines can depend on a fixed set of fields without extra
null-checks.

## Changed

- The existing `monvif profiles` command is preserved unchanged.
  `stream profiles` is a new command that extends it with URI retrieval and
  optional stream probing. Both commands remain available.

## Security

- Passwords are sourced from `MONVIF_PASSWORD` or `--password`; never logged.
- Stream URIs containing credentials are redacted before output or JSON
  serialisation.
- `ffprobe` is invoked via `exec.Command` (not a shell), so credentials in
  RTSP URIs are passed as a process argument and never appear in shell history.
- If ffprobe writes a credential-bearing URI to stderr, it is redacted before
  being shown to the user.

## Upgrade

```bash
go install github.com/arturbaumann/monvif@v0.7.0
# or use the install script:
VERSION=v0.7.0 curl -fsSL https://raw.githubusercontent.com/arturbaumann/monvif/main/scripts/install.sh | bash
```

## Validation

```sh
go fmt ./...
go test ./...
go vet ./...
go build -o monvif .
./monvif version

# Without probe (no ffprobe required):
MONVIF_PASSWORD=secret ./monvif stream profiles --ip <ip> --port 81 --user <user>

# With probe (requires ffprobe):
MONVIF_PASSWORD=secret ./monvif stream profiles --ip <ip> --port 81 --user <user> --probe --transport tcp

# JSON:
MONVIF_PASSWORD=secret ./monvif stream profiles --ip <ip> --port 81 --user <user> --probe --format json | jq .

# Verify no password leakage:
MONVIF_PASSWORD=secret ./monvif --debug stream profiles --ip <ip> --port 81 --user <user> --probe 2>&1 | grep -i password
```
