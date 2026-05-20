BINARY    := monvif
MODULE    := github.com/artur/monvif
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT    ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE      ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS   := -X $(MODULE)/cmd.version=$(VERSION) \
             -X $(MODULE)/cmd.commit=$(COMMIT) \
             -X $(MODULE)/cmd.date=$(DATE)

.PHONY: fmt test build install clean version snapshot

fmt:
	go fmt ./...

test:
	go test ./...

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install:
	go install -ldflags "$(LDFLAGS)" .

clean:
	rm -f $(BINARY)
	rm -rf dist/

version: build
	./$(BINARY) version

snapshot:
	@mkdir -p dist
	@for GOOS in linux darwin; do \
	  for GOARCH in amd64 arm64; do \
	    echo "Building $$GOOS/$$GOARCH..."; \
	    GOOS=$$GOOS GOARCH=$$GOARCH go build -ldflags "$(LDFLAGS)" -o dist/$(BINARY)-$$GOOS-$$GOARCH .; \
	    tar -czf dist/$(BINARY)-$(VERSION)-$$GOOS-$$GOARCH.tar.gz \
	      -C dist $(BINARY)-$$GOOS-$$GOARCH \
	      -C .. README.md LICENSE; \
	    rm dist/$(BINARY)-$$GOOS-$$GOARCH; \
	  done; \
	done
	@echo "Snapshot archives in dist/"
