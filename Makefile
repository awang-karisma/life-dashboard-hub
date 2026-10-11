.PHONY: dev swagger build embed test test-pg docker clean

SWAG := $(shell which swag 2>/dev/null || echo $(shell go env GOPATH)/bin/swag)

dev:
	go run ./cmd/server

swagger:
	$(SWAG) init -g cmd/server/main.go -o docs

build: swagger
	go build -o bin/life-dashboard-hub ./cmd/server

embed: swagger
	rm -rf cmd/server/dist && cp -r web/dist cmd/server/dist
	go build -tags embed_frontend -o bin/life-dashboard-hub ./cmd/server
	rm -rf cmd/server/dist

test:
	go test -v ./...

test-pg:
	DATABASE_URL=$(DATABASE_URL) go test -v ./...

docker:
	docker build -t life-dashboard-hub .

clean:
	rm -rf bin/ docs/
