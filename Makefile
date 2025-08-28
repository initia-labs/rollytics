# don't override user values of COMMIT and VERSION
ifeq (,$(COMMIT_HASH))
  COMMIT_HASH := $(shell git rev-parse HEAD 2>/dev/null || echo 'unknown')
endif

ifeq (,$(VERSION))
  VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo 'dev')
endif

LDFLAGS := -ldflags "-s -w -X main.Version=$(VERSION) -X main.CommitHash=$(COMMIT_HASH) -X main.BuildTime=$(shell date -u '+%Y-%m-%d_%H:%M:%S')"

.PHONY: test install

all: test

test:
	go test ./...

test-coverage:
	go test -race -coverprofile=coverage.out -covermode=atomic ./...
	go tool cover -func=coverage.out

install: go.sum
	go install -mod=readonly $(LDFLAGS) ./cmd/rollytics

clean:
	go clean
	rm -f rollytics

build: go.sum
	go build -mod=readonly $(LDFLAGS) -o rollytics ./cmd/rollytics

lint:
	golangci-lint run --fix --timeout=15m

swagger-gen:
	swag init -g cmd/api.go --output api/docs

docker:
	docker build \
		--build-arg VERSION=$(VERSION) \
		--build-arg COMMIT_HASH=$(COMMIT_HASH) \
		-t rollytics:$(VERSION) .
