package transformer

import (
	"fmt"
	"sync"
)

// Registry manages transformer instances.
type Registry struct {
	mu           sync.RWMutex
	transformers map[string]Transformer
}

// NewRegistry creates a new transformer registry.
func NewRegistry() *Registry {
	return &Registry{
		transformers: make(map[string]Transformer),
	}
}

// Register adds a transformer to the registry.
func (r *Registry) Register(t Transformer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.transformers[t.Name()] = t
}

// Get retrieves a transformer by name.
func (r *Registry) Get(name string) (Transformer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.transformers[name]
	if !ok {
		return nil, fmt.Errorf("transformer not found: %s", name)
	}
	return t, nil
}

// Has checks if a transformer exists.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.transformers[name]
	return ok
}

// Names returns all registered transformer names.
func (r *Registry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.transformers))
	for name := range r.transformers {
		names = append(names, name)
	}
	return names
}
