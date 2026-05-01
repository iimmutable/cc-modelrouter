// Package transformer provides the base transformer implementation.
package transformer

import (
	"encoding/json"
	"fmt"
)

// BaseTransformer provides common utilities for transformers.
type BaseTransformer struct {
	name string
}

// NewBaseTransformer creates a new base transformer.
func NewBaseTransformer(name string) *BaseTransformer {
	return &BaseTransformer{name: name}
}

// Name returns the transformer name.
func (b *BaseTransformer) Name() string {
	return b.name
}

// MarshalSSEEvent creates an SSEEvent from the provided data.
func (b *BaseTransformer) MarshalSSEEvent(eventType string, data any) (SSEEvent, error) {
	jsonData, err := json.Marshal(data)
	if err != nil {
		return SSEEvent{}, fmt.Errorf("failed to marshal %s event: %w", eventType, err)
	}
	if len(jsonData) == 0 {
		return SSEEvent{}, fmt.Errorf("%s event marshaled to empty JSON", eventType)
	}
	return SSEEvent{EventType: eventType, Data: jsonData}, nil
}