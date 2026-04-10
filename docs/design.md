# PropertyOps — Design Document

## 1. System Overview

PropertyOps is an offline-first, Docker Compose-deployed Go backend for multifamily property operations. It manages the full lifecycle of maintenance work orders, tenant communications, payment processing, compliance governance, analytics reporting, and system security — all backed by a single MySQL 8.0 database with no cloud dependency.

Primary roles:

- **Tenant** — residents who file and track maintenance requests
- **Technician** — field staff who receive and execute assigned work orders
- **PropertyManager** — staff who oversee properties they are actively assigned to
- **ComplianceReviewer** — staff who review governance reports and apply enforcement
- **SystemAdmin** — full administrative control over the entire system

Core capabilities:

- DB-backed opaque session authentication with bcrypt password hashing and sliding-window expiry
- Work order lifecycle with deterministic round-robin dispatch, SLA enforcement, cost item tracking, and tenant ratings
- Inline file attachments (JPEG/PNG, magic-byte validated, SHA-256 hashed) uploaded atomically with work order creation
- In-app notification center with Go text/template rendering, retry scheduler, and message threads
- Offline payment intents with 30-minute expiry, dual-approval gate for high-value amounts, and daily CSV reconciliation
- Governance reports with enforcement actions (warning, rate-limit, suspension), keyword blacklist, and risk rules
- AES-256-GCM field encryption with key versioning and rotation tracking
- Append-only audit log for every state change across all domains
- Encrypted daily database backups with configurable retention
- Role-scoped analytics (popularity, funnel, retention, quality) with saved and scheduled report generation
- Health endpoints with subsystem readiness checks

The system runs as a Go + Gin HTTP server consuming a MySQL database, with all migrations managed by a dedicated `migrate` binary. A Docker Compose stack provides single-command local deployment.

---

## 2. Design Goals

**Offline-first local deployment** — the entire stack runs on a single machine via Docker Compose. No cloud accounts, SaaS subscriptions, or internet access required after the initial image pull.

**Server-side authority** — all validation, authorization, encryption, SLA computation, and state transitions are enforced in the backend. Clients are presentation layers.

**Domain-driven module boundaries** — each functional area (auth, users, tenants, properties, workorders, attachments, notifications, governance, payments, analytics, security, audit, backups, health, admin) is a self-contained Go package with its own model, DTO, repository, service, and handler files. Cross-module dependencies flow through explicit interfaces, not direct struct imports.

**Scoped access by design** — PropertyManagers see only the properties they are actively assigned to. The `ScopedToPropertyIDs` sentinel in list requests ensures an empty managed-property list returns zero results rather than falling through to return all records. This invariant is enforced in every list repository method.

**Deterministic workflows** — work order lifecycle follows a strict, code-enumerated state machine; dispatch follows deterministic round-robin via a cursor table; payment approval follows an explicit order-counted flow.

**Encryption at rest** — AES-256-GCM protects sensitive PII fields. Backups are separately encrypted before writing to disk.

**Full auditability** — every domain mutation is recorded in the append-only `audit_logs` table, including actor, resource, action, IP, and request ID.

**Testable architecture** — repositories accept `*gorm.DB` (allows transaction injection); services use constructor injection; interfaces are defined at module boundaries for mock substitution in tests.

---

## 3. High-Level Architecture

