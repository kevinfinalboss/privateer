package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Registries     []Registry     `yaml:"registries"`
	Kubernetes     Kubernetes     `yaml:"kubernetes"`
	GitHub         GitHub         `yaml:"github"`
	Settings       Settings       `yaml:"settings"`
	ImageDetection ImageDetection `yaml:"image_detection"`
}

type Registry struct {
	Name   string `yaml:"name"`
	Type   string `yaml:"type"`
	Region string `yaml:"region,omitempty"`
	URL    string `yaml:"url,omitempty"`
}

type Kubernetes struct {
	Context    string   `yaml:"context"`
	Namespaces []string `yaml:"namespaces,omitempty"`
}

type GitHub struct {
	Token        string   `yaml:"token"`
	Organization string   `yaml:"organization"`
	Repositories []string `yaml:"repositories,omitempty"`
}

type Settings struct {
	Language string `yaml:"language"`
	LogLevel string `yaml:"log_level"`
	DryRun   bool   `yaml:"dry_run"`
}

type ImageDetection struct {
	CustomPublicRegistries  []string `yaml:"custom_public_registries"`
	CustomPrivateRegistries []string `yaml:"custom_private_registries"`
	IgnoreRegistries        []string `yaml:"ignore_registries"`
}

func Load(path string) (*Config, error) {
	if path == "" {
		path = getDefaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return getDefaultConfig(), err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func getDefaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".privateer", "config.yaml")
}

func getDefaultConfig() *Config {
	return &Config{
		Settings: Settings{
			Language: "pt-BR",
			LogLevel: "info",
			DryRun:   false,
		},
		Kubernetes: Kubernetes{
			Context: "",
		},
	}
}
