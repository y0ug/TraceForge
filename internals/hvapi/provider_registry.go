package hvapi

import (
	"TraceForge/pkg/hvlib"
	"log"
)

// Register a provider
func (pr *ProviderRegistry) RegisterProvider(name string, provider hvlib.VirtualizationProvider) {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	pr.Providers[name] = provider
}

// Get a provider
func (pr *ProviderRegistry) GetProvider(name string) hvlib.VirtualizationProvider {
	pr.mu.Lock()
	defer pr.mu.Unlock()
	return pr.Providers[name]
}

// Initializes a provider and loads VMs
func InitializeProvider(provider hvlib.VirtualizationProvider, loader *hvlib.ConfigLoader, name string) hvlib.VirtualizationProvider {
	if err := provider.LoadVMs(loader); err != nil {
		log.Fatalf("Error loading %s VMs: %v", name, err)
	}
	return provider
}

func NewProvider() *ProviderRegistry {
	providers := make(map[string]hvlib.VirtualizationProvider)
	return &ProviderRegistry{Providers: providers}
}
