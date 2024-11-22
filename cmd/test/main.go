package main

import (
	"fmt"
	"hvapi/pkg/hvlib"
	"log"
	"time"
)

func manageVM(provider hvlib.VirtualizationProvider, vmName string) {
	err := provider.Start(vmName)
	if err != nil {
		fmt.Printf("Error starting VM %s: %v\n", vmName, err)
		return
	}
	fmt.Printf("VM %s started successfully.\n", vmName)

	vms, _ := provider.List()
	for _, vm := range vms {
		if vm.Name == vmName && vm.State == "running" {
			fmt.Printf("VM %s is running.\n", vmName)
		}
	}
	time.Sleep(3 * time.Second)
	err = provider.Suspend(vmName)
	if err != nil {
		fmt.Printf("Error stopping VM %s: %v\n", vmName, err)
		return
	}
	fmt.Printf("VM %s stopped successfully.\n", vmName)

	vms, _ = provider.List()
	for _, vm := range vms {
		if vm.Name == vmName {
			fmt.Printf("VM %s is %s.\n", vmName, vm.State)
		}
	}
	time.Sleep(3 * time.Second)
}

func snapVM(provider hvlib.VirtualizationProvider, vmName string) {
	err := provider.TakeSnapshot(vmName, "test1")
	if err != nil {
		fmt.Printf("%s\n", err)
	}
	time.Sleep(3 * time.Second)
	provider.RestoreSnapshot(vmName, "test1")
}

// Main function
func main() {
	configPath := "config.toml" // Replace with actual path
	configLoader, err := hvlib.NewConfigLoader(configPath)
	if err != nil {
		log.Fatalf("Error loading configuration: %v", err)
	}

	// Create instances of the providers
	vmware := &hvlib.VmwareVP{}
	hyperv := &hvlib.HypervVP{}

	// Load data into the providers
	if err := vmware.LoadVMs(configLoader); err != nil {
		log.Fatalf("Error loading VMware VMs: %v", err)
	}
	if err := hyperv.LoadVMs(configLoader); err != nil {
		log.Fatalf("Error loading Hyper-V VMs: %v", err)
	}

	// Print results
	fmt.Printf("VmwareVP: %+v\n", vmware)
	fmt.Printf("HypervVP: %+v\n", hyperv)

	vmwareVMs, _ := vmware.List()
	hypervVMs, _ := hyperv.List()

	fmt.Println("VMware VMs:", vmwareVMs)
	fmt.Println("Hyper-V VMs:", hypervVMs)

	// snapVM(vmware, "lab-win10-anti")
	//snapVM(hyperv, "sandbox-win10-001")
	//
	// // VMware Snapshots
	// vmwareSnapshots, _ := vmware.ListSnapshots("lab-win10-anti")
	// for _, snap := range vmwareSnapshots {
	// 	fmt.Printf("VMware Snapshot: %+v\n", snap)
	// }
	//
	// Hyper-V Snapshots
	hypervSnapshots, _ := hyperv.ListSnapshots("sandbox-win10-001")
	for _, snap := range hypervSnapshots {
		fmt.Printf("Hyper-V Snapshot: %+v\n", snap)
	}

	// hyperv.Revert("sandbox-win10-001")
	vmware.Revert("lab-win10-anti")

	// manageVM(vmware, "lab-win10-anti")
	// manageVM(hyperv, "sandbox-win10-001")
	//
}
