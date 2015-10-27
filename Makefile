all: dist

dist: dist/seaeye_darwin_amd64 dist/seaeye_linux_amd64

dist/seaeye_darwin_amd64:
	GOOS=darwin GOARCH=amd64 go build -o dist/seaeye_darwin_amd64

dist/seaeye_linux_amd64:
	GOOS=linux GOARCH=amd64 go build -o dist/seaeye_linux_amd64
