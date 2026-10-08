---
name: review-gate
description: >-
  Final Definition of Done (DoD), ADR compliance, and backend Test Case verification
  gate for Tag Samurai backend services. Use when conducting a PR review, auditing
  code changes against PRD/API specs and tc-<module>-backend.md, verifying 4-layer
  architecture boundaries (ADR 003), checking model/migration parity, or generating
  copy-pasteable review findings before merging. Read-only: reports findings and
  never edits, fixes, or generates code.
---

# Tag Samurai Backend Review Gate & DoD Enforcer

This skill is the authoritative final review gatekeeper for backend services. It audits
submitted code changes or pull requests against:

1. **Product Requirements** — `docs/<domain>/<module>/PRD/` (single-file `<module>.md`
   preferred; legacy 4-file fallback), resolved through the `ts-docs-uiux` MCP server
   first, local files only as fallback.
2. **API Specs** — every `*.yaml` and every `*.md` in `docs/<domain>/<module>/API/`.
   Modular specs use arbitrary filenames; never assume `low-level-design.md`.
3. **Backend Test Cases** — `docs/<domain>/<module>/TestCases/tc-<module>-backend.md`,
   the backend definition of done (`foundation/testcase-backend-conventions.md`).
   **Only** this file is compared; never `tc-<module>.md`, `tc-<module>-frontend.md`,
   or `tc-*-automation.md` (UI/automation scopes).
4. **Module audit baselines** — `audit/findings.md`, `audit/api-prd-gaps.md`, plus
   `<module>.changelog.md` and `<module>.resolved.md` for settled decisions.
5. **Engineering standards** — `engineering.md`, `architecture.md`, `AGENTS.md`, and
   the ADRs; DoD = `engineering.md` Section 15.
6. **Shared auth contract** — `auth/me/API/openapi.yaml` plus its `.md` design doc, for
   persona/permission patterns when testing Authorization.
7. **Foundation baselines** (mandatory and platform-wide, applied even when module
   docs are silent): `foundation/testcase-backend-conventions.md`,
   `foundation/process/prd-process/error-handling-convention.md` §5, and
   `foundation/process/prd-process/edge-case-matrix-convention.md` D1–D10.

This skill is read-only. It reports findings; it does not edit, fix, or generate code.

---

## 0. Mechanical preflight (run before any semantic review)

Deterministic checks come first: they are exact and cheap, and semantic review should
only handle what they cannot decide. If the build or tests fail, report that first and
mark every semantic dimension **conditional** — do not audit behavior on a red build.

Run what exists in the repo:

```bash
git diff --name-only <base>...HEAD          # changed files
go build ./...
go test ./... -count=1
golangci-lint run ./...
```

Plus these exact greps, adjusted to the repo layout:

- raw SQL strings in repositories (must use `be-core-utils/sql_query`):
  `rg -n '"SELECT |"INSERT |"UPDATE |"DELETE ' internal/module/*/repository/`
- Temporal SDK imported by handlers:
  `rg -n 'go\.temporal\.io/sdk' internal/module/*/handler/`
- SQL date formatting: `rg -n 'to_char\(' internal/`
- identity read from payload instead of gateway headers: `rg -n '"X-User-ID"|"X-Client-ID"' internal/`
  and inspect body-bound structs
- model↔migration parity: every `pkg/migrations/` column has a matching
  `internal/model/` struct tag, with pointer/nullability parity
- `export:"..."` tags present for every list/export row column (PRD §4.2a)
- Bruno: exactly one `.bru` per endpoint; no `tc/` matrix directory
- when the module has `API/`:
  `python3 .agents/skills/write-api-folder/scripts/verify_api_folder.py --api-dir docs/<domain>/<module>/API`

Report the mechanical outcome as a compact evidence block (command → result).

---

## 1. Source loading (MCP-first, bounded)

Resolve documentation through the `ts-docs-uiux` MCP server first (`search_files`,
`search_content`, `read_file`); use local `docs/...` only when the MCP is unreachable
or does not expose the file. The same module can exist in several repos — confirm the
intended one. Backend source code is never available via the MCP; read it from the
service repository.

Load in this order:

