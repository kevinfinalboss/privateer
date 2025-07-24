package types

type RegistryConfig struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"`
	Enabled   bool     `yaml:"enabled"`
	URL       string   `yaml:"url,omitempty"`
	Username  string   `yaml:"username,omitempty"`
	Password  string   `yaml:"password,omitempty"`
	Insecure  bool     `yaml:"insecure,omitempty"`
	Region    string   `yaml:"region,omitempty"`
	Project   string   `yaml:"project,omitempty"`
	AccountID string   `yaml:"account_id,omitempty"`
	Profiles  []string `yaml:"profiles,omitempty"`
	AccessKey string   `yaml:"access_key,omitempty"`
	SecretKey string   `yaml:"secret_key,omitempty"`
}

type KubernetesConfig struct {
	Context    string   `yaml:"context"`
	Namespaces []string `yaml:"namespaces"`
}

type GitHubConfig struct {
	Token        string   `yaml:"token"`
	Organization string   `yaml:"organization"`
	Repositories []string `yaml:"repositories"`
}

type SettingsConfig struct {
	Language    string `yaml:"language"`
	LogLevel    string `yaml:"log_level"`
	DryRun      bool   `yaml:"dry_run"`
	Concurrency int    `yaml:"concurrency"`
}

type ImageDetectionConfig struct {
	CustomPublicRegistries  []string `yaml:"custom_public_registries"`
	CustomPrivateRegistries []string `yaml:"custom_private_registries"`
	IgnoreRegistries        []string `yaml:"ignore_registries"`
}

type Config struct {
	Registries     []RegistryConfig     `yaml:"registries"`
	Kubernetes     KubernetesConfig     `yaml:"kubernetes"`
	GitHub         GitHubConfig         `yaml:"github"`
	Settings       SettingsConfig       `yaml:"settings"`
	ImageDetection ImageDetectionConfig `yaml:"image_detection"`
}
