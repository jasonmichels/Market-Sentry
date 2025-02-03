package utils

import (
	"log"
	"regexp"
	"strings"
	"unicode"
)

// phoneRegex now requires a leading '+' followed by 10 to 15 digits.
var phoneRegex = regexp.MustCompile(`^\+\d{10,15}$`)

// NormalizePhoneNumber removes all characters except digits and preserves a leading '+' if present.
func NormalizePhoneNumber(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}

	log.Printf("Normalizing phone number: %s\n", input)

	var builder strings.Builder
	if input[0] == '+' {
		builder.WriteByte('+')
	}
	for _, r := range input {
		if unicode.IsDigit(r) {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}

// ValidatePhoneWithCountry normalizes the input and, if a leading '+' is not present,
// prepends the provided country code. It returns the normalized phone number
// in E.164 format and a boolean indicating whether it is valid.
func ValidatePhoneWithCountry(input, countryCode string) (string, bool) {
	log.Printf("Validating phone with country code: %s, %s\n", input, countryCode)
	normalized := NormalizePhoneNumber(input)
	normalized = countryCode + normalized

	return normalized, phoneRegex.MatchString(normalized)
}
