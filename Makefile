.PHONY: build test clean install

BINARY=envmerge
VERSION?=dev

build:
	go build -ldflags "-X github.com/stackgen-cli/envmerge/cmd.version=$(VERSION)" -o $(BINARY) .

test:
	go test -v ./...

clean:
	rm -f $(BINARY)

install: build
	mv $(BINARY) $(GOPATH)/bin/

fmt:
	go fmt ./...

lint:
	golangci-lint run

.DEFAULT_GOAL := build
