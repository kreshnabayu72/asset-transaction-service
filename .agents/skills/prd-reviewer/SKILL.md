---
name: prd-reviewer
description: >-
  Audit, validate, and cleanse Product Requirement Documents (PRDs).
  Use when reviewing PRD quality, detecting technical leaks (raw SQL queries, DDL statements,
  API endpoint specs, crypto/code details), verifying document boundaries, and refactoring PRDs to product-level standards.
---

# PRD Reviewer & Quality Linter

This skill enforces strict separation of concerns for Product Requirement Documents (PRDs), ensuring PRDs define **what** and **why** (business logic, UX, logical entity/data requirements) while technical implementation details (**how**) remain in Low-Level Design (LLD) and OpenAPI specs.

---

## 1. Document Separation of Concerns

| Document Type | Location | Allowed Content | FORBIDDEN Content |
|---|---|---|---|
| **PRD** | `PRD/` | Problem statement, personas, user flows, UI layouts & verbatim copy (EN), business rules, **logical entity dictionary & data requirements (in `03-functional.md`)**, permissions, edge cases. | Raw SQL queries, DDL indexes, physical DB types (`BIGINT`, `VARCHAR`), NoSQL commands, REST endpoint paths/DTOs, HTTP status codes, runtime crypto functions, `[TODO Engineering]`, reader target banners (`> Pembaca: ...`). |
| **API Spec** | `API/openapi.yaml` | URL paths, HTTP verbs, JSON request/response schemas, HTTP status codes, headers, parameter types. | Long narrative user stories, UI copywriting, component states. |
| **Low-Level Design** | `API/low-level-design.md` | Handler logic, atomic DB transactions, DDL indexing, collation, query optimization, event consumer worker mechanics. | Full UI wireframe layouts, user-facing error copy. |
| **Test Cases** | `TestCases/` | Test case IDs, preconditions, test steps, expected vs actual behavior (single active `tc-<module>.md` per module). | Backend architecture definitions, superseded draft files. |

> **PRD Structure Standards:**
> 1. **Canonical Single-File Standard (`prd-single-file-convention.md`):**
>    - Primary specification: `<module>.md` with canonical sections §1 to §13 (Core Rules, Aturan Logika Modul, §4 UI Design, §5 Table Rules, §6 Validasi, §7 States, §8 Logging, §9 Functional Requirements with §9.9 Validation Rules Table M-3, §11 Constraints, §12 Dependencies).
>    - Companion files: `<module>.changelog.md` (specification revision history) and `<module>.resolved.md` (closed architecture/product open items).
> 2. **Legacy Modular 4-File Standard (for unmigrated modules):**
>    - `01-overview.md`: Summary, Personas, Access Matrix, User Flow, Global Rules.
>    - `02-ui-design.md`: Layouts, Wireframe Specs, Setup Column, States, Accessibility.
>    - `03-functional.md`: Functional Specs, Logical Entity Dictionary (Data Requirements), Edge Case Matrix.
>    - `04-dependencies.md`: Internal/External Dependencies, Module Sync Matrix, Open Items. *(Promoted from `05-dependencies.md` to keep file numbering contiguous).*

---

## 2. PRD Audit Checklist (Anti-Patterns to Detect)

When auditing a PRD or a set of PRD files, check against these 6 categories:

### 1. Database Table Specification & SQL Leaks (Physical DB Specs)
* ❌ **Raw SQL queries:** Any `SELECT`, `JOIN`, `UNION ALL`, `COUNT(*)` in PRD files (especially in `03-functional.md`).
* ❌ **DDL statements & Indexes:** `CREATE UNIQUE INDEX`, `ALTER TABLE`, `PRIMARY KEY / FOREIGN KEY` constraint statements.
* ❌ **Polyglot / NoSQL DB Leaks:** Driver commands like `db.collection.createIndex({...})`, Mongoose schemas, or mixing NoSQL syntax with SQL.
* ❌ **Naming Contradictions:** Field name mismatches between index definitions (e.g. camelCase `objectId`, `createdById`) and logical entity fields (e.g. `target_entity_id`, `modified_by_id`).
* ❌ **Engine-specific Directives & Physical Types:** `COLLATE "en-US-x-icu"`, `bigint (Snowflake)`, `JSONB`, `VARCHAR(120)`, `TIMESTAMPTZ`, physical table partition directives.
* ❌ **Multi-tenancy implementation leaks:** Explicit references to PostgreSQL tenant schema partitioning vs discriminator column.
* ❌ **Reader Target Banners / AI Prompts:** Headers like `> Ditujukan untuk: **Programmer (Backend)**` or `> Pembaca: Claude Design (AI)` / `> Pembaca: Programmer (FE + BE)`.
* ✅ **Proper way in PRD:** Store entities in `03-functional.md` as a **Logical Data Dictionary** (Business Field Name, Logical Type, Required?, Business Rule/Validation, Description), Delete Guard narrative, and Sorting rules.

