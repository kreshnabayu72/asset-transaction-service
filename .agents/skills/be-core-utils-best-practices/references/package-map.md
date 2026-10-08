# be-core-utils reference sources

This skill deliberately does not copy the API inventory: `be-core-utils`
changes often, and a duplicated symbol list goes stale the moment a package is
edited. The canonical, always-current source is the repository itself.

## Canonical source

- Repository: <https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils> (default branch `main`)
- Module path: `gitea.tagsamurai.local/Wangsit-Developer/be-core-utils`
- Local checkout (when available): `~/workspace/gitea.tagsamurai.local/Wangsit-Developer/be-core-utils`

## Read the version the service actually uses

Run these inside the service repo (not this skills repo) — the module version in
its `go.mod` is the exact code the service compiles against, which may lag the
`main` branch:

```sh
go list -m gitea.tagsamurai.local/Wangsit-Developer/be-core-utils  # version in use
go doc gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/service  # API for one package
go doc -all gitea.tagsamurai.local/Wangsit-Developer/be-core-utils | less
```

Treat `main` as the next release, not necessarily current behavior.

## Package index

Each package links to its directory on the default branch; read the source and
tests there for current signatures and semantics.

| Package | Purpose |
| --- | --- |
| [`cache`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/cache) | Typed, fail-soft Valkey JSON cache |
| [`config`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/config) | Env loading, DSN and outbox relay-interval helpers |
| [`constant`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/constant) | Shared header names and PeerDB/Kafka constants |
| [`db`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/db) | Postgres/Mongo/Redis/Valkey connection constructors, snowflake |
| [`dto`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/dto) | Pagination and option DTOs shared across services |
| [`entity`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/entity) | `HttpError`, workflow errors and the `UseCase` shape |
| [`helper`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/helper) | Pure generic utilities, HTTP response/export/Excel/query helpers |
| [`http`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/http) | Handler timeout wrapper with panic recovery |
| [`logger`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/logger) | The one process-wide `log/slog` setup |
| [`migrations`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/migrations) | Migration activities and tenant-schema mapping |
| [`outbox`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/outbox) | Transactional outbox repository and Kafka relay |
| [`pipeline`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/pipeline) | MongoDB aggregation pipeline builder and query fragments |
| [`provider`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/provider) | Time, password, reCAPTCHA, DB and schedule providers (+ mocks) |
| [`replication`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/replication) | PeerDB/Kafka CDC replicator (see its README) |
| [`service`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/service) | Service implementations, interfaces, DI manager, mocks and fakes |
| [`sql_query`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/sql_query) | Select/insert/update/delete builders, conditions, scanning |
| [`temporal_helper`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/temporal_helper) | Temporal execution helpers and activity/child options |
| [`validator`](https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/src/branch/main/validator) | Shared go-playground validator and error parsing |

## Where to look for specific answers

- **Logging standard** — `logger/logger.go` package comment (12-factor rules,
  `LOG_FORMAT`/`LOG_LEVEL`).
- **Transaction lifecycle** — `service/postgresql_service.go`
  (`UseTransactions` deprecation note; `SetTransaction`, `CommitTransaction`,
  `RollbackTransaction` do not clear the handle — clear it with
  `SetTransaction(nil)`).
- **SQL builder DSL** — `sql_query/builder_*_interface.go` and
  `builder_*_impl.go`, plus `builder_select_impl_test.go` for usage examples.
- **Mongo aggregation** — `pipeline/pipelinebuilder.go` and
  `pipeline/pipeline_queries.go`.
- **Outbox design** — package and `Relay` comments in `outbox/outbox.go` and
  `outbox/relay.go`; `Insert` must run on a transaction-enlisted service.
- **CDC and replica identity** — `replication/README.md`.
- **Migrations and tenant schemas** — `migrations/migrations.go` package
  comment.
- **Test doubles** — `service/mock_*.go`, `service/fake_*.go`,
  `provider/mock_*.go`.
- **Temporal defaults** — `temporal_helper/options.go`, `temporal_helper/temporal.go`.
- **Error and response envelope** — `entity/http_error.go`,
  `helper/response.go`.
- **Cache semantics** — `cache/engine.go` (fail-soft `Get`/`Set`, `Incr` for
  counters).
