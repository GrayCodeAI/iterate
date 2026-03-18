.PHONY: build run chat test lint install

build:
	go build -o iterate ./cmd/iterate

run: build
	./iterate --repo .

chat: build
	./iterate --chat --repo .

test:
	go test ./...

lint:
	go vet ./...

install:
	go install ./cmd/iterate

clean:
	rm -f iterate
