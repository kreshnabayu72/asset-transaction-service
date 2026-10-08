---
name: be-core-utils-best-practices
description: Conventions and best practices for the internal Go library gitea.tagsamurai.local/Wangsit-Developer/be-core-utils, shared by Wangsit-Developer and BETS-V2 backend services. Use when writing or reviewing Go code that imports be-core-utils packages (logger, helper, cache, config, db, service, sql_query, pipeline, outbox, replication, provider, entity, dto, migrations, validator, temporal_helper, http) or when about to write a utility that may already exist in the library.
license: MIT
metadata:
  author: refan
  version: "1.0.0"
---

# be-core-utils best practices

`be-core-utils` is the shared Go toolkit for Wangsit-Developer and BETS-V2 backend
services: structured logging, generic helpers, DB services, SQL and MongoDB
query builders, cache, outbox/CDC plumbing, providers, migrations, validation
and Temporal helpers.

Import path: `gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/<pkg>`.

## When to Apply

- Writing or reviewing Go code in a Wangsit-Developer or BETS-V2 service that imports `be-core-utils`.
- Wiring logging, config, connections, services, transactions, SQL/Mongo access, cache, outbox or Temporal.
- Choosing between a shared utility, provider or test double and a new local implementation.
- Reviewing changes that touch migrations, tenant schemas, replication or event publishing.

## Authoritative source

This skill carries the stable rules, not the whole API. Before relying on a
signature, check the version the service pins against the repository:

- Repository: <https://gitea.tagsamurai.local/Wangsit-Developer/be-core-utils> (default branch `main`)
- Local checkout: `~/workspace/gitea.tagsamurai.local/Wangsit-Developer/be-core-utils`
- `go list -m gitea.tagsamurai.local/Wangsit-Developer/be-core-utils`
- `go doc gitea.tagsamurai.local/Wangsit-Developer/be-core-utils/<pkg>`

Package-level index: [references/package-map.md](references/package-map.md).

Several packages carry design docs in their package comments (`logger`,
`migrations`, `outbox`, `replication/README.md`) — read those before changing
behavior.

## Ground rules

1. **Reuse before writing.** If a helper, provider or builder exists here, use
   it. Do not add a second implementation of the same thing in a service.
2. **Depend on the interfaces, not the concrete types.** Constructors return
   `*PostgreSQLService`, `*MongoDBService`, etc.; repositories and use cases
   should accept `service.PostgreSQLServicer`, `service.MongoDBServicer`,
   `service.ValkeyServiceInterface`, … so mocks and fakes can be injected.
3. **Keep transport concerns at the edges.** Services and repositories return
   errors; handlers translate them to `entity.HttpError` and send responses.
4. **Every shared write that must survive a crash goes through the same
   transaction as its event** (see Transactions and Outbox below).
5. **Never string-concatenate user input into SQL.** Use `sql_query` builders
   and positional args.
6. **Run the repo checks.** `make setup-hooks` once per clone; the pre-push
   hook runs `gofmt`, `go vet` and `go build`.

## Package map (details in references/package-map.md)

| Package | Use for |
| --- | --- |
| `logger` | The one process-wide `log/slog` setup |
| `config` | Env loading, DSN and relay-interval helpers |
| `constant` | Shared header names and PeerDB/Kafka constants |
| `helper` | Pure generic utilities, HTTP response helpers, export/Excel, query parsing |
| `entity` | `HttpError`, HTTP error constructors, workflow errors, `UseCase` |
| `dto` | Pagination and option DTOs shared across services |
| `db` | Postgres/Mongo/Redis/Valkey connection constructors, snowflake |
| `service` | Service implementations + interfaces, DI `Manager`, mocks and fakes |
| `provider` | Time, password, reCAPTCHA, DB, schedule providers (+ mocks) |
| `sql_query` | Fluent select/insert/update/delete builders, conditions, scanning, case conversion |
| `pipeline` | MongoDB aggregation pipeline builder |
| `cache` | Typed, fail-soft Valkey JSON cache |
| `outbox` | Transactional outbox repository and Kafka relay |
| `replication` | PeerDB/Kafka CDC replicator |
| `migrations` | Migration activities and tenant-schema mapping |
| `validator` | Shared go-playground validator + error parsing |
| `temporal_helper` | Temporal execution helpers, activity/child options |

