package connector

import (
	"fmt"
	"sync"
)

// Factory creates a Connector from decrypted credentials.
type Factory func(creds Credentials) (Connector, error)

// Registry manages connector factories per provider.
type Registry struct {
	mu        sync.RWMutex
	factories map[string]Factory
}

// NewRegistry creates an empty connector registry.
func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]Factory),
	}
}

// Register adds a connector factory for a provider.
func (r *Registry) Register(provider string, factory Factory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[provider] = factory
}

// Create instantiates a connector for the given provider using the provided credentials.
func (r *Registry) Create(provider string, creds Credentials) (Connector, error) {
	r.mu.RLock()
	factory, ok := r.factories[provider]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("unknown provider: %s", provider)
	}
	return factory(creds)
}

// Providers returns the list of registered provider names.
func (r *Registry) Providers() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	providers := make([]string, 0, len(r.factories))
	for p := range r.factories {
		providers = append(providers, p)
	}
	return providers
}

// Has checks if a provider is registered.
func (r *Registry) Has(provider string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[provider]
	return ok
}
