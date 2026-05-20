# monvif v0.6.3

## Highlights

- Fix remaining install-script README references from `artur/monvif` to `arturbaumann/monvif`.
- Correct raw GitHub install URLs for `scripts/install.sh`.
- Keep README installation examples aligned with the public repository path.

## Validation

Tested with:

~~sh
go fmt ./...
go test ./...
go build -o monvif .
./monvif version
~~

## Notes

Versions up to `v0.6.0` used the wrong Go module path. Use `v0.6.1` or newer for `go install`.
Versions before `v0.6.3` may contain stale README examples using `github.com/artur/monvif`.
