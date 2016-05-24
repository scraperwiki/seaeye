VERSION?=$(shell git describe --tags --always --dirty)

sm:
	git submodule update --init --recursive

build: sm
	docker build -t seaeye .

run: build
	docker run --rm -it \
		-p 19515:19515 \
		--env-file=seaeye.env \
		seaeye

.PHONY: build run sm
