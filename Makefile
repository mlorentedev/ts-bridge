.PHONY: build test lint security shellcheck check cover clean

BINARY  := ts-bridge
COVER   := coverage.out

build:
	go build -o $(BINARY) .

test:
	go test -race -v ./...

lint:
	golangci-lint run

security:
	gosec ./...

shellcheck:
	shellcheck scripts/dev.sh scripts/client/run.sh scripts/client/bootstrap.sh

cover:
	go test -race -coverprofile=$(COVER) ./...
	go tool cover -func=$(COVER)

check: lint test security shellcheck

clean:
	rm -f $(BINARY) $(COVER)