## 1. Logging

Call `logger.Init()` **exactly once**, in `main`, after config is loaded and
before anything logs. Everywhere else use `log/slog` — the package installs the
default handler, so `slog.*Context` calls route correctly with no per-call
setup. Application code must not know where logs land (no HTTP log clients, no
"send to logger" calls).

```go
func main() {
	appConfig := config.LoadConfig()
	logger.Init() // decides text vs JSON from LOG_FORMAT, level from LOG_LEVEL
	...
}
```

- Prefer the `*Context` variants and key/value pairs:
  `slog.WarnContext(ctx, "cache: set failed", "key", key, "err", err)`.
- `LOG_FORMAT=json` belongs in the deploy env (containers); leave it unset
  locally for readable text. Source `file:line` is added automatically.
- `log.Printf`/`fmt.Println` are only acceptable before `logger.Init()` and in
  package-level deprecation/panic paths.

## 2. Errors and HTTP responses

- Construct transport errors with `entity`: `entity.BadRequest(msg, err...)`,
  `Unauthorized`, `Forbidden`, `NotFound`, `Conflict`,
  `UnprocessableEntity`, `InternalServerError`.
- Normalize unknown errors with `entity.ToHttpError(err)`, then send at the
  handler boundary with `SendResponse(c)`.
- For 2xx responses prefer `helper.Success` / `helper.Created` (uniform
  `{message, data, error}` envelope) over hand-built `c.JSON`.
- In Temporal activities return `entity.WorkflowBadRequest(msg, true)` etc. — the
  `nonRetryable` flag stops retries for business failures. Convert at the edge
  with `entity.ParseWorkflowError(workflowName, err)` or `WrapHttpError`.
- Use `entity.UseCase[P, R]` for the invoke/`InitService` shape when it fits.

```go
func (h *Handler) Borrow(c fiber.Ctx) error {
	res, err := h.usecase.Invoke(param)
	if err != nil {
		return entity.ToHttpError(err).SendResponse(c)
	}
	return helper.Success(c, "Successfully borrowed asset", res)
}
```

## 3. Pure helpers — don't reimplement

`helper` is the first place to look before writing a small function:

- Slices: `Filter`, `Map`, `ForEach`, `All`, `Any`, `Find`, `IndexOf`,
  `Distinct`, `DistinctBy`, `Reduce`, `Contains`, `Intersect`.
- Pointers/values: `MakePointer`, `FromPtr`, `FromPtrOr`, `Empty`,
  `Ternary`, `IsZeroValue`, `ComparePointers`.
- Scalars: `Increment`, `GenerateRandomString`, `JSONStringify`, `JSONParse`,
  `ParseStringOrJSONArray`, `SerializeJSON`, `IntoString/IntoInt/IntoFloat/
  IntoBool/IntoTime` (for untyped external values).
- HTTP: `SendResponse`, `Success`, `Created`, `BadRequest`, `Unauthorized`,
  `Forbidden`, `NotFound`, `InternalServerError`.
- Results: `FormatPaginationResult`, `FormatListResult` (return empty slices
  instead of `nil`).
- Query params: `ParseQuery[T](map[string]string)` driven by `transform` tags
  (`int`, `float64`, `bool`, `array`, `objectId`, `objectIdArray`, `date`).
- Files: `Export`/`ExportColumns` (CSV/XLSX), `SanitizeFilename`,
  `DefaultExcelParser` (`GetRow`, `GetRows`), `ExcelColumnIndex` mapped getters.
- Dates/frequency: `DateFormat`, `ConvertFrequencyToNumber`, custom frequency.
- Testing: `SendTestRequest(t, app, method, path, body, clientID, userID)`.

## 4. Config and connections

- `config.LoadConfig(filenames...)` once at startup; it loads `.env` via
  godotenv and returns `ConfigOptions`.
