package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateProfileSlashCommand(t *testing.T) {
	// Use t.TempDir() for parallel safety and automatic cleanup
	tmpFile := filepath.Join(t.TempDir(), "test_profile_cmd.md")

	// Call the function (no config/token/port needed)
	err := createProfileSlashCommand(tmpFile)
	if err != nil {
		t.Fatalf("createProfileSlashCommand failed: %v", err)
	}

	// Read the generated file
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("Failed to read generated file: %v", err)
	}

	contentStr := string(content)

	// Verify template structure
	if !strings.Contains(contentStr, "name: profile") {
		t.Error("Missing 'name: profile' frontmatter")
	}

	if !strings.Contains(contentStr, "description:") {
		t.Error("Missing description in frontmatter")
	}

	// Verify dynamic discovery instructions are present
	if !strings.Contains(contentStr, "instances/") {
		t.Error("Template should reference instances/ directory for discovery")
	}

	if !strings.Contains(contentStr, "adminToken") {
		t.Error("Template should reference adminToken for dynamic discovery")
	}

	if !strings.Contains(contentStr, "instances/*.json") {
		t.Error("Template should reference instances/*.json glob pattern")
	}

	// Verify NO hardcoded secrets or ports
	if strings.Contains(contentStr, "localhost:808") {
		t.Error("Template must NOT contain hardcoded port numbers")
	}

	if strings.Contains(contentStr, "X-Admin-Token:") && !strings.Contains(contentStr, "$TOKEN") {
		// Allow X-Admin-Token only if it uses the discovered $TOKEN variable
		t.Error("Template must NOT contain hardcoded admin tokens")
	}

	// Verify curl is still mentioned for API calls
	if !strings.Contains(contentStr, "curl") {
		t.Error("Template should contain curl commands for API calls")
	}

	t.Logf("Generated content:\n%s", contentStr)
}
