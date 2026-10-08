# AI Coding Agent Operating Instructions (`AGENTS.md`)

> Repository status: this service currently contains only the Fiber v3 HTTP
> lifecycle, configuration, and health endpoint. The platform rules below
> apply as domain features are introduced; models, migrations, and shared
> libraries mentioned here are not yet present. See `README.md` for runnable
> commands and current capabilities.

You are an expert Go backend software engineer pair programming on this platform.
Before analyzing, writing, modifying, or refactoring any code, you MUST strictly adhere to the standards, architecture, and constraints defined below.

---

## 1. Authoritative References & Standards

You must treat the following self-contained documents located at the root of this service repository as the canonical source of truth:

- [**`engineering.md`**](./engineering.md): Comprehensive team conventions, error handling, logging, testing, and Definition of Done.
- [**`architecture.md`**](./architecture.md): System topology, Traefik perimeter, multi-tenancy, Temporal sagas, outbox event streaming, and multi-repo release workflows.
- **Architectural Decision Records (ADRs 001–020)**: The foundational rationales for platform decisions are cited and embedded directly throughout `engineering.md` and `architecture.md`.

---

## 2. The 4-Layer Service Architecture (ADR 003)

Every service strictly enforces a 4-layer separation of concerns:

```
HTTP Request ──► [Router] ──► [Handler] ──► [Service] ┬──► [Repository] ──► PostgreSQL
                                                      └──► [Workflow]   ──► [Activity] ──► External Services
```

### Layer Boundaries & Prohibitions

1. **Router (`<domain>_routes.go`)**:
   - Mounts routes, auth middleware, and authorization gates.
   - **Mounting Order Rule (ADR 013)**: Mount unauthenticated/internal routes first (`SetupInternalRoutes`) → Mount `tsMiddleware.LoadUserRoles` → Mount protected routes with `rbac.Require(...)`.
2. **Handler (`handler/`)**:
   - Parses HTTP body/params, validates input, calls **exactly one** service method, and formats the standard JSON envelope (`coreHelper.Success` or `coreEntity.ToHttpError(err).SendResponse(c)`).
   - **Request Body & Validation Rule**: Always use the 2-tier validation pattern:
     - On body bind failure: `coreEntity.BadRequest("Invalid request body", validator.ParseBindError(err)).SendResponse(c)`
     - On struct validation failure: `coreEntity.BadRequest("Validation failed", validator.ParseValidationErrors(err)).SendResponse(c)`
     - Never return raw, unparsed Go error strings or swallow bind errors with generic messages lacking field error details.
   - **STRICT PROHIBITION**: Handlers must **NEVER** write SQL queries or import the Temporal SDK directly.
3. **Service (`service/`)**:
   - Encapsulates business logic. Receives dependencies via **constructor injection** (`New<Domain>Service(...)`).
   - Delegates database reads to repositories and multi-step cross-service writes to Temporal workflows.
4. **Repository (`repository/`) & Database Queries**:
   - Constructs and executes database queries only. Accepts `context.Context` and `coreService.PostgreSQLServicer`.
   - **STRICT PROHIBITION**: Repositories must **NEVER** write raw SQL strings (e.g. `SELECT ...`, `INSERT INTO ...`, `UPDATE ...`, `DELETE FROM ...`) or format SQL queries via `fmt.Sprintf` / string concatenation.
   - **DATABASE-FIRST PROCESSING MANDATE**:
     - Push transformations, computations, formatting, aggregation, filtering, and tree traversals down to PostgreSQL via `sql_query`.
     - Keep Go application processing at an **absolute minimum**. Do NOT fetch broad datasets into Go memory to loop, filter, format dates/strings, or calculate summary totals.
     - Go in-memory processing is strictly permissible **ONLY** when an operation is genuinely impossible in SQL (e.g. calling an external crypto hardware module, invoking external third-party HTTP APIs, or evaluating dynamic Go runtime functions).
   - **QUERY BUILDER EXECUTION (`be-core-utils/sql_query`)**:
     - Construct all queries using `be-core-utils/sql_query` (`sql_query.NewSQLSelectBuilder[T]`) and execute via `coreService.PostgreSQLServicer` methods (`SelectOne`, `SelectMany`, `CountWithFilter`, `InsertOneWithData`, `UpdateOneWithData`, `DeleteManyWithFilter`).
     - For list queries with complex filtering or joins, colocate builders under `internal/module/<domain>/query_builder/query_builder.go`.
     - Consult the **`be-core-utils-best-practices`** skill for canonical syntax patterns: database formatting (`to_char`, `COALESCE`), conditional expressions (`SelectCaseWhen`), hierarchical CTEs (`WithRecursiveCTEBuilder`), lateral joins (`CrossJoinLateral`), JSON/array aggregations, and pagination summary aggregates (`Pagination.Aggregates`).
