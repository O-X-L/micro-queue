FROM golang:alpine AS builder

# BUILD: docker build -f Dockerfile -t queue-local --network=host --build-arg VERSION="$VERSION" --no-cache .
# RUN: docker run --network=host -it --rm --name micro-queue --volume "$(pwd)/micro_queue:/app" --volume "$(pwd)/../testdata/config.yml:/etc/micro-queue/config.yml" queue-local:latest

ARG VERSION

RUN test -n "$VERSION"

RUN apk add --no-cache build-base zip wget sed && \
    wget https://github.com/O-X-L/micro-queue/archive/refs/tags/${VERSION}.zip -O /tmp/repo.zip && \
    cd /tmp && \
    unzip /tmp/repo.zip

RUN cd /tmp/micro-queue-${VERSION} && \
    sed -i "s|^const\sVERSION\s=\s\".*\"$|const VERSION = \"${VERSION}\"|g" ./internal/config/config.go && \
    go mod tidy && \
    CGO_ENABLED=0 go build -ldflags="-w -s" -o /usr/local/bin/micro-queue ./cmd/main.go

FROM alpine:latest

ARG UID=7000
ARG USER=queue
ENV IN_DOCKER=1

RUN adduser --uid ${UID} --home /home/queue --shell /usr/sbin/nologin --disabled-password ${USER} ${USER} && \
    mkdir -p /etc/queue /app && \
    chown -R ${USER}:${USER} /etc/queue /app && \
    chmod 750 /etc/queue /app

VOLUME ["/app"]
COPY --from=builder /usr/local/bin/micro-queue /usr/local/bin/micro-queue

USER ${USER}

ENTRYPOINT ["/usr/local/bin/micro-queue"]
CMD ["-c", "/etc/queue/config.yml"]
