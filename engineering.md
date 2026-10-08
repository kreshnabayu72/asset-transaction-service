# Engineering Standards

> Shared standards copied from `iam-service`. This repository currently uses
> Go 1.27 and Fiber v3 and has no domain modules or persistence. Apply the
> domain, database, and workflow standards as those features are introduced;
> see `README.md` for the current setup and commands.

The engineering rules this team follows — how we work and how we write Go.

## 1. Purpose & Principles

### Why this document exists

This is the team's standard for how we work and how we write Go — the rules every service in the platform follows, stated plainly so a new engineer can read one file and know what "done" looks like. It is the self-contained operating handbook for this service repository; the foundational rationale behind the rules is recorded in the platform's Architectural Decision Records (ADRs 001–020) and cited throughout this document. A rule here that no longer matches the code is a bug in this document, not a licence to drift.

The rules are grounded in what the services actually do — the reference implementation this standard was extracted from — not in an idealised shape nobody has written yet. Where a convention exists in the code and is still evolving, the ADRs say so explicitly.

### Principles that decide most arguments

1. **A service reads only its own tables.** Everything else is a call —
   including "who are the tenants", which only the enterprise-management
   service knows. There are no cross-service joins. (ADR 001)
2. **Reads are HTTP. Writes cross services through Temporal**, on the owning
   service's task queue — because a write may need undoing, and a caller
   cannot be trusted to still be alive when the undo must happen. (ADR 006)
3. **One service owns each responsibility.** iam owns roles; nobody derives
   authorization from its own copy of a role. (ADR 005)
4. **A record of what happened does not get rewritten** when the world moves
   on. A frozen name on a historical record is correct, not stale. (ADR 007)
5. **A list has one audience; a detail has all of them.** Merging audiences
   into one gated list merges their blast radii. (ADR 009)
6. **Authorization is asked of the cached snapshot directly, never a copy of
   it.** A derived type is a place to drop a grant the source actually has.
   (ADR 010)
7. **The gate goes on the route**, where a missing one is visible as an
   absence — and it only works when the middleware that feeds it is mounted
   with it. (ADR 013)
8. **Failure is loud and close to the cause.** A degraded service reports
   itself degraded; a missing secret refuses requests rather than guessing; a
   migration runs before serving, not after the first query breaks. (ADRs
   001, 004)

### Principles about the standard itself

- **A decision earns its rationale.** Every convention exists because of a
  cost it avoids; the ADR records the cost, including the downsides of the
  choice. A rule without a reason becomes a superstition.
- **Change is made by writing an ADR, not by drift.** Conventions change when
  someone records the decision and its consequences (§16) — never silently,
  per-file, in passing.
- **What is undecided is written down too.** Unknowns and known gaps are
  tracked in the ADRs, not papered over, so a future engineer can tell a
  deliberate choice from an oversight.

## 2. Technology Standards

The platform is a set of Go services behind a Traefik gateway. Each choice
below has a reason, and where the reason was hard-won it is linked. Version
bumps and how we track dependencies are in §13; this section is about what we
use and why.

### Languages & frameworks

- **Go 1.26+** for every backend service. Stdlib-first, structured logging
  with `log/slog`, constructor-injected dependencies, no DI container (ADR
  003).
- **Fiber** — the HTTP framework. v2 today in most services, v3 in those
  already migrated (the migration is in progress, not a fork). Fiber's
  middleware model is what lets authorization gates mount cleanly on the
  route itself (ADR 013).
- **Traefik v3** — the gateway. Not Go, and deliberately: its built-in
  `forwardAuth`, CORS, TLS and rate-limit middlewares replace a hand-written
  auth plugin (ADR 020).

### Data stores

- **PostgreSQL via pgx/v5** — the primary store. One database per service,
  one schema per tenant (ADR 001). A service reads only its own tables; every
  cross-service read is a call, never a join.
- **MongoDB** — used by the notification and analytics-reporting services.
  The one non-Postgres store, kept at parity through `be-core-utils`'s
  `MakeMongoDBService`; tenant-scoped as one logical database per client.
- **Valkey** — Redis-compatible cache and session store. Holds sessions and
  the RBAC snapshot; read on the request path, written only by iam (ADR 005).
- **MinIO (S3 API)** — object storage for the file-manager service.

### Workflows & messaging

- **Temporal** — durable workflows and activities. Every multi-step write
  that may need undoing runs here, on the owning service's task queue, so a
  later failure can compensate it (ADR 006). Sagas compensate via
  `WorkflowCompensation` on a disconnected context.
- **Redpanda (Kafka API)** — the event bus. Writes emit through a
  transactional outbox that a relay drains to topics; consumers dedup by
  outbox id (ADR 017).

### Shared libraries (own repos, not forked per service)

- **ts-utils/v2** — cross-cutting middleware and RBAC: `ClientID`,
  `RequireServiceToken`, `LoadUserRoles`, `rbac.Require`.
- **be-core-utils** — the core toolkit: `coreEntity` (HttpError),
  `coreHelper` (responses, export), `service` (TemporalService, PostgreSQLServicer),
  `sql_query`, `validator`, `outbox`, `provider`, `config`.
- **mig** — the domain-agnostic migration engine behind every service's
  `pkg/migrations` (ADR 001).
- **service-contracts** — the shared wire types and typed Temporal wrappers
  that services use to call each other (ADR 006). A single module, versioned
  like any library.

### Packaging & deployment

- **Docker multi-stage builds** — builder stage with the Go toolchain,
  runtime stage a `distroless` nonroot image. Secrets for private module
  fetches are BuildKit secrets, never baked into an image.
- **No service publishes a host port.** Only the gateway is reachable; that
  network boundary, not a header check, is what stops a client from forging
  an identity (ADR 020).

## 3. Project Structure

Each service is its own repository and its own Go module. Every service
follows the same skeleton:

```
<service>/
  main.go                  # entry point: load config, build app, run
  internal/
    app/                   # app.go — DI wiring / bootstrap
    config/                # env-based config loader
    model/                 # entity structs shared across modules
    module/<domain>/       # one folder per business domain (e.g. client, user, asset)
      handler/             # parse request, call service, reply
      service/             # domain actions
      repository/          # query construction and execution
      workflow/            # Temporal workflows (if the domain has any)
      activity/            # Temporal activities (if the domain has any)
      dto/                 # request/response types
      constant/            # domain constants
      <domain>_routes.go   # route registration, at the domain root
  pkg/migrations/          # embedded SQL migrations
    public/                # shared, non-tenant tables
    tenant/                # per-tenant schema tables
```

Notes:

- `internal/model/` sits above the domain modules so more than one module can
  share the same entity struct.
- A service may have more than one domain under `internal/module/`, each with
  the same internal shape.
- Services that don't own persistent tables may not have `pkg/migrations/`.

Shared pieces live outside this service repository:

- **`service-contracts`** — the shared module holding the wire DTOs, event schemas, and Temporal wrappers that services use to communicate.
- **`ts-utils`** and **`be-core-utils`** — shared core libraries (see §2), maintained in their own repositories.
- **`go-code-standard`** — the central standards repository where canonical platform documentation and the full ADR catalog reside.
- **Dedicated E2E Test Repository** — the centralized repository containing cross-service end-to-end integration test suites and harnesses.

For complete architectural details (Traefik gateway perimeter, multi-tenancy, Temporal sagas, and outbox event streaming), refer to `architecture.md`.

## 4. Git & Branching

### Branch naming

`<prefix>/<short-description>` — the prefix names the kind of work, the
short-description is kebab-case and says what it does.

| Prefix      | Use                                                            |
| ----------- | -------------------------------------------------------------- |
| `feat/`     | a new feature or API endpoint                                  |
| `fix/`      | a bug fix                                                      |
| `hotfix/`   | an urgent fix to what is currently running or being tested     |
| `docs/`     | documentation updates, guides, and Bruno API collections       |
| `test/`     | automated test suites, unit tests, and cross-service E2E tests |
| `chore/`    | maintenance, tooling, dependencies                             |
| `refactor/` | restructuring with no behaviour change                         |

Examples: `feat/add-measurement-export`, `fix/cancel-request-status`,
`docs/add-measurement-bruno-docs`, `test/add-borrowing-e2e`,
`chore/bump-fiber-v3`, `refactor/consolidate-dsn`.

### Protected branches

Three protected branches today, a fourth if it's ever needed:

- `dev` — the integration branch. Feature work lands here.
- `staging` — the release candidate branch, promoted from `dev`.
- `main` — production.
- `prod` — only if `main` and `prod` are ever split; until then `main` is
  production.

