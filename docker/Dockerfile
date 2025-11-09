ARG VERSION=latest

FROM golang:alpine AS builder

RUN apk add --no-cache build-base zip wget && \
    wget https://github.com/O-X-L/micro-queue/archive/refs/heads/${VERSION}.zip -O /tmp/repo.zip && \
    cd /tmp && \
    unzip /tmp/repo.zip

RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/micro-queue /tmp/micro-queue-${VERSION}/cmd/main/main.go

FROM alpine:latest

RUN adduser -S micro-queue -G micro-queue && \
    mkdir -p /etc/micro-queue

RUN chown -R micro-queue:micro-queue /etc/micro-queue

COPY --from=builder /app/micro-queue /usr/local/bin/micro-queue

USER micro-queue

ENTRYPOINT ["/usr/local/bin/micro-queue"]
CMD ["-config", "/etc/micro-queue/config.yaml"]
