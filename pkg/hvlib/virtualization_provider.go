package hvlib

func (v *VP) LoadVMs(loader *ConfigLoader, provider string) error {
	v.VMs = make(map[string]VM)

	vmNodes := loader.GetSubTree(provider + ".vm")
	if vmNodes != nil {
		for key, node := range vmNodes.ToMap() {
			vmTree := node.(map[string]interface{})
			pluginsInterface := vmTree["plugins"].([]interface{})
			plugins := make([]string, len(pluginsInterface))
			for i, plugin := range pluginsInterface {
				plugins[i] = plugin.(string)
			}
			vm := VM{
				ID:      vmTree["id"].(string),
				AgentID: vmTree["agent_uuid"].(string),
				Plugins: plugins,
			}
			if path, ok := vmTree["path"]; ok {
				vm.Path = path.(string)
			}
			v.VMs[key] = vm
		}
	}
	return nil
}
