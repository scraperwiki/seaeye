VERSION?=$(shell git describe --tags --always --dirty)

sm:
	git submodule update --init --recursive

docker-build: sm
	docker build -t seaeye .

docker-run: docker-build
	docker run --rm -it -p 8080:19515 -v workspace:/seaeye/workspace seaeye

build: sm
	go build -ldflags "-X main.version=$(VERSION)" ./cmd/seaeye

dist: dist/seaeye_darwin_amd64 dist/seaeye_linux_amd64

dist/seaeye_darwin_amd64:
	GOOS=darwin GOARCH=amd64 go build -o dist/seaeye_darwin_amd64 -ldflags "-X main.version=$(VERSION)" ./cmd/seaeye

dist/seaeye_linux_amd64:
	GOOS=linux GOARCH=amd64 go build -o dist/seaeye_linux_amd64 -ldflags "-X main.version=$(VERSION)" ./cmd/seaeye

rel: dist
	hub release create -a dist $(VERSION)

.PHONY: build dist docker-run docker-build rel sm
