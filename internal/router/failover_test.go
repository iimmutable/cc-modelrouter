package router

import (
	"testing"
)

func TestFailoverMaxAttempts(t *testing.T) {
	targets := []Target{
		&mockTarget{provider: "p1", model: "m1"},
		&mockTarget{provider: "p2", model: "m2"},
	}

	fo := NewFailoverFromTargets(targets)

	// Max attempts should be 2x the number of targets
	expected := len(targets) * 2
	if fo.MaxAttempts() != expected {
		t.Errorf("expected max attempts %d, got %d", expected, fo.MaxAttempts())
	}
}

func TestFailoverIteration(t *testing.T) {
	targets := []Target{
		&mockTarget{provider: "p1", model: "m1"},
		&mockTarget{provider: "p2", model: "m2"},
	}

	fo := NewFailoverFromTargets(targets)

	// First iteration
	t1 := fo.Next()
	if t1.Provider() != "p1" {
		t.Errorf("expected p1, got %s", t1.Provider())
	}

	// Second
	t2 := fo.Next()
	if t2.Provider() != "p2" {
		t.Errorf("expected p2, got %s", t2.Provider())
	}

	// Loop back to first
	t3 := fo.Next()
	if t3.Provider() != "p1" {
		t.Errorf("expected p1 (loop), got %s", t3.Provider())
	}
}

func TestFailoverExhaustion(t *testing.T) {
	targets := []Target{
		&mockTarget{provider: "p1", model: "m1"},
		&mockTarget{provider: "p2", model: "m2"},
	}

	fo := NewFailoverFromTargets(targets)
	maxAttempts := fo.MaxAttempts()

	// Exhaust all attempts
	for i := 0; i < maxAttempts; i++ {
		target := fo.Next()
		if target == nil {
			t.Errorf("expected non-nil target at attempt %d", i)
		}
	}

	// Should return nil after max attempts
	target := fo.Next()
	if target != nil {
		t.Errorf("expected nil after max attempts, got %v", target)
	}
}

func TestFailoverHasMore(t *testing.T) {
	targets := []Target{
		&mockTarget{provider: "p1", model: "m1"},
	}

	fo := NewFailoverFromTargets(targets) // max 2 attempts

	if !fo.HasMore() {
		t.Error("expected HasMore to be true initially")
	}

	fo.Next()
	if !fo.HasMore() {
		t.Error("expected HasMore to be true after 1 attempt")
	}

	fo.Next()
	if fo.HasMore() {
		t.Error("expected HasMore to be false after max attempts")
	}
}

type mockTarget struct {
	provider string
	model    string
}

func (m *mockTarget) Provider() string { return m.provider }
func (m *mockTarget) Model() string    { return m.model }