```
┌──────────────────────────────────────────────────────────┐
│                  HTTP Clients (API consumers)             │
└──────────────────────────┬───────────────────────────────┘
                           │  HTTP (Bearer token)
┌──────────────────────────┴───────────────────────────────┐
│                   Go + Gin HTTP Server                    │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Global Middleware                                  │  │
│  │  RequestID → StructuredLogging → PanicRecovery      │  │
│  └──────────────────────┬──────────────────────────────┘  │
│  ┌──────────────────────┴──────────────────────────────┐  │
│  │  Auth Middleware                                    │  │
│  │  Authenticate → CheckSuspension                    │  │
│  └──────────────────────┬──────────────────────────────┘  │
│  ┌──────────────────────┴──────────────────────────────┐  │
│  │  Route Groups → Handlers (thin HTTP adapters)       │  │
│  └──────────────────────┬──────────────────────────────┘  │
│  ┌──────────────────────┴──────────────────────────────┐  │
│  │  Services (business logic, policy, audit calls)     │  │
│  └──────────────────────┬──────────────────────────────┘  │
│  ┌──────────────────────┴──────────────────────────────┐  │
│  │  Repositories (GORM, accepts *gorm.DB for tx inj.)  │  │
│  └──────────────────────┬──────────────────────────────┘  │
│  ┌──────────────────────┴──────────────────────────────┐  │
│  │  Security Layer (AES-256-GCM, key versioning)       │  │
│  └──────────────────────┬──────────────────────────────┘  │
└──────────────────────────┬───────────────────────────────┘
                           │
              ┌────────────┴────────────┐
              │        MySQL 8.0        │
              │  (~31 tables, managed   │
              │   by migrate binary)    │
              └─────────────────────────┘
```

---

## 4. Deployment Architecture

### Docker Compose Stack

| Service | Role | Notes |
|---|---|---|
| `db` | MySQL 8.0 | Persistent volume; health-checked before api/migrate start |
| `migrate` | One-shot migration runner | Applies all SQL migrations then exits 0; optionally runs `-seed` for initial data |
| `api` | Go HTTP server | Multi-stage Dockerfile; runs as non-root user; waits for db readiness |

```
docker compose -f deploy/docker-compose.yml up --build -d
```

The `migrate` service exits with code 0 after applying migrations and is shown as `Exited (0)` in `ps` output — this is expected, not an error.

### Admin Bootstrap

Initial role seeding and admin user creation:

```bash
ADMIN_BOOTSTRAP_PASSWORD=<secret> docker compose run --rm migrate -seed
```

Creates the five standard roles (`Tenant`, `Technician`, `PropertyManager`, `ComplianceReviewer`, `SystemAdmin`) and the `admin` user account with the provided password.

### Environment Configuration

All runtime behavior controlled via environment variables. No files need manual editing for a standard local deployment. See `Configuration Reference` in `api_spec.md`.

---

## 5. Backend Architecture

### 5.1 Framework & Tooling

| Concern | Technology |
|---|---|
| Language | Go 1.25 |
| HTTP | Gin (`github.com/gin-gonic/gin`) |
| ORM | GORM (`gorm.io/gorm`) |
| Database driver | `gorm.io/driver/mysql` |
| Migrations | SQL migration files applied by `cmd/migrate` |
| UUIDs | `github.com/google/uuid` |
| Bcrypt | `golang.org/x/crypto/bcrypt` (cost ≥ 12) |
| Config | Environment variable loading with struct validation |
| Logging | Structured JSON logging via `internal/logs` |
| Testing | `go test` + `gorm.io/driver/sqlite` (in-memory, CGO required) |

### 5.2 Module Structure

Each functional module follows a consistent file layout:

```
internal/{module}/
  ├── model.go       # GORM structs + TableName()
  ├── dto.go         # Request and response types
  ├── repository.go  # Database operations (accepts *gorm.DB)
  ├── service.go     # Business logic + audit calls
  ├── handler.go     # HTTP handlers (thin: parse → validate → service → format)
  └── routes.go      # RegisterRoutes() called from app/routes.go
```

### 5.3 Centralized Route Registration

`internal/app/routes.go` is the single location where all modules are wired together:

1. Shared services are constructed (audit, security, logs).
2. Middleware chain is built (RequestID, StructuredLogging, PanicRecovery, Authenticate, CheckSuspension).
3. Health endpoints are registered (public).
4. API v1 group is created; auth public routes and anomaly ingestion (pre-auth) are registered.
5. Protected route group (post-auth) registers all domain modules.
6. Adapters resolve circular import constraints at the wiring layer (e.g., `attachmentUploaderAdapter` wraps `*attachments.Service` to satisfy `workorders.AttachmentUploader`).

### 5.4 Service Layer Responsibilities