1. `audit/findings.md` and `audit/api-prd-gaps.md` — settled decisions, known gaps.
2. `<module>.changelog.md` and `<module>.resolved.md`.
3. API specs — all `*.yaml` and all `*.md` in `API/`.
4. PRD — contract-bearing sections only (see Dimension 1) plus §11 dependencies.
5. `tc-<module>-backend.md` — extract every TC ID and its expected behavior.
6. `auth/me/API/openapi.yaml` + its `.md` — persona/permission patterns.
7. Foundation baselines listed above.

**Freshness gate.** Before faulting code, confirm the API spec still matches the PRD
and the backend Test Cases. If an open `api-prd-gaps.md` item, a changelog entry, or a
direct conflict shows the spec is stale, mark affected findings **Blocked — spec
drift**; never treat code as non-compliant against a stale contract.

**Bounded context pass.** Scope reading to the endpoints and shared helpers the diff
touches; widen only when a shared path changes. Read the PRD in targeted slices
(`search_content` for section anchors, then `read_file` only those pages). Stop once
every diff-touched behavior has evidence or a reported gap. Never full-read the PRD
"just in case".

**Settled-item suppression.** Map every candidate finding against closed IDs
(e.g. B1–B3, S1–S3) in `findings.md`. Do not re-flag accepted tradeoffs; if a closed
item regressed, report it and name the closed ID it contradicts plus why.

---

## 2. The 5 Core Review Dimensions

Whenever reviewing a module or git diff, systematically evaluate these 5 dimensions.

### Dimension 1: PRD & Business Logic Compliance
* **Endpoint & Verb Match:** Verify HTTP method and path alignment with the API specs
  (`API/*.yaml` and `API/*.md`). Reject undocumented or extraneous verbs/paths.
  Cross-check endpoints against interactive controls in PRD **§4.1 Pages & Components**
  (row actions, bulk actions, toolbar) and **§4.2 Layout & Wireframe Spec** (header
  actions, avatar hover controls, tab actions).
* **DTO & Field Completeness:** Check that all form fields and data columns exist in DTOs.
  * **Single-file PRD (`<module>.md`):** cross-check against **§4.1 Pages & Components**,
    **§4.2a Column Visibility & Setup Column**, and **§9.9 Validation Rules Table (M-3)**
    (authoritative for field names, types, nullability, min/max, required/optional) plus
    **§9.2 / §9.8** (logical data entities).
  * **Legacy 4-file PRD:** cross-check against `02-ui-design.md` and `03-functional.md`.
* **Export DTO & Column Parity:** list/export DTO structs must carry matching
  `export:"<Column Name>"` tags for all visible data columns in **§4.2a Setup Column**,
  so columns are not silently omitted from CSV/XLSX.
* **Field-Level Data Redaction & Visibility (PRD §9.11):** response DTOs must
  conditionally mask, omit, or `null` entity-restricted fields (e.g. Principal-only
  relations such as `deviceNames` in **§9.2 [F-UOM-06]** / **§9.11**) for
  non-qualifying tenants.
* **Negative Constraints & Invariants (PRD §9.4.3 Must NOT):** verify the service
  rejects, blocks, or prevents all forbidden actions (e.g. self-target mutations under
  `USER-LOGIC-01`, Default User tier modification under `USER-LOGIC-02`, bypassing
  selection limits, illegal cascade purges of audit trails).
* **Action Resilience & Concurrency (PRD §9.6):** idempotency, partial-failure semantics
  (bulk per-item outcomes vs single-item atomic rollback), and optimistic locking via
  `updatedAt`/version check returning `409 Conflict`.
* **Business Validation & Error Codes:** validate against PRD **§6 (Validasi & Error
  Messages)** and **§9.9 (Validation Rules Table)**:
  * master data limit / conflict / concurrency race → `409 Conflict` (not `400`);
  * gating / access forbidden → `403 Forbidden` (against §2 Entity Gate/Role Matrix,
    §9.3 Permissions, §9.10 Plan Gating);
  * entity not found → `404 Not Found`;
  * invalid payload / type conversion → `400 Bad Request` or `422 Unprocessable Entity`.
* **Filters, Sorting & Search Scoping:** the list query builder must cover all filter
  parameters, sortable columns (including computed counts/subqueries), and text-search
  fields in **§4.1**, **§4.2a** (every `Searchable: ✅` column), and **§5 Global Table
  Rules**.
