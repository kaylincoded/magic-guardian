.PHONY: all build test test-race test-cover lint clean android-build android-apk release-binaries

# Go parameters
BINARY_NAME := magic-guardian
MAIN_PKG := ./cmd/magic-guardian/
VERSION ?= dev
VERSION_PKG := github.com/kaylincoded/magic-guardian/internal/updater
LDFLAGS := -s -w -X $(VERSION_PKG).Version=$(VERSION)

# Android NDK - auto-detect platform (override with: make android-build NDK=/path/to/ndk)
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Darwin)
    ANDROID_HOME ?= /opt/homebrew/share/android-commandlinetools
    NDK_HOST := darwin-x86_64
    JAVA_HOME ?= /opt/homebrew/opt/openjdk@21/libexec/openjdk.jdk/Contents/Home
else
    ANDROID_HOME ?= $(HOME)/android-sdk
    NDK_HOST := linux-x86_64
    JAVA_HOME ?= $(shell find /usr/lib/jvm -maxdepth 1 -type d -name 'java-*-openjdk' | sort -V | tail -1)
endif
NDK ?= $(ANDROID_HOME)/ndk/27.2.12479018
ANDROID_CC := $(NDK)/toolchains/llvm/prebuilt/$(NDK_HOST)/bin/aarch64-linux-android26-clang
ANDROID_CXX := $(NDK)/toolchains/llvm/prebuilt/$(NDK_HOST)/bin/aarch64-linux-android26-clang++

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

# x86-64 compiler for emulator testing
ANDROID_CC_X86 := $(NDK)/toolchains/llvm/prebuilt/$(NDK_HOST)/bin/x86_64-linux-android26-clang
ANDROID_CXX_X86 := $(NDK)/toolchains/llvm/prebuilt/$(NDK_HOST)/bin/x86_64-linux-android26-clang++

android-build:
	CGO_ENABLED=1 GOOS=android GOARCH=arm64 CC=$(ANDROID_CC) CXX=$(ANDROID_CXX) \
		go build -ldflags="$(LDFLAGS)" -o android/app/src/main/jniLibs/arm64-v8a/libguardian.so $(MAIN_PKG)

# Build for x86-64 emulator testing (not included in releases)
android-build-x86:
	CGO_ENABLED=1 GOOS=android GOARCH=amd64 CC=$(ANDROID_CC_X86) CXX=$(ANDROID_CXX_X86) \
		go build -ldflags="$(LDFLAGS)" -o android/app/src/main/jniLibs/x86_64/libguardian.so $(MAIN_PKG)

# Build APK with arm64 only (for releases)
android-apk: android-build
	ANDROID_HOME=$(ANDROID_HOME) JAVA_HOME=$(JAVA_HOME) \
		android/gradlew -p android assembleDebug
	@echo "APK: android/app/build/outputs/apk/debug/app-debug.apk"

# Build APK with both arm64 and x86-64 (for local emulator testing)
android-apk-dev: android-build android-build-x86
	ANDROID_HOME=$(ANDROID_HOME) JAVA_HOME=$(JAVA_HOME) \
		android/gradlew -p android assembleDebug
	@echo "APK (dev): android/app/build/outputs/apk/debug/app-debug.apk"

## Release binaries (for local cross-compilation on macOS)

release-binaries:
	@mkdir -p releases
	GOOS=darwin GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o releases/$(BINARY_NAME)-darwin-amd64 $(MAIN_PKG)
	GOOS=darwin GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o releases/$(BINARY_NAME)-darwin-arm64 $(MAIN_PKG)
	GOOS=linux GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o releases/$(BINARY_NAME)-linux-amd64 $(MAIN_PKG)
	GOOS=linux GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o releases/$(BINARY_NAME)-linux-arm64 $(MAIN_PKG)
	GOOS=windows GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o releases/$(BINARY_NAME)-windows-amd64.exe $(MAIN_PKG)
	@echo "Release binaries built in releases/"

## Clean

clean:
	rm -f $(BINARY_NAME) coverage.out
	rm -rf android/app/build android/.gradle
	rm -f releases/$(BINARY_NAME)-*
