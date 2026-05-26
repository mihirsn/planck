# Parsing Docker Logs

Planck natively supports analyzing logs directly from running Docker containers using the `--docker` flag. This avoids the need to pipe `docker logs` manually or deal with temporary log files.

## How it works

When you pass the `--docker` flag:

```bash
planck analyze --docker my-api
```

Planck communicates directly with the local Docker daemon using the official Docker Engine API (`github.com/docker/docker/client`). 

### Handling stdout and stderr

Docker containers typically emit access logs to `stdout` and error logs to `stderr`. Planck automatically multiplexes both streams together, ensuring that you don't miss any JSON log lines regardless of whether the application wrote them to standard output or standard error.

### Stream Filtering

To keep memory usage low and execution fast, Planck passes time filtering arguments (`--since`, `--tail`) directly to the Docker Engine. This means the Docker daemon does the heavy lifting of filtering the logs before sending them to Planck over the local socket.

## Known Limitations

- Currently, Planck only connects to the local Docker daemon socket (`/var/run/docker.sock`). Remote Docker hosts are not yet supported.
- Planck analyzes one container at a time. It cannot currently aggregate logs across multiple containers or read from Docker Compose directly (though this is on the roadmap).
