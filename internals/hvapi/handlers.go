package hvapi

import (
	"TraceForge/internals/commons"
	"TraceForge/pkg/hvlib"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
)

// ListProvidersHandler godoc
// @Summary List available providers
// @Description Get a list of available virtualization providers
// @Tags providers
// @Accept  json
// @Produce  json
// @Success 200 {object} commons.HttpResp
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /providers [get]
func (s *Server) ListProvidersHandler(w http.ResponseWriter, r *http.Request) {
	providerNames := make([]string, 0, len(s.Providers.Providers))
	for name := range s.Providers.Providers {
		providerNames = append(providerNames, name)
	}
	commons.WriteSuccessResponse(w, "", providerNames)
}

// ListVMsHandler godoc
// @Summary List virtual machines
// @Description Get a list of virtual machines for a given provider
// @Tags vms
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider} [get]
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

// SnapshotsVMHandler godoc
// @Summary List snapshots of a virtual machine
// @Description Get a list of snapshots for a specific virtual machine
// @Tags snapshots
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Param vmname path string true "Virtual Machine name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider}/{vmname}/snapshots [get]
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

// StartVMHandler godoc
// @Summary Start a virtual machine
// @Description Start a specific virtual machine
// @Tags vms
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Param vmname path string true "Virtual Machine name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 409 {object} commons.HttpResp // Conflict
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider}/{vmname}/start [get]
func (s *Server) StartVMHandler(w http.ResponseWriter, r *http.Request) {
	s.basicVMActionHandler(w, r, "start")
}

// StopVMHandler godoc
// @Summary Stop a virtual machine
// @Description Stop a specific virtual machine
// @Tags vms
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Param vmname path string true "Virtual Machine name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 409 {object} commons.HttpResp // Conflict
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider}/{vmname}/stop [get]
func (s *Server) StopVMHandler(w http.ResponseWriter, r *http.Request) {
	s.basicVMActionHandler(w, r, "stop")
}

// SuspendVMHandler godoc
// @Summary Suspend a virtual machine
// @Description Suspend a specific virtual machine
// @Tags vms
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Param vmname path string true "Virtual Machine name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 409 {object} commons.HttpResp // Conflict
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider}/{vmname}/suspend [get]
func (s *Server) SuspendVMHandler(w http.ResponseWriter, r *http.Request) {
	s.basicVMActionHandler(w, r, "suspend")
}

// RevertVMHandler godoc
// @Summary Revert a virtual machine
// @Description Revert a specific virtual machine
// @Tags vms
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Param vmname path string true "Virtual Machine name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 409 {object} commons.HttpResp // Conflict
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider}/{vmname}/revert [get]
func (s *Server) RevertVMHandler(w http.ResponseWriter, r *http.Request) {
	s.basicVMActionHandler(w, r, "revert")
}

// ResetVMHandler godoc
// @Summary Reset a virtual machine
// @Description Reset a specific virtual machine
// @Tags vms
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Param vmname path string true "Virtual Machine name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 409 {object} commons.HttpResp // Conflict
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider}/{vmname}/reset [get]
func (s *Server) ResetVMHandler(w http.ResponseWriter, r *http.Request) {
	s.basicVMActionHandler(w, r, "reset")
}

// TakeSnapshotHandler godoc
// @Summary Take a snapshot of a virtual machine
// @Description Take a snapshot with the specified name for a specific virtual machine
// @Tags snapshots
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Param vmname path string true "Virtual Machine name"
// @Param snapshotname path string true "Snapshot name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 409 {object} commons.HttpResp // Conflict
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider}/{vmname}/snapshot/{snapshotname} [get]
func (s *Server) TakeSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := s.getProviderFromRequest(w, r)
	if provider == nil {
		return
	}

	vmName := vars["vmname"]
	snapshotName := vars["snapshotname"]

	// Acquire the lock for this VM
	if !s.TryAcquireLock(vmName) {
		commons.WriteErrorResponse(w, "Another operation is in progress for this VM", http.StatusConflict)
		return
	}
	defer s.ReleaseLock(vmName) // Ensure the lock is released

	err := provider.TakeSnapshot(vmName, snapshotName)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if _, ok := err.(*hvlib.VmNotFoundError); ok {
			httpStatus = http.StatusNotFound
		}
		commons.WriteErrorResponse(w, err.Error(), httpStatus)
		return
	}

	commons.WriteSuccessResponse(w,
		fmt.Sprintf("Snapshot %s taken for VM %s",
			snapshotName, vmName),
		nil)
}

// DeleteSnapshotHandler godoc
// @Summary Delete a snapshot of a virtual machine
// @Description Delete a snapshot with the specified name for a specific virtual machine
// @Tags snapshots
// @Accept  json
// @Produce  json
// @Param provider path string true "Provider name"
// @Param vmname path string true "Virtual Machine name"
// @Param snapshotname path string true "Snapshot name"
// @Success 200 {object} commons.HttpResp
// @Failure 400 {object} commons.HttpResp
// @Failure 404 {object} commons.HttpResp
// @Failure 409 {object} commons.HttpResp // Conflict
// @Failure 500 {object} commons.HttpResp
// @Security ApiKeyAuth
// @Router /{provider}/{vmname}/snapshot/{snapshotname} [delete]
func (s *Server) DeleteSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	provider := s.getProviderFromRequest(w, r)
	if provider == nil {
		return
	}

	vmName := vars["vmname"]
	snapshotName := vars["snapshotname"]

	// Acquire the lock for this VM
	if !s.TryAcquireLock(vmName) {
		commons.WriteErrorResponse(w, "Another operation is in progress for this VM", http.StatusConflict)
		return
	}
	defer s.ReleaseLock(vmName) // Ensure the lock is released

	err := provider.DeleteSnapshot(vmName, snapshotName)
	if err != nil {
		httpStatus := http.StatusInternalServerError
		if _, ok := err.(*hvlib.VmNotFoundError); ok {
			httpStatus = http.StatusNotFound
		}
		commons.WriteErrorResponse(w, err.Error(), httpStatus)
		return
	}

	commons.WriteSuccessResponse(w,
		fmt.Sprintf("Snapshot %s deleted for VM %s",
			snapshotName, vmName),
		nil)
}

// Helper function for basic VM actions
func (s *Server) basicVMActionHandler(w http.ResponseWriter, r *http.Request, action string) {
	vars := mux.Vars(r)
	provider := s.getProviderFromRequest(w, r)
	if provider == nil {
		return
	}

	vmName := vars["vmname"]

	// Acquire the lock for this VM
	if !s.TryAcquireLock(vmName) {
		commons.WriteErrorResponse(w, "Another operation is in progress for this VM", http.StatusConflict)
		return
	}
	defer s.ReleaseLock(vmName)
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
