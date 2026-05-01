// Package util provides testing utilities for integration tests.
package util

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// AssertEqual asserts that two values are equal.
func AssertEqual(t *testing.T, expected, actual interface{}, msg string) {
	if expected != actual {
		t.Errorf("%s: expected %v, got %v", msg, expected, actual)
	}
}

// AssertNotEqual asserts that two values are not equal.
func AssertNotEqual(t *testing.T, expected, actual interface{}, msg string) {
	if expected == actual {
		t.Errorf("%s: expected %v != %v", msg, expected, actual)
	}
}

// AssertContains asserts that a string contains a substring.
func AssertContains(t *testing.T, s, substr string, msg string) {
	if !strings.Contains(s, substr) {
		t.Errorf("%s: expected %q to contain %q", msg, s, substr)
	}
}

// AssertNotContains asserts that a string does not contain a substring.
func AssertNotContains(t *testing.T, s, substr string, msg string) {
	if strings.Contains(s, substr) {
		t.Errorf("%s: expected %q to not contain %q", msg, s, substr)
	}
}

// AssertJSONEqual asserts that two JSON strings are semantically equal.
func AssertJSONEqual(t *testing.T, expected, actual string, msg string) {
	var expectedJSON, actualJSON interface{}

	if err := json.Unmarshal([]byte(expected), &expectedJSON); err != nil {
		t.Fatalf("%s: failed to unmarshal expected JSON: %v", msg, err)
	}

	if err := json.Unmarshal([]byte(actual), &actualJSON); err != nil {
		t.Fatalf("%s: failed to unmarshal actual JSON: %v", msg, err)
	}

	if !jsonEqual(expectedJSON, actualJSON) {
		t.Errorf("%s: JSON not equal\nExpected: %s\nActual:   %s", msg, expected, actual)
	}
}

// jsonEqual recursively compares two JSON values.
func jsonEqual(a, b interface{}) bool {
	switch a := a.(type) {
	case map[string]interface{}:
		b, ok := b.(map[string]interface{})
		if !ok {
			return false
		}
		if len(a) != len(b) {
			return false
		}
		for key, aVal := range a {
			bVal, ok := b[key]
			if !ok || !jsonEqual(aVal, bVal) {
				return false
			}
		}
		return true
	case []interface{}:
		b, ok := b.([]interface{})
		if !ok {
			return false
		}
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if !jsonEqual(a[i], b[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

// AssertStatusCode asserts that an HTTP response has the expected status code.
func AssertStatusCode(t *testing.T, resp *http.Response, expected int, msg string) {
	if resp.StatusCode != expected {
		body := new(bytes.Buffer)
		body.ReadFrom(resp.Body)
		t.Errorf("%s: expected status %d, got %d. Body: %s", msg, expected, resp.StatusCode, body.String())
	}
}

// AssertHeader asserts that an HTTP response contains a specific header.
func AssertHeader(t *testing.T, resp *http.Response, key, expected string, msg string) {
	actual := resp.Header.Get(key)
	if actual != expected {
		t.Errorf("%s: expected header %s=%q, got %q", msg, key, expected, actual)
	}
}

// AssertHasHeader asserts that an HTTP response contains a header (any value).
func AssertHasHeader(t *testing.T, resp *http.Response, key, msg string) {
	if resp.Header.Get(key) == "" {
		t.Errorf("%s: expected header %q to be present", msg, key)
	}
}

// AssertStreamEvent asserts that an SSE event has the expected structure.
func AssertStreamEvent(t *testing.T, event, eventType, dataPrefix string, msg string) {
	parts := strings.SplitN(event, ":", 2)
	if len(parts) != 2 {
		t.Errorf("%s: invalid SSE event format: %q", msg, event)
		return
	}

	actualType := strings.TrimSpace(parts[0])
	if actualType != eventType {
		t.Errorf("%s: expected event type %q, got %q", msg, eventType, actualType)
	}

	actualData := strings.TrimSpace(parts[1])
	if dataPrefix != "" && !strings.HasPrefix(actualData, dataPrefix) {
		t.Errorf("%s: expected data to start with %q, got %q", msg, dataPrefix, actualData)
	}
}

// AssertPositive asserts that a number is positive (> 0).
func AssertPositive(t *testing.T, n int, msg string) {
	if n <= 0 {
		t.Errorf("%s: expected positive number, got %d", msg, n)
	}
}

// AssertNonEmpty asserts that a string is not empty.
func AssertNonEmpty(t *testing.T, s, msg string) {
	if s == "" {
		t.Errorf("%s: expected non-empty string", msg)
	}
}

// AssertNil asserts that a value is nil.
func AssertNil(t *testing.T, v interface{}, msg string) {
	if v != nil {
		t.Errorf("%s: expected nil, got %v", msg, v)
	}
}

// AssertNotNil asserts that a value is not nil.
func AssertNotNil(t *testing.T, v interface{}, msg string) {
	if v == nil {
		t.Errorf("%s: expected non-nil value", msg)
	}
}

// AssertError asserts that an error is not nil.
func AssertError(t *testing.T, err error, msg string) {
	if err == nil {
		t.Errorf("%s: expected error, got nil", msg)
	}
}

// AssertNoError asserts that an error is nil.
func AssertNoError(t *testing.T, err error, msg string) {
	if err != nil {
		t.Errorf("%s: unexpected error: %v", msg, err)
	}
}

// AssertLen asserts that a slice/map has the expected length.
func AssertLen(t *testing.T, obj interface{}, expected int, msg string) {
	switch v := obj.(type) {
	case []interface{}:
		if len(v) != expected {
			t.Errorf("%s: expected length %d, got %d", msg, expected, len(v))
		}
	case []string:
		if len(v) != expected {
			t.Errorf("%s: expected length %d, got %d", msg, expected, len(v))
		}
	case map[string]interface{}:
		if len(v) != expected {
			t.Errorf("%s: expected length %d, got %d", msg, expected, len(v))
		}
	default:
		t.Errorf("%s: unsupported type for length assertion: %T", msg, obj)
	}
}

// AssertSSEStreaming asserts that SSE data contains required events.
func AssertSSEStreaming(t *testing.T, data string, requiredEvents []string, msg string) {
	lines := strings.Split(data, "\n")
	found := make(map[string]bool)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, event := range requiredEvents {
			if strings.Contains(line, `"type":"`+event+`"`) || strings.Contains(line, `type:`+event) {
				found[event] = true
			}
		}
	}

	for _, event := range requiredEvents {
		if !found[event] {
			t.Errorf("%s: SSE stream missing required event: %s", msg, event)
		}
	}
}

// SkipWithReason skips the test with the given reason.
func SkipWithReason(t *testing.T, reason string) {
	t.Skip(reason)
}

// SkipIfEnvNotSet skips the test if the given environment variable is not set.
func SkipIfEnvNotSet(t *testing.T, envVar string) {
	if value := GetAPIKey(envVar); value == "" {
		t.Skipf("%s not set, skipping test", envVar)
	}
}

// LogWithTimestamp logs a message with timestamp for debugging.
func LogWithTimestamp(t *testing.T, format string, args ...interface{}) {
	t.Logf(format, args...)
}