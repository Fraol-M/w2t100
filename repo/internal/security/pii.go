package security

import "propertyops/backend/internal/common"

// MaskUserPhone applies PII-aware masking to a phone number based on viewer context.
// The full phone number is returned only when the requester is the owner or a SystemAdmin.
// All other requesters receive a masked version showing only the last 4 digits.
func MaskUserPhone(phone string, ownerID, requesterID uint64, requesterRoles []string) string {
	// Owner can always see their own phone.
	if requesterID == ownerID {
		return phone
	}

	// SystemAdmin can see full phone numbers.
	for _, role := range requesterRoles {
		if role == common.RoleSystemAdmin {
			return phone
		}
	}

	// Everyone else gets a masked version.
	return MaskPhone(phone)
}
