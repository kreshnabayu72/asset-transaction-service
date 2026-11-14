### Summary

<!-- Provide a brief, high-level summary of what this PR introduces or fixes. -->

### Motivation & Context

<!-- Why is this change necessary? Reference the relevant task/ticket or ADR. -->

- Closes / Ref: #
- ADR Reference: ADR-

### Type of Change

- [ ] `feat`: New feature or API endpoint
- [ ] `fix`: Bug fix
- [ ] `refactor`: Code refactoring without behavioral changes
- [ ] `chore`: Tooling, build config, or dependency update
- [ ] `docs`: Documentation only changes
- [ ] `test`: Test suite additions or improvements

---

### Changes Made

<!-- Bullet points detailing the specific additions, updates, or removals. -->

-

---

### Verification & Testing

<!-- Describe how you verified these changes locally. -->

- [ ] `go test ./...` passed locally.
- [ ] Migrations tested with `mig verify --schema <tenant_schema>` (if applicable).
- [ ] API endpoint tested via Bruno / HTTP client.

---

### Pre-Merge Checklist (Definition of Done)

<!-- Review against engineering.md Section 15 before requesting review. -->

- [ ] **Layering & Scope**: Strictly follows 4-layer architecture (`handler` -> `service` -> `repository`). No SQL queries or Temporal SDK in handlers (ADR 003).
- [ ] **Data Isolation**: Service reads/writes ONLY its own database tables (ADR 001). No cross-service SQL joins.
- [ ] **Cross-Service Writes**: Dispatched via Temporal activities on the owning service's task queue with rollbacks registered (ADR 006).
- [ ] **Typing**: All Snowflake IDs (`id`, `user_id`, `client_id`, `asset_id`) use `int64`.
- [ ] **Security & Route Gates**: Route gate mounted with `rbac.Require(...)` in proper order after `LoadUserRoles` (ADR 013). Identity read strictly from gateway headers (`X-User-ID`, `X-Client-ID`).
- [ ] **Error Handling**: Uses `coreEntity.ToHttpError(err).SendResponse(c)` and `coreHelper.Success`. Internal errors wrapped with `%w`.
- [ ] **Unit Tests**: Repository tests use stateful in-memory fakes (ADR 015), not brittle mock chains.
