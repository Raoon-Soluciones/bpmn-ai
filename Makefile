.PHONY: test test-unit test-integration test-e2e test-coverage bench fuzz lint tidy build

build:
	go build -o bin/bpmn-engine ./cmd/engine

test: test-unit test-integration test-e2e

test-unit:
	go test -race -count=1 ./internal/... ./pkg/...

test-integration:
	go test -race -count=1 -tags=integration ./internal/... ./pkg/...

test-e2e:
	go test -race -count=1 -tags=e2e ./api/...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -html=coverage.out -o coverage.html
	go tool cover -func=coverage.out | grep total

bench:
	go test -bench=. -benchmem -cpuprofile=cpu.prof ./internal/engine/

fuzz:
	go test -fuzz=Fuzz -fuzztime=30s ./pkg/bpmn/

lint:
	golangci-lint run

tidy:
	go mod tidy
	go mod verify
