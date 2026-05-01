// Package transformer tests for base transformer utilities.
package transformer

import (
	"encoding/json"
	"testing"
)

func TestNewBaseTransformer(t *testing.T) {
	name := "test-transformer"
	bt := NewBaseTransformer(name)

	if bt == nil {
		t.Fatal("NewBaseTransformer returned nil")
	}

	if bt.Name() != name {
		t.Errorf("Expected name %q, got %q", name, bt.Name())
	}
}

func TestBaseTransformer_Name(t *testing.T) {
	tests := []struct {
		name       string
		transformer string
	}{
		{"openai", "openai"},
		{"anthropic", "anthropic"},
		{"gemini", "gemini"},
		{"openrouter", "openrouter"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bt := NewBaseTransformer(tt.transformer)
			if got := bt.Name(); got != tt.transformer {
				t.Errorf("Name() = %v, want %v", got, tt.transformer)
			}
		})
	}
}

func TestMarshalSSEEvent(t *testing.T) {
	tests := []struct {
		name      string
		eventType string
		data      any
		wantData  string
		wantErr   bool
	}{
		{
			name:      "message_start event",
			eventType: "message_start",
			data: map[string]any{
				"type":  "message_start",
				"message": map[string]any{
					"id":    "msg_123",
					"type":  "message",
					"role":  "assistant",
					"model": "claude-3-opus",
				},
			},
			wantErr: false,
		},
		{
			name:      "content_block_delta event",
			eventType: "content_block_delta",
			data: map[string]any{
				"type":  "content_block_delta",
				"index": 0,
				"delta": map[string]any{
					"type": "text_delta",
					"text": "Hello",
				},
			},
			wantErr: false,
		},
		{
			name:      "simple string",
			eventType: "ping",
			data:      "test",
			wantErr:   false,
		},
		{
			name:      "nil data",
			eventType: "error",
			data:      nil,
			wantErr:   false, // json.Marshal handles nil
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bt := NewBaseTransformer("test")

			event, err := bt.MarshalSSEEvent(tt.eventType, tt.data)

			if (err != nil) != tt.wantErr {
				t.Errorf("MarshalSSEEvent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if event.EventType != tt.eventType {
				t.Errorf("EventType = %v, want %v", event.EventType, tt.eventType)
			}

			// Verify the data is valid JSON
			if !tt.wantErr {
				var v any
				if err := json.Unmarshal(event.Data, &v); err != nil {
					t.Errorf("Data is not valid JSON: %v", err)
				}
			}
		})
	}
}

func TestMarshalSSEEvent_EmptyStruct(t *testing.T) {
	bt := NewBaseTransformer("test")

	// Create a type that marshals to empty object
	type emptyStruct struct{}

	// Empty structs marshal to "{}" which is valid JSON, so this should succeed
	event, err := bt.MarshalSSEEvent("test", emptyStruct{})
	if err != nil {
		t.Errorf("Expected success for empty struct, got error: %v", err)
	}
	if event.EventType != "test" {
		t.Errorf("Expected event type 'test', got %q", event.EventType)
	}
}

func TestMarshalSSEEvent_Unmarshalable(t *testing.T) {
	bt := NewBaseTransformer("test")

	// Create an unmarshalable type (channel)
	_, err := bt.MarshalSSEEvent("test", make(chan int))
	if err == nil {
		t.Error("Expected error for unmarshalable type, got nil")
	}
}

func TestSSEEvent_Struct(t *testing.T) {
	event := SSEEvent{
		EventType: "content_block_delta",
		Data:      []byte(`{"type":"text_delta","text":"Hello"}`),
	}

	if event.EventType != "content_block_delta" {
		t.Errorf("EventType = %v, want %v", event.EventType, "content_block_delta")
	}

	if string(event.Data) != `{"type":"text_delta","text":"Hello"}` {
		t.Errorf("Data = %v, want %v", string(event.Data), `{"type":"text_delta","text":"Hello"}`)
	}
}

func TestProvider_Struct(t *testing.T) {
	provider := Provider{
		BaseURL: "https://api.example.com",
		APIKey:  "sk-test123",
		Model:   "gpt-4",
	}

	if provider.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %v, want %v", provider.BaseURL, "https://api.example.com")
	}

	if provider.APIKey != "sk-test123" {
		t.Errorf("APIKey = %v, want %v", provider.APIKey, "sk-test123")
	}

	if provider.Model != "gpt-4" {
		t.Errorf("Model = %v, want %v", provider.Model, "gpt-4")
	}
}