# PropertyOps Backend

A production-shaped, Docker-first, offline-first Go backend for multifamily property operations. All features work without internet access. No external services — no SMS, email, push notifications, OAuth, cloud storage, Redis, Kafka, or external payment processors.

## Project Overview

PropertyOps manages the full lifecycle of maintenance work orders, tenant communications, payments, governance, and compliance reporting for multifamily residential properties. The system is designed for single-node, on-premise deployment where network connectivity to the public internet cannot be guaranteed.

**Design constraints:**
- Offline-first: every feature functions without external network access
- Single-node: no clustering, no distributed state, no message queue
- Docker-first: the authoritative run target is Docker Compose
- All notifications are in-app only (no email, SMS, or push delivery)
- All payments are tracked as internal ledger entries (no payment gateway)

## Architecture Summary

| Layer | Technology |
|-------|-----------|
| HTTP framework | Go + Gin |
| ORM | GORM |
| Database | MySQL 8.0 |
| Field encryption | AES-256-GCM (in-process) |
| Password hashing | bcrypt, cost ≥ 12 |
| Sessions | DB-backed opaque tokens |
| Background jobs | In-process goroutine scheduler |
| File storage | Local filesystem |
| Containerization | Docker multi-stage build + Compose |

**5 Roles:** Tenant, Technician, PropertyManager, ComplianceReviewer, SystemAdmin

**9 Domain Modules:** auth, users/tenants/properties, workorders, attachments, notifications, governance, payments, analytics, admin/ops

**31 database tables** across 10 migration files

## Quickstart

```bash
# 1. Copy and edit environment configuration
cp .env.example .env

# 2. Create host-side volume directories and secrets directory
mkdir -p .data/mysql .data/storage .data/backups .data/logs .secrets

# 3. Place AES-256-GCM key file(s) in .secrets/
#    The file must be named <version>.key (32 raw bytes), e.g. 1.key for key version 1.
#    Example: dd if=/dev/urandom of=.secrets/1.key bs=32 count=1

# 4. Build images
docker compose -f deploy/docker-compose.yml build

# 5. Start the database
docker compose -f deploy/docker-compose.yml up -d db

# 6. Run database migrations (one-shot container)
docker compose -f deploy/docker-compose.yml run --rm migrate

# 6b. Seed roles and bootstrap admin account
#     ADMIN_BOOTSTRAP_PASSWORD is required; the seed step is skipped if unset.
ADMIN_BOOTSTRAP_PASSWORD=YourSecurePasswordHere \
  docker compose -f deploy/docker-compose.yml run --rm migrate -seed
# After seeding, log in with username "admin" and the password you set above.
# Change the password immediately on first login.

# 7. Start the API server
docker compose -f deploy/docker-compose.yml up -d api

# 8. Verify health
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
```

## Configuration Reference

