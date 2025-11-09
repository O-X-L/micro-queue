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

ENV IN_DOCKER=1

RUN addgroup -S queue && \
    adduser -S queue -G queue && \
    mkdir -p /etc/micro-queue /app && \
    chown -R queue:queue /etc/micro-queue /app

COPY --from=builder /usr/local/bin/micro-queue /usr/local/bin/micro-queue

USER queue

ENTRYPOINT ["/usr/local/bin/micro-queue"]
CMD ["-c", "/etc/micro-queue/config.yml"]