- `config.PostgresDSN(dbName)` builds DSNs — don't assemble them by hand.
- `config.RelayInterval()` is the outbox relay cadence.
- Connections: `db.ConnectPostgres(dbName, opts...)`,
  `db.ConnectMongo()`, `db.ConnectRedis(opts...)`, `db.ConnectValkey(opts...)`;
  Mongo helpers `db.GetDB`, `db.GetCollection`; snowflake via
  `db.InitSnowflake()` / `db.Node`.

## 5. Services and dependency injection

- Construct with the `Make*` funcs: `service.MakePostgreSQLService`,
  `MakeMongoDBService`, `MakeValkeyService`, `MakeRedisService`,
  `MakeMQTTService`, `MakeEmailService`, `MakeTemporalService`.
- `service.Manager` (`MakeManager`) handles lifecycle: `Register` calls
  `Init()`, `Unregister` calls `Close()`, `Get` retrieves. Register once during
  wiring; don't hold concrete pointers.
- `PostgreSQLService.Debug(level...)` is a local debugging aid (0–3); leave it
  out of production paths.
- Transactions: the service holds a transaction handle. After
  `svc.SetTransaction(tx)`, every query made through that service joins the
  transaction; **always clear it** (`defer svc.SetTransaction(nil)`), because
  `CommitTransaction`/`RollbackTransaction` do not clear the handle and later
  queries then fail with "tx is closed".

```go
_, err := service.UseTransactions(ctx, svc.GetPool(), func(tx pgx.Tx) (struct{}, error) {
	svc.SetTransaction(tx)
	defer svc.SetTransaction(nil)
	return struct{}{}, repo.Save(ctx, svc, row)
})
```

`UseTransactions` is marked DEPRECATED in its doc — prefer explicit
`Begin`/`SetTransaction`/`CommitTransaction`/`RollbackTransaction` in new code.
`holdCommit=true` is almost never what you want: it leaves the caller
responsible for closing the tx and can exhaust the pool.

## 6. SQL access (Postgres)

Two layers, used together: `sql_query` builds SQL, `PostgreSQLServicer`
executes and scans it.

```go
query, args, err := sql_query.NewSQLSelectBuilder[assetRow]("assets", "a").
	Where(sql_query.WhereQuery{
		"a.id": {Operator: sql_query.SQLOperatorEqual, Value: id},
	}).
	Build()
if err != nil {
	return err
}

var rows []assetRow
if err := svc.SelectMany(&rows, ctx, query, args...); err != nil {
	return err
}
```

- Entry points: `NewSQLSelectBuilder[T]`, `NewSQLCountBuilder`,
  `NewSQLInsertBuilder`, `NewSQLUpdateBuilder`, `NewSQLDeleteBuilder`.
- Insert: `.Insert(values, returningColumns...)` (default `RETURNING id`),
  `.Conflict(constraint, do)`, `.Build()`.
- Update: `.Update(sql_query.UpdateQuery{...})` (auto-adds `updated_at = NOW()`
  unless `skipUpdatedAt`), `.UpdateEach(slice, "id")` for batch,
  `.Where(...)`, `.Build()`. Raw expressions use
  `sql_query.UpdateRawSQL{Expr: "...", Args: ...}` (placeholders are rewritten).
- Delete: `.Delete(returningColumns...)`, `.Where(...)`, `.Build()`.
- Conditions: `WhereQuery` is `map[string]SQLCondition`; set `Operator`
  (`SQLOperatorEqual`, `SQLOperatorIn`, `SQLOperatorIsNull`, …), `Value`,
  and the `IsRef`/`IsSubQuery`/`IsArray`/`IsEpochTime`/`ExtraArgs` flags when
  the filter is not a plain literal. `WhereOr` accumulates OR groups;
  `MultiFilterCondition` expresses `AND (… OR …)`.
- Convenience methods: `CountWithFilter`, `InsertOneWithData` (struct/map →
  insert with optional `ReturningConfig`), `SelectOne`, `SelectMany`,
  `Execute`.
- Manual pgx scanning: `sql_query.ScanRowObject` / `ScanRowsArray` (cached
  variants exist) map json-tagged struct fields to columns.
