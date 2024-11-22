package agent

import "fmt"

type PluginFactory func() (Plugin, error)

type PluginManager struct {
	plugins         map[string]Plugin
	pluginFactories map[string]PluginFactory
}

func NewPluginManager() *PluginManager {
	return &PluginManager{
		plugins:         make(map[string]Plugin),
		pluginFactories: make(map[string]PluginFactory),
	}
}

func (pm *PluginManager) RegisterPlugin(name string, plugin Plugin) {
	pm.plugins[name] = plugin
}

func (pm *PluginManager) RegisterPluginFactory(name string, factory PluginFactory) {
	pm.pluginFactories[name] = factory
}

func (pm *PluginManager) LoadPlugins(pluginNames []string) error {
	for _, name := range pluginNames {
		factory, exists := pm.pluginFactories[name]
		if !exists {
			return fmt.Errorf("no factory found for plugin: %s", name)
		}
		plugin, err := factory()
		if err != nil {
			return fmt.Errorf("failed to create plugin: %s, error: %v", name, err)
		}
		pm.RegisterPlugin(name, plugin)
	}
	return nil
}

func (pm *PluginManager) GetPlugin(name string) (Plugin, bool) {
	plugin, exists := pm.plugins[name]
	return plugin, exists
}
