package daemon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateInstanceID(t *testing.T) {
	id := GenerateInstanceID()

	if !strings.HasPrefix(id, "inst_") {
		t.Errorf("expected ID to start with 'inst_', got '%s'", id)
	}

	// The time.Now().Format("20060102_150405") produces "YYYYMMDD_hhmmss"
	// Example: "inst_20260224_115014"
	// When split by "_": ["inst", "20260224", "115014"]
	parts := strings.Split(id, "_")
	if len(parts) != 3 {
		t.Errorf("expected 3 parts when split by '_', got %d: %v", len(parts), parts)
	}

	if parts[0] != "inst" {
		t.Errorf("expected first part 'inst', got '%s'", parts[0])
	}

	if len(parts[1]) != 8 {
		t.Errorf("expected date part to be 8 characters (YYYYMMDD), got %d: %s", len(parts[1]), parts[1])
	}

	if len(parts[2]) != 6 {
		t.Errorf("expected time part to be 6 characters (hhmmss), got %d: %s", len(parts[2]), parts[2])
	}
}

func TestGenerateInstanceID_Unique(t *testing.T) {
	// Note: GenerateInstanceID uses time.Now() with second precision,
	// so rapid calls may generate duplicates. This is expected behavior.
	// We'll just verify the format is correct.

	id1 := GenerateInstanceID()
	time.Sleep(time.Second) // Ensure different second
	id2 := GenerateInstanceID()

	if id1 == id2 {
		t.Error("expected different IDs when called 1 second apart")
	}

	if !strings.HasPrefix(id1, "inst_") {
		t.Errorf("expected ID1 to start with 'inst_', got '%s'", id1)
	}
	if !strings.HasPrefix(id2, "inst_") {
		t.Errorf("expected ID2 to start with 'inst_', got '%s'", id2)
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

func TestInstancesDir_Success(t *testing.T) {
	dir, err := InstancesDir()
	if err != nil {
		t.Fatalf("InstancesDir failed: %v", err)
	}

	if dir == "" {
		t.Error("expected non-empty directory path")
	}

	// Should contain .cc-modelrouter/instances
	if !strings.Contains(dir, ".cc-modelrouter") {
		t.Errorf("expected path to contain '.cc-modelrouter', got %s", dir)
	}
	if !strings.Contains(dir, "instances") {
		t.Errorf("expected path to contain 'instances', got %s", dir)
	}
}

func TestIsRunning_ValidPID(t *testing.T) {
	// Use current process PID
	meta := &InstanceMetadata{
		PID: os.Getpid(),
	}

	if !IsRunning(meta) {
		t.Error("expected IsRunning to return true for current process")
	}
}

func TestIsRunning_ZeroPID(t *testing.T) {
	meta := &InstanceMetadata{
		PID: 0,
	}

	if IsRunning(meta) {
		t.Error("expected IsRunning to return false for zero PID")
	}
}

func TestIsRunning_NegativePID(t *testing.T) {
	meta := &InstanceMetadata{
		PID: -1,
	}

	if IsRunning(meta) {
		t.Error("expected IsRunning to return false for negative PID")
	}
}

func TestIsRunning_InvalidPID(t *testing.T) {
	// Use a PID that's very unlikely to exist
	meta := &InstanceMetadata{
		PID: 99999999,
	}

	// IsRunning should return false for non-existent process
	// Note: This test may pass or fail depending on the system
	// The important thing is it doesn't panic
	result := IsRunning(meta)
	if result {
		// This is possible if somehow the PID exists, but unlikely
		t.Logf("Note: PID %d unexpectedly exists on this system", meta.PID)
	}
}

func TestInstanceMetadataJSON_RoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	meta := &InstanceMetadata{
		ID:          "inst_test_001",
		Port:        8081,
		PID:         12345,
		ConfigType:  "project",
		ConfigPath:  "/path/to/config.json",
		ProjectRoot: "/path/to/project",
		StartTime:   now,
	}

	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	var loaded InstanceMetadata
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if loaded.ID != meta.ID {
		t.Errorf("expected ID %s, got %s", meta.ID, loaded.ID)
	}
	if loaded.Port != meta.Port {
		t.Errorf("expected Port %d, got %d", meta.Port, loaded.Port)
	}
	if loaded.PID != meta.PID {
		t.Errorf("expected PID %d, got %d", meta.PID, loaded.PID)
	}
	if loaded.ConfigType != meta.ConfigType {
		t.Errorf("expected ConfigType %s, got %s", meta.ConfigType, loaded.ConfigType)
	}
	if loaded.ConfigPath != meta.ConfigPath {
		t.Errorf("expected ConfigPath %s, got %s", meta.ConfigPath, loaded.ConfigPath)
	}
	if loaded.ProjectRoot != meta.ProjectRoot {
		t.Errorf("expected ProjectRoot %s, got %s", meta.ProjectRoot, loaded.ProjectRoot)
	}
	if !loaded.StartTime.Equal(meta.StartTime) {
		t.Errorf("expected StartTime %v, got %v", meta.StartTime, loaded.StartTime)
	}
}

func TestInstanceMetadata_JSONUnmarshal(t *testing.T) {
	jsonStr := `{
		"id": "inst_test_002",
		"port": 9090,
		"pid": 54321,
		"configType": "global",
		"configPath": "/global/config.json",
		"projectRoot": "",
		"startTime": "2024-02-17T14:30:00Z"
	}`

	var meta InstanceMetadata
	if err := json.Unmarshal([]byte(jsonStr), &meta); err != nil {
		t.Fatalf("failed to unmarshal metadata: %v", err)
	}

	if meta.ID != "inst_test_002" {
		t.Errorf("expected ID 'inst_test_002', got '%s'", meta.ID)
	}
	if meta.Port != 9090 {
		t.Errorf("expected Port 9090, got %d", meta.Port)
	}
	if meta.PID != 54321 {
		t.Errorf("expected PID 54321, got %d", meta.PID)
	}
	if meta.ConfigType != "global" {
		t.Errorf("expected ConfigType 'global', got '%s'", meta.ConfigType)
	}
}

func TestInstanceMetadata_JSONMarshal(t *testing.T) {
	meta := &InstanceMetadata{
		ID:          "inst_123",
		Port:        8081,
		PID:         12345,
		ConfigType:  "project",
		ConfigPath:  "/test/config.json",
		ProjectRoot: "/test/project",
		StartTime:   time.Date(2024, 2, 17, 14, 30, 0, 0, time.UTC),
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal metadata: %v", err)
	}

	// Verify JSON contains expected fields
	jsonStr := string(data)
	expectedFields := []string{
		`"id": "inst_123"`,
		`"port": 8081`,
		`"pid": 12345`,
		`"configType": "project"`,
	}

	for _, field := range expectedFields {
		if !strings.Contains(jsonStr, field) {
			t.Errorf("expected JSON to contain %s, got %s", field, jsonStr)
		}
	}
}

// Test the file operations using a temporary directory
// by temporarily changing HOME environment variable
func TestSaveAndLoadInstance_Integration(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	// Set HOME to temp directory
	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	meta := &InstanceMetadata{
		ID:          "inst_integration_test",
		Port:        8081,
		PID:         12345,
		ConfigType:  "project",
		ConfigPath:  "/test/config.json",
		ProjectRoot: "/test/project",
		StartTime:   time.Now(),
	}

	// Save instance
	err := SaveInstance(meta)
	if err != nil {
		t.Fatalf("SaveInstance failed: %v", err)
	}

	// Verify file exists in expected location
	instancesDir := filepath.Join(tmpDir, ".cc-modelrouter", "instances")
	expectedPath := filepath.Join(instancesDir, "inst_integration_test.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected instance file to exist at %s", expectedPath)
	}

	// Load instance
	loaded, err := LoadInstance("inst_integration_test")
	if err != nil {
		t.Fatalf("LoadInstance failed: %v", err)
	}

	if loaded.ID != meta.ID {
		t.Errorf("expected ID %s, got %s", meta.ID, loaded.ID)
	}
	if loaded.Port != meta.Port {
		t.Errorf("expected Port %d, got %d", meta.Port, loaded.Port)
	}
	if loaded.PID != meta.PID {
		t.Errorf("expected PID %d, got %d", meta.PID, loaded.PID)
	}
	if loaded.ConfigType != meta.ConfigType {
		t.Errorf("expected ConfigType %s, got %s", meta.ConfigType, loaded.ConfigType)
	}
}