### 2. REST API & Network Protocol Leaks
* ❌ **Endpoint definitions in PRD:** `GET /auth/...`, `POST /v2/iam/users/...`, `PUT /general-settings/...`.
* ❌ **DTO Schemas in PRD:** Detailed Request/Response payload tables and PascalCase DTO models (e.g. `ValidateActivationResponse`, `SetPasswordBody`).
* ❌ **HTTP status code tables/references:** Lists or inline text mapping business rules or errors to `403 Forbidden`, `422 Unprocessable Entity`, `409 Conflict`, `404 Not Found`, `500 Server Error`.
* ✅ **Proper way in PRD:** Describe user action flow, client validation, and user-facing feedback messages. Reference `./API/openapi.yaml` for technical API contracts.

### 3. Cryptography, Runtime Code, Broker & Sync Internals
* ❌ **Code-level crypto calls:** `crypto.randomBytes(32)`, Base64URL 43-char encoding formulas, constant-time comparison algorithms.
* ❌ **Algorithm debates:** Byte-truncation differences (bcrypt 72-byte vs Argon2id).
* ❌ **Event broker mechanics:** Explaining `Kafka/Redpanda off outbox consumer event` workers.
* ❌ **Internal Repo Sync Debates & Architecture Questions:** In-text comments like *"cek dulu apakah repo auth/ sudah ikut me-rename..."* or debates on Materialized View vs Live Query.
* ✅ **Proper way in PRD:** State security and synchronization requirements simply (e.g. "One-time token valid for 24 hours", "Data reflects in real-time from source module"). Move architecture debates to LLD / ADR.

### 4. Stray Files, Non-Standard Layouts & Test Case Hoarding
* ❌ **Unofficial drafts in PRD:** Files like `activate-tag-wireframe.md` that state `(visualisasi, bukan file resmi PRD)`.
* ❌ **Deprecated `04-data.md` or skipped `05-dependencies.md`:** Using `04-data.md` or retaining `05-dependencies.md` without promoting to `04-dependencies.md`.
* ❌ **Editing non-canonical archive files:** Editing or citing legacy `01-overview.md`..`04-dependencies.md` files in modules where `<module>.md` already exists as the single canonical source of truth (marked with `[!WARNING] FILE INI SUDAH TIDAK KANONIK`).
* ❌ **Polluting active spec body with revision history:** In single-file PRDs, putting lengthy changelog narratives or closed open items inside `<module>.md` instead of separated companion files (`<module>.changelog.md` and `<module>.resolved.md`).
* ❌ **Superseded Test Case Drafts:** Keeping obsolete draft files like `TestCases/activity-log.md` (draft stubs) alongside active `TestCases/tc-activity-log.md`.
* ❌ **Review notes in root:** Files like `global-settings/user.md` (gap analysis) stored in root.
* ❌ **Missing PRDs or API Specs:** Modules having only `README.md` without PRD, or backend modules missing `./API/` specs.
* ✅ **Proper way in PRD:** Follow canonical single-file standard (`<module>.md` + companion `.changelog.md` / `.resolved.md`) or canonical 4-file standard (`01`–`04`) for unmigrated modules, maintain a single active `tc-<module>.md` in `TestCases/`, and place technical contracts in `./API/`.

### 5. Unresolved `[TODO]` & Forward-Dependency Speculations
* ❌ **Engineering placeholders:** `threshold [TODO Engineering]`, `Push notification [TODO]`.
* ❌ **Unbuilt product dependencies:** Speculative requirements for unbuilt systems (e.g. SAMS, SIEM).
* ❌ **Architecture questions in PRD:** `[TODO: via webhook or message queue?]`.
* ✅ **Proper way in PRD:** Resolve numbers before freezing PRDs; move architecture questions to LLD; clearly define active product scope.

