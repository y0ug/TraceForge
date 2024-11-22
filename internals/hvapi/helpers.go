package hvapi

import (
	"TraceForge/internals/commons"
	"TraceForge/pkg/hvlib"
	"net/http"

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
