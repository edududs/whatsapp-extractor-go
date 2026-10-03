.PHONY: build test check fmt fuzz

build:
	CGO_ENABLED=0 go build -trimpath -o bin/whatsapp-extractor-go ./cmd/whatsapp-extractor-go

test:
	go test -race ./...

fmt:
	gofmt -w .

check:
	test -z "$$(gofmt -l .)"
	go mod verify
	go vet ./...
	go tool staticcheck ./...
	go test -race ./...

fuzz:
	go test ./domain -run '^$$' -fuzz FuzzParseJID -fuzztime 10s
	go test ./adapters/whatsapp -run '^$$' -fuzz FuzzMap -fuzztime 10s
