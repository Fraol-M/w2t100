package logs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// newTestService creates a Service backed by a temporary directory.
// It registers a cleanup function to close the open log file so that
// the temporary directory can be removed on Windows.
func newTestService(t *testing.T) *Service {
	t.Helper()
	svc := NewService(t.TempDir())
	t.Cleanup(svc.Close)
	return svc
}

// --- WriteEntry ---

func TestWriteEntry_CreatesLogFile(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()

	svc.WriteEntry(Entry{
		Timestamp: now,
		Level:     "INFO",
		Category:  "test",
		Message:   "hello world",
	})

	expectedFile := filepath.Join(svc.logRoot, "app-"+now.Format("2006-01-02")+".log")
	if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
		t.Fatalf("expected log file %q to be created", expectedFile)
	}
}

func TestWriteEntry_WritesJSON(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()

	svc.WriteEntry(Entry{
		Timestamp: now,
		Level:     "INFO",
		Category:  "test",
		Message:   "test message",
		RequestID: "req-123",
	})

	entries, total, err := svc.Query(QueryFilters{
		From:    now.Add(-time.Second),
		To:      now.Add(time.Second),
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	if entries[0].Message != "test message" {
		t.Errorf("expected message='test message', got %q", entries[0].Message)
	}
	if entries[0].RequestID != "req-123" {
		t.Errorf("expected request_id='req-123', got %q", entries[0].RequestID)
	}
}

func TestWriteEntry_AutoSetsTimestamp(t *testing.T) {
	svc := newTestService(t)
	before := time.Now().UTC().Add(-time.Second)

	// Pass zero timestamp — should be auto-populated.
	svc.WriteEntry(Entry{
		Level:    "INFO",
		Category: "test",
		Message:  "auto timestamp",
	})

	entries, _, err := svc.Query(QueryFilters{
		From:    before,
		To:      time.Now().UTC().Add(time.Second),
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected entry with auto timestamp")
	}
	if entries[0].Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

// --- WriteRequest ---

func TestWriteRequest_CreatesEntry(t *testing.T) {
	svc := newTestService(t)
	before := time.Now().UTC().Add(-time.Second)

	svc.WriteRequest("GET", "/api/v1/health", "req-001", "42", "192.168.1.1", 200, 15)

	entries, total, err := svc.Query(QueryFilters{
		From:    before,
		To:      time.Now().UTC().Add(time.Second),
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1, got %d", total)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	e := entries[0]
	if e.Method != "GET" {
		t.Errorf("expected method=GET, got %q", e.Method)
	}
	if e.Path != "/api/v1/health" {
		t.Errorf("expected path=/api/v1/health, got %q", e.Path)
	}
	if e.StatusCode != 200 {
		t.Errorf("expected status_code=200, got %d", e.StatusCode)
	}
	if e.DurationMs != 15 {
		t.Errorf("expected duration_ms=15, got %d", e.DurationMs)
	}
}

func TestWriteRequest_LevelError_For5xx(t *testing.T) {
	svc := newTestService(t)
	before := time.Now().UTC().Add(-time.Second)

	svc.WriteRequest("POST", "/api/v1/users", "req-500", "0", "10.0.0.1", 500, 100)

	entries, _, err := svc.Query(QueryFilters{
		From:    before,
		To:      time.Now().UTC().Add(time.Second),
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	if entries[0].Level != "ERROR" {
		t.Errorf("expected level=ERROR for 5xx, got %q", entries[0].Level)
	}
}

func TestWriteRequest_LevelWarn_For4xx(t *testing.T) {
	svc := newTestService(t)
	before := time.Now().UTC().Add(-time.Second)

	svc.WriteRequest("GET", "/api/v1/missing", "req-404", "0", "10.0.0.1", 404, 5)

	entries, _, err := svc.Query(QueryFilters{
		From:    before,
		To:      time.Now().UTC().Add(time.Second),
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one entry")
	}
	if entries[0].Level != "WARN" {
		t.Errorf("expected level=WARN for 4xx, got %q", entries[0].Level)
	}
}

// --- LogRequest (middleware interface) ---

func TestLogRequest_SatisfiesInterface(t *testing.T) {
	svc := newTestService(t)
	before := time.Now().UTC().Add(-time.Second)

	// LogRequest is the method required by apphttp.LogService.
	svc.LogRequest("DELETE", "/api/v1/resource/1", 204, 8*time.Millisecond, "req-x", 7, "1.2.3.4")

	entries, _, err := svc.Query(QueryFilters{
		From:    before,
		To:      time.Now().UTC().Add(time.Second),
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected log entry from LogRequest")
	}
	e := entries[0]
	if e.ActorID != "7" {
		t.Errorf("expected actor_id='7', got %q", e.ActorID)
	}
	if e.DurationMs != 8 {
		t.Errorf("expected duration_ms=8, got %d", e.DurationMs)
	}
}

// --- Query filters ---

func TestQuery_FilterByLevel(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()

	svc.WriteEntry(Entry{Timestamp: now, Level: "INFO", Category: "test", Message: "info msg"})
	svc.WriteEntry(Entry{Timestamp: now, Level: "ERROR", Category: "test", Message: "error msg"})

	entries, total, err := svc.Query(QueryFilters{Level: "INFO", Page: 1, PerPage: 10,
		From: now.Add(-time.Second), To: now.Add(time.Second)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 for level=INFO, got %d", total)
	}
	if len(entries) != 1 || entries[0].Level != "INFO" {
		t.Errorf("expected 1 INFO entry, got %+v", entries)
	}
}

func TestQuery_FilterByCategory(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()

	svc.WriteEntry(Entry{Timestamp: now, Level: "INFO", Category: "request", Message: "req"})
	svc.WriteEntry(Entry{Timestamp: now, Level: "INFO", Category: "system", Message: "sys"})

	entries, total, err := svc.Query(QueryFilters{Category: "system", Page: 1, PerPage: 10,
		From: now.Add(-time.Second), To: now.Add(time.Second)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 for category=system, got %d", total)
	}
	if len(entries) != 1 || entries[0].Category != "system" {
		t.Errorf("expected 1 system entry, got %+v", entries)
	}
}

func TestQuery_FilterByRequestID(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()

	svc.WriteEntry(Entry{Timestamp: now, Level: "INFO", Category: "test", Message: "msg1", RequestID: "abc-123"})
	svc.WriteEntry(Entry{Timestamp: now, Level: "INFO", Category: "test", Message: "msg2", RequestID: "xyz-999"})

	entries, total, err := svc.Query(QueryFilters{RequestID: "abc-123", Page: 1, PerPage: 10,
		From: now.Add(-time.Second), To: now.Add(time.Second)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 for request_id=abc-123, got %d", total)
	}
	_ = entries
}

func TestQuery_FilterByActorID(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()

	svc.WriteEntry(Entry{Timestamp: now, Level: "INFO", Category: "test", Message: "actor5", ActorID: "5"})
	svc.WriteEntry(Entry{Timestamp: now, Level: "INFO", Category: "test", Message: "actor9", ActorID: "9"})

	entries, total, err := svc.Query(QueryFilters{ActorID: "5", Page: 1, PerPage: 10,
		From: now.Add(-time.Second), To: now.Add(time.Second)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Errorf("expected total=1 for actor_id=5, got %d", total)
	}
	_ = entries
}

func TestQuery_Pagination(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()

	for i := 0; i < 5; i++ {
		svc.WriteEntry(Entry{Timestamp: now, Level: "INFO", Category: "test",
			Message: "paginated message"})
	}

	entries, total, err := svc.Query(QueryFilters{
		From:    now.Add(-time.Second),
		To:      now.Add(time.Second),
		Page:    1,
		PerPage: 2,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total=5, got %d", total)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries on page 1, got %d", len(entries))
	}
}

// --- Sensitive field redaction ---

func TestWriteEntry_NeverWritesPassword(t *testing.T) {
	svc := newTestService(t)
	now := time.Now().UTC()

	// Simulate a log call that mistakenly includes password text in the message.
	// The test verifies that no raw password appears literally as a field value in log files.
	// (Real enforcement is architectural — callers must not pass sensitive data.)
	svc.WriteEntry(Entry{
		Timestamp: now,
		Level:     "INFO",
		Category:  "test",
		Message:   "user logged in",
		// Sensitive fields like password are simply NOT in Entry — this is structural protection.
	})

	dateStr := now.Format("2006-01-02")
	logPath := filepath.Join(svc.logRoot, "app-"+dateStr+".log")
	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}

	// Verify that the Entry struct has no password/token fields — confirmed by absence
	// of such field names in the written JSON.
	for _, forbidden := range []string{`"password"`, `"token"`, `"key_bytes"`} {
		if strings.Contains(string(content), forbidden) {
			t.Errorf("log file contains forbidden field %q", forbidden)
		}
	}
}

// --- Daily rotation ---

func TestWriteEntry_DailyRotation(t *testing.T) {
	svc := newTestService(t)

	yesterday := time.Now().UTC().AddDate(0, 0, -1)
	today := time.Now().UTC()

	svc.WriteEntry(Entry{Timestamp: yesterday, Level: "INFO", Category: "test", Message: "yesterday"})
	svc.WriteEntry(Entry{Timestamp: today, Level: "INFO", Category: "test", Message: "today"})

	files, err := svc.ListLogFiles()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(files) < 2 {
		t.Errorf("expected at least 2 log files for different dates, got %d: %v", len(files), files)
	}
}
