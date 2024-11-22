package hvapi

import (
	"TraceForge/internals/commons"
	"TraceForge/pkg/hvlib"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// Handler for listing providers
func (s *Server) ListProvidersHandler(w http.ResponseWriter, r *http.Request) {
	providerNames := make([]string, 0, len(s.Providers.Providers))
	for name := range s.Providers.Providers {
		providerNames = append(providerNames, name)
	}
	commons.WriteSuccessResponse(w, "", providerNames)
}

// Handler for listing VMs
func (s *Server) ListVMsHandler(w http.ResponseWriter, r *http.Request) {
	provider := s.getProviderFromRequest(w, r)
	if provider == nil {
		return
	}

	vms, err := provider.List()
	if err != nil {
		commons.WriteErrorResponse(w, err.Error(), http.StatusInternalServerError)
		return
	}
	commons.WriteSuccessResponse(w, "", vms)
}

// Handler for VM snapshots
func (s *Server) SnapshotsVMHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := s.getProviderFromRequest(w, r)
	if provider == nil {
		return
	}

	vmName := vars["vmname"]
	snapshots, err := provider.ListSnapshots(vmName)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if _, ok := err.(*hvlib.VmNotFoundError); ok {
			httpStatus = http.StatusNotFound
		}
		commons.WriteErrorResponse(w, err.Error(), httpStatus)
		return
	}
	commons.WriteSuccessResponse(w, "", snapshots)
}

// Generic handler for VM operations
func (s *Server) BasicVMHandler(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		provider := s.getProviderFromRequest(w, r)
		if provider == nil {
			return
		}

		vmName := vars["vmname"]
		var err error

		// Perform action
		switch action {
		case "start":
			err = provider.Start(vmName)
		case "stop":
			err = provider.Stop(vmName, true)
		case "suspend":
			err = provider.Suspend(vmName)
		case "revert":
			err = provider.Revert(vmName)
		case "reset":
			err = provider.Reset(vmName)
		case "snapshot":
			snapshotName := vars["snapshotname"]
			if r.Method == "DELETE" {
				err = provider.DeleteSnapshot(vmName, snapshotName)
			} else {
				err = provider.TakeSnapshot(vmName, snapshotName)
			}
		default:
			commons.WriteErrorResponse(w, "invalid action", http.StatusBadRequest)
			return
		}

		if err != nil {
			httpStatus := http.StatusInternalServerError
			if _, ok := err.(*hvlib.VmNotFoundError); ok {
				httpStatus = http.StatusNotFound
			}
			commons.WriteErrorResponse(w, err.Error(), httpStatus)
			return
		}

		commons.WriteSuccessResponse(w,
			fmt.Sprintf("%s on %s completed successfully",
				action, vmName),
			nil)
	}
}
