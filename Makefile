COMMIT := $(shell git rev-parse HEAD 2>/dev/null)
LDFLAGS := -X main.commit=$(COMMIT)

.PHONY: build install test

# bin/compass, for running from the clone.
build:
	go build -ldflags '$(LDFLAGS)' -o bin/compass ./cmd/compass

# compass into $(go env GOBIN), else $(go env GOPATH)/bin.
install:
	go install -ldflags '$(LDFLAGS)' ./cmd/compass

test:
	go vet ./...
	go test ./...
	./test/run-all.sh
