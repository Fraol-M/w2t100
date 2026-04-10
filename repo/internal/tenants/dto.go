package tenants

import (
	"time"
)

// CreateTenantProfileRequest is the payload to create a new tenant profile.
type CreateTenantProfileRequest struct {
	UserID           uint64  `json:"user_id" binding:"required"`
	UnitID           *uint64 `json:"unit_id"`
	EmergencyContact string  `json:"emergency_contact"`
	LeaseStart       string  `json:"lease_start"`
	LeaseEnd         string  `json:"lease_end"`
	MoveInDate       string  `json:"move_in_date"`
	Notes            string  `json:"notes"`
}

// UpdateTenantProfileRequest is the payload to update a tenant profile.
type UpdateTenantProfileRequest struct {
	UnitID           *uint64 `json:"unit_id"`
	EmergencyContact *string `json:"emergency_contact"`
	LeaseStart       *string `json:"lease_start"`
	LeaseEnd         *string `json:"lease_end"`
	MoveInDate       *string `json:"move_in_date"`
	Notes            *string `json:"notes"`
}

// TenantProfileResponse is the public representation of a tenant profile.
type TenantProfileResponse struct {
	ID               uint64     `json:"id"`
	UUID             string     `json:"uuid"`
	UserID           uint64     `json:"user_id"`
	UnitID           *uint64    `json:"unit_id"`
	EmergencyContact string     `json:"emergency_contact,omitempty"`
	LeaseStart       *time.Time `json:"lease_start,omitempty"`
	LeaseEnd         *time.Time `json:"lease_end,omitempty"`
	MoveInDate       *time.Time `json:"move_in_date,omitempty"`
	Notes            *string    `json:"notes,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// ToTenantProfileResponse converts a TenantProfile model to a response DTO.
// Emergency contact is masked unless revealContact is true.
func ToTenantProfileResponse(tp *TenantProfile, decryptedContact string, revealContact bool) TenantProfileResponse {
	contact := ""
	if decryptedContact != "" {
		if revealContact {
			contact = decryptedContact
		} else {
			contact = maskContact(decryptedContact)
		}
	}

	return TenantProfileResponse{
		ID:               tp.ID,
		UUID:             tp.UUID,
		UserID:           tp.UserID,
		UnitID:           tp.UnitID,
		EmergencyContact: contact,
		LeaseStart:       tp.LeaseStart,
		LeaseEnd:         tp.LeaseEnd,
		MoveInDate:       tp.MoveInDate,
		Notes:            tp.Notes,
		CreatedAt:        tp.CreatedAt,
		UpdatedAt:        tp.UpdatedAt,
	}
}

// maskContact masks all but the last 4 characters of a contact string.
func maskContact(contact string) string {
	if len(contact) <= 4 {
		return contact
	}
	masked := make([]byte, len(contact))
	for i := range masked {
		if i < len(contact)-4 {
			masked[i] = '*'
		} else {
			masked[i] = contact[i]
		}
	}
	return string(masked)
}
