.PHONY: all build test test-race test-cover lint clean android-build android-apk release-binaries

# Go parameters
BINARY_NAME := magic-guardian
MAIN_PKG := ./cmd/magic-guardian/
LDFLAGS := -s -w

# Android NDK (override with: make android-build NDK=/path/to/ndk)
ANDROID_HOME ?= /opt/homebrew/share/android-commandlinetools
NDK ?= $(ANDROID_HOME)/ndk/27.2.12479018
ANDROID_CC := $(NDK)/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android26-clang
ANDROID_CXX := $(NDK)/toolchains/llvm/prebuilt/darwin-x86_64/bin/aarch64-linux-android26-clang++
JAVA_HOME ?= /opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home

all: test build

## Build

build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY_NAME) $(MAIN_PKG)

## Test

test:
	go test ./internal/... -count=1

test-race:
	go test ./internal/... -race -count=1

test-cover:
	go test ./internal/... -coverprofile=coverage.out -count=1
	go tool cover -func=coverage.out | tail -1
	@echo "Run 'go tool cover -html=coverage.out' for detailed report"

test-verbose:
	go test ./internal/... -race -v -count=1

## Lint

lint:
	@which golangci-lint > /dev/null 2>&1 || (echo "Install: brew install golangci-lint" && exit 1)
	golangci-lint run ./...

vet:
	go vet ./...

## Android

android-build:
	CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=$(ANDROID_CC) CXX=$(ANDROID_CXX) \
		go build -ldflags="$(LDFLAGS)" -o android/app/src/main/jniLibs/arm64-v8a/libguardian.so $(MAIN_PKG)

android-apk: android-build
	ANDROID_HOME=$(ANDROID_HOME) JAVA_HOME=$(JAVA_HOME) \
		android/gradlew -p android assembleDebug
	@echo "APK: android/app/build/outputs/apk/debug/app-debug.apk"

## Release binaries (for local cross-compilation on macOS)

release-binaries:
	@mkdir -p releases
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o releases/$(BINARY_NAME)-darwin-amd64 $(MAIN_PKG)
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o releases/$(BINARY_NAME)-darwin-arm64 $(MAIN_PKG)
	@echo "macOS binaries built. Linux/Windows require native build or CI."

## Clean

clean:
	rm -f $(BINARY_NAME) coverage.out
	rm -rf android/app/build android/.gradle
	rm -f releases/$(BINARY_NAME)-*
