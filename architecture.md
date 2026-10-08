# Platform Architecture

> This shared platform reference is copied from `iam-service`. The current
> asset-transaction-service is an HTTP health-service skeleton; the database,
> authorization, workflow, and event components below are platform targets,
> not implemented capabilities of this repository. See `README.md`.

Deep-dive technical documentation into the platform's architecture, design patterns, and runtime mechanics.

## Table of Contents

- [1. Executive Architecture Overview](#1-executive-architecture-overview)
  - [System Paradigm](#system-paradigm)
  - [The Multi-Repo Ecosystem](#the-multi-repo-ecosystem)
  - [High-Level System Topology](#high-level-system-topology)
- [2. Network Topology & Ingress Security](#2-network-topology--ingress-security)
- [3. Multi-Tenancy & Data Architecture](#3-multi-tenancy--data-architecture)
- [4. Database Migration Engine (mig)](#4-database-migration-engine-mig)
- [5. Identity, RBAC & Authorization Engine](#5-identity-rbac--authorization-engine)
- [6. Inter-Service Communication & Distributed Sagas](#6-inter-service-communication--distributed-sagas)
- [7. Internal Service Layering & Boot Lifecycle](#7-internal-service-layering--boot-lifecycle)
- [8. Multi-Repo Dependency & Release Model](#8-multi-repo-dependency--release-model)

---

## 1. Executive Architecture Overview

This platform is a distributed, multi-tenant enterprise system built with Go. In this architecture, all domain services and shared toolkits are decoupled across independent repositories, communicating through strongly typed contracts, durable workflow sagas, and event streams.

### System Paradigm

The architecture is built on six foundational architectural pillars:

1. **Gateway-Centric Authentication & Network Perimeter**: All external traffic terminates at a Traefik reverse proxy. Authentication is resolved at the edge via `forwardAuth` to IAM, injecting trusted identity headers downstream. Backend services do not expose host ports, making direct network bypass impossible (ADR 020).
2. **Schema-Isolated Multi-Tenancy**: Every service owns an independent PostgreSQL database. Multi-tenancy is achieved by provisioning a dedicated schema per client, plus a `public` schema for service-wide plumbing (ADR 001). Cross-service database joins and foreign keys are prohibited.
3. **Decoupled Inter-Service Communications**:
   - **Reads**: Performed synchronously over internal HTTP APIs (`net/http`) using shared contracts before opening local database transactions (ADR 002, ADR 006 §4b).
   - **Writes**: Dispatched asynchronously via **Temporal Workflows & Activities** running on the owning service's task queue, enabling durable multi-step sagas and automated compensation rollbacks (ADR 006).
4. **Asynchronous Event Projections**: Event-driven state synchronization and denormalization occur via the **Transactional Outbox Pattern** committed alongside business entities, drained by outbox relays to **Redpanda (Kafka API)** (ADR 017).
5. **Centralized RBAC with Cached Snapshots**: IAM is the sole authority for roles and permissions. Authorization decisions in downstream services are evaluated directly against an immutable Valkey-cached snapshot (`UserAccessCache`), preventing permission drift (ADR 005, ADR 010).
6. **Strict 4-Layer Service Internals**: Every service follows a uniform internal layering: `Router` → `Handler` → `Service` → `Repository` / `Workflow` → `Activity` (ADR 003).

---

### The Multi-Repo Ecosystem

In production, code is organized across distinct repository categories:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                          Standards & Contracts Repos                        │
├───────────────────────────────┬─────────────────────────────────────────────┤
│  go-code-standard             │  engineering.md, ADRs, architecture.md      │
│  service-contracts            │  Shared wire DTOs, Temporal activity        │
│                               │  wrappers, and domain event schemas         │
└───────────────────────────────┴─────────────────────────────────────────────┘
                                       │ (imported via Go modules)
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Shared Core Toolkits                              │
├───────────────────────────────┬─────────────────────────────────────────────┤
│  be-core-utils                │  HTTP errors, responses, db pools, outbox   │
│  ts-utils                     │  RBAC middleware, LoadUserRoles, gate rules │
│  mig                          │  Schema-aware SQL migration engine          │
└───────────────────────────────┴─────────────────────────────────────────────┘
                                       │ (imported via Go modules)
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                           Domain Service Repos                              │
├───────────────────────────────┬─────────────────────────────────────────────┤
│  iam-service                  │  AuthN, session store, user & role catalog  │
│  enterprise-management-service│  Client tenants, client onboarding sagas    │
│  master-asset-registry-service│  Asset registry, state, measurements        │
│  asset-transaction-service    │  Borrowing, check-in/out sagas & approvals  │
│  import-service               │  Batch CSV/Excel ingestion workflows        │
│  file-manager-service         │  S3/MinIO attachment storage & signed tokens│
│  notification-service         │  In-app & email notification dispatch       │
│  analytics-reporting-service  │  Changelog tracking, audit & report records │
└───────────────────────────────┴─────────────────────────────────────────────┘
                                       │ (packaged as Docker containers)
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Gateway & Deployment Repos                            │
├───────────────────────────────┬─────────────────────────────────────────────┤
│  gateway-config / compose     │  Traefik v3 routing, middleware, Docker env │
└───────────────────────────────┴─────────────────────────────────────────────┘
```

- **`service-contracts`**: The single source of truth for cross-service interfaces. If Service A needs to trigger a Temporal activity or call an internal HTTP endpoint on Service B, the parameter structs, response DTOs, and typed activity invocation functions reside here.
- **No Direct Service-to-Service Repo Dependencies**: Services never import code directly from another service's repository.

---

### High-Level System Topology

The diagram below illustrates how client traffic, inter-service HTTP calls, Temporal workflow tasks, outbox events, and data stores interact at runtime:

```
                            [ External Client (Browser / Mobile) ]
                                              │
                                              │ HTTPS / HTTP (Port 80/443)
                                              ▼
                        ┌───────────────────────────────────────────┐
                        │        Traefik v3 API Gateway             │
                        │  (forwardAuth, Header Stripping/Injection)│
                        └─────┬───────────────────────────────┬─────┘
                              │                               │
            Auth Verification │ (1)                           │ Proxied Request (with X-User-ID,
            /auth/validate    ▼                               │                  X-Client-ID)
                      ┌───────────────┐                       │ (2)
                      │  iam-service  │                       ▼
                      └───────┬───────┘          ┌───────────────────────────┐
                              │                  │      Domain Services      │
                 Valkey       ▼                  │  (MAR, Asset Transaction, │
                 Session ┌─────────┐             │   Enterprise Mgmt, ...)   │
                 Lookup  │ Valkey  │             └─┬───────┬───────┬───────┬─┘
                         └─────────┘               │       │       │       │
             ┌─────────────────────────────────────┘       │       │       └──────────────────────┐
             │ Reads (HTTP /internal/*)                    │       │                              │
             ▼                                             │       │                              │ Writes
    ┌─────────────────┐                                    │       │                              ▼
    │ Upstream Service│                                    │       │                       ┌─────────────┐
    └─────────────────┘                                    │       │                       │ MinIO (S3)  │
             ┌─────────────────────────────────────────────┘       │                       │ File Storage│
             │ Multi-step Writes (Sagas)                           │ Outbox Events         └─────────────┘
             ▼                                                     ▼
     ┌───────────────┐                                     ┌───────────────┐
     │ Temporal SDK  │                                     │  Outbox Relay │
     │ Task Queues   │                                     └───────┬───────┘
     └───────┬───────┘                                             │ Stream Events
             ▼                                                     ▼
     ┌───────────────┐                                     ┌───────────────┐
     │Temporal Server│                                     │Redpanda/Kafka │
     └───────────────┘                                     └───────┬───────┘
             │                                                     │
             └──────────────────────┐                              ▼
                                    ▼                      ┌───────────────┐
                          ┌──────────────────┐             │Event Consumers│
                          │ PostgreSQL /     │             │(Analytics,    │
                          │ MongoDB Database │             │ Notification) │
                          │ (Schema-Isolated)│             └───────────────┘
                          └──────────────────┘
```

---

## 2. Network Topology & Ingress Security

The system enforces a strict zero-trust network perimeter. Authentication is resolved at the edge, services are physically isolated from external networks, and identity headers are injected through a hardened gateway pipeline (ADR 020).

### Physical Network Boundary & Port Isolation

In production and containerized environments, **only the Traefik API Gateway publishes host ports** (`80` / `443`):

```
                        PUBLIC INTERNET
                              │
                      [ Port 80 / 443 ]
                              ▼
                   ┌─────────────────────┐
                   │  Traefik v3 Gateway │
                   └──────────┬──────────┘
                              │
══════════════════════════════╪═════════════════════════════════════════════
  INTERNAL DOCKER NETWORK     │ (Private Container Network: app-network)
                              ▼
     ┌────────────────┬────────────────┬────────────────┐
     │  iam-service   │  mar-service   │  ats-service   │  ... (No host ports)
     └────────────────┴────────────────┴────────────────┘
═════════════════════════════════════════════════════════════════════════════
```

- **Why services publish no host ports**: Backend services trust `X-User-ID` and `X-Client-ID` headers without re-verifying session signatures. If a service published port `:8092` to the host, an attacker could directly query `http://host:8092/v2/measurements/list` with a forged `-H "X-User-ID: 1"`, bypassing authentication entirely.
- **The network boundary IS the authentication boundary**: External clients cannot access backend service IP addresses or ports directly.

---

### Ingress Lifecycle & `forwardAuth` Pipeline

Every external request entering Traefik follows a deterministic 5-step security pipeline:

```
Client                     Traefik                    iam-service               Target Service
  │                           │                            │                          │
  │ (1) Request + Cookie      │                            │                          │
  ├──────────────────────────►│                            │                          │
  │                           │ (2) Strip incoming         │                          │
  │                           │     X-User-ID, X-Client-ID │                          │
  │                           │                            │                          │
  │                           │ (3) GET /auth/validate     │                          │
  ├──────────────────────────►│                            │                          │
  │                           │◄───────────────────────────┤                          │
  │                           │     200 OK + Auth Headers  │                          │
  │                           │     X-User-ID: 101         │                          │
  │                           │     X-Client-ID: client_a  │                          │
  │                           │                            │                          │
  │                           │ (4) Proxy request with injected headers               │
  │                           ├──────────────────────────────────────────────────────►│
  │                           │                            │                          │ (5) Evaluate
  │                           │                            │                          │     RBAC Gate
  │                           │◄──────────────────────────────────────────────────────┤
  │ (6) Response              │                     200 OK Response                   │
  │◄──────────────────────────┤                                                       │
```

1. **Client Request**: Client sends an HTTP request accompanied by a session cookie.
2. **Header Sanitization**: Traefik strips any incoming client headers matching `X-User-ID`, `X-Client-ID`, or `X-Service-Token` to eliminate spoofing attempts.
3. **Edge Verification (`forwardAuth`)**: Traefik dispatches a sub-request to `http://iam-service:8080/auth/validate`:
   - If the cookie is missing or invalid/expired in Valkey, IAM responds with `401 Unauthorized`. Traefik short-circuits and immediately returns `401` to the client.
   - If valid, IAM extracts the user ID and tenant schema name, writing them into response headers (`X-User-ID: 101`, `X-Client-ID: client_a`), and returns `200 OK`.
4. **Header Injection & Forwarding**: Traefik copies the verified headers (`authResponseHeaders`) to the original request and routes it to the target backend service.
5. **Route Gate Evaluation**: The target service reads `X-Client-ID` to set the database schema context, loads roles via `LoadUserRoles`, and evaluates route permissions (`rbac.Require`).

---

### Route Classification & Gateway Rules

Traefik routing rules explicitly categorize endpoints into three security classes:

| Class                           | Paths                                              | Authentication & Routing Rules                                                                                                                 |
| ------------------------------- | -------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------- |
| **Authenticated User Routes**   | `/v2/*`, `/clients/*`, `/users/*`                  | Protected by `forwardAuth`. Requires active session cookie. Traefik injects verified `X-User-ID` and `X-Client-ID`.                            |
| **Public / Auth-Bypass Routes** | `/auth/*`, `/file-manager/:token`, `/v2/ws/reader` | Bypasses `forwardAuth`. `/auth/login` is how credentials become sessions; signed file tokens carry authorization in their URL.                 |
| **Internal Service Routes**     | `/internal/*`                                      | **Explicitly NOT routed by Traefik (returns 404)**. Reachable only within the private Docker network by peer services using `X-Service-Token`. |
| **Health Probes**               | `/health`                                          | Unauthenticated. Probed directly by container orchestrators on the private network.                                                            |

## 3. Multi-Tenancy & Data Architecture

Every service owns its datastore completely. Multi-tenancy is enforced through logical schema isolation, preventing cross-tenant data leakage while eliminating cross-service database coupling (ADR 001).

### PostgreSQL: Schema-per-Tenant Model (ADR 001, ADR 008)

Each PostgreSQL-backed service owns an independent database (e.g., `iam_db`, `master_asset_registry_db`, `asset_transaction_db`). Within that database, data is separated into schemas:

```
                            [ PostgreSQL Service Database ]
                                           │
         ┌─────────────────────────────────┼─────────────────────────────────┐
         ▼                                 ▼                                 ▼
┌─────────────────┐               ┌─────────────────┐               ┌─────────────────┐
│  public schema  │               │ tenant_a schema │               │ tenant_b schema │
├─────────────────┤               ├─────────────────┤               ├─────────────────┤
│ • outbox        │               │ • assets        │               │ • assets        │
│ • system_config │               │ • categories    │               │ • categories    │
│ • schema_migr.  │               │ • schema_migr.  │               │ • schema_migr.  │
└─────────────────┘               └─────────────────┘               └─────────────────┘
```

1. **`public` Schema**: Holds service-wide plumbing that spans all tenants:
   - The transactional `outbox` table (ADR 017).
   - Public migration ledger (`schema_migrations`).
2. **`tenant` Schemas**: Dedicated schema per client (e.g. `client_acme`, `client_globex`):
   - Contains all client-specific business tables (`assets`, `users`, `positions`, `transactions`).
   - Owns a dedicated `schema_migrations` table tracking the tenant's migration version.
3. **Runtime Schema Routing**:
   - The gateway injects `X-Client-ID`.
   - The service validates the client ID regex (`^[a-zA-Z0-9_]{1,50}$`) and derives a tenant-scoped database servicer (`tenantSvc`).
   - Queries execute against the tenant's `search_path` (`SET search_path = <tenant>, public`).
4. **Connection Pool Safety (ADR 004)**: `PostgreSQLService` instances carry active transaction context (`pgx.Tx`). Services create or acquire a `tenantSvc` per request and **never share an instance across concurrent request goroutines**.

---

### MongoDB: Database-per-Client Model

Services handling high-throughput, unstructured, or append-heavy data (`notification-service`, `analytics-reporting-service`) use MongoDB:

- **Tenancy**: Scoped as a separate logical database per client (`notifications_<client_id>`, `analytics_<client_id>`).
- **Connection Management**: Managed through `be-core-utils`'s `MakeMongoDBService`, maintaining connection pooling per tenant database.

---

### Data Boundary Rules

1. **Zero Cross-Service Joins**: A service reads and writes **only its own tables**. There are no shared tables, no cross-database foreign keys, and no database links.
2. **Cross-Service Entity References**: When a service references an entity owned by another service (e.g., Asset Transaction referencing a `user_id` or `asset_id`), it stores the Snowflake `int64` ID as a plain scalar, never a foreign key.

---

### Snapshot vs. Live Data (ADR 007)

A critical architectural distinction is made between historical audit records and live query state:

```
[ Transaction Occurs at T1 ] ──► Store Frozen Snapshot (Borrower: "Alice", Asset: "Laptop A")
                                  └─► Alice changes name to "Alice Smith" at T2
[ Historical Query at T3 ]   ──► Returns "Alice" (Correct: Alice was "Alice" at T1)
[ Live Query at T3 ]         ──► Queries IAM/Redpanda for live display name "Alice Smith"
```

- **Historical Records Freeze Data**: Historical transaction logs, approval trails, and changelogs store frozen names and values (`borrower_name`, `asset_name`, `department_name`) as they existed at execution time. A frozen name on a past record is **correct, not stale** (ADR 007).
- **Live Display Fields**: If live current state is required alongside historical records, it is stored in a separate column populated asynchronously via Redpanda events (ADR 017), keeping the historical snapshot untouched.

## 4. Database Migration Engine (`mig`)

Database schema evolution is managed by `mig` (`gitea.tagsamurai.local/Wangsit-Developer/mig`), a domain-agnostic, schema-aware SQL migration engine embedded directly into each service binary (ADR 001, ADR 008).

### Embedding & File Organization

Migrations are raw SQL files embedded via Go `embed.FS` (`pkg/migrations/migrations.go`):

```
<service>/pkg/migrations/
  ├── migrations.go       # Embeds publicFS and tenantFS, exports Public() and Tenant() runners
  ├── public/             # 20260804100100_create_outbox_table.up.sql / .down.sql
  └── tenant/             # 20260804100000_create_positions_table.up.sql / .down.sql
```

```go
// pkg/migrations/migrations.go
//go:embed public/*.sql
var publicFS embed.FS

//go:embed tenant/*.sql
var tenantFS embed.FS

func Public(connStr string) (*runner.Runner, error) { return runner.NewSub(connStr, publicFS, "public") }
func Tenant(connStr string) (*runner.Runner, error) { return runner.NewSub(connStr, tenantFS, "tenant") }
```

- **File Naming**: `<14-digit-timestamp>_<snake_case_name>.<up|down>.sql` (`YYYYMMDDHHMMSS`).
- **Symmetric Up/Down Pairs**: Every migration must define both `.up.sql` and `.down.sql`. `mig` verifies rollbacks during CI.

---

### Dual Migration Lifecycles

Migrations execute under two distinct runtime lifecycles:

```
                  ┌────────────────────────────────────────────────────────┐
                  │                 Service Boot Lifecycle                 │
                  │ (Runs during app.go Phase 1 before accepting traffic)  │
                  └───────────────────────────┬────────────────────────────┘
                                              │
                                              ▼
                                ┌───────────────────────────┐
                                │   1. Public Migration     │
                                │ (migrates 'public' schema)│
                                └─────────────┬─────────────┘
                                              │
                                              ▼
                                ┌───────────────────────────┐
                                │ 2. Atomic All-or-Nothing  │
                                │   Tenant Migrations       │
                                │ (runner.UpAllSchemas)     │
                                └─────────────┬─────────────┘
                                              │
                     ┌────────────────────────┴────────────────────────┐
                     │ Success: Begin serving traffic                  │ Failure: Rollback DB transaction &
                     │                                                 │          abort boot immediately
                     ▼                                                 ▼
               [ Service Healthy ]                               [ Service Crashes ]
```

#### 1. Boot-Time All-or-Nothing Migrations (ADR 004, ADR 008)

- During service startup (`internal/app/app.go` phase 1), migrations execute before the HTTP listener or Temporal worker polls:
  1. `Public(connStr).Up()` applies pending public schema migrations.
  2. `runner.UpAllSchemas(connStr, tenantFS, "tenant")` discovers all registered tenant schemas and executes pending migrations sequentially across all tenants inside **a single PostgreSQL transaction**.
- **All-or-Nothing Guarantee**: If a migration fails on any tenant schema (e.g. syntax error or lock timeout), the entire multi-tenant migration transaction rolls back, and service startup terminates loudly. A service is never permitted to serve requests in a partially migrated state.

#### 2. Dynamic Tenant Onboarding Sagas (ADR 001, ADR 006)

- When a new client is onboarded via Enterprise Management Service:
  - An onboarding Temporal saga executes an activity on each service's task queue.
  - The activity creates the new tenant schema (`CREATE SCHEMA client_new`) and invokes `Tenant(connStr)` against that specific schema, applying all tenant migrations from version 0 to latest.

---

### Version Tracking & State Tables

Migration versions are tracked locally within each schema:

- **`public.schema_migrations`**: Tracks public migrations.
- **`<tenant_schema>.schema_migrations`**: Tracks migrations applied to that specific tenant schema.

| Column       | Type                 | Description                                      |
| ------------ | -------------------- | ------------------------------------------------ |
| `version`    | `BIGINT PRIMARY KEY` | 14-digit timestamp of the migration              |
| `applied_at` | `TIMESTAMPTZ`        | Timestamp when the migration was executed        |
| `dirty`      | `BOOLEAN`            | Set to `true` if a migration fails mid-execution |

---

### Developer CLI Workflow

Developers author and verify migrations locally using the standalone `mig` CLI:

```bash
mig create create_asset_categories_table  # Generates timestamped .up.sql and .down.sql pair
mig test                                  # Executes up -> down -> up round-trip on scratch DB
mig verify --schema test_tenant           # Runs schema-aware verification
```

## 5. Identity, RBAC & Authorization Engine

Authorization decisions are decoupled from database queries. Roles and permissions are owned exclusively by `iam-service` and evaluated across the platform using immutable, cached snapshots (ADR 005, ADR 010).

### Centralized Role Ownership & Valkey Snapshot (ADR 005)

- **Single Authority**: `iam-service` alone writes to the role, permission, and group mapping tables. No other service connects to IAM's database or duplicates role schemas.
- **Snapshot Generation**: On user login, role assignment, or group update, IAM materializes the user's complete permission set into a `UserAccessCache` JSON document and stores it in Valkey (Redis-compatible):
  - **Cache Key**: `user_access_cache:<user_id>:<client_id>`
  - **TTL**: 24 hours (invalidated immediately by IAM on role mutation).

```
                      [ IAM Service ]
                             │ (Write Role Change)
                             ▼
                    ┌─────────────────┐
                    │     Valkey      │◄─────────────────────────────┐
                    │ user_access_    │                              │
                    │ cache:<uid>:<cid│                              │ (1) Direct Valkey Lookup
                    └─────────────────┘                              │     (Sub-millisecond)
                                                                     │
                                                      ┌──────────────┴──────────────┐
                                                      │    Downstream Services      │
                                                      │  (MAR, ATS, Enterprise Mgmt)│
                                                      └──────────────┬──────────────┘
                                                                     │
                                                                     │ (2) Fallback on Cache Miss:
                                                                     │     GET /internal/users/:id/roles
                                                                     ▼
                                                              [ IAM Service ]
```

- **Consumer Read Path**: Downstream services read the `UserAccessCache` directly from Valkey on the request path. If a cache miss occurs, the service falls back to calling `GET /internal/users/:id/roles` on IAM using `X-Service-Token`.

---

### Evaluating Authorization on the Snapshot (ADR 010)

Downstream services evaluate permissions directly against the unmutated `UserAccessCache` snapshot via typed methods:

```go
// Direct evaluation on the snapshot without lossy intermediate types
if !userAccess.HasGroupPermission(groupID, permission) {
    return coreEntity.Forbidden("user lacks permission for this asset group")
}

// Extract authorized group IDs for SQL WHERE IN clause
authorizedGroupIDs := userAccess.GroupIDsFor(tsMiddleware.PermView, enum.ModuleAsset)
```

- **Why derived `Scope` types are forbidden (ADR 010)**: Transforming permissions into intermediate helper structs introduces places where leaf grants or group inheritances are dropped. Querying the snapshot directly guarantees fidelity to IAM's grant model.

---

### Route-Level Authorization Gates (ADR 013)

Authorization gates are declared directly on route definitions in `<domain>_routes.go` using `ts-utils/rbac`:

```go
// Example: master-asset-registry-service route gate
group.Post("/list", h.HandleListMeasurements, rbac.Require(rbac.SystemRole(tsMiddleware.PermView, key)))
group.Post("/", h.HandleCreateMeasurement, rbac.Require(rbac.SystemRole(tsMiddleware.PermCreate, key)))
group.Delete("/", h.HandleDeleteMeasurements, rbac.Require(rbac.SystemRole(tsMiddleware.PermDelete, key)))
```

#### Load-Bearing Route Mounting Order (ADR 013 §3)

Route registration inside `internal/app/app.go` must follow a strict 3-phase mounting sequence:

```
1. SetupInternalRoutes (Public / Service-Token Routes)
   ├── /health
   ├── /auth/login
   └── /internal/*  (Protected by RequireServiceToken)
        │
        ▼
2. fiberApp.Use(tsMiddleware.LoadUserRoles(iamBaseURL, serviceToken))
   └── (Loads UserAccessCache from Valkey into request context)
        │
        ▼
3. Register Protected Domain Routes
   └── group.Post("/list", handler, rbac.Require(...))
```

_Mounting `rbac.Require` before `LoadUserRoles` results in a 401 on every request because the snapshot has not yet been populated in context._

---

### List vs. Detail Authorization Model (ADR 009)

The platform distinguishes authorization rules between collection lists and single-record details:

| Dimension             | Collection List (`POST /x/list`)                                                                                                                                 | Single-Record Detail (`GET /x/:id`)                                                                                                                   |
| --------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Audience**          | **Single Audience**: The user querying the specific list (e.g. Asset Manager viewing pending approvals).                                                         | **Multiple Audiences**: Multiple legitimate stakeholders (Requester, Approver, Department Manager, Auditor).                                          |
| **Enforcement Point** | **Route Gate + SQL Query**: Route gate checks module view permission; SQL query filters by `GroupIDsFor(...)` _before_ pagination totals are computed (ADR 002). | **Route Gate + Service Layer**: Route gate checks baseline module access; domain service inspects record ownership (e.g. `caller_id == borrower_id`). |
| **Blast Radius**      | Low: Filtered in SQL at source.                                                                                                                                  | Controlled: Gated per record relation.                                                                                                                |

## 6. Inter-Service Communication & Distributed Sagas

Cross-service coordination is divided into three distinct communication mechanisms based on failure semantics and durability requirements (ADR 006).

### The Deciding Decision Matrix (ADR 006)

The fundamental question deciding the communication path is: **"Does this step need undoing if a later step fails?"**

```
                                [ Inter-Service Operation ]
                                             │
                       ┌─────────────────────┴─────────────────────┐
                       ▼                                           ▼
                 [ Read Query ]                              [ Write Action ]
                       │                                           │
                       ▼                                           ▼
              [ Synchronous HTTP ]                       Does it need undoing /
             (via service-contracts)                     delivery guarantees?
                                                           │               │
                                                      Yes  ▼          No   ▼
                                                  [ Temporal ]       [ Outbox + Redpanda ]
                                                  (Saga Activity)    (Event Broadcast)
```

| Scenario                                           | Mechanism                                     | Why This Choice                                                                                        |
| -------------------------------------------------- | --------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| **Reads (Nothing to undo)**                        | **Synchronous HTTP**                          | Immediate consistency; reads never need rolling back (ADR 002).                                        |
| **Writes requiring rollback / compensation**       | **Temporal Activity on Owner's Queue**        | Durably registered sagas can be compensated automatically if a downstream step fails (ADR 006).        |
| **Writes without rollback, but delivery-critical** | **Temporal Child Workflow (Fire-and-Forget)** | Durability and unbounded retries live on the owner's worker, surviving restarts (ADR 006).             |
| **Event broadcast & async denormalization**        | **Transactional Outbox + Redpanda**           | Guaranteed at-least-once delivery without distributed locks; consumers dedup by `outbox.id` (ADR 017). |

---

### 1. Synchronous HTTP Internal API Pattern

Used strictly for reads and lookups between services:

```go
// Upstream service makes an internal HTTP call using shared wire types
// service-contracts/services/iam/http/dto.go
req, _ := http.NewRequestWithContext(ctx, http.MethodGet, iamBaseURL+"/internal/users?ids=1,2,3", nil)
req.Header.Set("X-Service-Token", serviceToken)
req.Header.Set("X-Client-ID", clientID)
```

- **Wire Contracts**: DTOs live in `service-contracts/services/<owner>/http/`.
- **Prefetch Boundaries (ADR 002)**: Batch lookup endpoints must be capped in code (e.g. max 200 IDs) to prevent unbounded memory allocation and query degradation.
- **Transaction Ordering Rule (ADR 006 §4b)**: **Always perform remote HTTP reads before opening a local database transaction**. Never hold an open PostgreSQL connection lock while awaiting remote network I/O.

---

### 2. Temporal Distributed Sagas & Workflows

Multi-step writes spanning multiple services execute as Temporal workflows to guarantee eventual consistency and automated compensation rollbacks.

#### Task Queue Topology (Caller vs. Owner)

- **Caller Service**: Orchestrates the workflow and dispatches activities.
- **Owner Service**: Registers the activity implementation on its **own task queue** (e.g., `master-asset-registry-task-queue`).
- **Typed Wrappers**: Callers invoke activities via typed helpers in `service-contracts`:

```go
// service-contracts/services/master_asset_registry/temporal/reserve_assets.go
func ExecuteReserveAssets(ctx workflow.Context, param ReserveAssetsParam) error {
    opts := workflow.ActivityOptions{
        TaskQueue: "master-asset-registry-task-queue",
        StartToCloseTimeout: 30 * time.Second,
    }
    return workflow.ExecuteActivity(workflow.WithActivityOptions(ctx, opts), ActivityReserveAssets, param).Get(ctx, nil)
}
```

#### Automated Saga Compensation (`WorkflowCompensation`)

When an irreversible step succeeds, its compensating undo step is registered onto a LIFO stack (`be-core-utils/service.WorkflowCompensation`). If a later step fails, the stack executes rollbacks in reverse order on a **disconnected context** (ADR 006 §2):

```go
func AssetCheckoutWorkflow(ctx workflow.Context, param CheckoutParam) (err error) {
    compensation := &service.WorkflowCompensation{}
    defer func() {
        if err != nil {
            // DisconnectedContext guarantees compensation runs even if workflow times out/cancels
            dCtx, _ := workflow.NewDisconnectedContext(ctx)
            compensation.Compensate(dCtx, false)
        }
    }()

    // Step 1: Reserve assets
    err = temporalMAR.ExecuteReserveAssets(ctx, param.Assets)
    if err != nil { return err }
    compensation.AddCompensation(temporalMAR.ActivityReleaseAssets, param.Assets)

    // Step 2: Create transaction record (fails)
    err = workflow.ExecuteActivity(ctx, ActivityCreateRecord, param).Get(ctx, nil)
    if err != nil { return err } // Triggers defer rollback -> ExecuteReleaseAssets executes

    return nil
}
```

- **Idempotency Rule**: All compensation activities must be idempotent and safe to retry unconditionally.
- **Pivot Points**: Operations prior to the pivot point are compensated on failure; operations after the pivot point (e.g. final status commit) use forward recovery (retries) rather than rolling back completed work.

#### Workflow Execution Patterns

1. **Fire-and-Forget Child Workflows (`ParentClosePolicy: ABANDON`)**: Spawns an independent child workflow (e.g. audit changelog recording in Analytics) without blocking the caller. Survives parent completion (ADR 006).
2. **Parallel Pipelines (`workflow.NewSelector` - ADR 018)**: Used for processing $N$ independent multi-activity items concurrently without spawning unmanaged goroutines.
3. **Large Batch Workflows (`ContinueAsNew` - ADR 019)**: Keeps workflow event history bounded by checkpointing and restarting state (e.g. 500-row batch import chunks).
4. **Idempotency**: Workflows enforce `WorkflowIDReusePolicy: REJECT_DUPLICATE` to reject duplicate triggers for the same business event.

---

### 3. Event-Driven Denormalization (Outbox + Redpanda) (ADR 017)

For asynchronous projection updates, cross-service notifications, and read-model denormalization:

```
[ Domain Service DB Transaction ]
  ├── 1. UPDATE assets SET status = 'IN_USE' WHERE id = 101;
  └── 2. INSERT INTO public.outbox (event_type, payload) VALUES ('AssetStatusChanged', {...});
                               │ (Transaction Commits Atomically)
                               ▼
                    [ Outbox Relay Worker ]
                               │ (Polls & Publishes)
                               ▼
                   [ Redpanda Topic: asset-events ]
                               │
            ┌──────────────────┴──────────────────┐
            ▼                                     ▼
 [ Notification Service ]             [ Analytics Service ]
 (Sends alert to manager)             (Updates utilization report)
 (Deduplicates by outbox.id)          (Deduplicates by outbox.id)
```

1. **Transactional Outbox**: The event is written to `public.outbox` inside the exact same local database transaction as the entity mutation.
2. **Outbox Relay**: A background worker polls pending outbox records and publishes them to Redpanda topics.
3. **Consumer Idempotency**: Consumers record processed `outbox.id`s in a local deduplication table inside their local database transaction, guaranteeing exactly-once processing semantics at the application level.

## 7. Internal Service Layering & Boot Lifecycle

Every service strictly separates concerns across four internal layers and boots through a deterministic, fail-fast sequence (ADR 003, ADR 004).

### The 4-Layer Architecture (ADR 003)

```
HTTP Request
     │
     ▼
┌──────────────┐
│    Router    │  <domain>_routes.go (Mounts LoadUserRoles, rbac.Require)
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   Handler    │  handler/ (Parse body, validate, format HTTP envelope)
└──────┬───────┘  *Knows nothing about SQL or Temporal*
       │
       ▼
┌──────────────┐
│   Service    │  service/ (Domain singleton, action methods)
└──────┬───────┘  *Thin: repository call OR workflow dispatch*
       │
       ├─────────────────────────────────┐ (Simple Read)
       │ (Complex / Multi-step Write)    ▼
       ▼                        ┌─────────────────┐
┌──────────────┐                │   Repository    │  repository/
│   Workflow   │  workflow/     └────────┬────────┘  (SQL queries via pgx/v5)
└──────┬───────┘                         │
       │                                 ▼
       ▼                            [ Database ]
┌──────────────┐
│   Activity   │  activity/ (Atomic step registered by Temporal worker)
└──────────────┘
```

#### Layer Responsibilities

| Layer          | Location                            | Rules & Boundaries                                                                                                                                                                                                                       |
| -------------- | ----------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Router**     | `<domain>_routes.go`                | Lives at the domain root. Mounts internal routes, auth middleware, and route gates (ADR 013).                                                                                                                                            |
| **Handler**    | `handler/<action>_handler.go`       | Parses request payload, executes validation, calls **exactly one** service method, and returns `coreHelper.Success` or `coreEntity.ToHttpError(err).SendResponse(c)`. Contains zero business logic, zero SQL, and zero Temporal imports. |
| **Service**    | `service/<action>_service.go`       | Domain action surface (`ClientService`, `AssetService`). Holds dependencies via **constructor injection**. Stays thin: calls a repository method or dispatches a Temporal workflow (ADR 003).                                            |
| **Repository** | `repository/<entity>_repository.go` | Owns SQL query construction and execution. Interface (`ClientRepositor`) and struct (`ClientRepository`) are colocated. Accepts `context.Context` and `coreService.PostgreSQLServicer`.                                                  |
| **Model**      | `internal/model/<entity>_model.go`  | Plain data structures shared across domain modules. Contains JSON and SQL column tags. Uses `int64` for all Snowflake ID fields.                                                                                                         |

---

### The Deterministic Boot Sequence (ADR 004)

`internal/app/app.go` initializes dependencies top-down in seven fixed phases. If any phase encounters a failure, the application immediately aborts with a loud error:

```
[ app.New(config) *App ]
  │
  ├──► Phase 1: Database Migrations (ADR 001, ADR 008)
  │    └── Public schema migration -> Atomic all-or-nothing tenant schema migrations
  │
  ├──► Phase 2: Infrastructure & Connections
  │    └── Initialize PostgreSQL pool, Valkey client, MinIO client, Temporal client
  │
  ├──► Phase 3: Repositories & Shared Utilities
  │    └── Instantiate clientRepo, userRepo, assetRepo, validator, outbox
  │
  ├──► Phase 4: Temporal Activities & Workflow Registration
  │    └── Register Activity structs on ServiceManager; call RegisterWorkflows(temporalWorker)
  │
  ├──► Phase 5: Domain Services (Constructor Injection)
  │    └── Wire repos, temporalSvc, and config into Domain Services (NewClientService, ...)
  │
  ├──► Phase 6: HTTP Router, Middleware & Route Mounting (ADR 013)
  │    └── SetupInternalRoutes -> Mount LoadUserRoles -> Mount Protected Domain Routes
  │
  └──► Phase 7: Start Serving & Workers
       └── Listen on private network port; start Outbox Relay worker; poll Temporal worker
```

---

### Connection State Safety: Why `PostgreSQLService` Is Never Shared (ADR 004)

`be-core-utils`'s `PostgreSQLService` holds the active database transaction reference (`pgx.Tx`) when executing queries:

- **The Danger of Shared Servicers**: If a singleton `PostgreSQLService` were injected into HTTP handlers or shared across concurrent requests, concurrent goroutines would overwrite each other's active transaction state, resulting in silent data corruption or race conditions.
- **The Correct Pattern**: Services obtain a tenant-isolated servicer (`tenantSvc := s.db.ForTenant(clientID)`) locally per request or activity invocation.

## 8. Multi-Repo Dependency & Release Model

In production, each service and shared library resides in its own Git repository. This section defines how dependencies are governed, how private Go modules are securely fetched, and how contract-first releases prevent breaking changes across services.

### Repository Hierarchy & Dependency Flow

The multi-repo ecosystem is strictly organized into four hierarchical layers with one-way dependency flows:

```
┌─────────────────────────────────────────────────────────────────────────────┐
│ 1. Standards & Documentation Repo (go-code-standard)                       │
│    └── engineering.md, ADRs, architecture.md                                │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │ (Defines rules)
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 2. Wire Contracts & Shared Toolkits                                         │
│    ├── service-contracts  (Wire DTOs, Temporal activity wrappers, events)   │
│    ├── be-core-utils      (Database servicers, HTTP errors, outbox)         │
│    ├── ts-utils           (RBAC middleware, LoadUserRoles, gate rules)      │
│    └── mig                (Embedded SQL migration engine)                   │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │ (Imported via Go modules)
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 3. Domain Service Repos (Independent Go Modules)                            │
│    ├── iam-service                   ├── master-asset-registry-service      │
│    ├── enterprise-management-service ├── asset-transaction-service          │
│    ├── import-service                ├── file-manager-service               │
│    ├── notification-service          └── analytics-reporting-service        │
└─────────────────────────────────────────────────────────────────────────────┘
                                       │ (Packaged as Docker images)
                                       ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│ 4. Deployment & Gateway Configuration                                       │
│    └── Traefik v3 reverse proxy, Docker Compose / Kubernetes manifests       │
└─────────────────────────────────────────────────────────────────────────────┘
```

---

### Dependency Governance Rules

1. **Strict Prohibition of Direct Cross-Service Imports**:
   - A service repository (e.g. `asset-transaction-service`) must **never** declare a Go module dependency on another service repository (e.g. `master-asset-registry-service`).
   - All shared interfaces, parameter structs, and response types must be published to and imported from **`service-contracts`**.
2. **Private Go Module Resolution (`GOPRIVATE`)**:
   - Internal repositories hosted on private Git servers (e.g. `gitea.tagsamurai.local/*`) are configured via `GOPRIVATE`.
3. **BuildKit Secrets in Docker Builds**:
   - Private Git credentials must be injected at build time using Docker BuildKit secrets:
     ```dockerfile
     # Build stage
     RUN --mount=type=secret,id=git_token \
         git config --global url."https://oauth2:$(cat /run/secrets/git_token)@gitea.tagsamurai.local/".insteadOf "https://gitea.tagsamurai.local/" && \
         go mod download
     ```
   - _Private tokens are never written to `ENV`, `ARG`, or filesystem image layers._

---

### Contract-First Release & Deployment Workflow

To allow services to deploy independently without breaking peer services, all API and workflow updates follow a 3-phase rollout:

```
[ Phase 1: Contract Update ]
  └── Add new DTO / Temporal activity signature to service-contracts (v1.4.0)
       │
       ▼
[ Phase 2: Producer Deployment ]
  └── Deploy owning service implementing the new activity/endpoint (backward-compatible)
       │
       ▼
[ Phase 3: Consumer Deployment ]
  └── Consumer services bump service-contracts to v1.4.0 and deploy calling logic
```

1. **Additive Changes**: New fields in DTOs and activity parameters must be optional or backward-compatible (`omitempty`).
2. **Major Breaking Changes**: Require a major version bump in the Go module path (e.g. `ts-utils/v2`, `service-contracts/v2`), allowing services to migrate incrementally without lock-step deployments.
3. **Independent CI/CD**: Each service maintains its own build, test, and release pipeline. Services are deployed as stateless, immutable containers without requiring simultaneous whole-platform redeployments.
