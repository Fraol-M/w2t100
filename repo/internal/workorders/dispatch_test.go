package workorders

import (
	"fmt"
	"testing"
)

// mockPropertyQuerier implements PropertyQuerier for testing.
type mockPropertyQuerier struct {
	techs       []uint64
	cursor      int
	techErr     error
	cursorErr   error
	updateErr   error
	lastCursor  int
	lastUserID  uint64
}

func (m *mockPropertyQuerier) FindTechniciansByPropertyAndSkill(propertyID uint64, skillTag string) ([]uint64, error) {
	if m.techErr != nil {
		return nil, m.techErr
	}
	return m.techs, nil
}

func (m *mockPropertyQuerier) GetDispatchCursor(propertyID uint64, skillTag string) (int, error) {
	if m.cursorErr != nil {
		return 0, m.cursorErr
	}
	return m.cursor, nil
}

func (m *mockPropertyQuerier) UpdateDispatchCursor(propertyID uint64, skillTag string, position int, userID uint64) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.lastCursor = position
	m.lastUserID = userID
	m.cursor = position
	return nil
}

func TestDispatch_NextTechSelection(t *testing.T) {
	mock := &mockPropertyQuerier{
		techs:  []uint64{10, 20, 30},
		cursor: 0,
	}
	ds := NewDispatchService(mock)

	result, err := ds.Dispatch(1, "plumbing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Assigned {
		t.Fatal("expected assignment to succeed")
	}
	if result.TechnicianID != 10 {
		t.Errorf("expected technician 10, got %d", result.TechnicianID)
	}
	if mock.lastCursor != 1 {
		t.Errorf("expected cursor to advance to 1, got %d", mock.lastCursor)
	}
}

func TestDispatch_SecondTechSelection(t *testing.T) {
	mock := &mockPropertyQuerier{
		techs:  []uint64{10, 20, 30},
		cursor: 1,
	}
	ds := NewDispatchService(mock)

	result, err := ds.Dispatch(1, "plumbing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TechnicianID != 20 {
		t.Errorf("expected technician 20, got %d", result.TechnicianID)
	}
	if mock.lastCursor != 2 {
		t.Errorf("expected cursor to advance to 2, got %d", mock.lastCursor)
	}
}

func TestDispatch_WrapAround(t *testing.T) {
	mock := &mockPropertyQuerier{
		techs:  []uint64{10, 20, 30},
		cursor: 3, // wraps to index 0
	}
	ds := NewDispatchService(mock)

	result, err := ds.Dispatch(1, "plumbing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TechnicianID != 10 {
		t.Errorf("expected technician 10 (wrap-around), got %d", result.TechnicianID)
	}
	if mock.lastCursor != 4 {
		t.Errorf("expected cursor to advance to 4, got %d", mock.lastCursor)
	}
}

func TestDispatch_FullCycle(t *testing.T) {
	mock := &mockPropertyQuerier{
		techs:  []uint64{10, 20, 30},
		cursor: 0,
	}
	ds := NewDispatchService(mock)

	expected := []uint64{10, 20, 30, 10, 20, 30}
	for i, want := range expected {
		result, err := ds.Dispatch(1, "plumbing")
		if err != nil {
			t.Fatalf("dispatch %d: unexpected error: %v", i, err)
		}
		if result.TechnicianID != want {
			t.Errorf("dispatch %d: expected technician %d, got %d", i, want, result.TechnicianID)
		}
	}
}

func TestDispatch_NoTechsFound(t *testing.T) {
	mock := &mockPropertyQuerier{
		techs:  []uint64{},
		cursor: 0,
	}
	ds := NewDispatchService(mock)

	result, err := ds.Dispatch(1, "plumbing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Assigned {
		t.Error("expected no assignment when no technicians available")
	}
	if result.Message == "" {
		t.Error("expected a message explaining why dispatch failed")
	}
}

func TestDispatch_TechQueryError(t *testing.T) {
	mock := &mockPropertyQuerier{
		techErr: fmt.Errorf("db connection failed"),
	}
	ds := NewDispatchService(mock)

	_, err := ds.Dispatch(1, "plumbing")
	if err == nil {
		t.Fatal("expected error when tech query fails")
	}
}

func TestDispatch_CursorQueryError(t *testing.T) {
	mock := &mockPropertyQuerier{
		techs:     []uint64{10, 20},
		cursorErr: fmt.Errorf("db error"),
	}
	ds := NewDispatchService(mock)

	_, err := ds.Dispatch(1, "plumbing")
	if err == nil {
		t.Fatal("expected error when cursor query fails")
	}
}

func TestDispatch_SingleTech(t *testing.T) {
	mock := &mockPropertyQuerier{
		techs:  []uint64{42},
		cursor: 0,
	}
	ds := NewDispatchService(mock)

	// Should always pick the same technician.
	for i := 0; i < 5; i++ {
		result, err := ds.Dispatch(1, "electrical")
		if err != nil {
			t.Fatalf("dispatch %d: unexpected error: %v", i, err)
		}
		if result.TechnicianID != 42 {
			t.Errorf("dispatch %d: expected technician 42, got %d", i, result.TechnicianID)
		}
	}
}
