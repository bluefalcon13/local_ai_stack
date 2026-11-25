.PHONY: all build clean
APP_NAME := onnx-goworker

all: build

build:
	@echo "==> Compiling CGO-enabled Go binary for Linux AMD64..."
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o ../$(APP_NAME) .
	@echo "==> Built successfully: $(APP_NAME)"

clean:
	@echo "==> Cleaning up build artifacts..."
	rm -f ../$(APP_NAME)