All configuration is provided via environment variables. Copy `.env.example` to `.env` before starting.

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_PORT` | `8080` | HTTP listen port |
| `GIN_MODE` | `release` | Gin mode (`release` or `debug`) |
| `DB_HOST` | `db` | MySQL host |
| `DB_PORT` | `3306` | MySQL port |
| `DB_USER` | `propertyops` | MySQL username |
| `DB_PASSWORD` | _(required)_ | MySQL password |
| `DB_NAME` | `propertyops` | MySQL database name |
| `DB_MAX_OPEN_CONNS` | `25` | Max open DB connections |
| `DB_MAX_IDLE_CONNS` | `10` | Max idle DB connections |
| `DB_CONN_MAX_LIFETIME_MINUTES` | `30` | Connection max lifetime |
| `MYSQL_ROOT_PASSWORD` | _(required)_ | MySQL root password (Docker init) |
| `BCRYPT_COST` | `12` | bcrypt hashing cost (minimum 12) |
| `SESSION_IDLE_TIMEOUT_MINUTES` | `30` | Session idle timeout |
| `SESSION_MAX_LIFETIME_HOURS` | `168` | Session absolute maximum lifetime (7 days) |
| `ENCRYPTION_KEY_DIR` | `/run/propertyops/keys` | Directory containing AES-256-GCM key files |
| `ENCRYPTION_ACTIVE_KEY_ID` | `1` | Active key version number |
| `ENCRYPTION_ROTATION_DAYS` | `180` | Days between key rotation reminders |
| `STORAGE_ROOT` | `/var/lib/propertyops/storage` | Root directory for uploaded files and reports |
| `BACKUP_ROOT` | `/var/lib/propertyops/backups` | Root directory for database backups |
| `LOG_ROOT` | `/var/log/propertyops` | Root directory for structured request logs |
| `BACKUP_SCHEDULE_CRON` | `0 2 * * *` | Cron expression (minute hour \* \* \*) controlling daily backup time (UTC); falls back to 24-hour interval if expression is invalid |
| `BACKUP_RETENTION_DAYS` | `30` | Days to retain backup files |
| `BACKUP_ENCRYPTION_ENABLED` | `true` | Encrypt backup files with AES-256-GCM |
| `FINANCIAL_RETENTION_YEARS` | `7` | Minimum retention for financial records |
| `MESSAGE_RETENTION_YEARS` | `2` | Minimum retention for message content |
| `RATE_LIMIT_MAX_SUBMISSIONS_PER_HOUR` | `10` | Max work order submissions per user per hour |
| `NOTIFICATION_POLL_INTERVAL_SECONDS` | `60` | Interval for notification delivery scheduler |
| `NOTIFICATION_MAX_RETRIES` | `3` | Max delivery retries per notification |
| `NOTIFICATION_RETRY_DELAY_SECONDS` | `300` | Delay between retry attempts |
| `PAYMENT_INTENT_EXPIRY_MINUTES` | `30` | Minutes before a pending payment intent expires |
| `PAYMENT_DUAL_APPROVAL_THRESHOLD` | `500.00` | USD amount requiring dual approval |
| `REPORT_SCHEDULE_POLL_INTERVAL_SECONDS` | `300` | Interval for scheduled report generation check |
| `ANOMALY_ALLOWED_CIDRS` | `127.0.0.1/32,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16` | Allowed CIDRs for anomaly ingestion endpoint |
| `DEFAULT_TIMEZONE` | `America/New_York` | Default property timezone |

## Route Group Summary

| Group | Base Path | Description |
|-------|-----------|-------------|
| Auth | `/api/v1/auth` | Login (public), logout, current user |
| Users | `/api/v1/users` | User management (admin) and self-service profile |
| Tenants | `/api/v1/tenants` | Tenant profile management |
| Properties | `/api/v1/properties` | Property and unit management |
| Work Orders | `/api/v1/work-orders` | Full work order lifecycle + attachments |
| Notifications | `/api/v1/notifications` | In-app notifications and message threads |
| Governance | `/api/v1/governance` | Reports, enforcements, blacklist, risk rules |
| Payments | `/api/v1/payments` | Intents, settlements, reversals, reconciliation |
| Analytics | `/api/v1/analytics` | Metrics, saved reports, CSV export |
| Admin | `/api/v1/admin` | Audit logs, log query, backups, keys, settings, PII export, anomaly ingestion |
| Health | `/health/live`, `/health/ready` | Liveness and readiness probes |

## Running Tests

```bash
go test ./...
```

Run a specific package:

```bash
go test ./internal/workorders/... -v
go test ./internal/auth/... -v
go test ./internal/payments/... -v
```

## Docker Commands

```bash
# Build images
docker compose -f deploy/docker-compose.yml build

# Start all services
docker compose -f deploy/docker-compose.yml up -d

# Start only the database
docker compose -f deploy/docker-compose.yml up -d db

# Run migrations (one-shot, exits when done)
docker compose -f deploy/docker-compose.yml run --rm migrate

# Start only the API server
docker compose -f deploy/docker-compose.yml up -d api

# View API logs
docker compose -f deploy/docker-compose.yml logs -f api

# View all service logs
docker compose -f deploy/docker-compose.yml logs -f

# Check running containers
docker compose -f deploy/docker-compose.yml ps

# Stop all services
docker compose -f deploy/docker-compose.yml down

# Stop and remove volumes (destructive — deletes all data)
docker compose -f deploy/docker-compose.yml down -v
```

## Storage and Volume Layout

Host-side directories are bind-mounted into containers:

```
.data/
  mysql/          → /var/lib/mysql (MySQL data files)
  storage/        → /var/lib/propertyops/storage (uploaded attachments, reports)
    attachments/  → work order attachment files
    reports/      → generated CSV report files
  backups/        → /var/lib/propertyops/backups (encrypted database backups)
  logs/           → /var/log/propertyops (structured request logs)

.secrets/         → /run/propertyops/keys (AES-256-GCM key files, read-only mount)
  1.key           → 32-byte key for key version 1 (format: <version>.key)
```

The `.secrets/` directory is mounted read-only (`ro`) into all containers that need encryption/decryption.

## Manual Verification

After starting the stack, verify the system is operational:

```bash
# Liveness probe
curl -s http://localhost:8080/health/live | jq .

# Readiness probe (checks DB, storage, keys, backup dir)
curl -s http://localhost:8080/health/ready | jq .

# Login and obtain a session token
curl -s -X POST http://localhost:8080/api/v1/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"yourpassword"}' | jq .

# Use the returned token for authenticated requests
export TOKEN="<token from login response>"

curl -s http://localhost:8080/api/v1/auth/me \
  -H "Authorization: Bearer $TOKEN" | jq .
```

See `docs/manual-verification.md` for the complete step-by-step verification guide.