### The flow

- Feature branches branch off `dev` and are merged back into `dev` (PR
  required).
- A release promotes `dev` → `staging` → `main`. Nothing is committed
  directly to a protected branch.
- `hotfix/` branches off the current testing state — `staging` if that is
  what's in flight, otherwise `main` — and is merged downstream to
  production first, then propagated back upstream through `staging` into
  `dev`, so the fix survives the next release. The back-propagation is a
  merge, never a cherry-pick, and happens before the next promotion.
- Once a release is cut, `staging` accepts only fixes until it is promoted;
  new features wait for the next cut.
- A branch is deleted once it is merged; only the protected branches
  persist.

## 5. Commit Convention

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<optional-scope>): <short-description>

[optional body explaining context / rationale]

[optional footer(s), e.g. Co-authored-by, Refs, Breaking Changes]
```

### Types

| Type       | Purpose                                                             |
| ---------- | ------------------------------------------------------------------- |
| `feat`     | A new user-facing or API feature                                    |
| `fix`      | A bug fix                                                           |
| `refactor` | Code change that neither fixes a bug nor adds a feature             |
| `chore`    | Tooling, build config, dependency bumps, or repository housekeeping |
| `docs`     | Documentation changes only (e.g. `engineering.md`, ADRs, API specs) |
| `test`     | Adding or correcting tests with no production code change           |
| `perf`     | A code change that improves performance                             |

### Scopes

The scope is optional but strongly recommended. It names the affected service, domain, or shared module:

- **Service scope**: `feat(gateway): route all services behind a Traefik forwardAuth gateway`, `chore(master-asset-registry): migrate to Fiber v3`
- **Domain scope**: `fix(measurement): correctness and standards pass on write path`, `refactor(borrowing): assign ids in fake repository`
- **Shared package scope**: `refactor(be-core-utils): adopt shared validator and pagination query`, `feat(service-contracts): add reserve asset wire types`
- **Multi-service / global scope**: `chore(services): strip UTF-8 BOMs and apply gofmt`, `fix(services): bound and allowlist sort parameters`

### Rules

1. **Imperative mood**: Use "add", "fix", "change", "refactor" — not "added", "fixing", "fixes", or "changed".
2. **Lowercase and no trailing period**: The first line must be concise (≤72 chars), lowercase summary, without a period at the end.
3. **Atomic commits**: One logical change per commit. Do not combine unrelated refactors with feature changes or migrations.
4. **Explain the "why" in the body**: If the change is subtle, non-obvious, or touches architectural boundaries, explain the motivation, tradeoffs, and context in the commit body. Reference the relevant ADR when applicable.

   **Example:**

   ```
   fix(changelog): dispatch changelog writes over Temporal, not HTTP

   Direct HTTP writes from enterprise-management-service to analytics-reporting-service
   were dropped whenever analytics-reporting was restarting during client updates.
   Because changelog records are delivery-critical audit entries, route them through
   a fire-and-forget Temporal child workflow on analytics-reporting's task queue with
   unbounded retry policy (ADR 006).
   ```

## 6. Pull Requests & Code Review

Pull requests are the primary gate for code quality, architectural consistency, and knowledge sharing.

### PR Title & Scope

- **Title format**: Follow the commit convention (`<type>(<scope>): <summary>`), e.g., `feat(measurement): add export excel on measurement list`.
- **Keep PRs focused**: A PR should address one feature, bug fix, or refactor. Avoid bundling formatting changes, dependency bumps, and business logic into a single PR.
- **Size guideline**: Aim for PRs under 400 lines of change. If a feature touches multiple services or requires cross-service sagas, consider splitting it into:
  1. Wire contracts and shared DTOs (`service-contracts`)
  2. Producer service activity/workflow registration
  3. Consumer service integration

### PR Description Template

Every PR description must answer three questions:

1. **Context & Problem**: What problem does this PR solve, or what feature is being added?
2. **What Changed**: Summary of key changes across handlers, services, repositories, or workflows. Link any relevant ADRs.
3. **Verification**: How was this tested? (Unit tests, Bruno collection, live Docker Compose verification, migration round-trip).

```markdown
### Summary

Brief summary of what this PR does.

### Motivation & Context

Why this change is needed. (Link to ticket, issue, or ADR).

### Changes

- Added `ExportExcel` repository method and service handler in MAR.
- Mounted route `POST /v2/measurements/export-excel` with `rbac.Require(rbac.SystemRole(tsMiddleware.PermView, key))`.

### Verification

- [x] `go test ./...` passes.
- [x] `mig verify --schema client_test` executed cleanly for new migrations.
- [x] Tested endpoint manually via Bruno collection through Traefik gateway.
```

### Reviewer Checklist & Focus Areas

When reviewing code, focus on correctness and architectural boundaries rather than nitpicking style that formatters handle:

- [ ] **Architecture & Boundaries**:
  - Does the service touch only its own tables? (No cross-service DB access — ADR 001).
  - Are cross-service writes routed through Temporal activities on the owner's task queue? (ADR 006).
  - Are reads routed via HTTP and performed _before_ opening local DB transactions? (ADR 006 §4b).
- [ ] **Authorization & Security**:
  - Is the route gate mounted properly on the route with `rbac.Require` after `LoadUserRoles`? (ADR 013).
  - Are user identities pulled strictly from `X-User-ID` / `X-Client-ID` headers set by the gateway, never request bodies? (ADR 020).
- [ ] **Data & Migrations**:
  - Do new tables have both `.up.sql` and `.down.sql` migrations? (ADR 001).
  - Are migrations tenant-safe and backward compatible? (ADR 008).
- [ ] **API Documentation**:
  - Does the PR include a corresponding Bruno API collection request file (`.bru`) under `docs/bruno/` for each new or modified endpoint, enabling immediate manual verification by reviewers?
- [ ] **Testing**:
  - Are repository-level unit tests backed by stateful fakes rather than fragile testify mocks? (ADR 015).
  - Are edge cases (empty lists, invalid inputs, unauthorized roles) covered?

### Merging & Housekeeping

1. **Approvals**: At least one peer review approval is required before merging into `dev` or `staging`.
2. **CI status**: All CI checks (build, lint, unit tests) must be green.
3. **Merge strategy**: Squash and merge into `dev` (or standard merge if preserving structured multi-commit history).
4. **Branch cleanup**: Automatically delete head branches upon merge to prevent stale branch accumulation.

## 7. Go Coding Conventions

We write idiomatic Go with explicit dependencies, strict layering, and stdlib-first patterns.

### Formatting & Tooling

- **Formatting**: Code must be formatted with `gofmt` and `goimports`. No manual formatting debates.
- **File encoding**: UTF-8 without BOM.
- **Go version**: Go 1.26+. Use stdlib wherever possible (e.g., `log/slog`, `net/http`, `context`).

### Naming Conventions

| Entity              | Convention                                      | Example                                                                      | Bad Example                                |
| ------------------- | ----------------------------------------------- | ---------------------------------------------------------------------------- | ------------------------------------------ |
| **Files**           | `<action>_<package>.go` (ADR 012)               | `list_clients_handler.go`, `create_asset_service.go`, `client_repository.go` | `list_clients.go`, `handler.go`            |
| **Types / Structs** | Domain noun, PascalCase (ADR 003)               | `ClientService`, `AssetRepository`, `User`                                   | `GetClientsService`, `CreateAssetUseCase`  |
| **Interfaces**      | Suffix with `-er` or `-or`, colocated with repo | `ClientRepositor`, `PostgreSQLServicer`                                      | `IClientRepo`, `ClientRepositoryInterface` |
| **Methods**         | Action verb + target (ADR 003)                  | `ListLiveClientCodes`, `GetAvailableList`, `Create`                          | `Invoke`, `Execute`, `Handle` (in service) |
| **Receivers**       | 1–2 lowercase letters matching type             | `(s *ClientService)`, `(h *ClientHandler)`, `(r *ClientRepository)`          | `(this *ClientService)`, `(self *Handler)` |
| **DTOs**            | `<Action><Target>Request` / `Response`          | `CreateClientRequest`, `ListClientsResponse`                                 | `ClientReqDTO`, `DataIn`                   |

### Layer Responsibilities & Strict Boundaries

Every service follows the four-layer architecture (ADR 003):

```
HTTP Request → [Router] → [Handler] → [Service] ┬→ [Repository] → PostgreSQL
                                                └→ [Workflow]   → [Activity] → External Service / DB
