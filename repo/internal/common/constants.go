package common

// Role names — must match seeded role data.
const (
	RoleTenant             = "Tenant"
	RoleTechnician         = "Technician"
	RolePropertyManager    = "PropertyManager"
	RoleComplianceReviewer = "ComplianceReviewer"
	RoleSystemAdmin        = "SystemAdmin"
)

// Work order statuses — ordered lifecycle.
const (
	WOStatusNew              = "New"
	WOStatusAssigned         = "Assigned"
	WOStatusInProgress       = "InProgress"
	WOStatusAwaitingApproval = "AwaitingApproval"
	WOStatusCompleted        = "Completed"
	WOStatusArchived         = "Archived"
)

// Work order priority levels.
const (
	PriorityLow       = "Low"
	PriorityNormal    = "Normal"
	PriorityHigh      = "High"
	PriorityEmergency = "Emergency"
)

// Cost item types.
const (
	CostTypeLabor    = "Labor"
	CostTypeMaterial = "Material"
)

// Cost responsibility attribution.
const (
	ResponsibilityTenant   = "Tenant"
	ResponsibilityVendor   = "Vendor"
	ResponsibilityProperty = "Property"
)

// Report target types.
const (
	ReportTargetTenant    = "Tenant"
	ReportTargetWorkOrder = "WorkOrder"
	ReportTargetThread    = "Thread"
)

// Report statuses.
const (
	ReportStatusOpen      = "Open"
	ReportStatusInReview  = "InReview"
	ReportStatusResolved  = "Resolved"
	ReportStatusDismissed = "Dismissed"
)

// Enforcement action types.
const (
	EnforcementWarning    = "Warning"
	EnforcementRateLimit  = "RateLimit"
	EnforcementSuspension = "Suspension"
)

// Payment kinds.
const (
	PaymentKindIntent           = "Intent"
	PaymentKindSettlementPosting = "SettlementPosting"
	PaymentKindMakeupPosting    = "MakeupPosting"
	PaymentKindReversal         = "Reversal"
)

// Payment statuses.
const (
	PaymentStatusPending  = "Pending"
	PaymentStatusPaid     = "Paid"
	PaymentStatusExpired  = "Expired"
	PaymentStatusReversed = "Reversed"
	PaymentStatusSettled  = "Settled"
)

// Notification statuses.
const (
	NotificationStatusPending   = "Pending"
	NotificationStatusSent      = "Sent"
	NotificationStatusFailed    = "Failed"
	NotificationStatusCancelled = "Cancelled"
)

// Unit statuses.
const (
	UnitStatusVacant      = "Vacant"
	UnitStatusOccupied    = "Occupied"
	UnitStatusMaintenance = "Maintenance"
)

// Audit log action names.
const (
	AuditActionCreate       = "Create"
	AuditActionUpdate       = "Update"
	AuditActionDelete       = "Delete"
	AuditActionStatusChange = "StatusChange"
	AuditActionLogin        = "Login"
	AuditActionLogout       = "Logout"
	AuditActionExport       = "Export"
	AuditActionApproval     = "Approval"
	AuditActionEnforcement  = "Enforcement"
	AuditActionKeyRotation  = "KeyRotation"
	AuditActionBackup       = "Backup"
	AuditActionRestore      = "Restore"
)
