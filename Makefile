.PHONY: build run clean

BINARY_NAME=golang-test-server

build:
	mkdir -p bin
	go build -o bin/$(BINARY_NAME) main.go

run:
	go run main.go

clean:
	rm -rf bin
