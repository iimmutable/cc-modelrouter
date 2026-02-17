package daemon

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestGenerateInstanceID(t *testing.T) {
	id := GenerateInstanceID()

	if !strings.HasPrefix(id, "inst_") {
		t.Errorf("expected ID to start with 'inst_', got '%s'", id)
	}
}

func TestInstanceMetadata(t *testing.T) {
	meta := &InstanceMetadata{
		ID:         "inst_20250217_143000",
		Port:       8081,
		PID:        12345,
		ConfigType: "project",
		StartTime:  time.Now(),
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	var unmarshaled InstanceMetadata
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if unmarshaled.ID != meta.ID {
		t.Errorf("expected ID %s, got %s", meta.ID, unmarshaled.ID)
	}
}
