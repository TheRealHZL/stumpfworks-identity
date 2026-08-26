.PHONY: build build-server build-agent build-pam-helper build-client-status build-linux-amd64 build-camera-macos run-server run-agent test lint clean
build: build-server build-agent build-pam-helper build-client-status
build-server:
	@mkdir -p bin
	go build -o bin/sw-badge-server ./cmd/server
build-agent:
	@mkdir -p bin
	go build -o bin/sw-badge-agent ./cmd/agent
build-pam-helper:
	@mkdir -p bin
	go build -o bin/sw-badge-pam-helper ./cmd/pam-helper
build-client-status:
	@mkdir -p bin
	go build -o bin/sw-badge-client-status ./cmd/client-status
build-linux-amd64:
	@mkdir -p dist/linux-amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/linux-amd64/sw-badge-server ./cmd/server
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/linux-amd64/sw-badge-pam-helper ./cmd/pam-helper
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/linux-amd64/sw-badge-client-status ./cmd/client-status
build-agent-linux:
	@mkdir -p dist/linux-amd64
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -o dist/linux-amd64/sw-badge-agent ./cmd/agent
build-camera-macos:
	@mkdir -p bin
	@mkdir -p /tmp/swbadge-swift-cache
	swiftc -module-cache-path /tmp/swbadge-swift-cache cmd/camera-macos/main.swift -framework AVFoundation -framework CoreMedia -framework Foundation -framework Vision -Xlinker -sectcreate -Xlinker __TEXT -Xlinker __info_plist -Xlinker deploy/macos/camera-info.plist -o bin/sw-badge-camera-macos
run-server:
	go run ./cmd/server --config config.yaml
run-agent:
	go run ./cmd/agent
test:
	go test ./...
lint:
	go vet ./...
clean:
	rm -f bin/sw-badge-server bin/sw-badge-agent bin/sw-badge-pam-helper bin/sw-badge-client-status
	rm -rf dist/linux-amd64
