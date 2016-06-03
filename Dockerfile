FROM golang:1.6.2-alpine
MAINTAINER Uwe Dauernheim <uwe@scraperwiki.com>

## Install runtime and build dependencies
RUN set -x \
 && apk add --no-cache \
    bash \
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
## See: https://github.com/docker-library/docker/blob/master/1.10/Dockerfile
RUN set -x \
 && curl -fSL "https://get.docker.com/builds/Linux/x86_64/docker-1.10.3" -o /usr/local/bin/docker \
 && echo "d0df512afa109006a450f41873634951e19ddabf8c7bd419caeb5a526032d86d  /usr/local/bin/docker" | sha256sum -c - \
 && chmod +x /usr/local/bin/docker

## Install Docker-Compose
RUN pip install --upgrade docker-compose==1.7.1

## Allow run docker commands against daemon without having to prefix docker with
## `sudo` by giving a docker proxy script sudo rights.
COPY buildfiles/docker         /usr/local/sbin/docker
COPY buildfiles/docker-compose /usr/local/sbin/docker-compose
RUN echo 'nobody ALL=(ALL) NOPASSWD:SETENV: /usr/local/bin/docker, /usr/bin/docker-compose' >> /etc/sudoers

## Configure nobody user
COPY buildfiles/config /home/nobody/.ssh/config
COPY buildfiles/known_hosts /home/nobody/.ssh/known_hosts
RUN set -x \
 && sed -i 's#nobody:x:65534:65534:nobody:/:/sbin/nologin#nobody:x:65534:65534:nobody:/home/nobody:/sbin/nologin#' /etc/passwd \
 && mkdir -p /home/nobody /seaeye/logs /seaeye/workspace \
 && chown -R nobody:nogroup /home/nobody /seaeye
ENV HOME=/home/nobody

## Configure environment
COPY buildfiles/known_hosts /etc/ssh/ssh_known_hosts
USER nobody:nogroup
ENTRYPOINT ["seaeye"]
EXPOSE 19515
WORKDIR /seaeye
VOLUME /seaeye/logs /seaeye/workspace

## Configure seaeye (install vendor first for docker container caching)
COPY vendor /go/src/github.com/scraperwiki/seaeye/vendor
RUN go install -v $(cat /go/src/github.com/scraperwiki/seaeye/vendor/dependencies)
COPY cmd /go/src/github.com/scraperwiki/seaeye/cmd
COPY pkg /go/src/github.com/scraperwiki/seaeye/pkg
RUN go install -v -ldflags "-X main.version=$SEAEYE_VERSION" github.com/scraperwiki/seaeye/cmd/seaeye