- Metadata/caching: `ExtractJSONTags`, `MakeMapJSONTags*`, `FieldMapCache`;
  call `sql_query.ClearCache()` in tests that register new struct shapes.
- Pagination: build with `sql_query.PaginationQuery` and return
  `dto.PaginationResult[T]` via `helper.FormatPaginationResult`.
- Case conversion helpers: `CamelToSnake`, `ToCamelCase`, `PascalToCamelCase`.

### Database-First Processing & Advanced `sql_query` Patterns

Always push transformations, computations, formatting, aggregation, filtering, and tree traversals down to PostgreSQL via `sql_query`. Keep Go application processing at an absolute minimum (permissible only when genuinely impossible in SQL).

#### 1. Column Formatting & Expressions in `.Select(...)`
Project computed expressions and format dates/strings directly in `.Select(...)` instead of parsing in Go:
- **Date/Timestamp**: `to_char(a.created_at, 'YYYY-MM-DD"T"HH24:MI:SS"Z"') AS created_at`, `EXTRACT(EPOCH FROM a.created_at) * 1000 AS created_at_ms`
- **Null handling / Fallbacks**: `COALESCE(u.phone, '-') AS phone`, `COALESCE(u.description, '') AS description`
- **String manipulations**: `LOWER(TRIM(u.email)) AS email`, `CONCAT(u.first_name, ' ', u.last_name) AS full_name`

#### 2. Conditional Values & Branching (`CASE WHEN`)
Use `SelectCaseWhen` or `SelectMultiCaseWhen` instead of looping/evaluating in Go `if`/`switch` blocks:
```go
// Single condition
builder.SelectCaseWhen(
	sql_query.CaseWhenPart{When: "u.status = 1", Then: "'Active'"},
	"'Inactive'",
	"status_label",
)

// Multiple conditions
parts := []sql_query.CaseWhenPart{
	{When: "quantity > ?", Then: "'High'", Args: []any{100}},
	{When: "quantity > ?", Then: "'Medium'", Args: []any{50}},
}
builder.SelectMultiCaseWhen(parts, "'Low'", "stock_level")
```

#### 3. CTEs & Hierarchical Tree Traversals (`WITH` & `WITH RECURSIVE`)
For hierarchical structures (category trees, organization hierarchies, location paths), use recursive CTEs. Never recursively fetch and assemble node trees in Go memory:
```go
// Standard CTE
cte := sql_query.NewSQLSelectBuilder[Order]("orders", "o").Select("id", "user_id")
builder.WithCTEBuilder("recent_orders", cte.(*sql_query.SelectBuilder).SQLEloquentQuery).
	Join("recent_orders ro", "ro.user_id = u.id")

// Recursive CTE for tree traversals
baseBuilder := sql_query.NewSQLSelectBuilder[Category]("categories", "c").
	Select("c.id", "c.name", "c.parent_id", "1 AS depth").
	Where(sql_query.WhereQuery{"c.parent_id": {Operator: sql_query.SQLOperatorIsNull}})

recursiveBuilder := sql_query.NewSQLSelectBuilder[Category]("categories", "c").
	Select("c.id", "c.name", "c.parent_id", "ct.depth + 1 AS depth").
	Join("category_tree ct", "c.parent_id = ct.id")

builder.WithRecursiveCTEBuilder(
	"category_tree",
	baseBuilder.UnionAll(recursiveBuilder.(*sql_query.SelectBuilder).SQLEloquentQuery).(*sql_query.SelectBuilder).SQLEloquentQuery,
).Join("category_tree ct", "ct.id = items.category_id")
```

#### 4. Lateral Joins & Correlated Subqueries
```go
builder.CrossJoinLateral(
	"SELECT (a.purchase_cost - (vars.monthly_depreciation * vars.months_elapsed)) AS book_value",
	"value",
)
builder.LeftJoinLateralWithQuery(
	"recent_activity",
	activityBuilder.(*sql_query.SelectBuilder).SQLEloquentQuery,
	"recent_activity.user_id = u.id",
)
```

