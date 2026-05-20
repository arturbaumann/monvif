# monvif v0.6.2

## Highlights

- Fix remaining README/install references from `github.com/artur/monvif` to `github.com/arturbaumann/monvif`.
- Keep Go module path aligned with the public GitHub repository.

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
