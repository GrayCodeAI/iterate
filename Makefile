.PHONY: build run chat test lint install

build:
	go build -o iterate ./cmd/iterate

run: build
	./iterate

evolve: build
	./iterate --evolve --repo .

test:
	go test ./...

lint:
	go vet ./...

install:
	go install ./cmd/iterate

clean:
	rm -f iterate