#### 5. JSON & Array Aggregations
Eliminate N+1 queries and Go-side map grouping by building nested JSON arrays/objects directly in SQL:
```go
// Aggregating child rows into a JSON array on parent entity
builder.SelectJSONAggregate(
	"roles",
	map[string]string{"id": "r.id", "name": "r.name"},
	"r.is_active = TRUE",
	true, // asArrayAggregation
	"r.name ASC",
)

// With COALESCE fallback (e.g. empty array '[]' instead of null)
builder.SelectJSONAggregateCoalesce(
	"tags",
	map[string]string{"id": "t.id", "label": "t.name"},
	"t.deleted_at IS NULL",
	true,
	"'[]'::jsonb",
)
```

#### 6. Pagination Summary Aggregates
Calculate summary metrics (e.g. conditional counts, sums) across the filtered dataset directly in the pagination CTE using `Pagination.Aggregates` with `Pagination.ExtraFilteredColumns`:
```go
builder.Paginate(sql_query.Pagination{
	Page:  1,
	Limit: 10,
	ExtraFilteredColumns: []string{"users.is_default"},
	Aggregates: []sql_query.PaginationAggregate{
		{
			JSONKey:    "totalDisabledRecords",
			Expression: "COUNT(id) FILTER (WHERE is_default = TRUE)",
		},
	},
})
```
*Never run separate standalone counting queries or Go loops to compute envelope summary counts.*

#### 7. Multi-Field & Array Search
```go
// Searches text columns and unnested array columns with case-insensitive ILIKE
builder.Search(keyword, []string{"name", "code", "tags:array"})
```

## 7. MongoDB

- Use `MongoDBServicer` (`Find`, `FindOne`, `InsertOne/Many`,
  `UpdateOne/Many`, `FindOneAndUpdate`, `DeleteOne/Many`, `Aggregate`,
  `AggregateExplain`).
- Build aggregations with `pipeline.NewPipelineBuilder()` and its stages
  (`Match`, `Group`, `Project`, `Lookup`, `Unwind`, `AddFields`, …) rather
  than assembling `bson.D` by hand; keep pipelines in repository code.
  `pipeline_queries.go` holds reusable query fragments.
- Schemaless does not mean constraint-free: map Mongo model names through the
  shared `db`/model-name constants used by the service.

## 8. Cache

```go
engine := cache.NewEngine(valkeySvc, 10*time.Minute)

asset, ok := cache.Get[Asset](ctx, engine, key)
if !ok {
	asset, err = repo.FindByID(ctx, id) // cache miss: rebuild from DB
	if err != nil {
		return err
	}
	engine.Set(ctx, key, asset)
}
```

- `cache.Get[T]` returns `(value, ok)`; a miss, decode failure or transport
  error all yield `ok=false`. Fall back to the database; never fail a request
  because the cache is down.
- `Set` is fail-soft (logs and swallows). Use `SetWithErr` when the caller must
  know a write failed (e.g. to invalidate other keys).
- Default TTL is 24h; pass a TTL to `NewEngine` or per-call to `Set`.
- Counters: use `ValkeyServiceInterface.Incr`, never GET-then-SET (races lose
  increments).

## 9. Outbox (durable events)

- Insert events through `outbox.OutboxRepository.Insert(ctx, svc, partitionKey,
  payload)` **with the same `PostgreSQLServicer` that has the business
  transaction set** — an insert on its own connection is a promise to publish
  something that may never have been committed.
- A payload only needs `Topic() string`.
- The `Relay` drains the outbox into RedPanda; run it as a goroutine in the
  owning service: `outbox.NewRelay(dbName, repo, broker, interval).Run(ctx)`
  with `config.RelayInterval()`.
- Don't publish domain events directly to Kafka when they must survive a crash.

## 10. CDC replication (PeerDB/Kafka)

- `replication.NewCDCReplicator` plus `ReplicationConfig`, `CDCSchemaPair`,
  `TargetTableMapping`; flows (`TenantToTenant`, `NonTenantToTenant`, …) decide
  how schemas map.
- For `NonTenantToTenant` tables with a `client_id` column, the source table
  **must** use `REPLICA IDENTITY FULL`. Without it Postgres omits `OLD` values,
  tenant moves become upserts and the old schema keeps ghost rows. Read
  `replication/README.md` before touching replication.

