package hvlib

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

func (h *HypervVP) LoadVMs(loader *ConfigLoader) error {
	return h.VP.LoadVMs(loader, "hyperv")
}

func (h *HypervVP) List() ([]VMStatus, error) {
	return h.listVMs("Get-VM | Select-Object Id,Name,State | ConvertTo-Json")
}

// listVMs is a helper function for listing VMs (common for Hyper-V)
func (h *HypervVP) listVMs(command string) ([]VMStatus, error) {
	cmd := exec.Command("powershell", "-Command", command)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	// Parse JSON output
	var vms []struct {
		ID    string `json:"Id"`
		Name  string `json:"Name"`
		State int    `json:"State"`
	}
	if err := json.Unmarshal(output, &vms); err != nil {
		return nil, err
	}

	// Map state codes to human-readable states
	stateMap := map[int]string{
		2: "running",
		3: "stopped",
		6: "saved",
		9: "suspended",
	}

	// Match with the config
	var results []VMStatus
	for _, vm := range vms {
		if _, exists := h.VMs[vm.Name]; exists {
			results = append(results, VMStatus{
				ID:    vm.ID,
				Name:  vm.Name,
				State: stateMap[vm.State],
			})
		}
	}
	return results, nil
}

func (h *HypervVP) ListSnapshots(vmName string) ([]Snapshot, error) {
	cmd := exec.Command("powershell",
		"-Command",
		fmt.Sprintf("Get-VMSnapshot -VMName \"%s\" | Select-Object Id, Name, CreationTime | ConvertTo-Json", vmName))
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	// Parse JSON output
	var snapshots []struct {
		ID           string `json:"Id"`
		Name         string `json:"Name"`
		CreationTime string `json:"CreationTime"`
	}
	if err := json.Unmarshal(output, &snapshots); err != nil {
		return nil, err
	}

	// Convert to Snapshot struct
	var results []Snapshot
	for _, snap := range snapshots {
		creationTime, err := parseMicrosoftDate(snap.CreationTime)
		if err != nil {
			fmt.Printf("Error parsing date: %v\n", err)
			continue
		}
		results = append(results, Snapshot{
			ID:           snap.ID,
			Name:         snap.Name,
			CreationTime: creationTime,
		})
	}
	return results, nil
}

func (h *HypervVP) Start(vmName string) error {
	return h.execVmCommand(vmName, "Start-VM")
}

func (h *HypervVP) Stop(vmName string, force bool) error {
	forceCmd := map[bool]string{true: "-TurnOff", false: ""}[force]
	return h.execVmCommand(vmName, fmt.Sprintf("Stop-VM %s", forceCmd))
}

func (h *HypervVP) Suspend(vmName string) error {
	return h.execVmCommand(vmName, "Suspend-VM")
}

func (h *HypervVP) Reset(vmName string) error {
	return h.execVmCommand(vmName, "Reboot-VM")
}

func (h *HypervVP) TakeSnapshot(vmName, snapshotName string) error {
	snapshots, err := h.ListSnapshots(vmName)
	if err != nil {
		return err
	}
	for _, s := range snapshots {
		if s.Name == snapshotName {
			return fmt.Errorf("A snapshot with the name already exists")
		}
	}

	return h.execVmCommand(vmName,
		fmt.Sprintf("Checkpoint-VM -SnapshotName %s", snapshotName))
}

func (h *HypervVP) RestoreSnapshot(vmName, snapshotName string) error {
	return h.execVmCommand(vmName,
		fmt.Sprintf("Restore-VMSnapshot -Name %s -Confirm:$false", snapshotName))
}

func (h *HypervVP) DeleteSnapshot(vmName, snapshotName string) error {
	return h.execVmCommand(vmName,
		fmt.Sprintf("Remove-VMSnapshot -Name %s -Confirm:$false", snapshotName))
}

func (h *HypervVP) Revert(vmName string) error {
	_, exists := h.VMs[vmName]
	if !exists {
		return &VmNotFoundError{VmName: vmName}
	}

	cmd := exec.Command("powershell",
		"-Command",
		fmt.Sprintf("Get-VM -VMName %s | Get-VMSnapshot | Sort CreationTime | Select -Last 1 | Restore-VMSnapshot -Confirm:$false", vmName))
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to execute command on VM %s: %v, output: %s", vmName, err, string(output))
	}
	return nil
}

// execVmCommand is a helper function for executing Hyper-V commands
func (h *HypervVP) execVmCommand(vmName, command string) error {
	_, exists := h.VMs[vmName]
	if !exists {
		return &VmNotFoundError{VmName: vmName}
	}

	cmd := exec.Command("powershell",
		"-Command",
		fmt.Sprintf("%s -VMName %s", command, vmName))
	output, err := cmd.CombinedOutput()
	println(string(output))
	if err != nil {
		return fmt.Errorf("failed to execute command on VM %s: %v, output: %s", vmName, err, string(output))
	}
	return nil
}

func parseMicrosoftDate(msDate string) (time.Time, error) {
	// Use a regex to extract the numeric timestamp
	re := regexp.MustCompile(`/Date\((\d+)\)/`)
	matches := re.FindStringSubmatch(msDate)
	if len(matches) < 2 {
		return time.Time{}, fmt.Errorf("invalid Microsoft date format: %s", msDate)
	}

	// Convert the timestamp to an integer
	msTimestamp, err := strconv.ParseInt(matches[1], 10, 64)
	if err != nil {
		return time.Time{}, err
	}

	// Convert milliseconds to seconds and parse the Unix time
	return time.UnixMilli(msTimestamp), nil
}
