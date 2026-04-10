package logs

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Entry represents a single structured log entry written as a JSON line to disk.
// Sensitive fields (passwords, raw tokens, encryption keys, raw PII) must NEVER appear here.
type Entry struct {
	Timestamp  time.Time `json:"timestamp"`
	Level      string    `json:"level"`
	Category   string    `json:"category"`
	RequestID  string    `json:"request_id,omitempty"`
	ActorID    string    `json:"actor_id,omitempty"`
	Message    string    `json:"message"`
	Method     string    `json:"method,omitempty"`
	Path       string    `json:"path,omitempty"`
	StatusCode int       `json:"status_code,omitempty"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	IPAddress  string    `json:"ip_address,omitempty"`
	Error      string    `json:"error,omitempty"`
}

// Service is a structured logging service that writes newline-delimited JSON to
// daily log files under logRoot/app-YYYY-MM-DD.log.
type Service struct {
	logRoot     string
	mu          sync.Mutex
	currentFile *os.File
	currentDate string // YYYY-MM-DD of the open file
}

// NewService creates a new logs Service that writes to logRoot.
func NewService(logRoot string) *Service {
	return &Service{logRoot: logRoot}
}

// WriteEntry writes a log entry as a JSON line to the current daily log file.
// NEVER write passwords, raw tokens, encryption keys, or raw PII.
func (s *Service) WriteEntry(entry Entry) {
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}

	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("logs.Service.WriteEntry: failed to marshal entry: %v", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.ensureFile(entry.Timestamp); err != nil {
		log.Printf("logs.Service.WriteEntry: failed to open log file: %v", err)
		return
	}

	if _, err := fmt.Fprintf(s.currentFile, "%s\n", data); err != nil {
		log.Printf("logs.Service.WriteEntry: failed to write log entry: %v", err)
	}
}

// LogRequest is called by the StructuredLogging middleware for each HTTP request.
// It satisfies the apphttp.LogService interface:
//
//	LogRequest(method, path string, status int, duration time.Duration, requestID string, actorID uint64, ip string)
func (s *Service) LogRequest(method, path string, status int, duration time.Duration, requestID string, actorID uint64, ip string) {
	level := "INFO"
	if status >= 500 {
		level = "ERROR"
	} else if status >= 400 {
		level = "WARN"
	}

	var actorStr string
	if actorID != 0 {
		actorStr = fmt.Sprintf("%d", actorID)
	}

	s.WriteEntry(Entry{
		Timestamp:  time.Now().UTC(),
		Level:      level,
		Category:   "request",
		RequestID:  requestID,
		ActorID:    actorStr,
		Message:    fmt.Sprintf("%s %s %d", method, path, status),
		Method:     method,
		Path:       path,
		StatusCode: status,
		DurationMs: duration.Milliseconds(),
		IPAddress:  ip,
	})
}

// WriteRequest is an alternate entry point for manual request logging.
func (s *Service) WriteRequest(method, path, requestID, actorID, ip string, statusCode int, durationMs int64) {
	level := "INFO"
	if statusCode >= 500 {
		level = "ERROR"
	} else if statusCode >= 400 {
		level = "WARN"
	}

	s.WriteEntry(Entry{
		Timestamp:  time.Now().UTC(),
		Level:      level,
		Category:   "request",
		RequestID:  requestID,
		ActorID:    actorID,
		Message:    fmt.Sprintf("%s %s %d", method, path, statusCode),
		Method:     method,
		Path:       path,
		StatusCode: statusCode,
		DurationMs: durationMs,
		IPAddress:  ip,
	})
}

// QueryFilters holds all filter parameters for querying log entries.
type QueryFilters struct {
	From      time.Time
	To        time.Time
	Level     string
	Category  string
	RequestID string
	ActorID   string
	Page      int
	PerPage   int
}

// Query reads log entries from disk files matching the filters and returns the
// paginated result along with the total number of matching entries.
func (s *Service) Query(filters QueryFilters) ([]Entry, int64, error) {
	page := filters.Page
	if page < 1 {
		page = 1
	}
	perPage := filters.PerPage
	if perPage < 1 {
		perPage = 20
	}
	if perPage > 100 {
		perPage = 100
	}

	files, err := s.logFilesInRange(filters.From, filters.To)
	if err != nil {
		return nil, 0, fmt.Errorf("Query: failed to list log files: %w", err)
	}

	var matched []Entry
	for _, fpath := range files {
		entries, err := readLogFile(fpath, filters)
		if err != nil {
			// Log the error but continue with other files.
			log.Printf("WARN logs.Service.Query: error reading %q: %v", fpath, err)
			continue
		}
		matched = append(matched, entries...)
	}

	total := int64(len(matched))
	offset := (page - 1) * perPage
	if offset >= len(matched) {
		return []Entry{}, total, nil
	}
	end := offset + perPage
	if end > len(matched) {
		end = len(matched)
	}
	return matched[offset:end], total, nil
}

// --- internal helpers ---

// ensureFile opens (or rotates to) the daily log file for the given timestamp.
// Must be called with s.mu held.
func (s *Service) ensureFile(ts time.Time) error {
	date := ts.UTC().Format("2006-01-02")
	if s.currentFile != nil && s.currentDate == date {
		return nil
	}

	// Close previous file if open.
	if s.currentFile != nil {
		_ = s.currentFile.Close()
		s.currentFile = nil
	}

	if err := os.MkdirAll(s.logRoot, 0750); err != nil {
		return fmt.Errorf("ensureFile: failed to create log directory %q: %w", s.logRoot, err)
	}

	path := filepath.Join(s.logRoot, fmt.Sprintf("app-%s.log", date))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("ensureFile: failed to open log file %q: %w", path, err)
	}

	s.currentFile = f
	s.currentDate = date
	return nil
}

// logFilesInRange returns the paths of all daily log files whose dates fall within
// [from, to]. If the range is zero, all files in logRoot are returned.
func (s *Service) logFilesInRange(from, to time.Time) ([]string, error) {
	entries, err := os.ReadDir(s.logRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if len(name) < len("app-2006-01-02.log") {
			continue
		}
		dateStr := name[4:14] // "app-YYYY-MM-DD.log" → positions 4..13
		fileDate, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}
		if !from.IsZero() && fileDate.Before(from.Truncate(24*time.Hour)) {
			continue
		}
		if !to.IsZero() && fileDate.After(to.Truncate(24*time.Hour)) {
			continue
		}
		files = append(files, filepath.Join(s.logRoot, name))
	}
	return files, nil
}

// readLogFile reads all JSON-line entries from a file, applying the given filters.
func readLogFile(path string, filters QueryFilters) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var results []Entry
	scanner := bufio.NewScanner(f)
	// Allow up to 1 MB per line.
	scanner.Buffer(make([]byte, 1*1024*1024), 1*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry Entry
		if err := json.Unmarshal(line, &entry); err != nil {
			continue
		}
		if !matchesFilters(entry, filters) {
			continue
		}
		results = append(results, entry)
	}
	return results, scanner.Err()
}

// matchesFilters returns true when the entry satisfies all active filters.
func matchesFilters(e Entry, f QueryFilters) bool {
	if !f.From.IsZero() && e.Timestamp.Before(f.From) {
		return false
	}
	if !f.To.IsZero() && e.Timestamp.After(f.To) {
		return false
	}
	if f.Level != "" && e.Level != f.Level {
		return false
	}
	if f.Category != "" && e.Category != f.Category {
		return false
	}
	if f.RequestID != "" && e.RequestID != f.RequestID {
		return false
	}
	if f.ActorID != "" && e.ActorID != f.ActorID {
		return false
	}
	return true
}

// Close closes the currently open log file, if any. Intended for use in tests.
func (s *Service) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currentFile != nil {
		_ = s.currentFile.Close()
		s.currentFile = nil
		s.currentDate = ""
	}
}

// ListLogFiles returns the file names of all log files present in the log directory.
func (s *Service) ListLogFiles() ([]string, error) {
	entries, err := os.ReadDir(s.logRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("ListLogFiles: failed to read log directory: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
