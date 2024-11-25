package sbapi

import (
	"fmt"

	"github.com/pelletier/go-toml"
)

func LoadAgentsConfig(filePath string) (*AgentsConfig, error) {
	// Load the TOML file
	configTree, err := toml.LoadFile(filePath)
	if err != nil {
		fmt.Println("Error loading TOML file:", err)
		return nil, err
	}

	// Initialize the AgentsConfig struct
	agentsConfig := &AgentsConfig{}

	// Unmarshal the TOML tree into the struct
	err = configTree.Unmarshal(agentsConfig)
	if err != nil {
		fmt.Println("Error unmarshaling TOML into struct:", err)
		return nil, err
	}

	// Apply defaults to agents
	applyDefaults(agentsConfig)

	// Now you can access your configuration
	fmt.Println("Hvapi Config:")
	for name, hvapi := range agentsConfig.Hvapi {
		fmt.Printf("  %s: %+v\n", name, hvapi)
	}

	fmt.Println("Agents:")
	for _, agent := range agentsConfig.Agents {
		fmt.Printf("  Name: %s\n", agent.Name)
		fmt.Printf("    UUID: %s\n", agent.AgentUUID)
		fmt.Printf("    Provider: %s\n", agent.Provider)
		fmt.Printf("    Plugins: %v\n", agent.Plugins)
		fmt.Printf("    HvName: %s\n", agent.HvapiName)
	}

	return agentsConfig, nil
}

func applyDefaults(config *AgentsConfig) {
	for i := range config.Agents {
		agent := &config.Agents[i]
		if len(agent.Plugins) == 0 {
			agent.Plugins = config.AgentDefaults.Plugins
		}
		if agent.HvapiName == "" {
			agent.HvapiName = config.AgentDefaults.HvapiName
		}
		if agent.Provider == "" {
			agent.Provider = config.AgentDefaults.Provider
		}
	}
}
