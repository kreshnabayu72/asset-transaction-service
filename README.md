# Asset Transaction Service

Boilerplate Go service for BETS V2. The repository currently contains only the HTTP application lifecycle, configuration loading, health endpoint, container build, CI, and development hooks.

## Requirements

- Go 1.27.0
- Docker, when building the container image
- Bash (Git Bash on Windows) and Make for the existing scripts/Makefile
- Optional: Task and golangci-lint v2 for the development commands below

## Run locally

```bash
cp .env.example .env
go run ./main.go
```

The service listens on `PORT` and exposes `GET /health`.

On PowerShell, use `Copy-Item .env.example .env` before `go run ./main.go`.
The current configuration reads only `PORT` (default `5013`) and
`SERVICE_NAME` (default `asset-transaction-service`). `ENV` and `JWT_KEY` in
the example environment are placeholders; authentication is not implemented.

## Run with Docker Compose

```bash
docker compose up -d --build
docker compose logs -f asset-transaction
docker compose down
```

Compose exposes the health endpoint at `http://localhost:5013/health` by
default, bound to loopback for local development. Set `ASSET_TRANSACTION_PORT`
to change the host port. Production ingress must go through the gateway.
No database, cache, or workflow infrastructure is required by this skeleton.

## Development commands

```bash
make test
make vet
make build
make setup-hooks
```

Alternatively, use `task dev`, `task test`, `task vet`, `task build`,
`task lint`, or `task setup-hooks`. Task's build produces a native binary in
`bin/`; Make's build targets Linux. Hook setup runs through Bash and sets
the repository's `core.hooksPath` to `.githooks`.

## Repository documentation

- [Engineering standards](engineering.md): shared Go conventions and Definition of Done.
- [Platform architecture](architecture.md): service boundaries and platform design.
- [Agent instructions](AGENTS.md): implementation and review rules.
- [Bruno collection](docs/bruno/README.md): the implemented health endpoint.

The shared standards and agent skills are copied from `../iam-service`.
They describe the platform target; this repository currently implements only
`main.go`, `internal/app`, and `internal/config`. Domain modules, models,
migrations, authorization, and workflows must be added when their feature
specifications require them.
