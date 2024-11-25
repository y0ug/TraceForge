package hvlib

import (
	"fmt"
	"time"
)

type VMStatus struct {
	ID    string
	Name  string
	State string
}

type Snapshot struct {
	ID           string
	Name         string
	CreationTime time.Time
}

type VM struct {
	ID      string
	Path    string
	AgentID string
	Plugins []string
}

type VmNotFoundError struct {
	VmName string
}

func (e *VmNotFoundError) Error() string {
	return fmt.Sprintf("vm %s not found", e.VmName)
}

type VirtualizationError struct {
	Operation string
	VMName    string
	Err       error
}

func (e *VirtualizationError) Error() string {
	return fmt.Sprintf("%s failed for VM %s: %v", e.Operation, e.VMName, e.Err)
}

type HypervVP struct {
	VP
}

type VP struct {
	VMs map[string]VM
}

type VmwareVP struct {
	VP
	InstallPath string
}

type VirtualizationProvider interface {
	LoadVMs(loader *ConfigLoader) error
	List() ([]VMStatus, error)
	ListSnapshots(vmName string) ([]Snapshot, error)
	TakeSnapshot(vmName, snapshotName string) error
	RestoreSnapshot(vmName, snapshotName string) error
	DeleteSnapshot(vmName, snapshotName string) error
	Start(vmName string) error
	Stop(vmName string, force bool) error
	Suspend(vmName string) error
	Reset(vmName string) error
	Revert(vmName string) error
}
