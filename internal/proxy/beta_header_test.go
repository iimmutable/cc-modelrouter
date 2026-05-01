package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestFilesAPIBetaHeaderValidation tests that the Files API requires
// the anthropic-beta header with the correct beta version.
func TestFilesAPIBetaHeaderValidation(t *testing.T) {
	t.Run("POST /v1/files without beta header returns 400", func(t *testing.T) {
		handler := NewHandler(50 * 1024 * 1024)
		setupHandler(t, handler, "test-beta")

		req := httptest.NewRequest("POST", "/v1/files", strings.NewReader(`{"filename":"test.pdf"}`))
		req.Header.Set("Content-Type", "application/json")
		// Note: NOT setting anthropic-beta header

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request for missing beta header, got %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "anthropic-beta") {
			t.Errorf("expected error message to mention anthropic-beta header, got: %s", body)
		}
	})

	t.Run("POST /v1/files with correct beta header succeeds", func(t *testing.T) {
		handler := NewHandler(50 * 1024 * 1024)
		setupHandler(t, handler, "test-beta")

		req := httptest.NewRequest("POST", "/v1/files", strings.NewReader(`{"filename":"test.pdf"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-beta", "files-api-2025-04-14")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200 OK with valid beta header, got %d", w.Code)
		}
	})

	t.Run("POST /v1/files with wrong beta version returns 400", func(t *testing.T) {
		handler := NewHandler(50 * 1024 * 1024)
		setupHandler(t, handler, "test-beta")

		req := httptest.NewRequest("POST", "/v1/files", strings.NewReader(`{"filename":"test.pdf"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("anthropic-beta", "wrong-beta-version")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request for wrong beta version, got %d", w.Code)
		}

		body := w.Body.String()
		if !strings.Contains(body, "files-api-2025-04-14") {
			t.Errorf("expected error message to mention correct beta version, got: %s", body)
		}
	})

	t.Run("GET /v1/files also requires beta header", func(t *testing.T) {
		handler := NewHandler(50 * 1024 * 1024)
		setupHandler(t, handler, "test-beta")

		req := httptest.NewRequest("GET", "/v1/files", nil)
		// Not setting anthropic-beta header

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status 400 Bad Request for missing beta header on GET, got %d", w.Code)
		}
	})

	t.Run("Messages API does NOT require beta header", func(t *testing.T) {
		handler := NewHandler(50 * 1024 * 1024)
		setupHandler(t, handler, "test-beta")

		body := `{
			"model": "claude-sonnet-4-20250514",
			"max_tokens": 1024,
			"messages": [{"role": "user", "content": "Hello"}]
		}`

		req := httptest.NewRequest("POST", "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		// NOT setting anthropic-beta header - should be OK for messages

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		// Should get a 401 (unauthorized) because we're not setting up routing,
		// NOT a 400 for missing beta header
		if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "anthropic-beta") {
			t.Error("Messages API should not require Files API beta header")
		}
	})
}
