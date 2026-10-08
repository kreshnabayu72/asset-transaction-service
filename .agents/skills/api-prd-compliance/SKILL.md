---
name: api-prd-compliance
description: >-
  Verify compliance and detect drift between Product Requirement Documents (PRDs) and API Specifications (OpenAPI YAML & LLD).
  Use when conducting gap analysis, auditing whether an API fully covers PRD requirements,
  or identifying missing fields, mismatched enums, outdated filters, and route discrepancies.
---

# API vs PRD Compliance & Gap Checker

This skill performs **traceability verification and gap analysis** between Product Requirement Documents (`PRD/`) and API specifications (`API/`). It ensures that technical API contracts completely and accurately implement all business and UI requirements without drift.

---

## 1. Five Verification Dimensions

When checking a module's API specifications against its PRD, systematically inspect these 5 dimensions:

### 1. Route & HTTP Method Alignment
* **Path Prefix & Structure:** Do all endpoints use consistent prefixing (e.g. `/v2/iam/...` vs `/v2/...`)?
* **HTTP Method Correctness:** Does the HTTP verb match the business action semantics?
  * Read data → `GET`
  * Create entity → `POST`
  * Full replacement / State change → `PUT` / `PATCH`
  * Remove / Invalidate → `DELETE`
* **Path Parameters & ID Standard (ADR 020):** Are ID parameter schemas strictly standardized as 64-bit Snowflake decimal strings (`type: string, pattern: ^[0-9]+$`)? **Reject `format: uuid`.**
* **Action Alignment & Completeness:**
  * Verify paths against **§9.1 Ringkasan Fungsi** / **§9.2 Core Functions & Business Logic** (or legacy `03-functional.md`).
  * Cross-check all interactive controls in **§4.1 Pages & Components** (row actions, bulk actions, toolbar actions) and **§4.2 Layout & Wireframe Spec** (header avatar controls, modal sub-actions, inline-edits). Ensure dedicated endpoints (e.g. photo delete `DELETE /v2/iam/users/{userId}/photo`, resend communications, pair/unpair) are explicitly declared in OpenAPI `paths:`.

### 2. DTO & Field Coverage
* **Form Inputs ↔ Request Body:** Are all form fields present in the OpenAPI Request Body schema?
  * **Single-file PRD (`<module>.md`):** Cross-check fields against **§9.9 Validation Rules Table (M-3)** (the canonical authority for field names, types, nullability, min/max, and required/optional status), **§4.1 Pages & Components**, and **§4.2 Layout & Wireframe Spec**.
  * **Legacy 4-file PRD:** Cross-check against `02-ui-design.md` and `03-functional.md`.
  * **Naming & Casing:** Check camelCase vs snake_case consistency and field aliases.
  * **Required Fields:** `required` array in OpenAPI matches `Required: Yes` in the PRD validation rules.
  * **Snowflake ID Fields:** All entity ID fields in DTOs (`id`, `user_id`, `client_id`, `asset_id`) must be `string` matching `^[0-9]+$`. Reject `format: uuid`.
  * **Date & Timestamp Fields:** All date/time fields must standardize on epoch milliseconds (`type: integer, format: int64`) rather than inconsistent string dates (`YYYY-MM-DD`), unless explicitly specified otherwise.
* **UI Elements ↔ Response Body:** Do response DTOs provide all data displayed on cards, headers, detail views, and logical entities (**§4.1**, **§4.2a Setup Column**, and **§9.8/§9.2**)?
* **Derived Display States vs Physical Database Enums:** Clarify whether status fields represent raw database enums or derived UI display states (e.g. `Pending Email Confirmation` is a derived display state of `Activated` + `pendingEmail != null`, not a distinct DB enum).