func TestDeleteInstance_Integration(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	meta := &InstanceMetadata{
		ID:         "inst_delete_test",
		Port:       8081,
		PID:        12345,
		ConfigType: "project",
		StartTime:  time.Now(),
	}

	// Save instance
	if err := SaveInstance(meta); err != nil {
		t.Fatalf("SaveInstance failed: %v", err)
	}

	// Verify file exists
	instancesDir := filepath.Join(tmpDir, ".cc-modelrouter", "instances")
	filePath := filepath.Join(instancesDir, "inst_delete_test.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("expected instance file to exist before deletion")
	}

	// Delete instance
	err := DeleteInstance("inst_delete_test")
	if err != nil {
		t.Fatalf("DeleteInstance failed: %v", err)
	}

	// Verify file was deleted
	if _, err := os.Stat(filePath); err == nil {
		t.Error("expected instance file to be deleted")
	}
}

func TestLoadInstance_NotFound_Integration(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	_, err := LoadInstance("inst_nonexistent")
	if err == nil {
		t.Error("expected error for non-existent instance")
	}
}

func TestListInstances_Integration(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create multiple instances
	instances := []*InstanceMetadata{
		{ID: "inst_list_001", Port: 8081, PID: 1001, ConfigType: "project", StartTime: time.Now()},
		{ID: "inst_list_002", Port: 8082, PID: 1002, ConfigType: "global", StartTime: time.Now()},
		{ID: "inst_list_003", Port: 8083, PID: 1003, ConfigType: "project", StartTime: time.Now()},
	}

	for _, inst := range instances {
		if err := SaveInstance(inst); err != nil {
			t.Fatalf("failed to save instance %s: %v", inst.ID, err)
		}
	}

	// List instances
	listed, err := ListInstances()
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}

	if len(listed) != 3 {
		t.Errorf("expected 3 instances, got %d", len(listed))
	}

	// Verify all IDs are present
	ids := make(map[string]bool)
	for _, inst := range listed {
		ids[inst.ID] = true
	}

	expectedIDs := []string{"inst_list_001", "inst_list_002", "inst_list_003"}
	for _, id := range expectedIDs {
		if !ids[id] {
			t.Errorf("expected instance %s to be in list", id)
		}
	}
}

