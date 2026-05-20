# monvif

A Linux CLI tool for ONVIF camera discovery and querying.

## Install / Build

```bash
git clone https://github.com/artur/monvif
cd monvif
go build -o monvif .
# Optionally install to $GOPATH/bin
go install .
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

monvif info         --ip 192.168.1.10 --user admin
monvif capabilities --ip 192.168.1.10 --user admin
monvif profiles     --ip 192.168.1.10 --user admin
monvif stream-uri   --ip 192.168.1.10 --user admin
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
monvif stream-uri --ip <ip> --user <user> [--port <port>] [--profile-token <token>] [--format table|json]
```

Returns the RTSP stream URI for a profile. Uses the first profile if
`--profile-token` is omitted.

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

## Run tests

```bash
go test ./...
```
