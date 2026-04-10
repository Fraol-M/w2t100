# Previous-Issue Fix Verification (Static)

Date: 2026-04-10
Scope: Static code review only (no runtime execution, no Docker, no test execution)
Reference baseline: previous report in .tmp/audit_report-2.md

## Overall Result
- Total issues reviewed: 5
- Fixed: 5
- Partially fixed: 0
- Not fixed: 0

## Issue-by-Issue Status

### 1) PropertyManager can perform technician operational transitions
- Previous severity: High
- Status: Fixed
- Verification:
  - Transition authorization now explicitly restricts PropertyManager to manager-owned transitions and blocks technician-only transitions.
- Evidence:
  - internal/workorders/handler.go:552
  - internal/workorders/handler.go:573
  - internal/workorders/handler.go:576

### 2) Analytics test evidence too shallow for core business metrics
- Previous severity: High
- Status: Fixed
- Verification:
  - New DB-backed analytics tests now cover repository metric logic (popularity, funnel, quality, retention, saved-report CRUD).
  - New analytics API integration tests exist for role gating and saved-report flow.
- Evidence:
  - internal/analytics/db_test.go:60
  - internal/analytics/db_test.go:115
  - internal/analytics/db_test.go:170
  - internal/analytics/db_test.go:206
  - internal/analytics/db_test.go:267
  - test/analytics_flow_test.go:35
  - test/analytics_flow_test.go:174

### 3) Governance rate-limit window modeled but not enforced
- Previous severity: Medium
- Status: Fixed
- Verification:
  - Middleware rate-limit logic now reads both rate_limit_max and rate_limit_window_minutes from active enforcement and uses dynamic windowStart.
- Evidence:
  - internal/http/middleware.go:263
  - internal/http/middleware.go:277
  - internal/http/middleware.go:282
  - internal/governance/service.go:250
  - internal/governance/service.go:287

### 4) Backup schedule config accepted but not used for timing
- Previous severity: Medium
- Status: Fixed
- Verification:
  - Runtime scheduler now uses cron-based backup scheduling via cfg.Backup.ScheduleCron and parseDailyCron.
  - README now documents BACKUP_SCHEDULE_CRON as an active cron expression controlling daily backup time, matching runtime behavior.
- Evidence:
  - internal/app/scheduler.go:58
  - internal/app/scheduler.go:96
  - internal/config/config.go:137
  - README.md:99

### 5) Integration-test fidelity limited vs MySQL runtime behavior
- Previous severity: Medium
- Status: Fixed
- Verification:
  - Test harness now supports a real MySQL 8.0 execution path through TEST_MYSQL_DSN, with dedicated setup/cleanup for full-fidelity behavior.
  - Repository entrypoints now include a dedicated test-mysql target that validates TEST_MYSQL_DSN and runs integration suites against MySQL.
- Evidence:
  - test/setup_test.go:26
  - test/setup_test.go:47
  - test/setup_test.go:73
  - Makefile:26
  - Makefile:32

## Conclusion
- The highest-priority functional/security issues from the previous report (workorder transition role boundary, analytics test depth, and governance rate-limit window enforcement) are fixed.
- The backup scheduling consistency issue is fixed in both runtime and documentation.
- MySQL-fidelity integration coverage now has a first-class repository entrypoint and dedicated test setup path.
