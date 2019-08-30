.PHONY: run all clean fmt

all: fmt bin/server.exe bin/uploader.exe

clean:
	rm bin/*.*

run: bin/server.exe
	bin/server.exe

fmt:
	go fmt ./...

bin/%.exe: cmd/%.go *.go
	go build -o $@ $<