```

1. **Router (`<domain>_routes.go`)**:
   - Lives at the domain root (not inside `handler/`).
   - Mounts routes, auth middleware (`LoadUserRoles`), and route gates (`rbac.Require`) (ADR 013).
   - Splits public/auth routes from `/internal/*` service-token routes (`SetupInternalRoutes`).

2. **Handler (`handler/`)**:
   - **Responsibility**: Parse request body/params, validate input, call exactly one service method, and serialize response.
   - **Strict bounds**: Never touch SQL, never import Temporal packages, and contain zero business logic.
   - Always return errors via `coreEntity.ToHttpError(err).SendResponse(c)` and success via `coreHelper.Success(c, msg, data)`.
   - **Request Validation Standard**: Handlers follow a two-tier validation pipeline:
     1. **Tier 1 (Binding / Deserialization)**: When `c.Bind().Body(&body)` fails, always return `coreEntity.BadRequest("Invalid request body", validator.ParseBindError(err)).SendResponse(c)`.
     2. **Tier 2 (Struct Validation)**: When `validator.Validate.Struct(&body)` fails, always return `coreEntity.BadRequest("Validation failed", validator.ParseValidationErrors(err)).SendResponse(c)`.

3. **Service (`service/`)**:
   - **Responsibility**: Domain action surface.
   - Holds dependencies (repository interfaces, Temporal client, config) via **constructor injection** (`New<Domain>Service(...)`).
   - Methods stay thin: a simple repository call (reads) or a workflow dispatch (multi-step/cross-service writes) (ADR 003).

4. **Repository (`repository/`)**:
   - **Responsibility**: Construct and execute SQL/Mongo queries only.
   - One file per entity (`client_repository.go`), not split per action.
   - Interface and concrete implementation live together in `repository/`.
   - Interface methods accept `context.Context` and `coreService.PostgreSQLServicer`.

5. **Model (`internal/model/`)**:
   - Shared entity structs placed at service root so multiple domains can share them without circular dependencies.
   - Plain data structs with `json` and `column:"table.field"` tags. No methods or business logic.
   - **Strict Model-to-Migration Parity**:
     - Go model fields must be **100% identical and consistent** with the table schema in `pkg/migrations/`.
     - Every column in the SQL table must have a matching Go struct field, and every struct field must exist in SQL.
     - **Tag Consistency**: Every field must have matching `json:"<column_name>"` and `column:"<table_name>.<column_name>"` tags.
     - **Type Consistency**: SQL `BIGINT` must map to `int64`, `VARCHAR`/`TEXT` to `string`, `BOOLEAN` to `bool`, and `TIMESTAMPTZ`/`TIMESTAMP`/`DATE` columns must map strictly to `time.Time` (or `*time.Time` if nullable). Never map date/timestamp columns to `string` or `int64` in models.
     - **Nullability**: Nullable SQL columns must use Go pointers (e.g. `*string`, `*int64`, `*time.Time`) to prevent scan errors.
   - **ID types**: Always use `int64` for Snowflake ID fields (`id`, `client_id`, `user_id`, `asset_id`, etc.) to prevent integer overflow. Use `int` only for counts, limits, or sequence numbers.

6. **DTOs (`internal/modules/<domain>/dto/`)**:
   - Request, response, and projection data structures scoped to the module.
   - **Date & Timestamp DTO Typing**:
     - Always use `time.Time` (or `*time.Time` if nullable) for all date and timestamp fields in response and entity projection DTOs.
     - **OpenAPI Schema Translation**: Although OpenAPI specs define date fields as `type: string, format: date-time` (or `format: date`), **never** define DTO fields as `string` or `*string`. Go's `encoding/json` natively serializes `time.Time` into standard RFC3339 strings.
     - **Zero SQL Date Formatting**: Never use `to_char(...)` or SQL string formatting on date/timestamp columns. Map the database column directly (`column:"table.column_name"`). The `pgx` driver scans PostgreSQL `TIMESTAMPTZ`, `TIMESTAMP`, and `DATE` types directly into `time.Time`.
     - **Filter Query DTOs**: Request DTOs accepting date range filters use epoch milliseconds (`[]int64`).

### Inter-Service Communication Patterns

The fundamental question deciding cross-service communication: **"Does this step need undoing if a later step fails?"** (ADR 006)

| Scenario                                                       | Mechanism                                     | Implementation & Rules                                                                                                                                                                                                     |
| -------------------------------------------------------------- | --------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Reads** (nothing to undo)                                    | **HTTP Internal API**                         | Plain `net/http` client using shared wire types from `service-contracts`. Authenticated via `X-Service-Token` and `X-Client-ID`. **Always perform remote HTTP reads before opening a local DB transaction** (ADR 006 §4b). |
| **Writes needing compensation or delivery guarantees**         | **Temporal Activity**                         | Activity runs on the **owning service's task queue** using typed wrappers in `service-contracts/services/<owner>/temporal` (ADR 006).                                                                                      |
| **Independent durable sub-steps / delivery surviving outages** | **Temporal Child Workflow (Fire-and-Forget)** | Spawn with `ParentClosePolicy: ABANDON` on owner's task queue (e.g. audit logs). Durable retry lives on the owner's side (ADR 006).                                                                                        |
| **Reusing workflow logic synchronously**                       | **Temporal Child Workflow (Blocking)**        | Default `ParentClosePolicy: TERMINATE`, block on `.Get(ctx, nil)`.                                                                                                                                                         |
| **Multi-step Saga Rollbacks**                                  | **`WorkflowCompensation`**                    | Register compensation immediately after forward activity succeeds (`AddCompensation`). Run in `defer` on a **`workflow.NewDisconnectedContext`** (ADR 006 §2). Compensations must be idempotent.                           |
| **Event Broadcast / Async Projections**                        | **Transactional Outbox + Redpanda**           | Commit outbox record in the same DB transaction as entity write; relay publishes to Redpanda topics. Consumers dedup by `outbox.id` (ADR 017).                                                                             |

### Dependency Injection

- **Service & Repository layer**: Always use **constructor injection** (`NewClientService(repo, temporalSvc, db)`). Do not use reflection-based DI containers or global singletons (ADR 003, ADR 004).
- **Temporal Activity layer**: Uses `ServiceManager` (service locator) because Temporal registers activity methods wholesale (ADR 003 §3).

### Concurrency & Context

- **Always pass `context.Context` as the first argument** of any I/O, database, or cross-service call.
- **Never spawn unmanaged goroutines** (`go func()`) inside HTTP handlers that outlive the request lifecycle.
- **In Temporal workflows**: Use Temporal's deterministic concurrency primitives (`workflow.Go`, `workflow.NewSelector` for N parallel pipelines — ADR 018), never native Go `go func()` or `time.Sleep`.

### Struct Tags

- **JSON**: CamelCase (`json:"totalRecords,omitempty"`).
- **SQL Column Mapping**: Table-qualified snake_case (`column:"clients.client_code"`).
- **Validation**: Declarative validator tags (`validate:"required,min=1"`).
- **File Export**: Human-readable column header (`export:"Modified By"`, `export:"Created At"`).

### File Export Pattern (CSV & XLSX)

When building CSV or Excel export endpoints across any service:

1. **DTO**: Tag the export row item struct fields with `export:"<Header Name>"`. Unexported or untagged fields will be omitted automatically. Pointer fields and `time.Time` values are formatted automatically (`"2006-01-02 15:04:05"`).
2. **Service**: Fetch data rows and delegate rendering directly to `helper.Export` from `be-core-utils`:
   ```go
   return helper.Export(rows, helper.ExportParam{
       Format:       param.Format,       // "csv" or "xlsx"
       FileName:     param.FileName,     // requested base name
       FallbackName: "Domain-Entity-Log", // default fallback name
   })
   ```
3. **Handler**: Stream the returned `*helper.ExportResult` without custom buffering:
   ```go
   c.Set("Content-Type", file.ContentType)
   c.Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, file.Filename))
   return c.Send(file.Data)
   ```
4. **Strict Prohibition**: Never hand-roll `encoding/csv` or `excelize` generation logic inside domain services or handlers. Always use `be-core-utils/helper.Export`.

## 8. Error Handling

Errors must be explicit, typed where necessary, preserved with wrapping, and translated cleanly at the HTTP boundary.

### The Core Pattern

1. **Repositories & low-level calls**: Return raw or contextual wrapped errors (`fmt.Errorf("failed to query client: %w", err)`).
2. **Services**: Evaluate domain logic and return domain/HTTP errors (`coreEntity.NotFound("asset not found")` or `coreEntity.InternalServerError("failed to update asset", err)`).
3. **Handlers**: Always convert the error via `coreEntity.ToHttpError(err)` and send it via `.SendResponse(c)`:

```go
// handler/get_asset_handler.go
res, err := h.assetSvc.GetByID(c.Context(), assetID)
if err != nil {
    return coreEntity.ToHttpError(err).SendResponse(c)
}
return coreHelper.Success(c, "Successfully retrieve asset", res)
```

### Standard HTTP Errors (`be-core-utils/entity`)

Use the standard error constructors from `coreEntity` rather than hand-rolling status codes:

| Constructor                                | HTTP Status | Use When                                               |
| ------------------------------------------ | ----------- | ------------------------------------------------------ |
| `coreEntity.BadRequest(msg)`               | 400         | Invalid payload, malformed JSON, query syntax error    |
| `coreEntity.Unauthorized(msg)`             | 401         | Missing or invalid session/service token               |
| `coreEntity.Forbidden(msg)`                | 403         | Authenticated user lacks permission or module access   |
| `coreEntity.NotFound(msg)`                 | 404         | Resource does not exist for the given ID / tenant      |
| `coreEntity.Conflict(msg)`                 | 409         | Duplicate unique constraint, state transition conflict |
| `coreEntity.UnprocessableEntity(msg)`      | 422         | Semantic validation failure (e.g. date range invalid)  |
| `coreEntity.InternalServerError(msg, err)` | 500         | Database, I/O, or unexpected internal system failure   |

### Error Rules

1. **Wrap with context (`%w`)**: When bubbling up an error from an underlying layer, add context using `fmt.Errorf("<action>: %w", err)`. Never discard the original error.
2. **Inspect with `errors.Is` and `errors.As`**: Never compare error strings directly (e.g., `err.Error() == "not found"`).
3. **Do not log and return**: Either handle the error (log it and recover) or return it up the call stack to let the caller handle it. Logging at every layer creates noisy, duplicate logs.
4. **Sanitize client error messages**: Never expose internal table names, SQL queries, database stack traces, or infrastructure credentials to the client. `coreEntity.InternalServerError` captures the internal error for logging while returning a safe message to the client.
5. **Non-retryable Temporal errors**: In Temporal activities, if an error is a permanent business validation failure (e.g. invalid arguments or non-existent entity), return `temporal.NewNonRetryableApplicationError(...)` so Temporal does not wastefully retry an unrecoverable failure.

### Two-Tier Request Validation Pattern

When validating incoming HTTP request bodies, handlers must translate failures into structured, field-attributed errors using `be-core-utils/validator`:

```go
var body dto.MyRequest
// Tier 1: Deserialization & type-binding check
if err := c.Bind().Body(&body); err != nil {
    return coreEntity.BadRequest("Invalid request body", validator.ParseBindError(err)).SendResponse(c)
}
// Tier 2: Semantic struct tag validation check
if err := validator.Validate.Struct(&body); err != nil {
    if errs := validator.ParseValidationErrors(err); errs != nil {
        return coreEntity.BadRequest("Validation failed", errs).SendResponse(c)
    }
    return coreEntity.BadRequest(err.Error()).SendResponse(c)
}
body.Defaults()
```

- **`validator.ParseBindError(err)`**: Unwraps Fiber v3's `*fiber.BindError` and `*json.UnmarshalTypeError` into `[]validator.BindFieldError{Field, Message}` so the frontend can immediately locate which JSON property had the invalid data type or syntax.
- **`validator.ParseValidationErrors(err)`**: Converts `validator.ValidationErrors` into `[]validator.ValidationError{Field, Rule, Param}`.

## 9. Logging

We use Go's standard structured logging package (`log/slog`). Logs must be machine-readable, contextual, and sanitized.

### Logger Setup & Format

- **Engine**: `log/slog` stdlib logger configured during app bootstrap (`internal/app/app.go`).
- **Production format**: JSON handler (`slog.NewJSONHandler`) to stdout.
- **Context propagation**: Always use context-aware logging functions (`slog.InfoContext`, `slog.ErrorContext`, `slog.WarnContext`, `slog.DebugContext`) so `ctx`-attached metadata (`X-Request-ID`, `X-Client-ID`, `X-User-ID`) is automatically emitted with the record.

### Log Levels

| Level       | When to Use                                            | Example                                                                           |
| ----------- | ------------------------------------------------------ | --------------------------------------------------------------------------------- |
| **`Debug`** | Granular diagnostic details, disabled in production.   | Outbox relay batch size, raw cache key lookups                                    |
| **`Info`**  | Normal operational milestones and state transitions.   | Service boot complete, migrations applied, workflow completed                     |
| **`Warn`**  | Recoverable issues, degradations, or fallback paths.   | RBAC Valkey cache miss (falling back to IAM HTTP call), transient network retries |
| **`Error`** | Failures requiring engineer investigation or alerting. | Database query failure, saga compensation triggered, panic recovered              |

### Structured Attributes vs String Formatting

Always use structured attributes instead of string interpolation:

```go
// GOOD: Structured, indexable, searchable
slog.InfoContext(ctx, "asset status updated",
    slog.Int64("asset_id", asset.ID),
    slog.String("new_status", asset.Status),
    slog.String("client_id", clientID),
)

// BAD: String interpolation breaks log indexing and searching
slog.InfoContext(ctx, fmt.Sprintf("asset %d status updated to %s for client %s", asset.ID, asset.Status, clientID))
```

### Logging Rules & Prohibitions

1. **No `fmt.Println` or `log.Print`**: Raw, unformatted stdout logging is strictly forbidden.
2. **Never log sensitive data (PII & Secrets)**: Never log passwords, OTP codes, session tokens, service tokens, authorization cookies, or user PII.
3. **Log once at the boundary**: Log errors at the outermost handling boundary (e.g. Fiber error handler, background worker entry, or Temporal activity root). Do not log in the repository, service, and handler simultaneously.
4. **Include correlation IDs**: Ensure `X-Request-ID` is present in logs for every HTTP request and is propagated to any spawned Temporal workflows.

## 10. Database Conventions

Every service owns its data completely. Storage follows schema-isolated multi-tenancy with deterministic, boot-time migrations.

### Multi-Tenancy Architecture (ADR 001, ADR 008)

- **PostgreSQL (`pgx/v5`)**: One database per service, one schema per tenant.
  - **`public` schema**: Shared, service-wide tables only (e.g., the transactional `outbox` table, system metadata).
  - **`tenant` schemas**: Client-specific domain data. Schema names follow `^[a-zA-Z0-9_]{1,50}$` derived from `X-Client-ID`.
- **MongoDB**: Used for high-volume append/document stores (`notification-service`, `analytics-reporting-service`). Tenant-isolated as one logical database per client via `be-core-utils`'s `MakeMongoDBService`.
- **Zero Cross-Service Joins**: A service reads and writes **only its own tables**. Cross-service joins and cross-database foreign keys are strictly forbidden. Cross-service entity references must store Snowflake `int64` IDs or immutable snapshot fields (ADR 007).

### Migrations with `mig` (ADR 001, ADR 008)

Migrations are authored as plain SQL files and run automatically on boot.

#### File Structure & Naming

```
<service>/pkg/migrations/
  public/   # Applied once to the public schema
  tenant/   # Applied across all tenant schemas
```

- **File format**: `<14-digit-timestamp>_<snake_case_name>.<up|down>.sql`
- **Timestamp format**: `YYYYMMDDHHMMSS` (e.g., `20260804100000_create_positions_table.up.sql`).
- **Symmetric Up/Down pairs**: Every `.up.sql` migration MUST have a corresponding `.down.sql`. `mig` enforces this during verification.

#### Boot-Time & All-or-Nothing Execution

- **Automatic run on boot**: Migrations run during app startup (`internal/app/app.go` phase 1) before the HTTP server or Temporal worker accepts traffic (ADR 004).
- **All-or-nothing tenant migrations (ADR 008)**: All tenant schemas are migrated sequentially inside **one single transaction** (`runner.UpAllSchemas`). If a migration fails on any tenant, the entire transaction rolls back and service startup halts loudly. A platform running mixed schema versions is never allowed.
- **Local authoring & validation**:
  ```bash
  mig create <name>          # scaffold timestamped pair
  mig test                   # up -> down -> up round trip against scratch DB
  mig verify --schema <name> # tenant-schema round trip verification
  ```

### Model and Migration Parity & Consistency

Database schemas and Go entity models must stay in strict, 1-to-1 synchronization:

1. **Zero Schema Drift**: Every table column in `pkg/migrations/` must exist as a tagged field in `internal/model/`, and no orphan fields are permitted.
2. **Column Tag Mapping**: Every field in `internal/model/` must define explicit `column:"<table_name>.<column_name>"` and `json:"<column_name>"` tags matching the SQL definition.
3. **Type Consistency**:
   - `BIGINT` columns (Snowflake IDs and timestamps) must map strictly to `int64`.
   - `VARCHAR` / `TEXT` columns map to `string`.
   - `BOOLEAN` columns map to `bool`.
   - `TIMESTAMPTZ`, `TIMESTAMP`, and `DATE` columns map strictly to `time.Time` or `*time.Time` (never `string` or `int64` in models).
   - `JSONB` columns map to typed Go structs or `map[string]any`.
4. **Nullability**: Columns without `NOT NULL` in SQL must be represented as pointers (e.g. `*string`, `*int64`) in the Go model to avoid runtime scanning errors.

### Schema & Query Conventions

1. **Identifiers**: Plural `snake_case` table names (`users`, `assets`, `transactions`), `snake_case` column names.
2. **Primary keys**: `id BIGINT PRIMARY KEY` (Snowflake IDs, 64-bit integer).
3. **Timestamps**: All temporal columns must be timezone-aware:
   ```sql
   created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
   updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
   ```
4. **Connection state safety (ADR 004)**: Never share a `PostgreSQLService` instance across concurrent requests or goroutines — it holds active transaction state (`pgx.Tx`) and will race. Obtain the tenant servicer dynamically per request.
5. **Keep transactions short**: Never perform remote HTTP calls, external API requests, or Temporal workflow calls while holding an open database transaction lock (ADR 006 §4b). Read remote data _first_, then begin the local database transaction.
6. **Query pagination & sorting**: All paginated queries must specify deterministic sorting (e.g. `ORDER BY id DESC`) and bounded limits. Filter and sort columns must be validated against allowlists to prevent SQL injection.

## 11. API Conventions

Our APIs follow uniform envelope structures, strict header-based identity propagation, and explicit route-level authorization.

### Standard Response Envelope (`be-core-utils/helper`)

Every API endpoint returns a standardized JSON envelope:

```json
{
  "message": "Successfully retrieve measurements",
  "data": {},
  "error": null
}
```

- `data` is omitted when empty; `error` is omitted when nil.
- Handlers produce success responses using `coreHelper.Success(c, message, data)` and errors using `coreEntity.ToHttpError(err).SendResponse(c)`.

### List & Option Endpoints

- **`POST /x/list` for paginated queries**: Every paginated list is `POST` and accepts pagination, search, sorting, and filters as a **JSON body** (not URL query parameters).
  - The payload must contain at least `{}` — sending `Content-Length: 0` returns a `400 Bad Request`.
  - Responses adhere to `be-core-utils/dto.PaginationResult`:
    ```json
    {
      "totalRecords": 42,
      "data": [ ... ]
    }
    ```
- **`POST /x/options` for filter feeds**: Dropdown options backing table filters are `POST /x/options` accepting filter criteria in the JSON body.
- **`GET` endpoints are unpaginated**: `GET /internal/*` by-ID lookups and single-resource detail queries (`GET /v2/borrowings/:id`) stay `GET`.

> [!NOTE]
> **Under Consideration (HTTP `QUERY` Method)**:
> We are evaluating migrating heavy read/query endpoints from `POST /x/list` to the HTTP `QUERY` method (safe, idempotent, with a request body). Until an ADR is drafted and accepted confirming framework, gateway, and client tooling support, `POST /x/list` remains the binding convention.

### Date & Timestamp Serialization

- **Wire Format (OpenAPI & JSON Envelopes)**: In OpenAPI specifications and JSON response payloads, dates and timestamps are represented as RFC3339 strings (`"2026-09-18T10:30:00Z"`, schema: `type: string, format: date-time` or `format: date`).
- **Backend Go Struct Typing**: In Go entity models and response/item DTOs, date/timestamp fields are **always** typed as `time.Time` (or `*time.Time` if nullable). They are **never** typed as `string` or `*string`. Go's `encoding/json` serializes `time.Time` to standard RFC3339 strings automatically.
- **Database Query Mapping**: Map database columns directly (`column:"table.updated_at"`). Never use `to_char(...)` or SQL string formatting on timestamp or date columns. The PostgreSQL driver scans native temporal types directly into `time.Time`.
- **Filter Query Payloads**: Client requests filtering list queries by date ranges send epoch millisecond integers (`[]int64`).

### Identity & Header Rules (ADR 020)

Client traffic enters through Traefik, which validates sessions via `forwardAuth` to IAM. Downstream services receive identity strictly via headers:

| Header            | Injected / Sent By      | Purpose                                             |
| ----------------- | ----------------------- | --------------------------------------------------- |
| `X-Client-ID`     | Gateway (`forwardAuth`) | The tenant Postgres schema (`^[a-zA-Z0-9_]{1,50}$`) |
| `X-User-ID`       | Gateway (`forwardAuth`) | The authenticated user's Snowflake ID (`int64`)     |
| `X-Service-Token` | Calling Service         | Shared secret for `/internal/*` routes (ADR 005 §3) |
| `X-Request-ID`    | Gateway / Client        | Distributed trace correlation ID                    |

> **CRITICAL RULE**: Services must **NEVER** accept or trust user identity or tenant identity from JSON request bodies or query parameters. Always read identity from gateway-injected headers.

### Route-Level Authorization Gates (ADR 009, ADR 010, ADR 013)

Route authorization gates are mounted directly on the route in `<domain>_routes.go`:

```go
group.Post("/list", h.HandleListMeasurements, rbac.Require(rbac.SystemRole(tsMiddleware.PermView, key)))
group.Post("/", h.HandleCreateMeasurement, rbac.Require(rbac.SystemRole(tsMiddleware.PermCreate, key)))
group.Delete("/", h.HandleDeleteMeasurements, rbac.Require(rbac.SystemRole(tsMiddleware.PermDelete, key)))
```

#### Mounting Order (Load-Bearing)

Route registration order in `internal/app/app.go` is strictly enforced:

1. **Unauthenticated / Internal routes**: Register `/health`, `/auth/login`, and `/internal/*` (`SetupInternalRoutes`) first.
2. **Mount Snapshot Loader**: Mount `tsMiddleware.LoadUserRoles(iamBaseURL, serviceToken)`.
3. **Mount Protected Routes**: Register user routes protected by `rbac.Require(...)`.

_Mounting `rbac.Require` before `LoadUserRoles` results in silent 401s._

#### List vs. Detail Authorization (ADR 009)

- **Lists have one audience**: A list query is gated by a single role and filtered in SQL _before_ pagination totals are calculated (ADR 002).
- **Details have multiple audiences**: A detail route (`GET /:id`) may be accessed by multiple legitimate actors (e.g. borrower, approver, manager). The route gate checks module access; row-level actor checks live in the service layer.

### API Documentation via Bruno Collections

Every service repository maintains an executable API collection using [Bruno](https://www.usebruno.com/) under `docs/bruno/`:

1. **Mandatory Documentation for Every Endpoint**: Creating or modifying an endpoint requires adding or updating its corresponding `.bru` file in the service's Bruno collection.
2. **File Structure & Content**:
   - Organized by domain module (e.g. `docs/api/bruno/measurements/create-measurement.bru`).
   - Pre-configured with environment variables (e.g. `{{baseUrl}}`, `{{tenantId}}`).
   - Includes standard headers (`X-Client-ID`, `X-User-ID`, `Content-Type: application/json`).
   - Contains a valid, realistic JSON payload and documented query parameters.
3. **Executable Verification**: Reviewers and QA engineers use these `.bru` files to manually verify the endpoint against local or dev environments without manually typing curl commands.

## 12. Testing

Our testing strategy is organized into two distinct tiers: **Unit Tests** (which live inside each individual service repository for isolated logic testing) and **API Integration & End-to-End (E2E) Tests** (which live in this centralized test repository to verify cross-service user journeys, Traefik gateway routing, header injection, and data consistency).

### 1. Unit Tests (Service Repository Scope)

- **Location**: Resides directly inside each service repository (`*_test.go` colocated with handlers, services, and repositories).
- **Scope**: Verifies HTTP handlers, input validation, domain business logic, SQL query builders, and Temporal saga orchestration in complete isolation.
- **Execution & Tooling**:
  - Uses stateful in-memory repository fakes rather than fragile mock assertions (ADR 015).
  - Uses Temporal's in-memory test suite (`go.temporal.io/sdk/testsuite`) to execute workflows and activities deterministically without running a Temporal server.
  - Runs instantly with `go test ./...` with zero peer service containers or external network requirements.

---

### 2. End-to-End (E2E) & Integration Tests (Dedicated Test Repository Scope)

All cross-service end-to-end integration test suites reside in this dedicated repository (`end-to-end-test`). Tests here treat the entire platform as a black box, executing HTTP requests against the Traefik API Gateway and asserting against real multi-service state transitions, background workflows, and datastores.

#### E2E Repository Architecture & Directory Layout

The repository is structured into reusable core infrastructure (`internal/`) and domain flow packages (`flows/`):

```
end-to-end-test/
├── internal/
│   ├── harness/             # Stack environment config, health checks, & preflight guards
│   │   ├── harness.go       # Gateway/Mailpit base URLs & RequireStack health poller
│   │   └── harness_test.go  # Unit tests for harness readiness polling
│   ├── client/              # Generic type-safe HTTP client & envelope decoders
│   │   └── http.go          # Post[T], Patch[T], Delete[T], DeleteWithBody[T], Get[T], PostRaw
│   └── mail/                # Mailpit client for async activation token retrieval
│       └── mailpit.go       # ActivationToken polling & extraction
├── flows/
│   ├── common/              # Shared flow utilities & tenant orchestration
│   │   └── shared_client.go # WithSharedClient, SharedClient, DTOs (LoginResponse, Paginated[T])
│   └── <service>/           # Organized by owning service (e.g. enterprise-management, iam)
│       └── <module>/        # One folder per business domain (e.g. measurement, user)
│           ├── main_test.go             # TestMain, shared tenant init, helper fixtures, pollers
│           ├── types_test.go            # Request/response DTOs & unmarshaling schemas
│           ├── <module>_test.go         # Full end-to-end lifecycle composition test
│           ├── create_<module>_test.go  # Creation validation, options feed, & duplicate tests
│           ├── edit_<module>_test.go    # Mutation, partial edits, no-op edits, & 404 tests
│           ├── delete_<module>_test.go  # Single & bulk deletion, partial skip, & 400 tests
│           └── list_<module>_test.go    # Search, sort, filter, pagination, & export matrix
```

---

#### Ephemeral Multi-Tenant Test Lifecycle (`WithSharedClient`)

To guarantee strict tenant isolation and deterministic execution, each test package executes against a **fresh, dynamically provisioned tenant**:

```
TestMain(m) ──► RequireStack (Health check Gateway & Mailpit)
                   │
                   ▼
             POST /dev/clients (Create ephemeral tenant: isTestClient=true, unique timestamp)
                   │
                   ▼
             Poll Mailpit (Retrieve async activation token)
                   │
                   ▼
             POST /auth/activation/:token/set-password (Activate admin user)
                   │
                   ▼
             POST /auth/login (Login as mobile client, obtain session ID)
                   │
                   ▼
             m.Run() (Execute all test files in package)
                   │
         ┌─────────┴─────────┐
         ▼ (code == 0)       ▼ (code != 0)
    DELETE /dev/clients   PRESERVE TENANT AS FORENSIC EVIDENCE
    (Drop DB schemas)     (Keep Postgres schemas for local debugging)
```

1. **One Tenant per Flow Package (`TestMain`)**:
   - Every domain test package declares a `TestMain` that invokes `common.WithSharedClient(m, &sharedClient, "<module>")`.
   - The harness generates a unique timestamped tenant (`companyName: "E2E <module> Tenant <timestamp>"`, `picEmail: "e2e-<module>-<timestamp>@e2e.test"`).
   - Provisions the tenant via the unauthenticated `POST /dev/clients` endpoint with `"isTestClient": true` (an architectural safety interlock that prevents accidental deletion of non-test databases).
2. **Mailpit Activation & Authentication**:
   - The harness polls Mailpit (`http://localhost:8025/api/v1/messages`) to retrieve the activation email and token.
   - Sets the initial admin password via `POST /auth/activation/:token/set-password`.
   - Logs in via `POST /auth/login` using `platform: "mobile"` (which bypasses reCAPTCHA requirements in automated suites) and captures the active `SessionID`.
3. **Forensic Evidence Teardown Policy (Preserve on Failure)**:
   - **On Green Run (`code == 0`)**: The harness issues `DELETE /dev/clients/:clientCode` to clean up the tenant, dropping the tenant schema across PostgreSQL databases and clearing IAM session records. If teardown fails on a passing run, the run fails loudly to prevent tenant leakage.
   - **On Red Run (`code != 0`)**: The harness **deliberately preserves the tenant and database schemas**. The untouched database state serves as the post-mortem forensic evidence for developers investigating test failures.
4. **Per-Test Cleanup (`t.Cleanup`)**:
   - Fixture helper functions (`create<Entity>`) register automatic entity deletion using `t.Cleanup(func() { ... })`, ensuring intermediate test objects are pruned even if assertions fail mid-test.

---

#### Flow Package Structure & Separation of Concerns

Flow tests for a domain module are split into dedicated test files, preventing 2,000-line monolithic test files and establishing a predictable test layout:

| File Pattern                  | Responsibility & Content                                                                                                                                                            |
| ----------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **`main_test.go`**            | `TestMain` bootstrapper, shared tenant session reference (`auth()`), unique naming helpers (`uniqueName()`), entity CRUD fixtures (`createX`, `deleteX`, `findX`), async pollers.   |
| **`types_test.go`**           | Flow-specific DTOs, response models, delete result envelopes, and changelog item schemas.                                                                                           |
| **`<module>_test.go`**        | Full end-to-end lifecycle composition test (`Test<Module>Flow`): Unauthenticated gate → Create → List → Dropdown → Edit → List → Export → Bulk Delete → List confirms gone.         |
| **`create_<module>_test.go`** | Creation endpoint tests: Happy path, appearance in dropdown/option feeds, empty field validations (`422`), duplicate entity conflicts (`409`), and unauthenticated checks (`401`).  |
| **`edit_<module>_test.go`**   | Update endpoint tests: Multi-field update, single-field partial update, **no-op edits** (asserting `updatedAt` remains untouched and no changelog is emitted), unknown ID (`404`).  |
| **`delete_<module>_test.go`** | Deletion endpoint tests: Single delete, **bulk delete with unknown ID** (verifying partial deletion: valid rows deleted, missing rows skipped), empty payload (`400`), auth checks. |
| **`list_<module>_test.go`**   | Complete query matrix: Search, multi-column search scoping, sort orders, pagination partitioning, date filters, combined filters, and file export parity (`POST /export`).          |

---

#### The List, Search, Sort & Export Testing Standard

Query endpoints (`POST /<module>/list` and `POST /<module>/export`) must follow strict testing rules to avoid flaky assertions across shared tenants:

1. **Fixture Isolation via Run Tokens**:
   - List tests must generate a unique run token (e.g. `tok := fmt.Sprintf("z%06d", time.Now().UnixNano()%1000000)`).
   - Fixture records incorporate `tok` into their names and symbols.
   - All list queries scope by `search: tok`. This guarantees that rows created by other test files in the shared tenant never distort counts or sort orders.
2. **Asymmetric Fixtures for Sort Testing**:
   - Fixture sets must be designed such that sorting alphabetically by Name yields a **different** sequence than sorting by Symbol or Code. Tests where all attributes sort identically cannot distinguish `sortBy` from default sorting.
3. **Search Invariants**:
   - Verify multi-column matching by default.
   - Verify case-insensitivity and partial substring matching (`ILIKE`).
   - Verify column-scoped searching (`columns: ["name"]`), ensuring search terms matching other columns are excluded.
   - Verify invalid columns in `columns` fall back safely without error.
   - Verify unmatched searches return `totalRecords: 0` and empty `data: []`.
4. **Sort Invariants**:
   - Verify ascending (`sortOrder: 1`) and descending (`sortOrder: -1`) for strings, numbers, and `updatedAt`.
   - **SQL Injection Safety**: Passing arbitrary strings to `sortBy` must safely fall back to default sort columns without SQL syntax errors.
   - Verify invalid sort directions (e.g. `sortOrder: 2`) are rejected with `400 Bad Request`.
5. **Pagination Invariants**:
   - **Page Partitioning**: Fetching all pages sequentially (e.g. limit 2 across pages 1, 2, 3) must reconstruct the full sorted list exactly with zero dropped rows and zero duplicates across page boundaries.
   - **`totalRecords` Integrity**: `totalRecords` must always reflect the total matched records across the entire dataset, never the count of the current page slice.
   - **Boundary Enforcement**: Verify default pagination values (page 1, limit 10) and validation rejection on invalid values (`limit > 100`, `page <= 0`).
6. **Filter Invariants**:
   - Date range filters (`updatedAt` array) must be tested with open-ended bounds (`[start]`), closed ranges (`[start, end]`), and non-matching future/past windows.
   - Verify combined queries (search + filter + sort + pagination) compose accurately.
7. **Export Parity (`POST /export`)**:
   - Verify `/export` honors the identical search and filter criteria as `/list`.
   - Verify export is unpaginated (ignores `limit` and exports all matching rows).
   - Verify `Content-Disposition` header contains `attachment; filename=...`.
   - Verify empty search matches produce a valid header-only export file.

---

#### Asynchronous Assertions & Eventual Consistency (`awaitChangelogs`)

Platform actions that dispatch asynchronous Temporal child workflows (e.g., recording audit changelogs in `analytics-reporting-service`) or publish Outbox events cannot be asserted synchronously:

```go
// Asynchronous poller with deadline in main_test.go
func awaitChangelogs(t *testing.T, ctx context.Context, search, objectID, action string, want int) []changelogItem {
    t.Helper()
    deadline := time.Now().Add(30 * time.Second)
    for {
        resp, err := client.Post[common.Paginated[changelogItem]](ctx, harness.ARBaseURL+"/change-logs/list", auth(), map[string]any{
            "page": 1, "limit": 100, "sortBy": "createdAt", "sortOrder": -1,
            "search": search, "columns": []string{"objectName"},
        }, 200)
        require.NoError(t, err)

        var found []changelogItem
        for _, l := range resp.Data {
            if l.ObjectID != nil && *l.ObjectID == objectID && l.Action == action {
                found = append(found, l)
            }
        }
        if len(found) >= want {
            return found
        }
        if time.Now().After(deadline) {
            t.Fatalf("changelog: %s had %d/%d %q rows before deadline", objectID, len(found), want, action)
        }
        time.Sleep(500 * time.Millisecond)
    }
}
```

1. **Deadline-Bounded Polling**: Poll the read endpoint with a generous deadline (e.g. 30 seconds) and periodic intervals (500ms). Never use hardcoded arbitrary `time.Sleep` before a single assertion.
2. **Action & Entity Scoping**: Pollers must filter by both `objectID` and `action` (`Create`, `Edit`, `Delete`) so that earlier audit entries created during fixture setup do not satisfy later mutation assertions prematurely.
3. **Exact Field & Boundary Verification**:
   - Verify that creates log `-` as `oldValue`, deletes log `-` as `newValue`, and updates log both `oldValue` and `newValue`.
   - Verify that no-op updates emit **zero** changelog rows and leave `updatedAt` untouched.

---

#### Execution & Environment Configuration

- **Default Execution**: Tests run against the standard local Docker Compose stack (`http://localhost:8080` for the Gateway, `http://localhost:8025` for Mailpit).
- **Environment Overrides**:
  - The test harness automatically locates `.env` at the module root (walking up to `go.mod`).
  - Configure custom environments via `.env` or direct shell variables:

    ```bash
    # Run full flow suite
    go test ./flows/... -v

    # Run specific domain package
    go test ./flows/enterprise-management/measurement/... -v

    # Run against remote or custom compose stack
    E2E_GATEWAY_BASE_URL=http://localhost:9090 E2E_MAILPIT_BASE_URL=http://localhost:9025 go test ./flows/...
    ```

- **Pre-Flight Health Guard (`RequireStack`)**: Test packages check `/health` on Core Platform Services before running. If the stack is unreachable, tests fail immediately with a clear diagnostic message: `stack not ready ... run: docker compose up -d`.

---

#### E2E Testing Rules & Prohibitions

1. **All Traffic Goes Through the Gateway**: Tests must interact strictly with `E2E_GATEWAY_BASE_URL` (Traefik Gateway). Direct connections to internal service ports (which are not published anyway — ADR 020) are forbidden.
2. **No Fixed / Static Entity Names**: Always use `uniqueName(prefix)` to prevent collisions in unique database indices across test executions.
3. **Assert Auth Gates on Every Module**: Every module test suite must include subtests confirming that requests without a session cookie or with an invalid token receive `401 Unauthorized`.
4. **Branch Naming**: Feature and test additions in the dedicated E2E test repository strictly use the **`test/`** branch prefix (e.g. `test/add-borrowing-e2e`, `test/client-onboarding-journey`).

> [!NOTE]
> **Service Interdependencies in Integration Tests**:
> Because backend services rely on `iam-service` for session/role resolution (`forwardAuth` & `UserAccessCache`) and `enterprise-management-service` for tenant discovery (`/internal/clients`), full API integration tests require the running multi-service environment managed by Docker Compose or the dedicated test harness.

## 13. Dependencies

We maintain lean dependency trees, favor the Go standard library, and manage shared internal packages cleanly.

### Shared Internal Libraries

Shared code is split into versioned, dedicated modules rather than duplicated across services:

| Package                 | Repository / Path      | Purpose                                                                                                                                                             |
| ----------------------- | ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **`service-contracts`** | `service-contracts`    | Wire types (DTOs), API constants, and typed Temporal activity wrappers for cross-service calls (ADR 006).                                                           |
| **`be-core-utils`**     | Shared toolkit repo    | Core infrastructure: `coreEntity` (HttpError), `coreHelper` (responses), `service` (`PostgreSQLServicer`, `TemporalService`), validator, outbox, and config loader. |
| **`ts-utils/v2`**       | Shared middleware repo | Cross-cutting HTTP middleware, RBAC gate rules (`rbac.Require`), `LoadUserRoles`, and client constants.                                                             |
| **`mig`**               | `pkg/runner`           | Domain-agnostic SQL migration engine embedded in each service (ADR 001).                                                                                            |

### Service Dependency Topology

Our services fall into two tiers: **Core Platform Services** (which almost all services depend on for baseline infrastructure) and **Domain Services** (which interact via `service-contracts` and Temporal):

```
                       [ Traefik Gateway ]
                               │
               ┌───────────────┴───────────────┐
               ▼                               ▼
       [ Domain Services ]             [ Core Platform Services ]
  (MAR, Asset Transaction,              • iam-service (AuthN / RBAC cache)
   Import, File Manager, ...)           • enterprise-management (Tenants)
               │                               │
               └───────────────┬───────────────┘
                               ▼
                    [ Infrastructure Tier ]
              (PostgreSQL, Valkey, Temporal, Redpanda)
```

- **Core Platform Dependencies (Baseline)**:
  - **`iam-service`**: Required by the Gateway (session validation) and by services for RBAC snapshot resolution (`LoadUserRoles`).
  - **`enterprise-management-service`**: Required for multi-tenant boot-up and tenant discovery (`/internal/clients`).
  - **`Valkey`**: Required for user session and RBAC cache lookup.
- **Domain Service Dependencies**:
  - Domain-to-domain interactions (e.g., Asset Transaction invoking MAR reservations) are declared via **`service-contracts`** and executed over Temporal activities (ADR 006).
  - For local development and full integration testing, start the baseline environment via `docker-compose up`.

### Dependency Rules

1. **Stdlib-First**: Always check if Go standard library packages (`net/http`, `log/slog`, `context`, `encoding/json`, `crypto`) solve the problem before introducing an external third-party library.
2. **Private Go Modules (`GOPRIVATE`)**:
   - Internal modules hosted on private git instances must be declared in `GOPRIVATE` (e.g. `GOPRIVATE=gitea.tagsamurai.local/*`).
   - Private module credentials must be injected into Docker builds via **BuildKit secrets** (`--mount=type=secret`), never baked into Dockerfiles or image layers.
3. **Semantic Versioning & Major Bumps**:
   - Breaking changes to shared libraries (`ts-utils`, `be-core-utils`) require a major version bump in the Go module path (e.g. `ts-utils/v2`).
   - Services upgrade shared dependencies deliberately via PRs, verifying compatibility before bumping.
4. **No Direct Inter-Service Go Module Dependencies**:
   - Service repositories must **never** import Go packages directly from another service's repository.
   - All inter-service data models, DTOs, and activity signatures must be imported exclusively from **`service-contracts`**.
5. **No Forked Dependencies**: Do not fork or vendor private copies of shared packages inside a service's `internal/` directory. Fixes belong upstream in the shared repository.

## 14. Security

Security is enforced at the network boundary, the route gate, and the data isolation layer — not through ad-hoc checks.

### 1. Network Boundary & Port Isolation (ADR 020)

- **Single Ingress**: Only the Traefik Gateway publishes host ports (`80`/`443`).
- **No Service Host Ports**: Backend services must **never** bind or publish host ports in `docker-compose.yml` or production. They communicate exclusively on the internal Docker network.
- **Physical Boundary**: This network isolation is load-bearing: it is what prevents malicious clients from bypassing authentication or directly forging trusted headers.

### 2. Identity & Header Integrity (ADR 020)

- **Gateway-Driven Authentication**: Traefik authenticates every request via `forwardAuth` to `iam-service /auth/validate`.
- **Header Stripping**: Traefik strips any incoming `X-User-ID`, `X-Client-ID`, and `X-Service-Token` headers sent by external clients before injecting verified values.
- **Never Trust Request Payloads for Identity**: Services must never extract user or tenant identity from JSON request bodies or query parameters. Always read from gateway-injected headers (`X-User-ID`, `X-Client-ID`).
- **Internal Route Protection**: `/internal/*` endpoints are protected by `X-Service-Token` and are blocked from external access at the gateway (returning 404).

### 3. Centralized RBAC Ownership (ADR 005, ADR 010, ADR 013)

- **One Authority**: `iam-service` alone touches role tables and writes the `UserAccessCache` snapshot into Valkey. No other service duplicates or modifies role definitions (ADR 005).
- **Direct Snapshot Queries**: Services query the `UserAccessCache` snapshot directly via helper methods (`HasGroupPermission`, `GroupIDsFor`), never copying grants into derived intermediate types that risk dropping permissions (ADR 010).
- **Visible Route Gates**: Authorization gates (`rbac.Require`) are mounted directly on route definitions where a missing gate is visibly obvious (ADR 013).

### 4. Tenant Isolation & SQL Injection Prevention (ADR 001)

- **Schema Isolation**: Each tenant's data lives in an isolated PostgreSQL schema named after the validated `X-Client-ID`.
- **Schema Name Validation**: All tenant identifiers are strictly validated against `^[a-zA-Z0-9_]{1,50}$` before setting schema search paths.
- **Parameterized Queries**: All database queries must use parameterized placeholders (`$1`, `$2`) via `pgx/v5`. Raw SQL string concatenation is strictly forbidden.

### 5. Secrets & Logging Hygiene

- **No Hardcoded Secrets**: Secrets and tokens must be loaded from environment variables on service boot.
- **BuildKit Secrets**: Private module tokens for Go builds must use Docker BuildKit secrets (`--mount=type=secret`), never `ENV` or `ARG` values baked into image layers.
- **Sanitized Logs**: Never log credentials, passwords, OTPs, session cookies, or authorization tokens.

### 6. Blast Radius Control (ADR 014)

- High-impact destructive operations (such as dropping all database schemas when purging a retired client) must **never have an HTTP route**. Reachability is governed by blast radius, not route permissions.

## 15. Definition of Done

A task, feature, or bugfix is considered **"Done"** only when it satisfies all of the following criteria before merging into protected branches:

### 1. Architectural & Code Quality Checklist

- [ ] **Layering strictly followed**: `handler` → `service` → `repository` / `workflow` → `activity` (ADR 003).
- [ ] **Cross-service boundaries respected**:
  - The service accesses **only its own database tables** (ADR 001).
  - Cross-service writes are routed through Temporal activities on the owning service's task queue with compensation registered where rollback is required (ADR 006).
  - Cross-service reads are performed via HTTP _before_ opening a local database transaction (ADR 006 §4b).
  - Shared types, wire DTOs, and activity wrappers are placed in `service-contracts`.
- [ ] **Go standards satisfied**: Formatted with `gofmt`/`goimports`, stdlib-first, constructor injection in services, and Snowflake IDs typed as `int64`.

### 2. Database & Migrations

- [ ] Schema changes include symmetric `.up.sql` and `.down.sql` migrations in `pkg/migrations/{public,tenant}` (ADR 001).
- [ ] **Model & Migration Parity**: Go struct fields in `internal/model/` strictly match SQL column names, data types (`BIGINT` as `int64`), and nullability in `pkg/migrations/`.
- [ ] Migrations tested and verified via `mig test` and `mig verify --schema <name>`.
- [ ] Schema operations are tenant-safe and backward-compatible (ADR 008).

### 3. Security & Authorization

- [ ] Protected endpoints mount `rbac.Require(...)` route gates properly ordered after `LoadUserRoles` (ADR 013).
- [ ] Internal service-to-service endpoints are placed in `SetupInternalRoutes` under `/internal/*` protected by `RequireServiceToken`.
- [ ] Identities (`X-User-ID`, `X-Client-ID`) are consumed exclusively from gateway-injected headers, never request bodies (ADR 020).

### 4. Error Handling & Logging

- [ ] Handlers return errors via `coreEntity.ToHttpError(err).SendResponse(c)` and successes via `coreHelper.Success(c, msg, data)`.
- [ ] Internal errors are wrapped with context (`%w`); client error messages do not leak SQL or internal stack traces.
- [ ] Logs use `slog` with `context.Context` and structured attributes (no `fmt.Println` or raw secrets).

### 5. Testing & Verification

- [ ] **Unit Tests**: Unit tests implemented for all new handlers, domain services, and repository queries using stateful in-memory fakes (ADR 015) and Temporal in-memory test suites (`go.temporal.io/sdk/testsuite`); `go test ./...` passes with zero failures.
- [ ] **API Integration & E2E Tests**: Cross-service user journeys and API workflows covered in the dedicated E2E test repository (or verified via the Bruno test collection against the running gateway stack); all scenarios pass cleanly.
- [ ] **API Documentation**: Bruno API collection updated with `.bru` file(s) under `docs/api/bruno/` for all new or modified endpoints, including sample headers, request body, and assertions.
- [ ] **PR & Review**: PR description includes context, changes, verification proof, and links to relevant ADRs; at least one peer review approval received and CI checks are green.

## 16. Exceptions & Changing Conventions

Engineering standards are living rules grounded in real systems. When a standard no longer fits, we change it deliberately — not through silent code drift.

### The Core Rule

> **Change is made by writing an Architecture Decision Record (ADR), not by drift.**
> Conventions change when someone records the decision, tradeoffs, and consequences — never silently, per-file, in passing. A rule in this document that no longer matches the codebase is a bug in this document, not a licence to drift.

### How to Propose or Change a Convention

To introduce a new architectural pattern, replace a library, or modify an existing standard:

1. **Author an ADR**: Create a numbered proposal file in the central `go-code-standard` repository (under `docs/adr/`, e.g., `021-introduce-http-query-method.md`).
2. **Follow the ADR Structure**:
   - **Header**: Status (`Proposed` → `Accepted` / `Rejected` / `Superseded`) and Date.
   - **Context**: The real problem, forces at play, and alternatives evaluated.
   - **Decision**: Plain statement of the rule or pattern being adopted.
   - **Consequences (Mandatory)**: Both positive benefits and negative costs/tradeoffs. _An ADR with no downsides is an advertisement, not an engineering decision._
   - **Verified**: Include only if the behavior was exercised and proven against live services.
   - **Known Gaps**: Explicitly list deferred concerns or unknowns.
   - **Length**: Keep it concise (1–2 pages). Do not include redundant summary sections.
3. **Team Review**: Open a Pull Request containing the ADR for team review and consensus.
4. **Update `engineering.md`**: Once accepted, update this standards document across service repositories to reflect the new standard and link to the ADR.

### Handling Legacy Code & Temporary Exceptions

- **Existing Legacy Code**: Where existing code predates a convention (e.g., legacy directory naming or unmigrated tests), it is tracked explicitly in the ADR's _Known Gaps_ table.
- **Explicit Comments for Deviations**: If a temporary exception is genuinely necessary, it must be marked with an explanatory comment referencing the tracking issue or ADR (e.g., `// LEGACY (ADR 003): pending migration to constructor injection`).
- **No Unrecorded Deviations**: Never bypass multi-tenancy, cross-service write rules, or route gates without an accepted ADR.
