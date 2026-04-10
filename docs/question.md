# Business Logic Questions Log — PropertyOps

---

## 1. Session Token Format and Lifetime

**Question:** The specification requires session-based authentication but does not define the token format, idle timeout, or absolute maximum session age.

**Assumption:** A 32-byte cryptographically random opaque token is sufficient. A 30-minute idle timeout with a 7-day hard maximum balances usability and security.

**Solution:** `crypto/rand.Read(32 bytes)` generates each session token. `auth.Session` stores `token_hash`, `created_at`, and `last_active_at`. `AuthMiddleware` checks `time.Since(lastActiveAt) > idleTimeout` and `time.Since(createdAt) > maxLifetime`, then refreshes `last_active_at` on valid requests. Both values are configurable via `SESSION_IDLE_TIMEOUT_MINUTES` and `SESSION_MAX_LIFETIME_HOURS`.

---

## 2. Bcrypt Cost Minimum

**Question:** The specification mentions bcrypt password hashing but does not define a minimum cost factor.

**Assumption:** Cost 12 is the accepted minimum for production use; cost 4 is used in tests to keep test suites fast.

**Solution:** Config validation enforces `BCRYPT_COST >= 12` at startup. `hashPassword` in test helpers explicitly passes `bcrypt.MinCost` (4). Production `users.Service` uses the configured cost.

---

## 3. Default Role on Registration

**Question:** The specification defines five roles but does not specify which role a new user receives when created.

**Assumption:** Users do not receive any role automatically. SystemAdmin explicitly assigns roles after account creation. This prevents accidental elevation.

**Solution:** `POST /api/v1/users` accepts an optional `role_names` array. If omitted, the user has no roles and can only reach public and unauthenticated endpoints (which effectively means they cannot do anything useful until a role is assigned). `POST /api/v1/users/:id/roles` is the role assignment endpoint, restricted to SystemAdmin.

---

## 4. Work Order Description Length

**Question:** The specification requires text descriptions on work orders but does not define minimum or maximum lengths.

**Assumption:** A minimum of 20 characters eliminates trivially short submissions ("broken", "fix it"). A maximum of 2000 characters is sufficient for detailed maintenance descriptions.

**Solution:** `common.ValidateStringLength("description", value, 20, 2000)` is called in `workorders.Service.Create`. The same 20–2000 range is applied to governance report descriptions.

---

## 5. SLA Computation — Priority Tiers and Business Days

**Question:** The specification mentions SLA enforcement but does not define the time thresholds per priority or how "business days" is computed for the Low priority tier.

**Assumption:** Emergency (4h) and High (24h) are standard incident response thresholds. Normal (72h) is 3 calendar days. Low (5 business days) skips weekends using a weekday iteration.

**Solution:** `workorders.Service.computeSLADueAt` switches on priority:
- Emergency: `now + 4h`
- High: `now + 24h`
- Normal: `now + 72h`
- Low: iterates from `now`, incrementing by 24h and skipping Saturday/Sunday until 5 weekdays have elapsed.

`sla_due_at` is set immediately on work order creation. A background scheduler flags breaches.

---

## 6. Auto-Dispatch — Trigger Condition

**Question:** The specification mentions automatic technician dispatch but does not specify when it fires or what happens if no match is found.

**Assumption:** Auto-dispatch fires only when the work order includes a `skill_tag`. Without a skill tag, the work order remains in `New` status and must be dispatched manually by a PropertyManager. This avoids arbitrary random assignment.

**Solution:** `workorders.Service.Create` calls `dispatchWorkOrder(wo)` after creation only if `wo.SkillTag != ""`. If the technician list for `(property_id, skill_tag)` is empty, dispatch is skipped and the work order remains `New`. PropertyManagers dispatch manually via `POST /:id/dispatch`.

---

## 7. Round-Robin Dispatch — Cursor Persistence

**Question:** The specification requires "deterministic round-robin dispatch" but does not define how the cursor position is stored or reset.

