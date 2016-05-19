FROM golang:1.6.2-alpine
MAINTAINER Uwe Dauernheim <uwe@scraperwiki.com>

## Install runtime and build dependencies
RUN set -x \
 && apk add --no-cache \
    ca-certificates \
    curl \
    git \
    make \
    openssh \
    perl \
    py-pip \
    sudo \
 && rm -rf /var/cache/apk/* /tmp/*

## Install Docker
## See: https://github.com/docker-library/docker/blob/f7ee50684c7ec92ce885c8b93a4ed22ddbb660f8/1.11/Dockerfile
RUN set -x \
 && curl -fSL "https://get.docker.com/builds/Linux/x86_64/docker-1.11.1.tgz" -o docker.tgz \
 && echo "893e3c6e89c0cd2c5f1e51ea41bc2dd97f5e791fcfa3cee28445df277836339d *docker.tgz" | sha256sum -c - \
 && tar -xzvf docker.tgz \
 && mv docker/* /usr/local/bin/ \
 && rmdir docker \
 && rm docker.tgz

## Install Docker Compose
RUN pip install --upgrade pip docker-compose

## Allow run docker commands against daemon without having to prefix docker with
## `sudo` by giving a docker proxy script sudo rights.
COPY buildfiles/docker         /usr/local/sbin/docker
COPY buildfiles/docker-compose /usr/local/sbin/docker-compose
RUN echo 'nobody ALL=(ALL) NOPASSWD:SETENV: /usr/local/bin/docker, /usr/bin/docker-compose' >> /etc/sudoers

## Configure nobody user
RUN set -x \
 && mkdir -p /home/nobody/.ssh \
 && ssh-keyscan github.com >> /home/nobody/.ssh/known_hosts \
 && chown -R nobody:nogroup /home/nobody
ENV HOME=/home/nobody

## Configure seaeye (install vendor first for docker container caching)
COPY vendor /go/src/github.com/scraperwiki/seaeye/vendor
RUN go install -v github.com/scraperwiki/seaeye/vendor/...
COPY cmd /go/src/github.com/scraperwiki/seaeye/cmd
COPY server /go/src/github.com/scraperwiki/seaeye/server
RUN go install -v github.com/scraperwiki/seaeye/cmd/seaeye
RUN set -x  \
 && mkdir -p /seaeye/log /seaeye/workspace \
 && chown -R nobody:nogroup /seaeye
WORKDIR /seaeye

VOLUME /seaeye/workspace
USER nobody:nogroup
EXPOSE 19515
ENTRYPOINT ["seaeye"]
