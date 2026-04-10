package security

// MaskString returns a masked version of s, showing only the last 4 characters.
// Strings shorter than or equal to 4 characters are fully masked as "****".
func MaskString(s string) string {
	runes := []rune(s)
	if len(runes) <= 4 {
		return "****"
	}
	masked := make([]rune, len(runes))
	for i := 0; i < len(runes)-4; i++ {
		masked[i] = '*'
	}
	copy(masked[len(runes)-4:], runes[len(runes)-4:])
	return string(masked)
}

// MaskPhone masks a phone number showing only the last 4 digits.
// All non-digit characters are stripped and only "****-****-XXXX" is returned.
func MaskPhone(phone string) string {
	digits := make([]rune, 0, len(phone))
	for _, r := range phone {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) <= 4 {
		return "****"
	}
	last4 := string(digits[len(digits)-4:])
	return "****-****-" + last4
}
