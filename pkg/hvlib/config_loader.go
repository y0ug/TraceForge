package hvlib

import (
	"fmt"
	"os"

	"github.com/pelletier/go-toml"
)

type ConfigLoader struct {
	config *toml.Tree
}

// NewConfigLoader initializes a ConfigLoader with a given file.
func NewConfigLoader(configPath string) (*ConfigLoader, error) {
	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	tree, err := toml.LoadBytes(file)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &ConfigLoader{config: tree}, nil
}

// Get retrieves a value from the configuration.
func (c *ConfigLoader) Get(key string) interface{} {
	return c.config.Get(key)
}

// GetString retrieves a string value from the configuration.
func (c *ConfigLoader) GetString(key string) string {
	if value, ok := c.config.Get(key).(string); ok {
		return value
	}
	return ""
}

// GetSubTree retrieves a subtree from the configuration.
func (c *ConfigLoader) GetSubTree(key string) *toml.Tree {
	return c.config.Get(key).(*toml.Tree)
}
