VERSION?=$(shell git describe --tags --always --dirty)

all: build

sm:
	git submodule update --init --recursive

build: sm
	go build -ldflags "-X main.version=$(VERSION)" ./cmd/seaeye

dist: dist/seaeye_darwin_amd64 dist/seaeye_linux_amd64

dist/seaeye_darwin_amd64:
	GOOS=darwin GOARCH=amd64 go build -o dist/seaeye_darwin_amd64 -ldflags "-X main.version=$(VERSION)" ./cmd/seaeye

dist/seaeye_linux_amd64:
	GOOS=linux GOARCH=amd64 go build -o dist/seaeye_linux_amd64 -ldflags "-X main.version=$(VERSION)" ./cmd/seaeye

rel: dist
	hub release create -a dist $(VERSION)
