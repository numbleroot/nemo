.PHONY: all clean deps plugins build

all: clean build

clean:
	go clean -i ./...
	sudo rm -rf results/ tmp/db/* tmp/logs/*

deps:
	rm -rf vendor
	govendor init
	govendor add +e

plugins:
	sudo rm -rf tmp/plugins
	sudo mkdir -p tmp/plugins
	sudo curl -o tmp/plugins/apoc-3.3.0.2-all.jar https://github.com/neo4j-contrib/neo4j-apoc-procedures/releases/download/3.3.0.2/apoc-3.3.0.2-all.jar

build:
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static"'
