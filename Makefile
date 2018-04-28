.PHONY: all clean deps plugins build

all: clean build

clean:
	go clean -i ./...
	rm -rf results/*
	sudo rm -rf tmp/*

deps:
	rm -rf vendor
	govendor init
	govendor add +e

build:
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static"'
