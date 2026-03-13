// Package logging provides header sanitization to prevent secret leakage in logs.
package logging

import (
	"net/http"
	"strings"
)

// SensitiveHeaders lists headers that contain secrets and must be redacted.
// These header names are compared case-insensitively.
var SensitiveHeaders = map[string]bool{
	"authorization":            true,
	"x-api-key":                true,
	"x-auth-token":             true,
	"cookie":                   true,
	"set-cookie":               true,
	"proxy-authorization":      true,
	"x-amz-security-token":     true,
	"x-goog-iam-authorization": true,
	"apikey":                   true,
	"x-bigmodel-api-key":       true,
	"x-openrouter-api-key":     true,
}

// SanitizeHeaders returns a copy of headers with sensitive values redacted.
// This is used to prevent API keys, tokens, and other secrets from appearing in logs.
func SanitizeHeaders(headers http.Header) map[string][]string {
	sanitized := make(map[string][]string)

	for key, values := range headers {
		lowerKey := strings.ToLower(key)

		if SensitiveHeaders[lowerKey] {
			// Redact sensitive headers
			redacted := make([]string, len(values))
			for i := range values {
				redacted[i] = redactValue(values[i])
			}
			sanitized[key] = redacted
		} else {
			// Keep non-sensitive headers as-is
			sanitized[key] = values
		}
	}

	return sanitized
}

// SanitizeHeadersString returns a string representation of headers with redacted secrets.
// The output format matches Go's default map printing style for compatibility with existing logs.
func SanitizeHeadersString(headers http.Header) string {
	if headers == nil {
		return "<nil>"
	}

	sanitized := SanitizeHeaders(headers)
	var builder strings.Builder
	builder.WriteString("map[")

	first := true
	for key, values := range sanitized {
		if !first {
			builder.WriteString(" ")
		}
		first = false

		builder.WriteString(key)
		builder.WriteString(":[")
		for i, v := range values {
			if i > 0 {
				builder.WriteString(" ")
			}
			builder.WriteString(v)
		}
		builder.WriteString("]")
	}

	builder.WriteString("]")
	return builder.String()
}

// redactValue redacts a secret value for safe logging.
// Shows a prefix for debugging but hides the actual secret.
func redactValue(value string) string {
	if len(value) == 0 {
		return "[EMPTY]"
	}

	// For very short values, fully redact
	if len(value) <= 8 {
		return "[REDACTED]"
	}

	// Show first 6 characters to help identify which key was used,
	// then hide the rest with asterisks and a clear marker
	prefix := value[:6]
	hiddenLen := len(value) - 6
	if hiddenLen > 16 {
		hiddenLen = 16 // Cap the number of asterisks
	}
	return prefix + strings.Repeat("*", hiddenLen) + " [REDACTED]"
}