| Service | Key Responsibilities |
|---|---|
| AuthService | Login, logout, session validation, session sliding-window refresh |
| UsersService | User CRUD, role assignment, PII masking, bcrypt hashing |
| TenantsService | Tenant profile CRUD, PM property scope enforcement |
| PropertiesService | Property/unit CRUD, staff assignments, skill tags, dispatch cursors |
| WorkOrdersService | WO lifecycle, SLA assignment, auto-dispatch, reassignment, cost items, ratings |
| AttachmentsService | MIME + magic-byte validation, SHA-256 hash, disk I/O, access scope checks |
| NotificationsService | Template rendering, send/schedule, retry, mark-read, threads, participants |
| GovernanceService | Reports, review workflow, enforcement application/revocation, keywords, risk rules |
| PaymentsService | Intent creation/expiry, mark-paid, dual-approval, settlements, reversals, makeups |
| ReconciliationService | Daily run, expected vs actual totals, discrepancy flagging, CSV generation |
| AnalyticsService | Aggregation queries, saved/generated reports, export |
| SecurityService | AES-256-GCM encrypt/decrypt, key versioning, rotation metadata |
| AuditService | Append-only audit log writes; queried by admin endpoints |
| BackupsService | mysqldump invocation, AES-256-GCM encryption, retention enforcement |
| LogsService | Structured log file reading and querying |
| HealthService | DB ping, storage path check, key check, backup dir check |

### 5.5 Repository Pattern

- Repositories accept `*gorm.DB` as their only DB dependency, enabling transaction injection from services.
- Multi-step operations use `db.Transaction(func(tx *gorm.DB) error { ... })`.
- List methods receive a filter struct; the `ScopedToPropertyIDs bool` sentinel is checked first — if `true` and `PropertyIDs` is empty, the method returns immediately with zero results without hitting the database.

### 5.6 Interface Boundaries

Interfaces at module boundaries prevent import cycles and improve testability:

| Interface | Defined In | Implemented By | Used By |
|---|---|---|---|
| `WorkOrderQuerier` | `attachments` | `*workorders.Repository` | `attachments.Service` (PM scope on WO attachments) |
| `WorkOrderChecker` | `notifications` | `*workorders.Repository` | `notifications.Service` (thread WO access check) |
| `AttachmentUploader` | `workorders` | `attachmentUploaderAdapter` in `app/routes.go` | `workorders.Handler` (inline multipart upload) |
| `PropertyChecker` | `analytics` | `*workorders.Repository` | `analytics.Handler` (PM property scope enforcement) |
| `AuditLogger` | multiple modules | `*audit.Service` | Payments, attachments, notifications, governance, etc. |
| `NotificationSender` | `payments`, `workorders` | `*notifications.Service` | Event-triggered notifications |

---

## 6. Authentication & Security Design

### 6.1 Authentication Model

- Local username/password authentication; no OAuth or external identity provider.
- Passwords hashed with bcrypt (`golang.org/x/crypto/bcrypt`), minimum cost factor 12.
- Sessions stored in the `sessions` MySQL table, identified by 32-byte cryptographically random opaque tokens.
- Token transmitted as `Bearer <token>` in the `Authorization` header.

### 6.2 Session Management

- Sliding-window idle timeout: each authenticated request refreshes `last_active_at`.
- Absolute maximum lifetime: 7 days from creation (configurable via `SESSION_MAX_LIFETIME_HOURS`).
- Default idle timeout: 30 minutes (configurable via `SESSION_IDLE_TIMEOUT_MINUTES`).
- `AuthMiddleware` validates token existence, checks both timeouts, loads user roles, and sets context keys for downstream handlers.
- Logout hard-deletes the session row.

### 6.3 Suspension Check

`CheckSuspension` middleware runs after authentication. It queries `enforcement_actions` for any active `Suspension` record against the current user. Suspended users receive 403 on every protected endpoint until the suspension expires or is revoked.

### 6.4 Rate Limiting

Rate limiting is DB-backed. The `RateMiddleware` tracks submissions per user per action type within a rolling hour window (configurable via `RATE_LIMIT_MAX_SUBMISSIONS_PER_HOUR`). Work order creation is rate-limited to prevent abuse. Returns 429 when exceeded.

### 6.5 Route-Level Authorization

| Route Group | Requirement |
|---|---|
| `/health/*` | Public |
| `POST /api/v1/auth/login` | Public |
| `POST /api/v1/admin/anomaly` | Local network CIDR (no token required) |
| All other `/api/v1/*` | Valid session token |

