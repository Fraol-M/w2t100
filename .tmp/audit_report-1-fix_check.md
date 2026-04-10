# Previous Inspection Issues - Fix Verification (Static)

Date: 2026-04-10
Method: static source review only (no runtime execution)

## Summary
- Verified issues from prior inspection: 11
- Fixed: 8
- Partially fixed / still risky: 0
- Not fixed: 3

## Verification Matrix

| # | Previous issue | Current status | Evidence |
|---|---|---|---|
| 1 | Tenant can create work orders for arbitrary properties/units | Fixed | `repo/internal/workorders/service.go:114`, `repo/internal/workorders/service.go:119`, `repo/internal/workorders/service.go:124` |
| 2 | Backup may silently degrade to metadata-only export | Fixed | `repo/internal/backups/service.go:305`, `repo/internal/backups/service.go:306`, `repo/internal/backups/service.go:308` |
| 3 | Notification retry semantics under-implemented | Fixed | `repo/internal/app/scheduler.go:98`, `repo/internal/app/scheduler.go:118`, `repo/internal/app/scheduler.go:128`, `repo/internal/app/scheduler.go:130` |
| 4 | Analytics tag search-term logic only used `skill_tag` | Fixed | `repo/internal/analytics/repository.go:139`, `repo/internal/analytics/repository.go:140`, `repo/internal/analytics/repository.go:154`, `repo/internal/analytics/repository.go:173` |
| 5 | Manual verification guide had stale backup/health expectations | Fixed | `repo/docs/manual-verification.md:31`, `repo/docs/manual-verification.md:111`, `repo/internal/backups/routes.go:20`, `repo/internal/health/handler.go:39` |
| 6 | Governance integration test category mismatch (`Safety`) | Fixed | No `Safety` usage found in `repo/test/governance_flow_test.go`; valid categories used at `repo/test/governance_flow_test.go:45`, `repo/test/governance_flow_test.go:82`, `repo/test/governance_flow_test.go:104`, `repo/test/governance_flow_test.go:409` |
| 7 | Local-network allowlist depended on `ClientIP` without trusted proxy hardening | Fixed | `repo/internal/app/app.go:50`, `repo/internal/http/middleware.go:310` |
| 8 | Images not integrated into initial work-order submission flow | Not fixed | `repo/internal/workorders/dto.go:9`, `repo/internal/workorders/dto.go:16`, separate attachments flow still exists (`repo/internal/attachments/routes.go`) |
| 9 | Manual docs health response mismatch | Fixed | `repo/docs/manual-verification.md:31`, `repo/internal/health/handler.go:39` |
| 10 | No integration coverage for anomaly endpoint | Not fixed | Search in `repo/test/**/*.go` shows only anomaly config reference in `repo/test/helpers_test.go:55`; no endpoint tests found |
| 11 | No integration coverage for attachment endpoints | Not fixed | Search in `repo/test/**/*.go` for `attachments` and `/attachments` returns no matches |

## Notes
- The trusted-proxy concern is materially improved because `SetTrustedProxies` is now explicitly configured in app startup (`repo/internal/app/app.go:50`).
- Notification retry now enforces max retries and increments retry counters on delivery update failure in scheduler path.
- Work-order create now includes tenant-property and unit-property validation guardrails.

## Additional observation (new)
- Resolved: the governance test now uses a valid category (`Damage`) at `repo/test/governance_flow_test.go:375`, and `Vandalism` remains only in description text.

## Conclusion
Most previously reported defects are now fixed. Remaining material gaps are:
1. image-in-create workflow semantics (still split across work-order create + attachment upload),
2. missing integration tests for anomaly endpoint protection,
3. missing integration tests for attachment API flows.

Update note (2026-04-10 re-check): the previously flagged risky governance-category observation is now fixed.