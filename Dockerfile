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
 && rm -rf /var/cache/apk/* /tmp/* \
 && pip install --upgrade pip

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
RUN pip install --upgrade docker-compose

## Allow run docker commands against daemon without having to prefix docker with
## `sudo` by giving a docker proxy script sudo rights.
COPY buildfiles/docker         /usr/local/sbin/docker
COPY buildfiles/docker-compose /usr/local/sbin/docker-compose
RUN echo 'nobody ALL=(ALL) NOPASSWD:SETENV: /usr/local/bin/docker, /usr/bin/docker-compose' >> /etc/sudoers

## Configure nobody user
RUN set -x \
 && mkdir -p /home/nobody /seaeye/logs /seaeye/ssh /seaeye/workspace \
 && chown -R nobody:nogroup /home/nobody /seaeye
ENV HOME=/home/nobody

## Configure environment
COPY buildfiles/known_hosts /etc/ssh/ssh_known_hosts
USER nobody:nogroup
COPY buildfiles/entrypoint /seaeye/entrypoint
EXPOSE 19515
ENTRYPOINT ["/seaeye/entrypoint"]
WORKDIR /seaeye
VOLUME /seaeye/logs /seaeye/ssh /seaeye/workspace

## Configure seaeye (install vendor first for docker container caching)
COPY vendor /go/src/github.com/scraperwiki/seaeye/vendor
RUN go install -v $(cat /go/src/github.com/scraperwiki/seaeye/vendor/dependencies)
COPY cmd /go/src/github.com/scraperwiki/seaeye/cmd
COPY pkg /go/src/github.com/scraperwiki/seaeye/pkg
RUN go install -v -ldflags "-X main.version=$SEAEYE_VERSION" github.com/scraperwiki/seaeye/cmd/seaeye