Role checks within protected routes are enforced by a combination of `RequireRoles()` middleware and object-level service-layer checks.

### 6.6 Object-Level Authorization

All resource access uses layered authorization:

1. Route middleware enforces minimum role requirement.
2. Service layer checks ownership/assignment/property management against the specific resource.

Property scope for PropertyManagers is always enforced via `AND role = 'PropertyManager' AND is_active = true` in the `property_staff_assignments` table. This prevents technicians or inactive staff from being counted as managing a property.

### 6.7 Request ID Propagation

Every request receives a unique UUID request ID (from the `X-Request-ID` header or generated if absent). The request ID is:
- Added to the response as `X-Request-ID`.
- Stored in Gin's context for downstream handler use.
- Written to all audit log entries for end-to-end traceability.

---

## 7. Encryption Design

### 7.1 At-Rest Field Encryption

| Property | Value |
|---|---|
| Algorithm | AES-256-GCM |
| IV | 96-bit (12 bytes), random per field value |
| Auth Tag | 128-bit |
| Key Storage | Files in `ENCRYPTION_KEY_DIR` |
| Active Key | `ENCRYPTION_ACTIVE_KEY_ID` |
| Storage Format | Base64-encoded (IV prepended to ciphertext) |
| Rotation Period | 180 days (configurable) |

Key versioning allows multiple key versions to coexist. The active key encrypts new values; older keys decrypt existing values. `GET /api/v1/admin/keys` shows active key version and `rotation_due` date. `POST /api/v1/admin/keys/rotate` creates a new key version.

### 7.2 Backup Encryption

When `BACKUP_ENCRYPTION_ENABLED=true`:
- `mysqldump` output is piped through AES-256-GCM encryption using the active key.
- Encrypted file written to `BACKUP_ROOT` with `.enc` extension.
- Integrity and restoration tested via `POST /api/v1/admin/backups/validate`.

### 7.3 PII Masking

Sensitive fields (emergency contacts, phone numbers) are masked in API responses by default. The full value is accessible only via the PII export endpoint (`POST /api/v1/admin/pii-export`), which requires SystemAdmin, a stated purpose, and produces a full audit log entry.

---

## 8. Work Order Domain Design

### 8.1 State Machine

```
New ──→ Assigned ──→ InProgress ──→ AwaitingApproval ──→ Completed ──→ Archived
```

Backward transition (AwaitingApproval → InProgress) is permitted when a PropertyManager sends a work order back for additional work. All other reverse transitions are rejected.

Every state change writes an immutable `work_order_events` row: `actor_id`, `event_type`, `from_status`, `to_status`, `description`, `metadata`, `created_at`.

### 8.2 SLA Assignment

SLA due date is computed at creation time from UTC now + priority offset:

| Priority | Deadline |
|---|---|
| Emergency | +4 hours |
| High | +24 hours |
| Normal | +72 hours |
| Low | +5 business days |

Background scheduler polls every ~5 minutes. Breached work orders receive `sla_breached_at` timestamp, a `sla_breached` event, and in-app notifications to the tenant and assigned technician.

### 8.3 Round-Robin Dispatch

Triggered at work order creation if `skill_tag` is non-empty:

1. Fetch all `property_staff_assignments` for the property where `role = 'Technician'` and `user_id` exists in `technician_skill_tags` with the matching tag.
2. Read `dispatch_cursors` for `(property_id, skill_tag)`.
3. Select technician at `cursor_position % len(technicians)`.
4. Upsert cursor with incremented position and `last_assigned_user_id`.
5. Set `work_order.assigned_to` and transition status to `Assigned`.

If `skill_tag` is empty or no matching technicians exist, the work order stays `New`. PropertyManagers dispatch manually via `POST /work-orders/:id/dispatch`.

### 8.4 Multipart Work Order Creation

Clients may submit work order creation as `multipart/form-data`:
- Metadata JSON in the `data` field.
- Up to 6 files in `attachments[]`.