* **Core Rules & Module Logic:** adherence to `Core Rules Implementation`
  (e.g. `plan-entitlement`, `system-limits`, 5-surface logging) and `Aturan Logika
  Modul` (`<PREFIX>-LOGIC-xx`).

### Dimension 2: Architecture & ADR Compliance
* **4-Layer Separation (ADR 003):** Router → Handler → Service → Repository.
  Handlers must NOT write SQL or import the Temporal SDK directly. Repositories must
  NOT contain business logic or formatting.
* **Multi-Tenancy & Schema Isolation (ADR 001, ADR 004):** no cross-tenant or
  cross-service SQL joins/foreign keys; queries target only the tenant schema
  (`client_<id>`); resolve tenant servicers per request (`s.db.ForTenant(clientID)`),
  never share a `PostgreSQLService` across goroutines.
* **Inter-Service Internal Route Contracts (ADR 013, ADR 020, PRD §11):**
  internal endpoints required by downstream services under `SetupInternalRoutes` must be
  implemented and protected by `tsMiddleware.RequireServiceToken(serviceToken)` — never
  empty stubs or commented-out code when referenced in PRD §11 / §12 / `04-dependencies.md`.
* **Cross-Service Writes & Outbox (ADR 006, ADR 008, PRD §9.7):** multi-step
  cross-service writes run via Temporal activities on the owning queue; rollbacks via
  `WorkflowCompensation` on a disconnected context; HTTP reads happen **before** opening
  local DB transactions; event sync via transactional outbox targets the matching pool
  (`public` vs tenant) and `outbox.NewRelay` runs in `app.go`; logging covers all
  applicable surfaces per PRD **§8.1–§8.5**.
* **Data Types & Security (ADR 020):** Snowflake IDs are `int64` internally and string
  `_id` on the wire (`pattern: ^[0-9]+$`, never UUID); entity/response DTO dates are
  `time.Time` / `*time.Time` (range filters use epoch milliseconds `[]int64`); identity
  comes only from gateway headers (`X-User-ID`, `X-Client-ID`); no direct Go module
  dependencies between service repos — shared contracts live in `service-contracts`.

### Dimension 3: Database & Model Parity
* **Model vs Migration Sync:** every column in `pkg/migrations/` matches
  `internal/model/` struct tags (`column:"..."`, `json:"..."`), pointer/nullability, and
  types (`BIGINT` ↔ `int64`, `TIMESTAMPTZ`/`DATE` ↔ `time.Time`/`*time.Time`).
* **Per-Endpoint Ownership (ADR 001):** every table in an endpoint's Database section
  belongs to the service under review. Remote data is reached through internal HTTP
  (`service-contracts` DTOs, `X-Service-Token`, `X-Client-ID`) or Temporal activities;
  any cross-service join/foreign key or another service's table is a finding.
* **DTO Date & Column Mapping:** reject date/timestamp fields typed `string`/`*string`
  or `to_char(...)` projections on timestamp/date columns; map the raw column to
  `time.Time`/`*time.Time`.
* **Query Builder Enforcement:** zero tolerance for raw SQL strings or string
  concatenation in repositories; use `be-core-utils/sql_query`.
* **Database-First Processing:** transformations, `COALESCE`, `CASE WHEN`, recursive
  CTEs, and aggregations must run in `sql_query`; in-memory Go post-processing is
  permitted only when genuinely impossible in SQL.
* **SQL Injection Prevention:** no raw interpolation; parameterize via
  `sql_query.WhereQuery` or `sql_query.SQLOperatorRaw` with `ExtraArgs`.

### Dimension 4: Test Suite Scope & Bruno Documentation
* **Unit Test Scope (ADR 015):** handler unit tests stay lean in one file
  (`<module>_handler_test.go`, ~300–400 lines) covering only HTTP transport concerns:
  body binding (`400`), struct validation (`422`), route/RBAC gates (`401`/`403`), and
  standard envelopes (`200`/`201`).
* **Strict Prohibition:** do not dump the `tc-*-backend.md` matrix into handler unit
  tests with bloated DB mocks (searching, sorting, pagination, tenant isolation,
  transactions). Full verification of that matrix belongs in `end-to-end-test`;
  Dimension 5 checks implementation compliance, not that unit tests contain the matrix.
