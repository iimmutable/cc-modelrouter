package transformer

import (
	"fmt"
	"sync"
)

// Registry manages transformer instances.
// Uses sync.Map for optimal read-heavy concurrency (write-once at startup,
// read on every request).
type Registry struct {
	m sync.Map // map[string]Transformer
}

// NewRegistry creates a new transformer registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a transformer to the registry.
func (r *Registry) Register(t Transformer) {
	r.m.Store(t.Name(), t)
}

// Get retrieves a transformer by name.
func (r *Registry) Get(name string) (Transformer, error) {
	v, ok := r.m.Load(name)
	if !ok {
		return nil, fmt.Errorf("transformer not found: %s", name)
	}
	return v.(Transformer), nil
}

// Has checks if a transformer exists.
func (r *Registry) Has(name string) bool {
	_, ok := r.m.Load(name)
	return ok
}

// Names returns all registered transformer names.
func (r *Registry) Names() []string {
	var names []string
	r.m.Range(func(key, _ any) bool {
		names = append(names, key.(string))
		return true
	})
	return names
}
