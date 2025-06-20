VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 'dev')
COMMIT_HASH ?= $(shell git rev-parse HEAD 2>/dev/null || echo 'unknown')
LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT_HASH) -X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"

.PHONY: test install

all: test

test:
	go test ./...

install: go.sum
	go install $(LDFLAGS) ./cmd

clean:
	go clean
	rm -f rollytics

build: go.sum
	go build $(LDFLAGS) -o rollytics ./cmd

lint:
	golangci-lint run --fix --output-format=tab --timeout=15m

swagger-gen:
	swag init -g cmd/api.go --output api/docs

docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT_HASH=$(COMMIT_HASH) \
		-t rollytics .