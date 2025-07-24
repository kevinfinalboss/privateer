package config

import (
	"os"
	"path/filepath"

	"github.com/kevinfinalboss/privateer/pkg/types"
	"gopkg.in/yaml.v3"
)

func Load(configFile string) (*types.Config, error) {
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		configFile = filepath.Join(home, ".privateer", "config.yaml")
	}

	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return GetDefaultConfig(), nil
		}
		return nil, err
	}

	var config types.Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	applyDefaults(&config)
	return &config, nil
}

func GetDefaultConfig() *types.Config {
	config := &types.Config{
		Registries: []types.RegistryConfig{},
		Kubernetes: types.KubernetesConfig{
			Context:    "",
			Namespaces: []string{},
		},
		GitHub: types.GitHubConfig{
			Token:        "",
			Organization: "",
			Repositories: []string{},
		},
		Settings: types.SettingsConfig{
			Language:    "pt-BR",
			LogLevel:    "info",
			DryRun:      false,
			Concurrency: 3,
		},
		ImageDetection: types.ImageDetectionConfig{
			CustomPublicRegistries:  []string{},
			CustomPrivateRegistries: []string{},
			IgnoreRegistries:        []string{"localhost", "127.0.0.1"},
		},
	}

	return config
}

func applyDefaults(config *types.Config) {
	if config.Settings.Language == "" {
		config.Settings.Language = "pt-BR"
	}
	if config.Settings.LogLevel == "" {
		config.Settings.LogLevel = "info"
	}
	if config.Settings.Concurrency == 0 {
		config.Settings.Concurrency = 3
	}
	if len(config.ImageDetection.IgnoreRegistries) == 0 {
		config.ImageDetection.IgnoreRegistries = []string{"localhost", "127.0.0.1"}
	}
}

func Save(config *types.Config, configFile string) error {
	if configFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		configDir := filepath.Join(home, ".privateer")
		if err := os.MkdirAll(configDir, 0755); err != nil {
			return err
		}
		configFile = filepath.Join(configDir, "config.yaml")
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}

	return os.WriteFile(configFile, data, 0644)
}
