.PHONY: build run chat test lint fmt vet check evolve install clean

build:
	go build -o iterate ./cmd/iterate

run: build
	./iterate

chat:
	go run ./cmd/iterate

evolve: build
	./iterate --evolve --repo .

test:
	go test ./...

lint:
	go vet ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

check: fmt vet build test

install:
	go install ./cmd/iterate

clean:
	rm -f iterate
