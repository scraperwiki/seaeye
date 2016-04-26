VERSION?=$(shell git describe --tags --always --dirty)

all: build

build:
	go build -o dist/seaeye_darwin_amd64 -ldflags "-X main.version=$(VERSION)"

dist: dist/seaeye_darwin_amd64 dist/seaeye_linux_amd64

dist/seaeye_darwin_amd64:
	GOOS=darwin GOARCH=amd64 go build -o dist/seaeye_darwin_amd64 -ldflags "-X main.version=$(VERSION)"

dist/seaeye_linux_amd64:
	GOOS=linux GOARCH=amd64 go build -o dist/seaeye_linux_amd64 -ldflags "-X main.version=$(VERSION)"

rel: dist
	hub release create -a dist $(VERSION)
