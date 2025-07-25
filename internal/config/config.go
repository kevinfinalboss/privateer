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
			Enabled:      false,
			Token:        "",
			Repositories: []types.GitHubRepositoryConfig{},
		},
		GitOps: types.GitOpsConfig{
			Enabled:       false,
			Strategy:      "smart_search",
			AutoPR:        true,
			BranchPrefix:  "privateer/migrate-",
			CommitMessage: "🏴‍☠️ Migrate {image} to private registry",
			SearchPatterns: []types.SearchPattern{
				{
					Pattern:     "image:\\s*([^\\s]+)",
					FileTypes:   []string{"yaml", "yml"},
					Description: "YAML image field",
					Enabled:     true,
				},
				{
					Pattern:     "repository:\\s*([^\\s]+)",
					FileTypes:   []string{"yaml", "yml"},
					Description: "Helm repository field",
					Enabled:     true,
				},
			},
			MappingRules: []types.RepositoryMapping{},
			ValidationRules: types.ValidationConfig{
				ValidateYAML:     true,
				ValidateHelm:     false,
				CheckImageExists: true,
				DryRunKubernetes: false,
			},
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
		Webhooks: types.WebhookConfig{
			Discord: types.DiscordWebhookConfig{
				Enabled: false,
				URL:     "",
				Name:    "Privateer 🏴‍☠️",
			},
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

	if config.GitOps.Strategy == "" {
		config.GitOps.Strategy = "smart_search"
	}
	if config.GitOps.BranchPrefix == "" {
		config.GitOps.BranchPrefix = "privateer/migrate-"
	}
	if config.GitOps.CommitMessage == "" {
		config.GitOps.CommitMessage = "🏴‍☠️ Migrate {image} to private registry"
	}

	if len(config.GitOps.SearchPatterns) == 0 {
		config.GitOps.SearchPatterns = []types.SearchPattern{
			{
				Pattern:     "image:\\s*([^\\s]+)",
				FileTypes:   []string{"yaml", "yml"},
				Description: "YAML image field",
				Enabled:     true,
			},
			{
				Pattern:     "repository:\\s*([^\\s]+)",
				FileTypes:   []string{"yaml", "yml"},
				Description: "Helm repository field",
				Enabled:     true,
			},
			{
				Pattern:     "newName:\\s*([^\\s]+)",
				FileTypes:   []string{"yaml", "yml"},
				Description: "Kustomize newName field",
				Enabled:     true,
			},
		}
	}

	if config.Webhooks.Discord.Name == "" {
		config.Webhooks.Discord.Name = "Privateer 🏴‍☠️"
	}

	for i := range config.GitHub.Repositories {
		repo := &config.GitHub.Repositories[i]
		if repo.BranchStrategy == "" {
			repo.BranchStrategy = "create_new"
		}
		if repo.PRSettings.CommitPrefix == "" {
			repo.PRSettings.CommitPrefix = "🏴‍☠️ Privateer:"
		}
		if len(repo.PRSettings.Labels) == 0 {
			repo.PRSettings.Labels = []string{"privateer", "security", "automated"}
		}
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