Atomicity guarantee: if any attachment fails validation (wrong MIME, too large, magic-byte mismatch), the handler:
1. Calls `AttachmentUploader.DeleteWorkOrderAttachments(wo.ID)` to remove all already-written attachment files and DB records.
2. Calls `WorkOrderService.Delete(wo.ID)` to remove the work order row.
3. Returns 422 to the client.

The client never receives a partially-created resource.

---

## 9. Payments Design

### 9.1 Payment Kind Relationships

```
Intent (Pending) ──→ Intent (Paid) ──→ Intent (Settled)  [after approval(s)]
                                   ──→ Intent (Reversed)

Intent (Paid) ──→ SettlementPosting (Settled)
              ──→ MakeupPosting (Paid)
              ──→ Reversal (Reversed)

Intent (Pending) ──→ Intent (Expired)  [background job]
```

### 9.2 Dual-Approval Gate

Applies when `payment.Amount > PAYMENT_DUAL_APPROVAL_THRESHOLD` (default $500.00):

- `PaymentApproval` records are ordered (`approval_order = 1, 2`).
- Two approvals from distinct users are required to settle.
- Same-user second approval → 409.
- Reversal or makeup requires prior dual approval on amounts above threshold.
- For amounts ≤ threshold, a single approval immediately settles.

### 9.3 Daily Reconciliation

`expected = sum(Intent.amount WHERE status IN ('Paid', 'Settled') AND date = run_date)`

Both `Paid` and `Settled` are included because `ApprovePayment` transitions intents to `Settled`. Counting only `Paid` would systematically undercount expected revenue for dual-approved payments.

`actual = totalSettlements + totalMakeups − totalReversals`

A discrepancy flag is raised when `|expected − actual| > $0.01`. CSV statement is written to `STORAGE_ROOT/reconciliation/YYYY-MM-DD.csv`.

---

## 10. Governance Design

### 10.1 Report Workflow

1. Any authenticated user files a report against a `Tenant`, `WorkOrder`, or `Thread`.
2. Report is created with status `Open`.
3. ComplianceReviewer sets status to `InReview` to claim it.
4. Reviewer resolves (`Resolved`) or closes without action (`Dismissed`), providing `resolution_notes`.
5. Reviewer may attach enforcement actions at any point during review.

### 10.2 Enforcement Mechanics

| Type | Effect |
|---|---|
| Warning | Logged only; no functional restriction |
| RateLimit | Applied against the DB-backed rate limiter for the user |
| Suspension | `CheckSuspension` middleware blocks all requests; returns 403 |

Active enforcement records have `is_active = true`. Revocation sets `revoked_at` and `revoked_by`; `is_active` transitions to false. Time-limited suspensions automatically expire when `ends_at < now`.

### 10.3 Keyword Blacklist

Keywords stored in `keywords_blacklist`. When governance-flagged content (e.g., descriptions, thread messages) is processed, it is checked against active keywords. Matches are logged for reviewer follow-up. SystemAdmin manages the keyword list.

### 10.4 Risk Rules

Risk rules define condition/action pairs stored as JSON parameters. The risk rule engine (`condition_type`, `condition_params`, `action_type`, `action_params`) allows extensible, configuration-driven policy enforcement. SystemAdmin manages rules.

---

## 11. Notification Design

### 11.1 Delivery Model

Notifications are in-app only. No external channels (email, SMS) are implemented. The notification service provides:
- Direct send (`SendDirect`): immediate, from subject/body strings.
- Template send (`SendFromTemplate`): renders Go `text/template` with a data map against a `notification_templates` record.
- Event send (`SendEvent`): convenience wrapper for template send used by other modules.
- Role fan-out (`SendEventToRole`): resolves user IDs by role, sends to each.
- Scheduled send (`ScheduleNotification`): sets `scheduled_for`; background scheduler delivers at time.

Background scheduler polls at `NOTIFICATION_POLL_INTERVAL_SECONDS` (default 60s). Failed deliveries are retried up to `NOTIFICATION_MAX_RETRIES` (default 3) times with `NOTIFICATION_RETRY_DELAY_SECONDS` (default 300s) between attempts.

### 11.2 Message Threads

