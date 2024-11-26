package hvlib

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (v *VmwareVP) LoadVMs(loader *ConfigLoader) error {
	v.InstallPath = loader.GetString("vmware.install_path")
	v.VMPath = loader.GetString("vmware.vm_path")
	v.VP.VMs = make(map[string]VM)

	// Scan VMPath for .vmx files
	err := filepath.Walk(v.VMPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// If it's a .vmx file, add it to the VMs map
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".vmx") {
			vmName := strings.TrimSuffix(info.Name(), ".vmx")
			v.VP.VMs[vmName] = VM{
				Path: path,
				// ID is optional or can be generated
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan VMs: %v", err)
	}

	return nil
}

func (v *VmwareVP) List() ([]VMStatus, error) {
	output, err := v.ExecVmrun("list")
	if err != nil {
		return nil, fmt.Errorf("%s (%v)", output, err)
	}
	// Parse the output of vmrun list
	lines := strings.Split(string(output), "\n")
	runningVMs := make(map[string]bool)
	for _, line := range lines[1:] { // Skip the first line: "Total running VMs: X"
		line = strings.TrimSpace(line)
		if line != "" {
			runningVMs[line] = true
		}
	}

	// Check registered VMs and determine their states
	var vms []VMStatus
	for vmName, vm := range v.VMs {
		state := "stopped" // Default state
		if runningVMs[vm.Path] {
			state = "running"
		} else {
			// If not running, check the checkpoint.vmState in the .vmx file
			vmxContent, err := os.ReadFile(vm.Path)
			if err != nil {
				return nil, fmt.Errorf("failed to read .vmx file for VM %s: %v", vmName, err)
			}

			vmxLines := strings.Split(string(vmxContent), "\n")
			for _, vmxLine := range vmxLines {
				if strings.HasPrefix(vmxLine, "checkpoint.vmState") {
					// Parse the vmState value
					parts := strings.SplitN(vmxLine, "=", 2)
					if len(parts) == 2 {
						vmState := strings.Trim(strings.TrimSpace(parts[1]), "\"")
						if vmState != "" {
							state = "suspended"
						}
					}
					break
				}
			}
		}

		// Append VM details
		vms = append(vms, VMStatus{
			ID:    vm.ID,
			Name:  vmName,
			State: state,
		})
	}

	return vms, nil
}

func (v *VmwareVP) ListSnapshots(vmName string) ([]Snapshot, error) {
	return v.listSnapshots(vmName, "listSnapshots")
}

// listSnapshots is a helper function for listing snapshots (common for VMware)
func (v *VmwareVP) listSnapshots(vmName, command string) ([]Snapshot, error) {
	vm, exists := v.VMs[vmName]
	if !exists {
		return nil, &VmNotFoundError{VmName: vmName}
	}

	output, err := v.ExecVmrun(command, vm.Path)
	if err != nil {
		return nil, fmt.Errorf("%s (%v)", output, err)
	}

	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return nil, nil // No snapshots
	}

	var snapshots []Snapshot
	for _, line := range lines[1:] { // Skip the first line (total count)
		name := strings.TrimSpace(line)
		if name != "" {
			snapshots = append(snapshots, Snapshot{Name: name})
		}
	}
	return snapshots, nil
}

func (v *VmwareVP) Start(vmName string) error {
	return v.execVmCommand(vmName, "start")
}

func (v *VmwareVP) Stop(vmName string, force bool) error {
	forceCmd := map[bool]string{true: "hard", false: "soft"}[force]
	return v.execVmCommand(vmName, "stop", forceCmd)
}

func (v *VmwareVP) Suspend(vmName string) error {
	return v.execVmCommand(vmName, "suspend")
}

func (v *VmwareVP) Reset(vmName string) error {
	return v.execVmCommand(vmName, "reset")
}

// execVmCommand is a helper function for executing vmrun commands (common for VMware)
func (v *VmwareVP) execVmCommand(vmName, command string, extraArgs ...string) error {
	vm, exists := v.VMs[vmName]
	if !exists {
		return &VmNotFoundError{VmName: vmName}
	}

	args := append([]string{command, vm.Path}, extraArgs...)
	output, err := v.ExecVmrun(args...)
	if err != nil {
		return fmt.Errorf("%s (%v)", output, err)
	}
	return nil
}

func (v *VmwareVP) ExecVmrun(args ...string) (string, error) {
	vmrunPath := filepath.Join(v.InstallPath, "vmrun.exe")
	cmd := exec.Command(vmrunPath,
		append([]string{"-T", "ws"}, args...)...)

	stdOutBuf, stdErrBuf := new(strings.Builder), new(strings.Builder)
	cmd.Stdout = stdOutBuf
	cmd.Stderr = stdErrBuf

	err := cmd.Run()
	// strStdout := stdOutBuf.String()
	strStdout := strings.TrimSuffix(stdOutBuf.String(), "\r\n")
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			// Error are in stdout not stderr
			return strStdout, fmt.Errorf("exit status: %08x", exiterr.ExitCode())
		} else {
			return "", fmt.Errorf("cmd.Wait %v", err)
		}
	}

	return strStdout, nil
}

func (v *VmwareVP) TakeSnapshot(vmName, snapshotName string) error {
	return v.execVmCommand(vmName, "snapshot", snapshotName)
}

func (v *VmwareVP) DeleteSnapshot(vmName, snapshotName string) error {
	return v.execVmCommand(vmName, "deleteSnapshot", snapshotName)
}

func (v *VmwareVP) RestoreSnapshot(vmName, snapshotName string) error {
	return v.execVmCommand(vmName, "revertToSnapshot", snapshotName)
}

func (v *VmwareVP) Revert(vmName string) error {
	snapshots, err := v.ListSnapshots(vmName)
	if err != nil {
		return err
	}
	snapshotName := snapshots[len(snapshots)-1].Name
	return v.execVmCommand(vmName, "revertToSnapshot", snapshotName)
}
