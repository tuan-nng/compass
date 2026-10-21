COMMIT := $(shell git rev-parse HEAD 2>/dev/null)
LDFLAGS := -X main.commit=$(COMMIT)
# The pinned okf, where test/lib.sh's okf_bin and the Go tests look for it.
OKF_PINNED := .cache/pinned/$(shell sed -n 's/^OKF_TAG=//p' pins/okf.env)/okf

.PHONY: build install test

# bin/compass, for running from the clone.
build:
	go build -ldflags '$(LDFLAGS)' -o bin/compass ./cmd/compass

# compass into $(go env GOBIN), else $(go env GOPATH)/bin.
install:
	go install -ldflags '$(LDFLAGS)' ./cmd/compass

test:
	go vet ./...
	test -x $(OKF_PINNED) || go run ./cmd/compass okf install --dest $(dir $(OKF_PINNED))
	go test ./...
	./test/run-all.sh