Threads are optionally linked to a work order. When `work_order_id` is provided at thread creation, the service enforces WO access scope (same rules as attachments):
- PropertyManager: must manage the WO's property.
- Technician: must be the assigned technician.
- Tenant: must be the WO creator.
- SystemAdmin / ComplianceReviewer: unconditional.

Participants are tracked in `thread_participants`. Only current participants may post messages or add further participants. Closed threads reject new messages and participants.

---

## 12. Analytics Design

### 12.1 Metrics Computed

| Metric | Query Basis |
|---|---|
| Popularity | `GROUP BY issue_type ORDER BY count DESC` on work orders |
| Funnel | Count WOs at each status stage |
| Retention | Distinct vs repeat units with WOs in 30d and 90d windows |
| Tags | `GROUP BY skill_tag ORDER BY count DESC` |
| Quality | Average rating; negative rate (`rating <= 2`); governance report count linked to WOs |

### 12.2 PropertyManager Scope Enforcement

Analytics handlers enforce PM scope via the `PropertyChecker` interface (implemented by `*workorders.Repository`). A PM must supply a `property_id` query parameter that they actively manage. If no `property_id` is supplied, or the PM does not manage it, the handler returns 403. SystemAdmin may query across all properties.

### 12.3 Saved and Scheduled Reports

`SavedReport` records store report type, filters (as JSON), output format, and optional cron/frequency schedule. A background scheduler polls for due reports and triggers generation via `AnalyticsService`. Generated reports are written to `STORAGE_ROOT` and tracked in `generated_reports`.

### 12.4 Data Export

Ad-hoc exports write CSV to local storage. `audit_logs` export is restricted to SystemAdmin. Other export types are restricted to PM (for their managed property) or SystemAdmin. The `purpose` field is mandatory and written to the audit log.

---

## 13. Data Persistence Design

### 13.1 Database Tables (~31 tables across 10 migration pairs)

| Migration | Tables |
|---|---|
| 000001 | users, roles, user_roles |
| 000002 | sessions |
| 000003 | tenant_profiles, properties, units, property_staff_assignments, technician_skill_tags, dispatch_cursors |
| 000004 | work_orders, work_order_events, cost_items |
| 000005 | attachments |
| 000006 | notification_templates, notifications, notification_receipts, message_threads, thread_participants, thread_messages |
| 000007 | reports, enforcement_actions, keywords_blacklist, risk_rules |
| 000008 | payments, payment_approvals, reconciliation_runs |
| 000009 | saved_reports, generated_reports |
| 000010 | audit_logs, encryption_key_versions, system_settings |

### 13.2 Key Indexes

| Table | Index |
|---|---|
| users | UNIQUE(username) |
| sessions | UNIQUE(token) |
| work_orders | (property_id, status, created_at), (tenant_id), (assigned_to) |
| work_order_events | (work_order_id, created_at) |
| attachments | (entity_type, entity_id), (sha256_hash) |
| payments | (property_id, status, created_at), (tenant_id) |
| payment_approvals | (payment_id, approver_id) |
| reports | (target_type, target_id), (reporter_id), (status) |
| enforcement_actions | (user_id, is_active) |
| notifications | (recipient_id, status) |
| message_threads | (work_order_id) |
| thread_participants | (thread_id, user_id) |
| audit_logs | (resource_type, resource_id), (actor_id), (created_at) |
| dispatch_cursors | UNIQUE(property_id, skill_tag) |

### 13.3 Money Representation

All monetary amounts are stored as `DECIMAL(12,2)`. No floating-point columns are used for financial data. Application-layer validation rejects non-positive amounts.

### 13.4 Soft Delete

Work orders use GORM's `DeletedAt` soft-delete. Other entities use hard delete or `is_active`/`is_revoked` flags.

---

## 14. Error Handling Strategy

### 14.1 AppError

All service methods return `(result, *common.AppError)`. `AppError` carries:
- `Code` — machine-readable error code string.
- `Message` — human-readable message.
- `HTTPStatus` — used by `RespondError` to set the HTTP status code.
- `FieldErrors []FieldError` — per-field validation errors (omitted from JSON if empty).

### 14.2 HTTP Status Mapping

