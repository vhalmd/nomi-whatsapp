ui-dev:
	cd ui/v1 && pnpm dev

build-ui:
	cd ui/v1 && pnpm build

build-windows: build-ui
	@go build -ldflags -H=windowsgui -o bin/nomi-whatsapp-windows-amd64.exe cmd/windows/main.go

run-windows: build-ui
	@go run cmd/windows/main.go