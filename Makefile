# speedify-check developer tasks.
# The binary currently builds from the repository root package; this
# moves to ./cmd/speedify-check when the thin main is extracted.

BINARY := speedify-check

.PHONY: build run test lint fmt vet tidy clean

build:
	go build -o bin/$(BINARY) .

run:
	go run .

test:
	go test -race ./...

lint:
	golangci-lint run

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
