package observability

import (
	"log/slog"
	"regexp"
	"strings"
)

// redacted is the only value emitted for a sensitive attribute.
const redacted = "[redacted]"

// sensitiveKeyExact covers short or ambiguous names that are unsafe as
// substring matches (for example "ip" would otherwise catch "description").
var sensitiveKeyExact = map[string]struct{}{
	"password":      {},
	"passwd":        {},
	"secret":        {},
	"token":         {},
	"authorization": {},
	"cookie":        {},
	"setcookie":     {},
	"session":       {},
	"email":         {},
	"address":       {},
	"phone":         {},
	"dob":           {},
	"birth":         {},
	"bio":           {},
	"details":       {},
	"instagram":     {},
	"url":           {},
	"uploadurl":     {},
	"presigned":     {},
	"apikey":        {},
	"key":           {},
	"signature":     {},
	"latitude":      {},
	"longitude":     {},
	"lat":           {},
	"lng":           {},
	"coordinates":   {},
	"ip":            {},
}

// sensitiveKeyParts catch compound names such as upload_url, e_mail,
// object_key and session_id after normalisation.
var sensitiveKeyParts = []string{
	"password",
	"passwd",
	"secret",
	"token",
	"authorization",
	"cookie",
	"session",
	"email",
	"address",
	"phone",
	"birth",
	"details",
	"instagram",
	"presigned",
	"apikey",
	"signature",
	"latitude",
	"longitude",
	"coordinates",
	"uploadurl",
}

var emailValuePattern = regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`)

// redactAttr is wired into slog.HandlerOptions.ReplaceAttr. It runs for every
// non-group attribute, including attributes nested inside groups, so the
// logger keeps a fixed small schema without per-request lookups.
func redactAttr(_ []string, attr slog.Attr) slog.Attr {
	if sensitiveKey(attr.Key) {
		return slog.String(attr.Key, redacted)
	}
	if attr.Value.Kind() == slog.KindString && sensitiveValue(attr.Value.String()) {
		return slog.String(attr.Key, redacted)
	}
	return attr
}

func sensitiveKey(key string) bool {
	normalised := normaliseKey(key)
	if _, banned := sensitiveKeyExact[normalised]; banned {
		return true
	}
	for _, part := range sensitiveKeyParts {
		if strings.Contains(normalised, part) {
			return true
		}
	}
	return false
}

func normaliseKey(key string) string {
	var builder strings.Builder
	builder.Grow(len(key))
	for _, char := range key {
		switch char {
		case '-', '_', ' ':
			continue
		case 'A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J', 'K', 'L', 'M',
			'N', 'O', 'P', 'Q', 'R', 'S', 'T', 'U', 'V', 'W', 'X', 'Y', 'Z':
			builder.WriteRune(char + ('a' - 'A'))
		default:
			builder.WriteRune(char)
		}
	}
	return builder.String()
}

// sensitiveValue rejects the three value shapes that can carry a credential or
// personal data even under an innocent attribute name.
func sensitiveValue(value string) bool {
	if emailValuePattern.MatchString(value) {
		return true
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{
		"x-amz-signature",
		"x-amz-credential",
		"signature=",
		"sig=",
		"token=",
		"bearer ",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return looksLikeOpaqueToken(value)
}

// looksLikeOpaqueToken catches long base64url session tokens while allowing
// lowercase hex request IDs (32 characters, no non-hex rune) to pass through.
func looksLikeOpaqueToken(value string) bool {
	if len(value) < 40 || strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	hasDigit := false
	hasNonHex := false
	for _, char := range value {
		switch {
		case char >= '0' && char <= '9':
			hasDigit = true
		case char >= 'a' && char <= 'f':
			// Lowercase hex is the request-ID and digest shape.
		case char >= 'A' && char <= 'Z', char >= 'g' && char <= 'z',
			char == '-', char == '_', char == '.', char == '=':
			hasNonHex = true
		default:
			return false
		}
	}
	return hasDigit && hasNonHex
}
