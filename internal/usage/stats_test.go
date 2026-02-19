package usage

import (
	"testing"
	"time"
)

func TestAggregateSummary(t *testing.T) {
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 100, Fallbacks: 0, Timestamp: time.Now()},
		{InstanceID: "inst1", Route: "/think", Model: "m2", Tokens: 200, Fallbacks: 1, Timestamp: time.Now()},
		{InstanceID: "inst1", Route: "/ultrathink", Model: "m1", Tokens: 300, Fallbacks: 0, Timestamp: time.Now()},
	}

	summary := AggregateSummary(records)

	if summary.TotalRequests != 3 {
		t.Errorf("requests = %d, want 3", summary.TotalRequests)
	}
	if summary.TotalTokens != 600 {
		t.Errorf("tokens = %d, want 600", summary.TotalTokens)
	}
	if summary.TotalFallbacks != 1 {
		t.Errorf("fallbacks = %d, want 1", summary.TotalFallbacks)
	}
}

func TestAggregateByRoute(t *testing.T) {
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 100, Fallbacks: 0},
		{InstanceID: "inst1", Route: "/think", Model: "m2", Tokens: 200, Fallbacks: 1},
		{InstanceID: "inst1", Route: "/ultrathink", Model: "m1", Tokens: 300, Fallbacks: 0},
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 50, Fallbacks: 0},
	}

	byRoute := AggregateByRoute(records)

	// Should have 2 routes
	if len(byRoute) != 2 {
		t.Fatalf("expected 2 routes, got %d", len(byRoute))
	}

	// Check /think
	think := byRoute["/think"]
	if think.Requests != 3 {
		t.Errorf("/think requests = %d, want 3", think.Requests)
	}
	if think.Tokens != 350 {
		t.Errorf("/think tokens = %d, want 350", think.Tokens)
	}
	if think.Fallbacks != 1 {
		t.Errorf("/think fallbacks = %d, want 1", think.Fallbacks)
	}

	// Check /ultrathink
	ultra := byRoute["/ultrathink"]
	if ultra.Requests != 1 {
		t.Errorf("/ultrathink requests = %d, want 1", ultra.Requests)
	}
	if ultra.Tokens != 300 {
		t.Errorf("/ultrathink tokens = %d, want 300", ultra.Tokens)
	}
}

func TestAggregateByModel(t *testing.T) {
	records := []*Record{
		{InstanceID: "inst1", Route: "/think", Model: "m1", Tokens: 100, Fallbacks: 0},
		{InstanceID: "inst1", Route: "/think", Model: "m2", Tokens: 200, Fallbacks: 1},
		{InstanceID: "inst1", Route: "/ultrathink", Model: "m1", Tokens: 300, Fallbacks: 0},
	}

	byModel := AggregateByModel(records)

	// Should have 2 models
	if len(byModel) != 2 {
		t.Fatalf("expected 2 models, got %d", len(byModel))
	}

	// Check m1
	m1 := byModel["m1"]
	if m1.Requests != 2 {
		t.Errorf("m1 requests = %d, want 2", m1.Requests)
	}
	if m1.Tokens != 400 {
		t.Errorf("m1 tokens = %d, want 400", m1.Tokens)
	}

	// Check m2
	m2 := byModel["m2"]
	if m2.Requests != 1 {
		t.Errorf("m2 requests = %d, want 1", m2.Requests)
	}
	if m2.Tokens != 200 {
		t.Errorf("m2 tokens = %d, want 200", m2.Tokens)
	}
}