| AppError Code | HTTP Status | Usage |
|---|---|---|
| VALIDATION_ERROR | 422 | Field-level validation failure |
| NOT_FOUND | 404 | Resource does not exist |
| UNAUTHORIZED | 401 | Missing or invalid session |
| FORBIDDEN | 403 | Insufficient permission |
| CONFLICT | 409 | Duplicate approval, state conflict |
| RATE_LIMITED | 429 | Submission rate limit exceeded |
| BAD_REQUEST | 400 | Malformed request |
| PAYLOAD_TOO_LARGE | 413 | File exceeds size limit |
| INTERNAL_ERROR | 500 | Unexpected server error |

### 14.3 Panic Recovery

`PanicRecovery` middleware catches any unhandled panics, logs a stack trace, and returns 500 to the client without crashing the server process.

---

## 15. Scheduled Tasks

| Task | Trigger | Description |
|---|---|---|
| SLA breach detection | ~5-minute poll | Sets `sla_breached_at`; creates `sla_breached` event; sends in-app notifications |
| Payment intent expiry | Continuous background | Marks `Pending` intents past `expires_at` as `Expired` |
| Notification delivery | 60-second poll (default) | Sends `Pending` notifications; retries `Failed` up to max retries |
| Daily backup | Cron `0 2 * * *` (default) | Encrypted mysqldump to `BACKUP_ROOT` |
| Report generation | 300-second poll (default) | Executes scheduled `saved_reports` and writes output to `STORAGE_ROOT` |

---

## 16. Testing Strategy

### 16.1 Integration Tests (`test/`)

Full-stack integration tests use SQLite in-memory databases (`gorm.io/driver/sqlite`) to exercise the real service and repository layers without a running MySQL instance. CGO must be enabled.

Test files:
- `setup_test.go` — DB setup, AutoMigrate, role seeding, user creation helpers.
- `helpers_test.go` — HTTP test router construction, `makeRequest`, `loginUser`, `assertStatus`, `parseResponse`.
- `workorder_lifecycle_test.go` — Full Tenant→Assigned→InProgress→AwaitingApproval→Completed→Rating lifecycle.
- `payment_flow_test.go` — Intent creation, mark-paid, dual-approval, settlement, reversal.
- `governance_flow_test.go` — Report creation, review, enforcement, suspension effect.
- `authorization_matrix_test.go` — Systematic per-role, per-endpoint coverage including negative PM property-scope tests.
- `auth_flow_test.go` — Login, logout, session expiry, me endpoint.

### 16.2 SQLite Caveats

SQLite integration tests do not reproduce MySQL-specific behaviors:
- JSON column semantics (`JSON_TABLE`, `JSON_CONTAINS`).
- Strict FK constraint enforcement and cascade ordering.
- DECIMAL precision and rounding rules.
- Index-level uniqueness enforcement differences.

For full schema fidelity, tests can be run against a real MySQL 8.0 instance:

```bash
TEST_MYSQL_DSN="user:pass@tcp(127.0.0.1:3306)/propertyops_test?parseTime=true" \
  go test ./test/... -tags=mysql_integration
```

### 16.3 Key Test Coverage

| Area | Coverage |
|---|---|
| Auth | Login, logout, session validation, me endpoint |
| Work orders | Full lifecycle, SLA set, PM property scope, technician assignment scope |
| Payments | Intent + expiry, dual-approval mechanics, reconciliation |
| Governance | Report → review → enforcement → suspension blocks subsequent requests |
| Authorization matrix | Unauthenticated → 401; wrong role → 403; correct role → 2xx |
| PM scope negative | PM with zero property assignments sees empty payment list; cannot delete unscoped attachments |

---

## 17. Implementation Constraints

- All data stored in a single MySQL instance — no distributed databases or read replicas.
- No external API calls — no third-party services, cloud providers, or SaaS integrations.
- Authentication is session-based (not JWT) — sessions stored in MySQL; no Redis or distributed cache.
- Attachment storage is local filesystem — no object storage (S3, GCS).
- No external email/SMS delivery — notifications are in-app only.
- AES key must be provisioned at deploy time via `ENCRYPTION_KEY_DIR`.
- CGO must be enabled to run integration tests (SQLite driver requirement).
- All schema changes managed by the `migrate` binary — no manual DDL.
