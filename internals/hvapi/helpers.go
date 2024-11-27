package hvapi

import (
	"TraceForge/internals/commons"
	"TraceForge/pkg/hvlib"
	"net/http"
	"sync"

	"github.com/gorilla/mux"
)

// Helper: Get provider from request
func (s *Server) getProviderFromRequest(w http.ResponseWriter, r *http.Request) hvlib.VirtualizationProvider {
	vars := mux.Vars(r)
	providerName := vars["provider"]

	provider := s.Providers.GetProvider(providerName)
	if provider == nil {
		commons.WriteErrorResponse(w, "Provider not found", http.StatusNotFound)
		return nil
	}
	return provider
}

// AcquireLock acquires the mutex for the given vmName.
// If the mutex doesn't exist, it creates one.
func (s *Server) AcquireLock(vmName string) {
	// Attempt to load the mutex from the map
	lockInterface, _ := s.vmLocks.LoadOrStore(vmName, &sync.Mutex{})
	lock := lockInterface.(*sync.Mutex)

	// If the mutex was already present, loaded is true
	// In either case, lock the mutex
	lock.Lock()
}

// TryAcquireLock attempts to acquire the lock for the given vmName.
// Returns true if the lock was acquired, false otherwise.
func (s *Server) TryAcquireLock(vmName string) bool {
	// Attempt to load or create a new Mutex for the VM
	lockInterface, _ := s.vmLocks.LoadOrStore(vmName, &sync.Mutex{})
	lock := lockInterface.(*sync.Mutex)

	return lock.TryLock()
}

// ReleaseLock releases the mutex for the given vmName.
func (s *Server) ReleaseLock(vmName string) {
	// Load the mutex from the map
	lockInterface, exists := s.vmLocks.Load(vmName)
	if !exists {
		// This should not happen; log a warning
		s.Logger.Warnf("Attempted to release a lock for vm '%s' which does not exist", vmName)
		return
	}

	lock := lockInterface.(*sync.Mutex)
	lock.Unlock()

	// Could delete the mutex to prevent the  map from growing
	// but since the number if VM don't change is not necessary
	// This is also prone to error if a goroutine is waiting on the mutex
	// s.vmLocks.Delete(vmName)
}
