VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS = -X github.com/pusk-platform/pusk/internal/api.Version=$(VERSION)
BINARY = pusk

.PHONY: build run test lint build-demo deploy

build:
	go build -o $(BINARY) -ldflags "$(LDFLAGS)" ./cmd/pusk/
	@echo "Built $(BINARY) $(VERSION)"

build-demo:
	go build -tags demo -o $(BINARY) -ldflags "$(LDFLAGS)" ./cmd/pusk/
	@echo "Built $(BINARY) $(VERSION) [demo]"

run: build
	./$(BINARY)

test:
	go test ./... -count=1 -timeout 60s

lint:
	go vet ./...
	gofumpt -l .

deploy:
	@bash scripts/deploy.sh
