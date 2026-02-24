package daemon

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestWritePIDFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pid")
	expectedPID := os.Getpid()

	err := WritePIDFile(tmpFile)
	if err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("failed to read PID file: %v", err)
	}

	pidStr := string(data)
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		t.Fatalf("failed to parse PID: %v", err)
	}

	if pid != expectedPID {
		t.Errorf("expected PID %d, got %d", expectedPID, pid)
	}

	// Check file permissions
	info, err := os.Stat(tmpFile)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	if info.Mode().Perm() != 0600 {
		t.Errorf("expected permissions 0600, got %04o", info.Mode().Perm())
	}
}

func TestWritePIDFile_ValidPath(t *testing.T) {
	tmpDir := t.TempDir()
	subDir := filepath.Join(tmpDir, "subdir")
	err := os.Mkdir(subDir, 0700)
	if err != nil {
		t.Fatalf("failed to create subdirectory: %v", err)
	}
	tmpFile := filepath.Join(subDir, "test.pid")

	err = WritePIDFile(tmpFile)
	if err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(tmpFile); os.IsNotExist(err) {
		t.Error("expected PID file to exist")
	}
}

func TestReadPIDFile_Success(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.pid")
	expectedPID := 12345

	err := os.WriteFile(tmpFile, []byte(strconv.Itoa(expectedPID)), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	pid, err := ReadPIDFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	if pid != expectedPID {
		t.Errorf("expected PID %d, got %d", expectedPID, pid)
	}
}

func TestReadPIDFile_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "nonexistent.pid")

	_, err := ReadPIDFile(tmpFile)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestReadPIDFile_InvalidContent(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.pid")

	err := os.WriteFile(tmpFile, []byte("not a number"), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	_, err = ReadPIDFile(tmpFile)
	if err == nil {
		t.Error("expected error for invalid content")
	}
}

func TestReadPIDFile_TrimWhitespace(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "whitespace.pid")
	expectedPID := 12345

	testCases := []string{
		"12345",
		" 12345",
		"12345 ",
		"\t12345",
		"12345\t",
		"\n12345\n",
		" \t\n12345\n\t ",
	}

	for i, content := range testCases {
		err := os.WriteFile(tmpFile, []byte(content), 0600)
		if err != nil {
			t.Fatalf("test case %d: failed to write test PID file: %v", i, err)
		}

		pid, err := ReadPIDFile(tmpFile)
		if err != nil {
			t.Errorf("test case %d: ReadPIDFile failed: %v", i, err)
			continue
		}

		if pid != expectedPID {
			t.Errorf("test case %d: expected PID %d, got %d", i, expectedPID, pid)
		}
	}
}

func TestReadPIDFile_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "empty.pid")

	err := os.WriteFile(tmpFile, []byte(""), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	_, err = ReadPIDFile(tmpFile)
	if err == nil {
		t.Error("expected error for empty file")
	}
}

func TestReadPIDFile_NegativeNumber(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "negative.pid")

	err := os.WriteFile(tmpFile, []byte("-12345"), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	pid, err := ReadPIDFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	if pid != -12345 {
		t.Errorf("expected PID -12345, got %d", pid)
	}
}

func TestReadPIDFile_LargeNumber(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "large.pid")
	largePID := 2147483647 // Max int32

	err := os.WriteFile(tmpFile, []byte(strconv.Itoa(largePID)), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	pid, err := ReadPIDFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	if pid != largePID {
		t.Errorf("expected PID %d, got %d", largePID, pid)
	}
}

func TestReadPIDFile_LeadingZeros(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "zeros.pid")
	expectedPID := 123

	err := os.WriteFile(tmpFile, []byte("00123"), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	pid, err := ReadPIDFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	if pid != expectedPID {
		t.Errorf("expected PID %d, got %d", expectedPID, pid)
	}
}

func TestReadPIDFile_WhitespaceOnly(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "whitespace-only.pid")

	err := os.WriteFile(tmpFile, []byte("   \t\n  "), 0600)
	if err != nil {
		t.Fatalf("failed to write test PID file: %v", err)
	}

	_, err = ReadPIDFile(tmpFile)
	if err == nil {
		t.Error("expected error for whitespace-only content")
	}
}

func TestWriteAndReadPIDFile_RoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "roundtrip.pid")

	originalPID := os.Getpid()

	err := WritePIDFile(tmpFile)
	if err != nil {
		t.Fatalf("WritePIDFile failed: %v", err)
	}

	readPID, err := ReadPIDFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	if readPID != originalPID {
		t.Errorf("expected PID %d, got %d", originalPID, readPID)
	}
}

func TestWritePIDFile_Overwrite(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "overwrite.pid")

	// Write initial PID
	err := WritePIDFile(tmpFile)
	if err != nil {
		t.Fatalf("initial WritePIDFile failed: %v", err)
	}

	// Overwrite with new PID content
	newPID := 99999
	err = os.WriteFile(tmpFile, []byte(strconv.Itoa(newPID)), 0600)
	if err != nil {
		t.Fatalf("failed to overwrite PID file: %v", err)
	}

	readPID, err := ReadPIDFile(tmpFile)
	if err != nil {
		t.Fatalf("ReadPIDFile failed: %v", err)
	}

	if readPID != newPID {
		t.Errorf("expected PID %d, got %d", newPID, readPID)
	}
}