### 3. Table Columns, Filters & Sort Alignment
* **Column List Coverage:** Does the list item DTO (`*ListItem`) return all curated and optional columns defined in **§4.1 Pages & Components** and **§4.2a Column Visibility & Setup Column**?
* **Filter Query Parameters:** Are all UI filters (dropdowns, date ranges, multi-selects) in **§4.1** supported as query parameters in the list endpoint?
* **Search Scoping (`Searchable: ✅`):** Does the list endpoint search parameter and query builder accommodate **all** columns flagged as `Searchable: ✅` in **§4.2a Setup Column** (e.g. Name, Email, TAG, Activation Status, Temporary User, Employee ID)? Reject descriptions or implementations that restrict search to only `name/email` when PRD designates additional attributes as searchable.
* **Sorting & Pagination:** Do sort parameters align with **§4.2a Setup Column** (sortable flags) and **§5 Global Table Rules** (default sort key, asc/desc directions, pagination)?
* **Export Column Header Parity:** Do CSV/Excel export schema examples and backend Go struct tags (`export:"..."`) match canonical PRD column titles defined in **§4.2a Setup Column** (e.g. `Temporary Until`, `TAG`, `Temporary User`, `Last Update`)?
* **Pruned/Renamed Columns:** If PRD updated or removed a column (e.g. `User Type` removed, `Active Period Until` → `Temporary Until`), verify that API specs have deprecated or removed the old filter/sort parameters.

### 4. Enums, Statuses & Action Types Sync
* **Status Badges & Enums:** Are all status values defined in PRD state machines and UI badges (**§7 States & Feedback**, **§9.2 Core Functions**) represented exactly in OpenAPI enums?
* **Action Types Across All 5 Log Surfaces:** Do enum lists in OpenAPI (e.g. `ChangelogItem.action`, audit filters, or system event enums) contain all actions defined across PRD **§8.1–§8.5**:
  * **§8.2 Changelog per Objek** (and §8.2a Changelog per Tab)
  * **§8.3 System Log**
  * **§8.4 User Log** (platform-level login/logout/session events)
  * **§8.5 Activity Log** (operational activities)

### 5. Validation Rules & HTTP Error Mapping
* **Error Status Codes & Messages:**
  * Validation / Business rule failure → `422 Unprocessable Entity` or `400 Bad Request` (verify against **§6 Validasi & Error Messages** and **§9.9 Validation Rules Table**).
  * Permission / Subscription gating denied → `403 Forbidden` (verify against **§2 Personas & Role Matrix**, **§9.3 Permissions / Gate Capability**, and **§9.10 Plan Gating**).
  * Resource not found → `404 Not Found`.
  * Conflict / Unique constraint violation / Concurrency race → `409 Conflict` (verify against **§9.6 Action Resilience Spec** for concurrency baseline checks by `updatedAt`/version).
* **Verbatim Success & Error Message Parity:**
  * Does the response message string (`ApiResponse.message`) in OpenAPI examples and schemas match the verbatim canonical copywriting defined in PRD **§6 Validasi & Error Messages** and **§7 States & Feedback (Toasts)**? Reject typos, grammatical errors (e.g. *"Successfully resetted"*), or drifted paraphrases.
* **Error Envelope:** Does the error response match the platform standard envelope (`ApiResponse`: `{ "message": "...", "data": ... }`)?
* **Server-Side Gating:** Does the backend handler validate subscription/access limits server-side (**§2.0 Entity Gate**, **Core Rules Implementation**, **§5 Selection Limits**) rather than blindly trusting frontend inputs?

---

## 2. Step-by-Step Audit Workflow