## 11. Migrations and tenants

- The `migrations` package only declares which SQL files exist; the `mig`
  runner performs the work. A tenant maps to a Postgres schema named after the
  client id, and that mapping lives here and only here.
- `migrations.NewActivities(runner)` exposes `Verify` (round-trip on a throwaway
  schema) and `MigrateTenant` (create/update named tenants). Register
  `MigrateTenant` as a Temporal activity so the create-client saga can call it.
- Provide `MigrateExisting` as the fallback for when the tenant list is
  unreachable; those local schemas are a stale copy of the list and better than
  nothing.

## 12. Temporal

- Options: `temporal_helper.DefaultActivityOptions(taskQueue...)`,
  `DefaultChildWorkflowOptions`, `FireAndForgetChildWorkflowOptions`.
- Handlers can call `temporal_helper.ExecuteWorkflowWithDefaults` /
  `ExecuteWorkflowWithContext` to run a workflow with an HTTP-shaped result and
  timeout handling.
- Activities: wrap bodies with `temporal_helper.SafeRunner` to convert panics.
  Use `StopWorkflowRetryOnNonRetryable` together with `entity.Workflow*`
  non-retryable errors.
- `provider.NewScheduleClient(temporalSvc)` + `ScheduleIntervalDaily/Weekly/…`
  for recurring work; don't add a separate cron system.
- Register workflows/activities through `service.TemporalServicer`
  (`RegisterWorkflow`, `RegisterActivity`, `Run`, `Close`).

## 13. Validation

- Use the shared `validator.Validate` singleton (already configured with
  `WithRequiredStructEnabled()`); don't construct new validators.
- Fiber bind errors: `validator.ParseBindError(err)`; model validation errors:
  `validator.ParseValidationErrors(err)`; `validator.HumanizeType` turns field
  types into message-friendly names.

## 14. HTTP handler timeouts

Wrap use-case calls in `http.RunWithTimeout` to enforce a context deadline and
centralize panic recovery/response sending:

```go
return http.RunWithTimeout(c, 5*time.Second, func(ctx context.Context) (*dto.Response, *entity.HttpError) {
	res, err := usecase.Invoke(param)
	if err != nil {
		return nil, entity.ToHttpError(err)
	}
	return res, nil
}, "Successfully fetched data", fiber.StatusOK)
```

## 15. Testing

- `testify` (`assert`/`require`) and table-driven tests are the norm.
- Use the provided doubles instead of real connections:
  `service.MockPostgreSQLService`, `MockMongoDBService`, `MockValkeyService`,
  `MockTemporalService`, `MockEmailService`, `MockFileService`, `MockMQTTService`,
  `provider.MockTimeProvider`, `MockPasswordProvider`, `MockRecaptchaProvider`,
  `MockScheduleProvider`, plus `service.FakePostgreSQLService`,
  `FakePgxPool`/`FakePgxTx` and `NewFakeValkeyService`.
- Handler tests: `helper.SendTestRequest` sends JSON with client/user headers to
  a `fiber.App`.
- `helper` and `service` include their own tests — mirror their style when
  adding coverage.

## Review checklist

- [ ] `logger.Init()` called once after config; new logging uses `slog.*Context`.
- [ ] Errors normalized with `entity.ToHttpError` and sent via
      `SendResponse`/`helper.*` at the handler boundary.
- [ ] No re-implemented slice/pointer/JSON/response helper.
- [ ] Repositories accept interfaces; services registered through the manager.
- [ ] No raw SQL string building; `sql_query` builders with args.
- [ ] Transactions set the handle and clear it; outbox inserts share the tx.
- [ ] Cache treated as disposable; `Incr` used for counters.
- [ ] `REPLICA IDENTITY FULL` present where CDC moves tenants.
- [ ] Migrations verified before applying; `MigrateTenant` registered as an
      activity.
- [ ] Temporal activities use shared options and non-retryable workflow errors.
- [ ] Tests use mocks/fakes, not live databases.

See [references/package-map.md](references/package-map.md) for the package
index and where to look for current APIs.
