.PHONY: all clean build

all: clean build

clean:
	go clean -i ./...
	rm -rf tmp/

deps:
	rm -rf vendor
	govendor init
	govendor add +e

build:
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static"'
