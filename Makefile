.PHONY: build dev ui clean release

ui:
	cd ui && npm run build

build: ui
	CGO_ENABLED=0 go build -o nestops ./cmd/nestops

build-linux-amd64: ui
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/nestops-linux-amd64 ./cmd/nestops

build-linux-arm64: ui
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/nestops-linux-arm64 ./cmd/nestops

release: build-linux-amd64 build-linux-arm64

dev:
	go run ./cmd/nestops

clean:
	rm -f nestops
	rm -rf dist/
