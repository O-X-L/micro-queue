# Micro Queue

[![Lint](https://github.com/O-X-L/micro-queue/actions/workflows/lint.yml/badge.svg?branch=latest)](https://github.com/O-X-L/micro-queue/actions/workflows/lint.yml)
[![Test](https://github.com/O-X-L/micro-queue/actions/workflows/test.yml/badge.svg)](https://github.com/O-X-L/micro-queue/actions/workflows/test.yml)

This is a very simple single-instance ultra-lightweight queue microservice that uses the filesystem for persistency.

* No additional server-side services required!

* No client dependencies - just simple HTTP calls.

* Running multiple instances in active-active mode is outside the scope of this project as it would require a much more complex logic.

  Running multiple nodes in failover- or active-standby-mode behind a load-balancer should be possible if you use a distributed filesystem. (*just make sure only one node is active at a time*)

----

## Usage

1. Create a Config file: [Example](https://github.com/O-X-L/micro-queue/blob/latest/testdata/config.yml)

2. Get the Executable

  * **Docker**: [oxlorg/micro-queue]() or [build it yourself]()

  * **Standalone**: [Download from Releases](https://github.com/O-X-L/micro-queue/releases) or [build it yourself](https://github.com/O-X-L/micro-queue?tab=readme-ov-file#build)

3. Start the Server:

  * **Docker**: `docker run --network=host -it --rm --name micro-queue --volume "$(pwd)/micro_queue:/app" --volume "$(pwd)/testdata/config.yml:/etc/micro-queue/config.yml" queue-local:latest` (*Note: use a volume to map the config & make the queue-data persistent*)

  * **Standalone**: `build/micro-queue -c $(pwd)/testdata/config.yml`

4. Use the queue:

  API: `http://<SERVER>:<PORT>/<in|out|compact>/<queue-name>`

```bash
# add job to queue - token requires 'post' permission
curl -v -X POST -H "Authorization: Bearer aa9d7b0a-8bee-47e4-887c-bdb799533793" http://127.0.0.1:10000/in/fetcher --data '{"context": "test"}'

# gets written to disk
tree /tmp/queue/
> /tmp/queue/
> ├── fetcher
> │   ├── data.log
> │   └── meta.json
> └── manager-reload
>     ├── data.log
>     └── meta.json

# read from queue - token requires 'get' permission
curl -v -X GET -H "Authorization: Bearer bb9d7b0a-8bee-47e4-887c-bdb799533773" http://127.0.0.1:10000/out/fetcher
> {"context": "test"}
```

For more detailed logging set the `MODE_DEBUG=1` env-var.

----

## Build

Use the script: `cd ${REPO} && bash scripts/build.sh`

Or manually: `cd ${REPO} && go build -o micro-queue ./cmd/main.go`

----

## Roadmap

* Adding a `broadcast` queue-mode where messages should be processed by every subscriber (one-to-many)