5. **Export Endpoints**:
   - Handlers and services must **NEVER** import `github.com/xuri/excelize/v2` or `encoding/csv` directly.
   - Tag row structs with `export:"<Header>"` tags and delegate file rendering directly to `be-core-utils/helper.Export`.

---

## 3. Data, Models & Multi-Tenancy Rules

1. **Pre-Prepared Models and Migrations**:
   - Database migrations (`pkg/migrations/`) and entity models (`internal/model/`) are designed and provided in advance.
   - Do **NOT** generate new SQL migrations or alter table schemas unless explicitly instructed.
   - Your initial step is to inspect existing models in `internal/model/` and verify consistency against the specification.
2. **Snowflake ID, Date & Timestamp Typing (ADR 020)**:
   - **Snowflake IDs**: Always use `int64` for all Snowflake ID fields (`id`, `user_id`, `client_id`, `asset_id`). On the wire (JSON envelopes, path parameters, OpenAPI), serialize as decimal strings (`pattern: ^[0-9]+$`, never UUID).
   - **Date & Timestamp DTO & Model Typing**:
     - Always type date and timestamp fields in entity models and response/item DTOs as `time.Time` (or `*time.Time` if nullable).
     - **OpenAPI Schema Translation**: Even though OpenAPI specifies dates as `type: string, format: date-time` (or `format: date`), **NEVER** type date fields as `string` or `*string` in Go DTOs. Go's standard `encoding/json` natively serializes `time.Time` to RFC3339 strings.
     - **Zero SQL Date Formatting**: **NEVER** use `to_char(...)` or SQL string formatting on date/timestamp columns. Map the raw database column directly (`column:"table.updated_at"`). The database driver (`pgx`) scans native temporal types directly into `time.Time`.
   - **Filter Inputs**: Request query DTOs accepting date range filters use epoch milliseconds (`[]int64`).
3. **Multi-Tenancy & Data Isolation (ADR 001)**:
   - Each service reads and writes **ONLY** its own database tables within the tenant's PostgreSQL schema (`client_<id>`).
   - **Cross-service SQL joins and foreign keys are strictly forbidden.**
4. **Connection Pool Safety (ADR 004)**:
   - Never share a single `PostgreSQLService` instance across concurrent goroutines (it carries active `pgx.Tx` state). Obtain tenant servicers dynamically per request (`s.db.ForTenant(clientID)`).
5. **Model & Migration Parity**:
   - The Go entity models in `internal/model/` and SQL schemas in `pkg/migrations/` must be **100% identical and consistent**.
   - Every column in the SQL migration must exist as a tagged field in the Go model struct with matching `column:"table.field"` and `json:"field"` tags, `int64` for `BIGINT`, and pointer types for nullable columns.

---

## 4. Inter-Service Communication & Multi-Repo Rules

Apply this rule: _"Does this step need to be undone (rolled back) if a subsequent step fails?"_ (ADR 006)

1. **Strict Prohibition of Cross-Service Imports**:
   - This service repository must **NEVER** declare a Go module dependency on another service repository.
   - All shared wire DTOs, event schemas, and Temporal activity wrappers MUST be imported exclusively from **`service-contracts`**.
2. **Synchronous Reads (Internal HTTP)**:
   - Fetch external data via `net/http` using DTOs from `service-contracts`.
   - Send `X-Service-Token` and `X-Client-ID` headers.
   - **Transaction Rule (ADR 006 §4b)**: Perform external HTTP calls **before** opening local database transactions.
3. **Cross-Service Writes (Temporal Sagas)**:
   - Execute multi-step writes via Temporal activities running on the **owning service's task queue** using wrappers from `service-contracts`.
   - Register rollbacks immediately via `WorkflowCompensation.AddCompensation` and execute in a `defer` block on `workflow.NewDisconnectedContext(ctx)` (ADR 006 §2).
4. **Event Broadcasting (Transactional Outbox - ADR 017)**:
   - Insert event records into `public.outbox` inside the same database transaction as the entity mutation.

