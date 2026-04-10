package workorders

import "fmt"

// PropertyQuerier abstracts the property-related queries needed by the dispatch service.
type PropertyQuerier interface {
	FindTechniciansByPropertyAndSkill(propertyID uint64, skillTag string) ([]uint64, error)
	GetDispatchCursor(propertyID uint64, skillTag string) (int, error)
	UpdateDispatchCursor(propertyID uint64, skillTag string, position int, userID uint64) error
}

// DispatchService handles deterministic round-robin dispatch of work orders
// to technicians based on property and skill tag.
type DispatchService struct {
	propQuerier PropertyQuerier
}

// NewDispatchService creates a new DispatchService.
func NewDispatchService(pq PropertyQuerier) *DispatchService {
	return &DispatchService{propQuerier: pq}
}

// DispatchResult holds the result of a dispatch attempt.
type DispatchResult struct {
	TechnicianID uint64
	Assigned     bool
	Message      string
}

// Dispatch selects the next technician using round-robin for the given property and skill tag.
// It returns the selected technician ID and whether assignment was successful.
// If no technicians are available, Assigned is false and Message explains why.
func (d *DispatchService) Dispatch(propertyID uint64, skillTag string) (*DispatchResult, error) {
	// Get eligible technicians sorted by ID for deterministic ordering.
	techs, err := d.propQuerier.FindTechniciansByPropertyAndSkill(propertyID, skillTag)
	if err != nil {
		return nil, fmt.Errorf("failed to find technicians: %w", err)
	}

	if len(techs) == 0 {
		return &DispatchResult{
			Assigned: false,
			Message:  "no technicians available for the requested property and skill",
		}, nil
	}

	// Get the current cursor position.
	cursor, err := d.propQuerier.GetDispatchCursor(propertyID, skillTag)
	if err != nil {
		return nil, fmt.Errorf("failed to get dispatch cursor: %w", err)
	}

	// Select next technician using modulo wrap-around.
	idx := cursor % len(techs)
	selectedTech := techs[idx]

	// Advance cursor to next position.
	nextCursor := cursor + 1
	if err := d.propQuerier.UpdateDispatchCursor(propertyID, skillTag, nextCursor, selectedTech); err != nil {
		return nil, fmt.Errorf("failed to update dispatch cursor: %w", err)
	}

	return &DispatchResult{
		TechnicianID: selectedTech,
		Assigned:     true,
		Message:      fmt.Sprintf("assigned to technician %d", selectedTech),
	}, nil
}
