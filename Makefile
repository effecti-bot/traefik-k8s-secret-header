.PHONY: vendor test lint

vendor:
	go mod tidy
	go mod vendor

test:
	go test -v -cover ./...

lint:
	golangci-lint run
