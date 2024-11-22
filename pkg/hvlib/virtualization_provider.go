package hvlib

func (v *VP) LoadVMs(loader *ConfigLoader, provider string) error {
	v.VMs = make(map[string]VM)

	vmNodes := loader.GetSubTree(provider + ".vm")
	if vmNodes != nil {
		for key, node := range vmNodes.ToMap() {
			vmTree := node.(map[string]interface{})
			vm := VM{
				ID: vmTree["id"].(string),
			}
			if path, ok := vmTree["path"]; ok {
				vm.Path = path.(string)
			}
			v.VMs[key] = vm
		}
	}
	return nil
}
