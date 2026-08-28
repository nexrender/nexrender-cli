.PHONY: build fmt generate test vet check snapshot

build:
	go build -o bin/nexrender ./cmd/nexrender

fmt:
	gofmt -w cmd internal openapi skills

generate:
	go generate ./internal/api

test:
	go test ./...

vet:
	go vet ./...

check: generate fmt test vet build
	bash -n scripts/install.sh

snapshot:
	goreleaser release --snapshot --clean
