.PHONY: all clean reset deps build

all: clean build

clean:
	go clean -i ./...
	sudo rm -rf tmp/*

reset:
	go clean -i ./...
	sudo docker-compose down
	sudo rm -rf tmp/*
	sudo rm -rf results/*

build:
	CGO_ENABLED=0 go build -ldflags '-extldflags "-static"'