**Assumption:** Per-`(property_id, skill_tag)` cursors are the correct granularity: different skills at the same property are dispatched independently.

**Solution:** `dispatch_cursors` table has a UNIQUE constraint on `(property_id, skill_tag)`. `GetDispatchCursor` returns 0 if no row exists. `UpdateDispatchCursor` uses UPSERT. Cursor wraps automatically via modulo (`cursor % len(technicians)`). There is no reset — the cursor increments monotonically.

---

## 8. Manual Dispatch Endpoint

**Question:** If auto-dispatch covers the common case, why is a separate `POST /:id/dispatch` endpoint needed?

**Assumption:** Work orders without a `skill_tag` (e.g., "general" requests) will never be auto-dispatched. PropertyManagers need an API path to manually assign a technician to such work orders.

**Solution:** `POST /api/v1/work-orders/:id/dispatch` accepts `{ "technician_id": uint64, "reason": string }`. It is permitted only for work orders in `New` status and only for PropertyManagers who manage the work order's property. It triggers the same `New → Assigned` transition as auto-dispatch, creating a `work_order_events` row.

---

## 9. Reassignment Reason Length

**Question:** The specification requires a reason for reassignment but does not define length constraints.

**Assumption:** A minimum of 10 characters ensures the reason is substantive. A maximum of 500 characters is sufficient without being overly restrictive.

**Solution:** `common.ValidateStringLength("reason", req.Reason, 10, 500)` is applied in `workorders.Service.Reassign`.

---

## 10. Cost Item Responsibility Attribution

**Question:** The specification requires cost item tracking but does not define who can be attributed financial responsibility.

