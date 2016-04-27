FROM alpine

EXPOSE 19515

ENTRYPOINT ["seaeye"]

WORKDIR /tmp

# The mandatory environment variables
#   - HOOKBOT_SUB_ENDPOINT
#   - GITHUB_USER
#   - GITHUB_TOKEN
# are provided by systemd/hanoverd.

RUN apk add --no-cache curl
RUN curl -s -L -o /usr/local/bin/seaeye https://github.com/scraperwiki/seaeye/releases/download/1.2/seaeye_linux_amd64 \
 && chmod +x /usr/local/bin/seaeye
