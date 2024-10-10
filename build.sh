#!/bin/bash

# Create bin directory if it doesn't exist
mkdir -p bin

# Linux AMD64
echo "Building for Linux (amd64)..."
GOOS=linux GOARCH=amd64 go build -o bin/nomi-whatsapp-linux-amd64 cmd/generic/main.go
if [ $? -ne 0 ]; then
    echo "Error building for Linux (amd64)"
    exit 1
fi

# Linux ARM64
echo "Building for Linux (arm64)..."
GOOS=linux GOARCH=arm64 go build -o bin/nomi-whatsapp-linux-arm64 cmd/generic/main.go
if [ $? -ne 0 ]; then
    echo "Error building for Linux (arm64)"
    exit 1
fi

# Windows AMD64
echo "Building for Windows (amd64)..."
GOOS=windows GOARCH=amd64 go build -ldflags -H=windowsgui -o bin/nomi-whatsapp-windows-amd64.exe cmd/windows/main.go
if [ $? -ne 0 ]; then
    echo "Error building for Windows (amd64)"
    exit 1
fi

# macOS AMD64
echo "Building for macOS (amd64)..."
GOOS=darwin GOARCH=amd64 go build -o bin/nomi-whatsapp-macos-amd64 cmd/generic/main.go
if [ $? -ne 0 ]; then
    echo "Error building for macOS (amd64)"
    exit 1
fi

# macOS ARM64
echo "Building for macOS (amd64)..."
GOOS=darwin GOARCH=arm64 go build -o bin/nomi-whatsapp-macos-arm64 cmd/generic/main.go
if [ $? -ne 0 ]; then
    echo "Error building for macOS (amd64)"
    exit 1
fi

echo "All builds completed successfully."
