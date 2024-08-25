.PHONY: run all clean fmt

HASH=$(shell git rev-parse HEAD)
VERSION=$(shell git describe --abbrev=0 --tags)

all: bin/server bin/uploader

clean:
	-rm -r bin/

run: bin/server
	bin/server

fmt:
	go fmt ./...

bin/%: cmd/%.go *.go
	CGO_ENABLED=0 go build -ldflags "-X github.com/zorchenhimer/steam-screenshots.gitCommit=${HASH} -X github.com/zorchenhimer/steam-screenshots.version=${VERSION}" -o $@ $<
