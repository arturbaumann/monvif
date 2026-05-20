# monvif v0.3.0

## Highlights

- Add `imaging get` for reading camera image settings.
- Add `imaging set` for changing brightness, contrast, saturation, and sharpness.
- Add `--dry-run` and `--yes` safety flow for imaging changes.
- Add `snapshot-uri` command.
- Improve `stream-uri` reliability and transport handling.
- Redact credentials from returned URIs where applicable.
- Keep JSON output script-friendly where supported.

## Validation

Tested with:

~~sh
go test ./...
go build -o monvif .

./monvif imaging get --ip 172.17.17.27 --port 81 --user ha
./monvif imaging get --ip 172.17.17.27 --port 81 --user ha --format json | jq .
./monvif imaging set --ip 172.17.17.27 --port 81 --user ha --brightness 50 --dry-run
./monvif snapshot-uri --ip 172.17.17.27 --port 81 --user ha
./monvif stream-uri --ip 172.17.17.27 --port 81 --user ha --transport tcp
~~

## Known limitations

- Not all ONVIF cameras expose all imaging fields.
- Some cameras may return zero values for imaging settings even when controls exist.
- `imaging set --yes` changes real camera picture settings; use `--dry-run` first.
- `snapshot-uri` depends on camera Media service support.