* **Repository & Service Tests:** stateful in-memory fakes (maps/mutexes), not `.On()`
  mock chains; Temporal sagas via `testsuite`.
* **Bruno API Collection:** exactly one `.bru` per endpoint under
  `docs/bruno/<module>/` or `docs/api/bruno/<module>/`, canonical happy path with
  realistic sample headers/body. Reject duplicate TC matrices (e.g. a `tc/`
  subdirectory). Assertions must validate HTTP status, the envelope message against
  canonical PRD §6/§7 strings, and structural keys (`res.body.data._id` per ADR 020;
  `res.body.data.data` for pagination envelopes — never fabricated keys).

### Dimension 5: Backend Test Case Compliance
* `tc-<module>-backend.md` is the backend acceptance list. For every TC, find the
  implementation path (handler → service → repository) that satisfies it, or mark it
  `Missing` / `Diverged` with evidence. Cite the TC ID in the finding.
* Build the coverage matrix (see report format). A module with an `API/` folder but no
  `tc-<module>-backend.md` is itself a DoD finding
  (`testcase-backend-conventions.md` §2).
* Cross-check every write endpoint against baseline D1–D10 and
  `error-handling-convention.md` §5 — double-submit guard, unique constraints,
  concurrent write / optimistic lock — even when the module docs never mention them.
* Always probe: range-filter boundary inclusivity; `404` (not `403`) for IDOR so
  existence is not leaked; tenant isolation across `client_<id>`; `data`/`totalRecords`
  counts; exact response schema (no leaked or renamed fields).
* UI-only cases are out of scope: never derive or require compliance from
  `tc-<module>.md`, `tc-<module>-frontend.md`, or `tc-*-automation.md`.

---

## 3. Review Protocol & Step-by-Step Procedure

1. **Mechanical preflight (§0)** and record results.
2. **Extract changed files:** `git diff` against `main`/`develop` or the target commit;
   derive the affected endpoints and shared helpers.
3. **Load sources (§1) MCP-first, bounded**, applying the freshness gate and
   settled-item suppression.
4. **Execute Dimensions 1–4** against the affected code paths.
5. **Build the Dimension 5 backend-TC coverage matrix.**
6. **Generate the verdict report**, including blocked / not-verifiable items.

---

## 4. Standard Output Report Format

Generate findings in this actionable, copy-pasteable structure:

````markdown
### 🔍 PR Review Verdict: [Module Name]

| Gate | Status | Summary |
| :--- | :---: | :--- |
| 0. Mechanical preflight | ✅ / ⚠️ / ❌ | build, tests, lint, parity |
| 1. PRD & API Compliance | ✅ / ⚠️ / ❌ | [short summary] |
| 2. Architecture & ADRs | ✅ / ⚠️ / ❌ | [short summary] |
| 3. Model & Migration Parity | ✅ / ⚠️ / ❌ | [short summary] |
| 4. Test Suite Scope & Bruno | ✅ / ⚠️ / ❌ | [short summary] |
| 5. Backend TC Compliance | ✅ / ⚠️ / ❌ | [covered x/y TCs] |

#### Backend TC Coverage

| TC ID | Endpoint / Behavior | Status | Evidence |
| :--- | :--- | :---: | :--- |
| `TC-XX-01` | `POST /...` — unique name returns 409 | Covered | `service/create.go:L42-L60` |
| `TC-XX-02` | date range boundary inclusive | Missing | no boundary test/logic found |

#### Findings

#### 1. 🛑 [Kategori: PRD / ADR / Bug / TC] [Severity: blocker / major / minor] [Judul Masalah]
• **Lokasi:** `path/to/file.go:Lxx-Lyy`
• **Masalah:** ringkasan apa yang salah atau menyimpang.
• **Rujukan:** PRD (`<module>.md §9.x` / `§4.x` / `§11`, atau `03-functional.md §x.y`), ADR (`engineering.md (ADR xxx)`), dan/atau TC ID (`tc-<module>-backend.md` `TC-XX-nn`).
• **Bukti:** jalur kode yang diperiksa + perilaku yang teramati.
• **Rekomendasi Fix:** perbaikan konkret yang siap diaplikasikan.

#### Blocked / Not Verifiable

- [item] — blocked by spec drift / missing schema / source unavailable; state what is needed to verify.
````