1. **Read PRD Specifications:**
   * Locate the target module under `docs/<domain>/<module>/PRD/` (or sibling `../docs/<domain>/<module>/PRD/` if running from within a service directory), where `<domain>` includes `admin-console`, `auth`, `global-settings`, etc.
   * **Single-file Canonical PRD (`<module>.md`):**
     * Review `Core Rules Implementation` and `Aturan Logika Modul` (`<PREFIX>-LOGIC-xx`).
     * Review **§1 Overview** (Problem Statement, Scope, Feature Summary).
     * Review **§2 Personas & Role Matrix** (§2.1 Entity Gate, §2.2 Access Level, §2.3 Group Role, §2.4 Special Access, §2.5 POV Matrix).
     * Review **§3 User Flow** (Status flow, lifecycle diagrams, destructive trio).
     * Review **§4 UI Design** (§4.1 Pages & Components, §4.2 Layout & Setup Column §4.2a, §4.8 UI Visibility per Role).
     * Review **§5 Global Table Rules** (Selection limit auto-cap/live-block, sorting, pagination, search scoping).
     * Review **§6 Validasi & Error Messages** (Client error copy, validation triggers, uniqueness debounces).
     * Review **§7 States & Feedback** (Empty states, badges, feedback toasts).
     * Review **§8 Logging** (§8.1 Overview, §8.2 Changelog Object, §8.2a Changelog Tab, §8.3 System Log, §8.4 User Log, §8.5 Activity Log).
     * Review **§9 Functional Requirements** (§9.1 Ringkasan Fungsi, §9.2 Core Functions, §9.3 Permissions, §9.4 Edge Cases & Must NOT §9.4.3, §9.6 Action Resilience, §9.7 Sync Matrix, and §9.9 Validation Rules Table M-3).
     * Review **§10 Constraints** and **§11 Dependencies & Breaking Changes** (and §12 Open Items).
     * Consult companion files:
       * `<module>.changelog.md`: Specification revision history and closed audit findings (explains intentional design decisions/updates).
       * `<module>.resolved.md`: Closed product/architecture open items.
   * **Legacy 4-file PRD Fallback (only if `<module>.md` is absent):**
     * Review `01-overview.md` (Scope, Feature List, Changelog dates).
     * Review `02-ui-design.md` (Form fields, table columns, filter panels, error copy).
     * Review `03-functional.md` (Business rules, validations, role permissions, edge cases, logical data dictionary).
     * Review `04-dependencies.md` (Internal/External dependencies, sync matrix).
   * **Consult Existing Audit Records:**
     * Inspect `docs/<domain>/<module>/audit/` (or `../docs/<domain>/<module>/audit/`):
       * `findings.md`: review previously closed/resolved consistency items to avoid re-opening settled decisions.
       * `api-prd-gaps.md`: review previously documented API vs PRD gaps.

2. **Read API Specifications:**
   * Inspect all API contract specifications in the module's `API/` folder (`docs/<domain>/<module>/API/` or `../docs/<domain>/<module>/API/`):
     * **Monolithic specs:** `openapi.yaml` and `low-level-design.md`.
     * **Modular endpoint specs:** all `*.yaml` and `*.md` files (e.g. `user-crud.yaml`, `user-status.yaml`, `reset-authenticator.yaml`, `user-email-change.yaml`).
     * Aggregate the complete set of paths, schemas, and endpoints across all modular files.

3. **Cross-Check & Detect Gaps:**
   * Compare each PRD requirement against the OpenAPI YAMLs and LLD handler logic across the 5 dimensions.

4. **Generate Gap Analysis Report:**
   * Produce a structured gap report following the canonical format below.
   * When auditing in a module repository with an `audit/` folder, synchronize or output findings directly to **`docs/<domain>/<module>/audit/api-prd-gaps.md`**.

---

## 3. Output Report Format

Generate findings using this clear, actionable markdown structure:

```markdown
# Rincian Gap: API Specs vs PRD (Modul: [Nama Modul])

### 1. [Judul Masalah / Gap]
- **Masalah:** Ringkasan singkat ketidaksesuaian.
- **Kondisi PRD:** Kutipan / referensi baris di `PRD/<module>.md §X.Y` (atau legacy `PRD/03-functional.md §x.y`) (`file:Lxx`).
- **Kondisi API:** Kutipan / referensi baris di `API/*.yaml` atau `API/*.md` (`file:Lxx`).
- **Aksi Perbaikan:** Langkah konkret untuk menyelaraskan API dengan PRD (atau sebaliknya jika PRD yang berubah).
```
