.PHONY: dev swagger build embed test test-pg docker docker-standalone docker-all clean

SWAG := $(shell which swag 2>/dev/null || echo $(shell go env GOPATH)/bin/swag)
VERSION ?= dev
COMMIT  ?= $(shell git rev-parse --short HEAD)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -ldflags "-s -w -X $(shell head -1 go.mod | awk '{print $$2}')/internal/version.Version=$(VERSION) -X $(shell head -1 go.mod | awk '{print $$2}')/internal/version.Commit=$(COMMIT) -X $(shell head -1 go.mod | awk '{print $$2}')/internal/version.Date=$(DATE)"

dev:
	go run ./cmd/server

swagger:
	$(SWAG) init -g cmd/server/main.go -o docs

build: swagger
	go build $(LDFLAGS) -o bin/life-dashboard-hub ./cmd/server

embed: swagger
	rm -rf cmd/server/dist && cp -r web/dist cmd/server/dist
	go build -tags embed_frontend $(LDFLAGS) -o bin/life-dashboard-hub ./cmd/server
	rm -rf cmd/server/dist

test:
	go test -v ./...

test-pg:
	DATABASE_URL=$(DATABASE_URL) go test -v ./...

docker:
	docker build -t life-dashboard-hub .

docker-standalone:
	docker build -t life-dashboard-hub:latest-standalone -f Dockerfile.standalone .

docker-all: docker docker-standalone

clean:
	rm -rf bin/ docs/
