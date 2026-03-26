VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -X github.com/pusk-platform/pusk/internal/api.Version=$(VERSION)
BINARY = pusk

.PHONY: build run test lint deploy

build:
	go build -o $(BINARY) -ldflags "$(LDFLAGS)" ./cmd/pusk/
	@echo "Built $(BINARY) $(VERSION)"

run: build
	./$(BINARY)

test:
	go test ./... -count=1 -timeout 60s

lint:
	go vet ./...
	gofmt -l .

deploy: build
	@echo "Use: make build && scp pusk prod:/path/"
