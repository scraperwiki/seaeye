FROM golang:1.6.2

RUN curl -s -L -o /usr/local/bin/docker-compose https://github.com/docker/compose/releases/download/1.4.2/docker-compose-Linux-x86_64 \
    && chmod +x /usr/local/bin/docker-compose

RUN go get github.com/scraperwiki/seaeye

EXPOSE 8080

# The mandatory environment variables
#   - HOOKBOT_SUB_ENDPOINT
#   - GITHUB_USER
#   - GITHUB_TOKEN
#   - PORT
# are provided by systemd/hanoverd.
ENTRYPOINT ["seaeye"]
