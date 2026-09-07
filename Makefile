.PHONY: test vet run-core run-node

test:
	go test ./...

vet:
	go vet ./...

run-core:
	go run ./cmd/core-api

run-node:
	go run ./cmd/signer-node
