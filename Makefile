.PHONY: run all clean fmt

all: bin/server bin/uploader

clean:
	-rm -r bin/

run: bin/server
	bin/server

fmt:
	go fmt ./...

bin/%: cmd/%.go *.go
	go build -o $@ $<
