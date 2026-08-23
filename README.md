# Asset Transaction Service

Boilerplate Go service for BETS V2. The repository currently contains only the HTTP application lifecycle, configuration loading, health endpoint, container build, CI, and development hooks.

## Requirements

- Go 1.27.0
- Docker, when building the container image

## Run locally

```bash
cp .env.example .env
go run ./main.go
```

The service listens on `PORT` and exposes `GET /health`.

## Development commands

```bash
make test
make vet
make build
make setup-hooks
```
