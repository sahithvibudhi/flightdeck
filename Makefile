.PHONY: build dev ui clean release test

ui:
	cd ui && npm run build

build: ui
	CGO_ENABLED=0 go build -o flightdeck ./cmd/flightdeck

build-linux-amd64: ui
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/flightdeck-linux-amd64 ./cmd/flightdeck

build-linux-arm64: ui
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/flightdeck-linux-arm64 ./cmd/flightdeck

release: build-linux-amd64 build-linux-arm64

test:
	go test ./internal/... -count=1

dev:
	go run ./cmd/flightdeck

clean:
	rm -f flightdeck
	rm -rf dist/
