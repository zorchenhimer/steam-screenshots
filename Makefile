.PHONY: run all clean fmt

all: fmt bin/server bin/uploader

clean:
	-rm -r bin/

run: bin/server
	bin/server

fmt:
	go fmt ./...

bin/%: cmd/%.go *.go
	go build -o $@ $<