---

## 5. Security & Ingress Conventions (ADR 020)

1. **Header-Based Identity**:
   - User identity (`X-User-ID`) and tenant context (`X-Client-ID`) are injected exclusively by the Traefik `forwardAuth` gateway.
   - Never extract identity from request bodies or URL query parameters.
2. **Direct Snapshot Evaluation (ADR 010)**:
   - Evaluate permissions directly against the `UserAccessCache` struct (e.g. `userAccess.HasGroupPermission(...)`). Never derive intermediate `Scope` structs.
3. **Zero Host Port Exposure**:
   - Backend services run strictly within the private container network and do not publish host ports.

---

## 6. Error Handling & Logging

1. **Error Return Format**:
   - Handlers convert errors via `coreEntity.ToHttpError(err).SendResponse(c)`.
   - Use standard constructors: `coreEntity.BadRequest`, `coreEntity.NotFound`, `coreEntity.Forbidden`, `coreEntity.InternalServerError`.
   - On request body binding and validation errors, always provide structured field error details via `validator.ParseBindError(err)` and `validator.ParseValidationErrors(err)` to `coreEntity.BadRequest`.
2. **Error Wrapping**: Always wrap internal errors with `%w` (`fmt.Errorf("failed to query asset: %w", err)`).
3. **Structured Logging**: Use `log/slog` with context-aware methods (`slog.InfoContext`, `slog.ErrorContext`) to include tracing metadata.

---

## 7. Testing Standards (ADR 015)

1. **Handler Unit Test Scope & Lean Boundary**:
   - Handler unit tests (`<module>_handler_test.go`) MUST remain lean and scoped strictly to HTTP transport concerns in **one single file** (~300–400 lines):
     - Request body binding (`400 Bad Request` on malformed JSON).
     - Input struct validation (`422 Unprocessable Entity` on invalid/missing fields).
     - Route & RBAC gating (`401 Unauthorized`, `403 Forbidden` across role variations).
     - Standard JSON envelope formatting (`200 OK`, `201 Created`).
   - **STRICT PROHIBITION**:
     - **NEVER implement the full `tc-*-backend.md` test matrix inside handler unit tests.**
     - Handlers must **NEVER** mock database queries (searching, sorting, pagination, tenant isolation, transaction locks) using bloated mock functions.
     - Full integration verification of `tc-*-backend.md` belongs **EXCLUSIVELY in `end-to-end-test`**.
2. **Stateful In-Memory Fakes**:
   - In repository unit tests, use **stateful in-memory fakes** backed by Go maps/mutexes, **NOT** fragile mock frameworks with brittle `.On()` chains.
3. **Temporal Workflow Tests**:
   - Use `go.temporal.io/sdk/testsuite` in-memory test environments to test saga orchestration and compensation rollbacks.

---

## 8. API Documentation (Bruno Collections)

1. **Mandatory Standard Endpoint Files**:
   - Maintain **exactly 1 `.bru` file per endpoint** under `docs/bruno/<module>/<Action>.bru` or `docs/api/bruno/<module>/<Action>.bru` (e.g. `List_Positions.bru`, `Create_Position.bru`, `Update_Position.bru`, `Delete_Positions.bru`).
   - Each file represents the canonical happy path with realistic sample headers, body, and status assertions (`res.status === 200/201`).
2. **STRICT PROHIBITION against Test Case Matrices (`tc/*.bru`)**:
   - **NEVER generate duplicate `.bru` files for every negative edge case or validation error** (e.g. creating a `docs/bruno/<module>/tc/` directory with dozens of `.bru` files).
   - Bruno is for human API exploration and documentation, NOT automated regression testing. Automated validation and error checking belong in `end-to-end-test`.

---

## 9. Operating Modes, Sub-Agent Policy & Execution Protocols

### Operating Modes (Audit vs Implementation)
Always distinguish the user's intent before taking action:

1. **Audit / Compliance / Review Mode (READ-ONLY)**:
   - **Trigger**: When asked to review, audit, check compliance, evaluate gaps, or verify code against PRDs or test cases (`tc-*-backend.md`).
   - **Action**: Inspect the code and specifications, perform gap analysis, and output an actionable Markdown findings report.
   - **STRICT PROHIBITION**: Do **NOT** generate, modify, or delete code, unit test suites, or Bruno `.bru` files during an audit. Await explicit user instruction before executing any fixes.

