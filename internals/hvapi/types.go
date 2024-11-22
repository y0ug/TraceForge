package hvapi

import (
	"TraceForge/internals/commons"
	"TraceForge/pkg/hvlib"
	"sync"
)

// Define a struct to hold provider instances
type ProviderRegistry struct {
	Providers map[string]hvlib.VirtualizationProvider
	mu        sync.Mutex
}

type Server struct {
	*commons.Server
	Providers *ProviderRegistry
	AuthToken string
}