**Assumption:** Three parties cover the meaningful attribution options: `Tenant` (damage caused by resident), `Vendor` (supplier defect), `Property` (owner's maintenance obligation).

**Solution:** `CostItem.Responsibility` is an enumerated string constrained to `Tenant | Vendor | Property`. Service-layer validation rejects any other value.

---

## 11. Post-Completion Rating Scale

**Question:** The specification mentions tenant ratings but does not define the scale or feedback constraints.

**Assumption:** A 1–5 integer scale is universally understood. Feedback up to 1000 characters covers detailed comments without becoming a general text field.

**Solution:** `RateWorkOrderRequest.Rating` is validated as 1 ≤ rating ≤ 5. `RateWorkOrderRequest.Feedback` is validated as ≤ 1000 characters. Rating is only accepted for work orders in `Completed` status and only from the tenant who created the work order.

---

## 12. Attachment Format Restrictions

**Question:** The specification requires file attachments for work orders but does not define allowed file types or size limits.

**Assumption:** Restricting to JPEG and PNG covers the vast majority of maintenance photos. 5 MB per file prevents abuse without excluding typical phone camera images. 6 attachments per work order provides ample evidence without creating storage bloat.

**Solution:** `attachments.Service.Upload` validates:
1. File size ≤ 5 MB.
2. Declared MIME type is `image/jpeg` or `image/png`.
3. Magic bytes match the declared MIME type (prevents MIME spoofing).
4. Current attachment count + 1 ≤ 6.

SHA-256 hash is computed and stored for integrity verification.

---

## 13. Magic Byte Validation

**Question:** Relying solely on the `Content-Type` header for MIME validation is insecure. How is this mitigated?

**Assumption:** File signatures (magic bytes) are the authoritative source of MIME type. The declared `Content-Type` must match the detected signature. A mismatch is rejected, preventing clients from uploading arbitrary files with a falsified header.

**Solution:** `common.ValidateFileSignature(header[:512])` reads the first 512 bytes and detects JPEG (`\xFF\xD8\xFF`) and PNG (`\x89PNG`) signatures. `attachments.Service.Upload` then compares the detected MIME against the declared `Content-Type`. A mismatch returns a 422 validation error.

---

## 14. Multipart Work Order Atomicity

**Question:** What happens if a work order is created successfully but a subsequent inline attachment fails validation?

**Assumption:** A partially-created resource (work order exists, some attachments missing) is worse than no resource at all. The client should be able to retry with a corrected payload.

**Solution:** The `workorders.Handler.Create` handler iterates over uploaded files sequentially. On any failure, it calls:
1. `h.fileUpload.DeleteWorkOrderAttachments(wo.ID)` — removes all already-written attachment DB records and disk files.
2. `h.service.Delete(wo.ID)` — removes the work order row.

The client receives a 422 with no partial resource created. The `AttachmentUploader` interface is defined in the `workorders` package (not `attachments`) to avoid an import cycle; the concrete `attachmentUploaderAdapter` in `app/routes.go` bridges the two packages.

---

## 15. Payment Intent Expiry

**Question:** The specification mentions payment intents but does not define how long they remain valid.

**Assumption:** 30 minutes is a standard intent window — long enough to complete a transaction flow, short enough to prevent stale records accumulating.

**Solution:** `expiresAt = now + PaymentIntentExpiryMinutes` is set at creation. `payments.Service.ExpireStaleIntents()` is called by the background scheduler; it finds all `Pending` intents past their expiry and marks them `Expired`. `MarkPaid` also checks expiry before accepting a payment.

---

## 16. Dual-Approval Threshold

**Question:** The specification requires dual approval for high-value payments but does not define the threshold.

**Assumption:** $500 is a reasonable default for property management contexts. The threshold should be configurable to accommodate different property sizes.

**Solution:** `PAYMENT_DUAL_APPROVAL_THRESHOLD` defaults to 500.00. The service reads `s.cfg.Payment.DualApprovalThreshold`. Reversal and makeup operations also check this threshold against the original payment amount. The threshold is applied consistently across approval, reversal, and makeup paths.

---

## 17. Dual-Approval — Same Approver Prevention

**Question:** The specification requires "two approvals" but does not specify whether the same person can approve twice.

**Assumption:** Two approvals from the same person defeat the purpose of dual approval. The second approval must come from a different user.

**Solution:** `payments.Service.ApprovePayment` iterates existing approvals and returns 409 Conflict if the current `approverID` matches any prior approval's `ApproverID`. The check happens before creating the new approval record.

---

## 18. Reconciliation Expected Total — Paid vs Settled

**Question:** After `ApprovePayment` transitions an intent to `Settled`, should the expected total include `Settled` intents or only `Paid` ones?

**Assumption:** After dual approval, the intent status changes from `Paid` to `Settled`. Counting only `Paid` would systematically undercount expected revenue for every fully-approved payment. Both `Paid` and `Settled` represent committed obligations.

**Solution:** Reconciliation's expected-total loop checks `p.Kind == "Intent" && (p.Status == "Paid" || p.Status == "Settled")`. This ensures the discrepancy calculation is accurate regardless of whether payments have been approved or not.

---

## 19. Governance Report Categories — Enumerated vs Free-Form

**Question:** The specification mentions "category-based reports" but does not define whether categories are a fixed enum or free-form text.

**Assumption:** Fixed categories prevent inconsistent classification and make analytics filtering tractable. Six categories cover the meaningful incident types for residential property management.

**Solution:** `CreateReportRequest.Category` is validated against: `Harassment | Damage | Noise | Maintenance | Fraud | Other`. Any other value returns a 422 validation error. The `Other` category provides an escape hatch for edge cases.

---

## 20. Enforcement Duration Encoding

**Question:** The specification mentions suspension durations (1 day, 7 days, indefinite) but does not specify how they are represented.

**Assumption:** String literals (`"1day"`, `"7day"`, `"indefinite"`) are more readable in API payloads than raw timestamps or integer counts.

**Solution:** `CreateEnforcementRequest.EndsAt` is an optional string. Service logic maps `"1day"` → `now + 24h`, `"7day"` → `now + 168h`, `"indefinite"` → nil (no end date). Stored as a nullable `DATETIME` in `enforcement_actions.ends_at`. `CheckSuspension` middleware compares `now` against `ends_at` (nil = never expires).

---

## 21. Suspension Check — Middleware vs Service Layer

**Question:** Should suspension checks happen in each service method or centrally?

**Assumption:** Centralizing the check in middleware eliminates the risk of forgetting it in any one service. All protected routes are covered by a single enforcement point.

**Solution:** `CheckSuspension` middleware runs after `AuthMiddleware` on all protected routes. It queries `enforcement_actions WHERE user_id = ? AND action_type = 'Suspension' AND is_active = true AND (ends_at IS NULL OR ends_at > now)`. If any row exists, it returns 403 immediately without reaching the handler.

---

## 22. PropertyManager Scope — Empty-List Bypass Prevention

**Question:** If a PropertyManager has no active property assignments, should they see all records or zero records on list endpoints?

**Assumption:** A PM with zero assignments should see zero records. Falling through to an unfiltered query (returning all records) would be a critical authorization leak.

**Solution:** `WorkOrderListRequest.ScopedToPropertyIDs bool` and `ListPaymentsRequest.ScopedToPropertyIDs bool` are set to `true` when a PM's scope is resolved. Repository list methods check this flag first: if `ScopedToPropertyIDs && len(PropertyIDs) == 0`, they return an empty result immediately without executing a query. This invariant is enforced in both `workorders.Repository.List` and `payments.Repository.List`.

---

## 23. PropertyManager Scope — Role Column in Assignments

**Question:** `property_staff_assignments` stores both PMs and Technicians. How are PM-scope queries prevented from including Technicians?

**Assumption:** The `role` column in `property_staff_assignments` must be included in every scope query. Without it, a Technician assigned to a property would be treated as managing it.

**Solution:** All `GetManagedPropertyIDs` and `IsManagedBy` queries include `AND role = 'PropertyManager' AND is_active = true`. This is enforced in `workorders.Repository`, `payments.Service`, and `tenants.Repository`. The `PropertyStaffAssignment` model includes `Role string` and `IsActive bool` with `gorm:"default:true"`.

---

## 24. ComplianceReviewer Scope — Global vs Property-Scoped

**Question:** Should ComplianceReviewers be scoped to specific properties like PropertyManagers?

**Assumption:** Compliance review is an oversight function that must be able to see incidents across all properties. Scoping it to individual properties would prevent cross-property pattern detection.

**Solution:** `ComplianceReviewer` has unconditional read access to governance reports, enforcement actions, and attachments. There is no property-scope filter applied for this role.

---

## 25. Notification Delivery — In-App Only

**Question:** The specification mentions notifications but does not define delivery channels.

**Assumption:** In-app delivery is the only channel required for an offline-first system. External channels (email, SMS) introduce internet dependencies incompatible with the offline-first constraint.

**Solution:** `NotificationStatus` follows: `Pending → Sent | Failed → (retry loop)`. Background scheduler delivers pending notifications (sets `sent_at`). Failed deliveries retry up to `NOTIFICATION_MAX_RETRIES` times. After exhausting retries, status becomes `Failed` permanently.

---

## 26. Thread Work Order Access — Handler vs Service Layer

**Question:** Should the work order access check for thread creation happen in the handler or the service?

**Assumption:** Access logic in the handler creates a dependency on the WO package from the notifications handler, risking import cycles. Moving it to the service layer keeps the check co-located with the business rule and allows the handler to stay thin.

**Solution:** `notifications.Service` defines the `WorkOrderChecker` interface. `WithWorkOrderChecker(wc WorkOrderChecker)` injects the implementation. `CreateThread` accepts `roles []string` and calls `checkWOAccess(woID, creatorID, roles)` internally. The handler passes roles from the Gin context. The handler never imports the workorders package.

---

## 27. Attachment Delete Authorization — PM Property Scope

**Question:** The original delete authorization checked only "is the actor a PropertyManager?" without checking which property they manage. Could a PM delete attachments from properties they do not manage?

**Assumption:** Delete authorization must apply the same property-scope rules as upload, download, and list. A PM managing property A must not be able to delete attachments from property B's work orders.

**Solution:** `attachments.Service.Delete` now routes through `canAccessWorkOrder(attachment.EntityID, actorID, roles)` for work-order attachments. `canAccessWorkOrder` already enforces property scope for PM. SystemAdmin is the only role with unconditional delete access, checked before the property-scope branch.

---

## 28. Reconciliation — Reversal Without Related Payment

**Question:** A reversal always references an original payment. Should orphaned reversals be flagged?

**Assumption:** A reversal with no `related_payment_id` is a data integrity anomaly that should be surfaced in the reconciliation report.

**Solution:** After computing totals, the reconciliation loop checks each `Reversal` payment: if `p.RelatedPaymentID == nil`, a discrepancy entry is created with the issue "Reversal without related payment". This increments `discrepancy_count` and appears in the CSV statement.

---

## 29. Anomaly Ingestion — No Bearer Token Required

**Question:** The anomaly ingestion endpoint needs to receive automated alerts from local monitoring systems, which may not have session tokens. How is it secured?

**Assumption:** Local network allowlist is a sufficient security boundary for an endpoint that receives (not sends) data and runs entirely within the local Docker network.

**Solution:** The anomaly route is registered before the `Authenticate()` middleware on the `/api/v1/admin` group. A CIDR check middleware validates the client IP against `ANOMALY_ALLOWED_CIDRS` (default: RFC-1918 private ranges + loopback). Requests from non-allowed IPs receive 403. Ingested data is stored and audit-logged.

---

## 30. PII Export — Purpose Field Requirement

**Question:** PII export gives access to unmasked sensitive data. What additional controls are needed beyond the SystemAdmin role check?

**Assumption:** Requiring an explicit stated purpose creates an accountability record that can be reviewed. This satisfies basic data governance requirements without implementing a separate approval workflow.

**Solution:** `POST /api/v1/admin/pii-export` requires a non-empty `purpose` field. The purpose is written to the audit log alongside the actor ID, IP, and request ID. Without a stated purpose, the request returns a 422 validation error.

---

## 31. Rate Limiting — DB-Backed vs In-Memory

**Question:** Should rate limiting use an in-memory store (faster) or a DB-backed store (durable)?

**Assumption:** In-memory rate limiting resets on process restart, allowing abuse through restarts. DB-backed rate limiting survives restarts and is more appropriate for a single-node deployment without a Redis instance.

**Solution:** Rate limit records are stored in MySQL. The `RateLimitMiddleware` queries and increments within a transaction. This is slightly slower than in-memory but acceptable for the submission frequency involved (work order creation, governance reports). The per-hour window is configurable.

---

## 32. Admin Bootstrap — Seed Mechanism

**Question:** How are the initial roles and admin user created in a fresh deployment?

**Assumption:** A separate seed step, explicitly invoked, is safer than running seeding automatically at startup (which could cause problems on restarts or upgrades).

**Solution:** The `cmd/migrate` binary accepts a `-seed` flag. When passed, `db/seeds/seed.go` creates the five standard roles and an `admin` user with the password from `ADMIN_BOOTSTRAP_PASSWORD`. The seed is idempotent — it uses `FirstOrCreate` so re-running does not create duplicates.

---

## 33. Pagination — Maximum Page Size

**Question:** The specification mentions pagination but does not define a maximum page size that prevents accidental full-table scans.

**Assumption:** A hard cap of 100 rows per page prevents clients from loading excessive data in a single request while still being practical for most list views.

**Solution:** All list handlers cap `per_page` at 100. The common helper `common.PaginationFromQuery` enforces this cap. Default `per_page` is 20. Default `page` is 1.

---

## 34. Attachment Storage — Local Filesystem Path Structure

**Question:** The specification requires attachment storage but does not define the directory layout.

**Assumption:** Organizing by work order UUID prevents any single directory from growing excessively large and makes per-work-order cleanup straightforward.

**Solution:** Attachment storage path: `{STORAGE_ROOT}/attachments/{wo_uuid}/{attach_uuid}{ext}`. `DeleteForWorkOrder` uses `FindByEntity("WorkOrder", woID)` to locate all records and removes both the DB row and the file from disk.

---

## 35. Technician Skill Tags — Many-to-Many via Repository

**Question:** Skill tag matching is needed at dispatch time. How does the query work across three tables?

**Assumption:** A direct join between `property_staff_assignments` and `technician_skill_tags` on `user_id` is sufficient and avoids a separate join table.

**Solution:** `properties.Repository.FindTechniciansByPropertyAndSkill(propertyID, skillTag)` joins `property_staff_assignments` (where `role = 'Technician'` and `is_active = true`) with `technician_skill_tags` (where `tag = skillTag`). Returns `[]PropertyStaffAssignment` with `UserID` values used by the dispatch algorithm.

---

## 36. Audit Log — Which Events Are Logged

**Question:** The specification requires an audit log but does not enumerate which specific operations should be logged.

**Assumption:** Every state-changing operation on domain resources should be audit-logged. Read-only queries are not logged (too verbose; no security value). Authentication events (login, logout) are logged.

**Solution:** `AuditService.Log(actorID, action, resourceType, resourceID, description, ip, requestID)` is called from every service method that creates, updates, deletes, or changes the status of a domain resource. Included: work order transitions, payment approvals, enforcement actions, attachment uploads/deletes, thread creation, message posting, notification reads, backup operations, key rotation, PII exports.

---

## 37. Structured Logging — PII Handling

**Question:** The specification requires structured logging but does not specify how PII is handled in log output.

**Assumption:** PII must not appear in log files in plain text. Masking or omitting PII from log fields is sufficient for an offline-first local system without a formal GDPR obligation.

**Solution:** The `logs.Service` writes structured JSON logs. Sensitive fields (usernames, email addresses, phone numbers) are masked before being written. Request bodies are not logged. The audit log stores resource IDs, not field values.

---

## 38. Health Check — Readiness Criteria

**Question:** The specification requires health endpoints but does not define what "ready" means.

**Assumption:** The four critical subsystems are: database connectivity, storage root accessibility, active encryption key availability, and backup directory writability.

**Solution:** `GET /health/ready` runs four concurrent checks:
- `db`: attempts a `db.Exec("SELECT 1")`.
- `storage`: verifies `STORAGE_ROOT` is a writable directory.
- `keys`: verifies at least one key file exists in `ENCRYPTION_KEY_DIR`.
- `backup_dir`: verifies `BACKUP_ROOT` is a writable directory.

Any failing check sets the overall status to `"error"` and returns HTTP 503.

---

## 39. Work Order Events — Append-Only Guarantee

**Question:** The specification requires an audit trail for work order changes. Can events be modified or deleted?

**Assumption:** Mutability would defeat the purpose of an audit trail. Events must be append-only.

**Solution:** `work_order_events` has no `UpdatedAt` or `DeletedAt` column. The `WorkOrderEvent` struct does not implement any update or delete methods in its repository. There is no handler or service method that modifies or deletes events — only `CreateEvent`.

---

## 40. Reconciliation CSV — Output Location

**Question:** Where should reconciliation CSV statements be written, and what happens if the write fails?

**Assumption:** Writing to local storage is consistent with the offline-first constraint. A failed write should not prevent the reconciliation run from completing and saving its summary — CSV generation is a non-fatal secondary output.

**Solution:** CSV is written to `{STORAGE_ROOT}/reconciliation/YYYY-MM-DD.csv`. If `os.MkdirAll` or file creation fails, `GenerateStatement` returns an error that is caught by `RunDaily`. The reconciliation run continues and saves its numeric summary; `statement_file_path` is left nil in the `ReconciliationRun` record. The run status is still set to `Completed`.