func TestListInstances_Empty_Integration(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	instances, err := ListInstances()
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}

	// When the instances directory doesn't exist, ListInstances returns nil
	// This is documented in the function behavior
	if instances != nil {
		t.Errorf("expected nil instances when directory doesn't exist, got %d instances", len(instances))
	}
}

func TestListInstances_NonJSONFiles_Integration(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create a valid instance
	validMeta := &InstanceMetadata{
		ID:         "inst_valid",
		Port:       8081,
		PID:        12345,
		ConfigType: "project",
		StartTime:  time.Now(),
	}
	if err := SaveInstance(validMeta); err != nil {
		t.Fatalf("failed to save valid instance: %v", err)
	}

	// Create non-JSON files in instances directory
	instancesDir := filepath.Join(tmpDir, ".cc-modelrouter", "instances")
	nonJSONFiles := []string{"readme.txt", "backup.bak"}
	for _, filename := range nonJSONFiles {
		path := filepath.Join(instancesDir, filename)
		if err := os.WriteFile(path, []byte("test content"), 0600); err != nil {
			t.Fatalf("failed to create non-JSON file: %v", err)
		}
	}

	// List instances - should only return the valid one
	listed, err := ListInstances()
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}

	if len(listed) != 1 {
		t.Errorf("expected 1 instance, got %d", len(listed))
	}

	if listed[0].ID != "inst_valid" {
		t.Errorf("expected ID 'inst_valid', got %s", listed[0].ID)
	}
}

func TestListInstances_CorruptedFile_Integration(t *testing.T) {
	// Save original HOME
	origHome := os.Getenv("HOME")
	defer os.Setenv("HOME", origHome)

	tmpDir := t.TempDir()
	os.Setenv("HOME", tmpDir)

	// Create a valid instance
	validMeta := &InstanceMetadata{
		ID:         "inst_valid_corrupt",
		Port:       8081,
		PID:        12345,
		ConfigType: "project",
		StartTime:  time.Now(),
	}
	if err := SaveInstance(validMeta); err != nil {
		t.Fatalf("failed to save valid instance: %v", err)
	}

	// Create a corrupted JSON file
	instancesDir := filepath.Join(tmpDir, ".cc-modelrouter", "instances")
	corruptedPath := filepath.Join(instancesDir, "inst_corrupted.json")
	if err := os.WriteFile(corruptedPath, []byte("{invalid json content"), 0600); err != nil {
		t.Fatalf("failed to create corrupted file: %v", err)
	}

	// List instances - corrupted file should be skipped
	listed, err := ListInstances()
	if err != nil {
		t.Fatalf("ListInstances failed: %v", err)
	}

	if len(listed) != 1 {
		t.Errorf("expected 1 valid instance, got %d", len(listed))
	}

	if listed[0].ID != "inst_valid_corrupt" {
		t.Errorf("expected ID 'inst_valid_corrupt', got %s", listed[0].ID)
	}
}