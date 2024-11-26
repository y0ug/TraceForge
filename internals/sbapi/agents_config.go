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

	hvapiServers := make(map[string]HvapiAgentsConfig)
	for name, hvapi := range agentsConfig.Hvapi {
		hvapiServers[name] = hvapi
	}

	// Apply defaults and resolve HVAPI servers for agents
	for i := range agentsConfig.Agents {
		agent := &agentsConfig.Agents[i]
		if len(agent.Plugins) == 0 {
			agent.Plugins = agentsConfig.AgentDefaults.Plugins
		}
		if agent.HvapiName == "" {
			agent.HvapiName = agentsConfig.AgentDefaults.HvapiName
		}
		if agent.Provider == "" {
			agent.Provider = agentsConfig.AgentDefaults.Provider
		}
		// Assign the HVAPI server configuration to the agent
		hvapiConfig, exists := hvapiServers[agent.HvapiName]
		if !exists {
			return nil, fmt.Errorf("HVAPI server %s not found for agent %s", agent.HvapiName, agent.Name)
		}
		agent.HvapiConfig = hvapiConfig
	}

	return agentsConfig, nil
}
