.PHONY: all clean deps plugins build

all: clean build

clean:
	go clean -i ./...
	sudo rm -rf tmp/
	sudo mkdir tmp

deps:
	rm -rf vendor
	govendor init
	govendor add +e

build:
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static"'
