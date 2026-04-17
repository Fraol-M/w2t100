# Test Coverage Audit

## Scope + Project Type
- Detected type: backend (declared at README top: repo/README.md:1).
- Inference check: repository contains Go backend modules and API tests; no frontend artifact detected via file scan (package.json/frontend source absent).

## Backend Endpoint Inventory
- Total endpoints discovered by static route extraction: 111
- Method distribution:
- DELETE: 9
- GET: 52
- PATCH: 3
- POST: 41
- PUT: 6
- Route composition evidence: repo/internal/app/routes.go:83-176 (central registration), module route files under repo/internal/*/routes.go, and health endpoints in repo/internal/health/handler.go:32-33.

## API Test Mapping Table
| Endpoint | Covered | Test Type | Test Files | Evidence |
|---|---|---|---|---|
| DELETE /api/v1/admin/backups/retention | yes | true no-mock HTTP | admin_backups_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| DELETE /api/v1/admin/data-retention | yes | true no-mock HTTP | admin_pii_retention_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| DELETE /api/v1/analytics/reports/saved/:id | yes | true no-mock HTTP | analytics_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| DELETE /api/v1/governance/keywords/:id | yes | true no-mock HTTP | governance_keywords_riskrules_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| DELETE /api/v1/governance/risk-rules/:id | yes | true no-mock HTTP | governance_keywords_riskrules_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| DELETE /api/v1/properties/:id/staff/:user_id/:role | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| DELETE /api/v1/properties/technicians/:user_id/skills/:tag | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| DELETE /api/v1/users/:id/roles/:role | yes | true no-mock HTTP | users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| DELETE /api/v1/work-orders/attachments/:id | yes | true no-mock HTTP | attachment_flow_test.go, authorization_matrix_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/admin/audit-logs | yes | true no-mock HTTP | admin_audit_logs_test.go, authorization_matrix_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/admin/audit-logs/:id | yes | true no-mock HTTP | admin_audit_logs_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/admin/backups | yes | true no-mock HTTP | admin_backups_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/admin/keys | yes | true no-mock HTTP | admin_settings_keys_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/admin/logs | yes | true no-mock HTTP | admin_audit_logs_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/admin/settings | yes | true no-mock HTTP | admin_settings_keys_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/analytics/funnel | yes | true no-mock HTTP | analytics_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/analytics/popularity | yes | true no-mock HTTP | analytics_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/analytics/quality | yes | true no-mock HTTP | analytics_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/analytics/reports/generated/:id | yes | true no-mock HTTP | analytics_reports_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/analytics/reports/saved | yes | true no-mock HTTP | analytics_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/analytics/reports/saved/:id | yes | true no-mock HTTP | analytics_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/analytics/retention | yes | true no-mock HTTP | analytics_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/analytics/tags | yes | true no-mock HTTP | analytics_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/auth/me | yes | true no-mock HTTP | auth_flow_test.go, authorization_matrix_test.go, governance_flow_test.go, users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/governance/enforcements | yes | true no-mock HTTP | authorization_matrix_test.go, governance_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/governance/keywords | yes | true no-mock HTTP | governance_keywords_riskrules_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/governance/reports | yes | true no-mock HTTP | authorization_matrix_test.go, governance_evidence_test.go, governance_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/governance/reports/:id | yes | true no-mock HTTP | authorization_matrix_test.go, governance_evidence_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/governance/reports/:id/evidence | yes | true no-mock HTTP | governance_evidence_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/governance/risk-rules | yes | true no-mock HTTP | governance_keywords_riskrules_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/governance/risk-rules/:id | yes | true no-mock HTTP | governance_keywords_riskrules_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/notifications | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/notifications/:id | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/notifications/threads | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/notifications/threads/:id | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/notifications/threads/:id/messages | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/notifications/threads/:id/participants | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/notifications/unread-count | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/payments | yes | true no-mock HTTP | authorization_matrix_test.go, payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/payments/:id | yes | true no-mock HTTP | authorization_matrix_test.go, payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/payments/reconciliation | yes | true no-mock HTTP | payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/payments/reconciliation/:id | yes | true no-mock HTTP | payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/properties | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/properties/:id | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/properties/:id/staff | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/properties/:id/units | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/properties/:id/units/:unit_id | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/properties/technicians/:user_id/skills | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/tenants/:id | yes | true no-mock HTTP | tenants_by_property_test.go, users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/tenants/by-property/:property_id | yes | true no-mock HTTP | tenants_by_property_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/tenants/by-user/:user_id | yes | true no-mock HTTP | users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/users | yes | true no-mock HTTP | authorization_matrix_test.go, users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/users/:id | yes | true no-mock HTTP | users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/work-orders | yes | true no-mock HTTP | attachment_flow_test.go, authorization_matrix_test.go, workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/work-orders/:id | yes | true no-mock HTTP | attachment_flow_test.go, authorization_matrix_test.go, workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/work-orders/:id/attachments | yes | true no-mock HTTP | attachment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/work-orders/:id/cost-items | yes | true no-mock HTTP | workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/work-orders/:id/events | yes | true no-mock HTTP | workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /api/v1/work-orders/attachments/:id | yes | true no-mock HTTP | attachment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /health/live | yes | true no-mock HTTP | health_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| GET /health/ready | yes | true no-mock HTTP | health_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PATCH /api/v1/governance/reports/:id/review | yes | true no-mock HTTP | governance_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PATCH /api/v1/notifications/:id/read | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PATCH /api/v1/users/:id/active | yes | true no-mock HTTP | users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/admin/anomaly | yes | true no-mock HTTP | anomaly_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/admin/backups | yes | true no-mock HTTP | admin_backups_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/admin/backups/validate | yes | true no-mock HTTP | admin_backups_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/admin/keys/rotate | yes | true no-mock HTTP | admin_settings_keys_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/admin/pii-export | yes | true no-mock HTTP | admin_pii_retention_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/analytics/export | yes | true no-mock HTTP | analytics_reports_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/analytics/reports/generate/:id | yes | true no-mock HTTP | analytics_reports_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/analytics/reports/saved | yes | true no-mock HTTP | analytics_flow_test.go, analytics_reports_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/auth/login | yes | true no-mock HTTP | auth_flow_test.go, helpers_test.go, users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/auth/logout | yes | true no-mock HTTP | auth_flow_test.go, authorization_matrix_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/governance/enforcements | yes | true no-mock HTTP | authorization_matrix_test.go, governance_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/governance/enforcements/:id/revoke | yes | true no-mock HTTP | authorization_matrix_test.go, governance_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/governance/keywords | yes | true no-mock HTTP | governance_keywords_riskrules_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/governance/reports | yes | true no-mock HTTP | authorization_matrix_test.go, governance_evidence_test.go, governance_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/governance/reports/:id/evidence | yes | true no-mock HTTP | governance_evidence_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/governance/risk-rules | yes | true no-mock HTTP | governance_keywords_riskrules_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/notifications/send | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/notifications/threads | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/notifications/threads/:id/messages | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/notifications/threads/:id/participants | yes | true no-mock HTTP | notifications_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/payments/:id/approve | yes | true no-mock HTTP | payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/payments/:id/makeup | yes | true no-mock HTTP | payment_makeup_settlement_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/payments/:id/mark-paid | yes | true no-mock HTTP | payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/payments/:id/reverse | yes | true no-mock HTTP | payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/payments/intents | yes | true no-mock HTTP | authorization_matrix_test.go, payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/payments/reconciliation/run | yes | true no-mock HTTP | payment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/payments/settlements | yes | true no-mock HTTP | payment_makeup_settlement_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/properties | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/properties/:id/staff | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/properties/:id/units | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/properties/technicians/:user_id/skills | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/tenants | yes | true no-mock HTTP | users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/users | yes | true no-mock HTTP | authorization_matrix_test.go, users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/users/:id/roles | yes | true no-mock HTTP | users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/work-orders | yes | true no-mock HTTP | attachment_flow_test.go, authorization_matrix_test.go, workorder_dispatch_test.go, workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/work-orders/:id/attachments | yes | true no-mock HTTP | attachment_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/work-orders/:id/cost-items | yes | true no-mock HTTP | workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/work-orders/:id/dispatch | yes | true no-mock HTTP | workorder_dispatch_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/work-orders/:id/rate | yes | true no-mock HTTP | workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/work-orders/:id/reassign | yes | true no-mock HTTP | workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| POST /api/v1/work-orders/:id/transition | yes | true no-mock HTTP | workorder_lifecycle_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PUT /api/v1/admin/settings/:key | yes | true no-mock HTTP | admin_settings_keys_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PUT /api/v1/governance/risk-rules/:id | yes | true no-mock HTTP | governance_keywords_riskrules_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PUT /api/v1/properties/:id | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PUT /api/v1/properties/:id/units/:unit_id | yes | true no-mock HTTP | properties_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PUT /api/v1/tenants/:id | yes | true no-mock HTTP | users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |
| PUT /api/v1/users/:id | yes | true no-mock HTTP | users_tenants_flow_test.go | HTTP call via makeRequest/postMultipart in listed integration test(s) |

## API Test Classification
1. True No-Mock HTTP
- repo/test/*_test.go integration suite (full app router via app.RegisterRoutes, real HTTP router.ServeHTTP, real handlers/services/repositories with SQLite/MySQL test DB). Evidence: repo/test/helpers_test.go:66-71, repo/test/helpers_test.go:77-108, repo/test/setup_test.go:44-90.
2. HTTP with Mocking
- repo/internal/users/handler_test.go (HTTP requests against handler routes, but service uses mocked dependencies: newMockRepository, mockAuditLogger, mockEncryptor). Evidence: repo/internal/users/handler_test.go:19-25.
3. Non-HTTP (unit/integration without full API HTTP surface)
- Service/repository/utility tests under repo/internal/** such as users/service_test.go, tenants/service_test.go, payments/service_test.go, notifications/service_test.go, workorders/dispatch_test.go, config/config_test.go, security/crypto_test.go (direct function/service testing).

## Mock Detection
- No jest.mock / vi.mock / sinon.stub patterns present (Go codebase).
- Mocked dependencies detected in backend unit/handler tests:
- repo/internal/users/handler_test.go: newMockRepository, mockAuditLogger, mockEncryptor.
- repo/internal/users/service_test.go: mocked repository + encryptor + audit logger.
- repo/internal/tenants/service_test.go: mockRepository, mockEncryptor, mockAuditLogger.
- repo/internal/properties/service_test.go: mockRepository, mockAuditLogger.
- repo/internal/notifications/service_test.go: mockRepo, mockAuditLogger.
- repo/internal/governance/service_test.go: mockAuditLogger, mockNotifier.
- repo/internal/payments/service_test.go: mockAuditLogger, mockNotifier.
- repo/internal/backups/service_test.go: fakeSecurityService, fakeAuditLogger.
- repo/internal/workorders/dispatch_test.go: mockPropertyQuerier.

## Coverage Summary
- Total endpoints: 111
- Endpoints with HTTP tests: 111
- Endpoints with TRUE no-mock tests: 111
- HTTP coverage: 100% (111/111)
- True API coverage: 100% (111/111)

## Unit Test Summary
### Backend Unit Tests
- Controllers/handlers covered: auth, users, health (repo/internal/auth/handler_test.go, repo/internal/users/handler_test.go, repo/internal/health/handler_test.go).
- Services covered: users, tenants, properties, workorders, payments, governance, notifications, attachments, backups, audit, analytics (repo/internal/*/service_test.go).
- Repositories/data logic covered: analytics DB/repo + payments reconciliation (repo/internal/analytics/db_test.go, repo/internal/payments/reconciliation_test.go).
- Middleware/auth/guards covered: HTTP middleware + auth core (repo/internal/http/middleware_test.go, repo/internal/auth/auth_test.go).
- Important backend modules with limited or missing direct unit coverage:
- Route registration files under repo/internal/*/routes.go (no dedicated direct unit assertions).
- Admin/logs/backups handlers beyond integration behavior are lightly covered at unit-handler level.

### Frontend Unit Tests (Strict Requirement)
- Frontend test files: NONE found.
- Frameworks/tools detected for frontend testing: NONE.
- Frontend components/modules covered: NONE.
- Important frontend components/modules not tested: Not applicable (backend-only repository).
- Frontend unit tests: MISSING (non-critical here because project type is backend, not web/fullstack).

### Cross-Layer Observation
- Backend-only codebase; frontend/backend balance check is not applicable.

## API Observability Check
- Strong observability patterns present in many tests: explicit method/path, explicit request body, and response body assertions (parseResponse, field-level checks), for example repo/test/auth_flow_test.go, repo/test/payment_flow_test.go, repo/test/workorder_lifecycle_test.go.
- Weak spots: some matrix/gating tests focus mainly on status codes with lighter response-content assertions (repo/test/authorization_matrix_test.go, parts of repo/test/analytics_flow_test.go).

## Test Quality & Sufficiency
- Success paths: broad coverage across all domains.
- Failure/negative cases: strong (auth failures, role forbiddance, validation errors, cross-property access limits).
- Edge cases: present in governance/payments/workorder lifecycle flows, but not uniformly deep per endpoint.
- Auth/permissions: strong matrix checks and role gating coverage (repo/test/authorization_matrix_test.go).
- Assertion quality: generally meaningful; a subset remains status-centric (shallower).
- run_test.sh check: Docker-based test execution present (compliant). Evidence: repo/run_test.sh builds deploy/Dockerfile.test and runs tests in containers.

## End-to-End Expectations
- Project type is backend; fullstack FE<->BE E2E requirement is not applicable.

## Tests Check
- Static inspection only performed; no runtime execution performed in this audit.

## Test Coverage Score (0-100)
- 91/100

## Score Rationale
- Full endpoint HTTP and true no-mock API coverage by static evidence.
- Strong role/security and negative-path testing breadth.
- Some test segments are status-only with limited payload/state assertions.
- Unit tests rely heavily on mocks in service layer; some route/handler units are uneven across modules.

## Key Gaps
- Increase deep assertions (response schema/content + persisted side effects) in matrix-style tests.
- Add direct unit-handler coverage for currently integration-only admin/log/backup route behaviors.

## Confidence & Assumptions
- Confidence: High for route inventory and HTTP mapping from static code evidence.
- Assumption: endpoint registration is fully centralized in discovered route files and app.RegisterRoutes.

---

# README Audit

## README Presence
- repo/README.md exists.

## Hard Gate Failures
- None.

## High Priority Issues
- None.

## Medium Priority Issues
- None.

## Low Priority Issues
- Minor encoding artifacts in rendered text (arrow/character substitutions) reduce polish.

## Engineering Quality
- Tech stack clarity: strong.
- Architecture explanation: strong.
- Access method (URL + port): present (localhost:8080).
- Verification method: present (curl health/login/me flow).
- Environment rules: no npm/pip/apt/runtime package installs documented; Docker-contained operational flow is present.
- Security/roles/workflows: well described, including demo credentials for all roles.
- Hard-gate evidence now satisfied:
- Startup command requirement met with explicit `docker-compose up` (repo/README.md:164).
- Demo credentials requirement met with per-role username/password matrix (repo/README.md:290-296).

## README Verdict
- PASS

## Final Verdicts
- Test Coverage Audit Verdict: PASS (with quality gaps).
- README Audit Verdict: PASS.
