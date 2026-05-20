# monvif v0.2.0

## Highlights

- Clean default output with debug logs hidden unless `--debug` is used.
- `check` supports both table and JSON output.
- `check --capabilities` includes ONVIF service URLs.
- Long-running checks can show progress with `--progress auto|always|never`.
- README documents WS-Discovery limitations and inventory-based workflows.

## Validation

Tested with:

~~~sh
go test ./...
go build -o monvif .
./monvif check --file cameras.tsv --user ha
./monvif check --file cameras.tsv --user ha --capabilities
./monvif check --file cameras.tsv --user ha --format json | jq .
./monvif check --file cameras.tsv --user ha --capabilities --format json | jq .
~~~

## Known limitation

`monvif discover` only finds cameras that respond to WS-Discovery multicast UDP 3702. Some ONVIF cameras work with direct IP queries but do not appear in discovery.