### 6. Section Numbering & Template Inconsistencies
* ❌ **Cross-file numbering confusion:** References to non-existent `04-data.md` or unpromoted `05-dependencies.md` in other files or TestCases.
* ❌ **Non-standard headers:** Custom headers instead of canonical template numbering.
* ✅ **Proper way in PRD:** Follow canonical numbering:
  - **Single-File Canonical (`<module>.md`):** Core Rules Implementation, Aturan Logika Modul, §1 Overview (§1.1 Problem Statement, §1.2 Goals, §1.3 Feature Summary, §1.4 Scope), §2 Personas & Role Matrix (§2.0 Entity Gate, §2.1 Global Role, §2.3 Special Access), §3 User Flow, §4 UI Design (§4.1 Pages & Components, §4.2 Layout & Setup Column, §4.8 UI Visibility), §5 Global Table Rules, §6 Validasi & Error Messages, §7 States & Feedback, §8 Logging (§8.1 Changelog, §8.2 System Log, §8.3 Activity Log), §9 Functional Requirements (§9.1 Ringkasan Fungsi, §9.2 Core Functions, §9.3 Permissions, §9.4 Edge Cases, §9.6 Action Resilience, §9.7 Sync Matrix, §9.9 Validation Rules Table M-3), §10/§11 Constraints, §12 Dependencies & Breaking Changes, §13 Open Items.
  - **Modular 4-File (Legacy):**
    - `01-overview.md` (§1 Overview, §2 Personas & Roles, §3 User Flow, §4 Global Table Rules, §5 UI Pages/Components summary, §6 Validation & Error Messages, §7 States, §8 Logging, §9 Entity Scope, §10 Feature List, §11 Constraints, §12 Open Items)
    - `02-ui-design.md` (§2 UI Design Requirements: Layouts, Wireframes, Setup Column, States, Interactions)
    - `03-functional.md` (§1 Permissions, §2 Core Functions, §3 Business Logic/Flows, §4 Logical Entity & Data Dictionary, §5 Edge Case Matrix)
    - `04-dependencies.md` (§1 Internal Dependencies, §2 External Dependencies, §3 Cross-Module Notes, §4 Open Items)

---

## 3. Workflow for Reviewing / Refactoring PRDs

When the user asks to review, audit, or clean up PRDs:

1. **Scan Target Files:** Determine if target module uses Single-File standard (`<module>.md`) or Modular 4-File standard (`01`–`04`). Search PRD files for SQL blocks (` ```sql `), HTTP verbs (`GET /`, `POST /`), HTTP status codes (`403`, `422`, `500`), technical DB types (`BIGINT`, `VARCHAR`, `TIMESTAMPTZ`), NoSQL syntax (`db.collection.`), reader banners (`> Pembaca:`), and `[TODO]` markers.
2. **Generate Audit Findings:**
   - Group findings by the 6 categories above.
   - For each finding, provide:
     - File path and line numbers.
     - **Contoh yang tidak perlu:** Exact snippet from the current PRD.
     - **Contoh yang lebih sesuai:** Clean, product-level rewrite (logical format).
     - **Tempat yang seharusnya:** Destination file (e.g. `<module>.md §9.9` / `03-functional.md §4` for logical entities, `./API/low-level-design.md` or `./API/*.md` for SQL/DDL/indexes, `./API/openapi.yaml` or `./API/*.yaml` for endpoints/status codes).
3. **Execute Refactor (if requested):**
   - Cleanse the PRD files by replacing technical leaks with logical product requirements.
   - Embed logical entity dictionaries into §9.9 / `03-functional.md`.
   - Separate spec revision history and closed open items into companion `<module>.changelog.md` and `<module>.resolved.md` files.
   - Promote `05-dependencies.md` to `04-dependencies.md` and remove any legacy `04-data.md` files if working with 4-file PRDs.
   - Relocate SQL queries, indexes, and technical logic to the appropriate `./API/` files.
   - Clean up superseded test case drafts in `TestCases/`.