2. **Feature / Implementation Mode (Write & Build)**:
   - **Trigger**: When assigned a new feature, endpoint, refactor, or approved bugfix.
   - **Action**: Follow the **4-Phase Execution Protocol** below.

### Sub-Agent Usage Policy
Apply sub-agents deliberately based on task scope:
1. **Full Feature / Module Implementation (Large Scope)**:
   - When scaffolding or implementing an entire domain module (spanning Router → Handler → Service → Repository → Tests):
   - You SHOULD delegate Phase 1 (Spec & M-3 Audit) and Phase 4 (Verification) to dedicated sub-agents to avoid context saturation.
   - The final DoD verification MUST be audited via an independent sub-agent running `review-gate` to prevent self-verification bias.
2. **Targeted Tasks / Bugfixes / Single Endpoints (Small Scope)**:
   - Execute directly in the primary agent thread.
   - Do NOT spawn sub-agents for isolated single-file fixes, small refactors, or local test debugging to maintain fast iteration and low latency.

### The 4-Phase Execution Protocol (Implementation Mode)

1. **Phase 1 (Plan First)**: Read the PRD/LLD, inspect existing code, and output a step-by-step implementation plan for review. **Do not write code until the plan is approved.**
   - **PRD Discovery & Source of Truth**: Locate target module specifications using the `ts-docs-uiux` MCP server (`search_files`, `read_file`) or under `docs/<domain>/<module>/PRD/` (or sibling `../docs/<domain>/<module>/PRD/`). If a single-file canonical PRD (`<module>.md`) exists, treat it as the sole canonical source of truth and consult companion files (`<module>.changelog.md` and `<module>.resolved.md`). **Never read or cite archived/deprecated `01-overview.md`..`04-dependencies.md` files when `<module>.md` is present.** Only fall back to modular 4-file PRDs for unmigrated modules lacking `<module>.md`.
   - **Audit & Baseline Discovery**: Inspect module audit files via `ts-docs-uiux` or under `docs/<domain>/<module>/audit/` (`findings.md`, `api-prd-gaps.md`) to incorporate settled architecture decisions, closed bugs, and known gaps into the plan before writing code.
   - **Critical PRD Sections for Implementation**:
     - **§9.4.3 Must NOT**: Explicit negative constraints that the implementation must actively block or prevent.
     - **§9.6 Action Resilience Spec**: Optimistic concurrency control (`updatedAt` / version check returning `409 Conflict`), idempotency, and partial-failure semantics.
     - **§4.2a Column Visibility & Setup Column**: Ensure all `Searchable: ✅` columns are covered by search query builders, and matching `export:"<Name>"` struct tags are provided.
     - **§11 Dependencies & Breaking Changes**: Verify required inter-service contracts under `SetupInternalRoutes`.
   - **API Specs**: Inspect API contracts via `ts-docs-uiux` or in `docs/<domain>/<module>/API/` (or `../docs/<domain>/<module>/API/`), including `openapi.yaml`, `low-level-design.md`, and modular endpoint specs (`*.yaml` and `*.md`).
   - **Strict Exclusion on UI TestCases**: Backend agents must **NEVER** inspect `TestCases/tc-*-automation.md` (Frontend E2E automation) or general UI-only manual test cases. Only inspect `TestCases/tc-<module>-backend.md` if present (via `ts-docs-uiux` or local path).
2. **Phase 2 (Model Verification)**: Inspect `internal/model/` and `service-contracts` to confirm all fields and types match the plan.
3. **Phase 3 (Layered Implementation)**: Build bottom-up: Repository (SQL) → Service / Temporal Workflow → Handler & Route Gates. Ensure all business validations and error conditions required by the PRD / `tc-*-backend.md` are implemented in the Go application logic.
4. **Phase 4 (Verification & DoD)**:
   - **Unit Tests**: Write lean handler unit tests (<400 lines) and stateful in-memory fakes for repository/service logic. Do NOT implement the `tc-*-backend.md` matrix in unit tests (that belongs in `end-to-end-test`).
   - **Bruno Docs**: Maintain exactly 1 canonical `.bru` file per endpoint (no `tc/*.bru` files).
   - **Linters & CI**: Run `go test ./...` and `golangci-lint run ./...` ensuring zero failures and zero layer/depguard violations.
   - **Gate Verification**: Verify against the Definition of Done in `engineering.md` Section 15 and audit with `review-gate`.
