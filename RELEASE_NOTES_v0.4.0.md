# monvif v0.4.0

## Highlights

- Add ONVIF network inspection commands.
- Add `network get` for summary network information.
- Add `network interfaces` for interface token, MAC, MTU, DHCP, IPv4 and IPv6 state.
- Add `network dns`, `network ntp`, and `network hostname`.
- Add safe dry-run support for network changes.
- Add `network set-ip`, `network set-dns`, `network set-ntp`, and `network set-hostname`.
- Require explicit safety flow for write operations with `--dry-run` or `--yes`.

## Validation

Tested with:

~~sh
go test ./...
go build -o monvif .

./monvif network get --ip 172.17.17.27 --port 81 --user ha
./monvif network interfaces --ip 172.17.17.27 --port 81 --user ha
./monvif network protocols --ip 172.17.17.27 --port 81 --user ha
./monvif network dns --ip 172.17.17.27 --port 81 --user ha
./monvif network ntp --ip 172.17.17.27 --port 81 --user ha
./monvif network hostname --ip 172.17.17.27 --port 81 --user ha

./monvif network set-ip --ip 172.17.17.27 --port 81 --user ha --interface net --dhcp --dry-run
./monvif network set-ip --ip 172.17.17.27 --port 81 --user ha --interface net --address 172.17.17.27 --prefix-length 24 --gateway 172.17.17.1 --dry-run
./monvif network set-ntp --ip 172.17.17.27 --port 81 --user ha --server se.pool.ntp.org --dry-run
~~

## Observed camera behavior

On tested Tiandy camera:

- Interface token: `net`
- Interface name: `eth0`
- IPv4 DHCP: enabled
- IPv6: disabled
- Gateway: `172.17.17.1`
- DNS: from DHCP
- NTP: from DHCP
- Hostname: `HostName`

## Known limitations

- Network write commands can make cameras unreachable. Use `--dry-run` first.
- Actual `--yes` network writes were not validated during this release.
- Some ONVIF cameras may expose incomplete or read-only network APIs.
- Some cameras may return no network protocol list even when HTTP/RTSP services are active.
