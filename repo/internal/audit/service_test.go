package audit

import (
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// setupTestDB opens an in-memory SQLite database and auto-migrates the AuditLog table.
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Skipf("SQLite unavailable (CGO disabled or no C compiler): %v", err)
	}
	if err := db.AutoMigrate(&AuditLog{}); err != nil {
		t.Fatalf("failed to auto-migrate: %v", err)
	}
	return db
}

func TestLog_CreatesRecord(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	svc.Log(1, "Create", "WorkOrder", 42, "Created work order #42", "127.0.0.1", "req-001")

	var count int64
	db.Model(&AuditLog{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 audit log, got %d", count)
	}

	var entry AuditLog
	db.First(&entry)
	if entry.Action != "Create" {
		t.Errorf("expected action=Create, got %s", entry.Action)
	}
	if entry.ResourceType != "WorkOrder" {
		t.Errorf("expected resource_type=WorkOrder, got %s", entry.ResourceType)
	}
	if entry.ResourceID == nil || *entry.ResourceID != 42 {
		t.Errorf("expected resource_id=42, got %v", entry.ResourceID)
	}
	if entry.ActorID == nil || *entry.ActorID != 1 {
		t.Errorf("expected actor_id=1, got %v", entry.ActorID)
	}
	if entry.IPAddress != "127.0.0.1" {
		t.Errorf("expected ip=127.0.0.1, got %s", entry.IPAddress)
	}
	if entry.RequestID != "req-001" {
		t.Errorf("expected request_id=req-001, got %s", entry.RequestID)
	}
	if entry.UUID == "" {
		t.Error("expected UUID to be set")
	}
}

func TestLog_ZeroActorID_StoresNil(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	svc.Log(0, "Login", "Session", 0, "Anonymous access", "10.0.0.1", "req-002")

	var entry AuditLog
	db.First(&entry)
	if entry.ActorID != nil {
		t.Errorf("expected actor_id to be nil for zero actorID, got %v", entry.ActorID)
	}
	if entry.ResourceID != nil {
		t.Errorf("expected resource_id to be nil for zero resourceID, got %v", entry.ResourceID)
	}
}

func TestLogWithValues_StoresJSON(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	oldVals := map[string]interface{}{"status": "New"}
	newVals := map[string]interface{}{"status": "Assigned"}

	svc.LogWithValues(5, "StatusChange", "WorkOrder", 10, "Status updated", "192.168.1.1", "req-003", oldVals, newVals)

	var entry AuditLog
	db.First(&entry)
	if len(entry.OldValues) == 0 {
		t.Error("expected OldValues to be set")
	}
	if len(entry.NewValues) == 0 {
		t.Error("expected NewValues to be set")
	}
}

func TestList_FilterByAction(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	svc.Log(1, "Create", "WorkOrder", 1, "created", "127.0.0.1", "req-a")
	svc.Log(1, "Delete", "WorkOrder", 2, "deleted", "127.0.0.1", "req-b")
	svc.Log(2, "Create", "User", 3, "user created", "127.0.0.1", "req-c")

	logs, total, err := svc.List(ListFilters{Category: "Create", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total=2 for action=Create, got %d", total)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}

func TestList_FilterByActorID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	svc.Log(1, "Create", "WorkOrder", 1, "desc", "127.0.0.1", "r1")
	svc.Log(2, "Create", "WorkOrder", 2, "desc", "127.0.0.1", "r2")
	svc.Log(1, "Update", "WorkOrder", 1, "desc", "127.0.0.1", "r3")

	actorID := uint64(1)
	logs, total, err := svc.List(ListFilters{ActorID: &actorID, Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total=2 for actor_id=1, got %d", total)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}

func TestList_FilterByRequestID(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	svc.Log(1, "Create", "WorkOrder", 1, "desc", "127.0.0.1", "req-target")
	svc.Log(1, "Update", "WorkOrder", 1, "desc", "127.0.0.1", "req-other")

	logs, total, err := svc.List(ListFilters{RequestID: "req-target", Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 for request_id=req-target, got %d", total)
	}
	_ = logs
}

func TestList_FilterByTimeRange(t *testing.T) {
	db := setupTestDB(t)
	repo := NewRepository(db)
	svc := NewService(repo)

	// Insert entries with explicit timestamps
	past := time.Now().UTC().Add(-2 * time.Hour)
	future := time.Now().UTC().Add(2 * time.Hour)

	oldEntry := &AuditLog{
		UUID:         "uuid-old",
		Action:       "Create",
		ResourceType: "WorkOrder",
		CreatedAt:    past,
	}
	recentEntry := &AuditLog{
		UUID:         "uuid-new",
		Action:       "Create",
		ResourceType: "WorkOrder",
		CreatedAt:    time.Now().UTC(),
	}
	_ = repo.Create(oldEntry)
	_ = repo.Create(recentEntry)

	from := time.Now().UTC().Add(-1 * time.Hour)
	logs, total, err := svc.List(ListFilters{From: from, To: future, Page: 1, PerPage: 20})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 for recent entries, got %d", total)
	}
	_ = logs
}

func TestList_Pagination(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	for i := 0; i < 5; i++ {
		svc.Log(1, "Create", "WorkOrder", uint64(i), "desc", "127.0.0.1", "")
	}

	logs, total, err := svc.List(ListFilters{Page: 1, PerPage: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs on page 1, got %d", len(logs))
	}
}

func TestGetByID_Found(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	svc.Log(1, "Create", "WorkOrder", 99, "desc", "127.0.0.1", "req-x")

	var stored AuditLog
	db.First(&stored)

	entry, appErr := svc.GetByID(stored.ID)
	if appErr != nil {
		t.Fatalf("unexpected error: %v", appErr)
	}
	if entry.ID != stored.ID {
		t.Errorf("expected ID=%d, got %d", stored.ID, entry.ID)
	}
}

func TestGetByID_NotFound(t *testing.T) {
	db := setupTestDB(t)
	svc := NewService(NewRepository(db))

	_, appErr := svc.GetByID(9999)
	if appErr == nil {
		t.Error("expected not-found error for unknown ID")
	}
	if appErr.Code != "NOT_FOUND" {
		t.Errorf("expected NOT_FOUND error code, got %s", appErr.Code)
	}
}
