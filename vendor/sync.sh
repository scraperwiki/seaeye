#!/bin/sh

export GOOS=linux
go list -f '{{join .Deps "\n"}}' github.com/scraperwiki/seaeye/cmd/seaeye \
| xargs go list -f '{{if not .Standard}}{{.ImportPath}}{{end}}' \
| egrep -v github.com/scraperwiki/seaeye/pkg \
> "$(dirname $(readlink -f "$0"))/dependencies"
