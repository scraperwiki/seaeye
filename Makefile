build:
	GOOS=darwin GOARCH=amd64 go build -o seaeye-darwin-amd64 github.com/scraperwiki/seaeye
	GOOS=linux GOARCH=amd64 go build -o seaeye-linux-amd64 github.com/scraperwiki/seaeye
