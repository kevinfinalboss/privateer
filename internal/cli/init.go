package cli

import (
	"os"
	"path/filepath"

	"github.com/kevinfinalboss/privateer/internal/logger"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Inicializa configuração do Privateer",
	Long:  "Cria arquivo de configuração inicial para o Privateer",
	PreRunE: func(cmd *cobra.Command, args []string) error {
		if log == nil {
			log = logger.New()
		}

		log.Info("app_started").
			Str("version", "v0.1.0").
			Str("operation", "init").
			Send()

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return initConfig()
	},
}

func initConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Error("operation_failed").Err(err).Send()
		return err
	}

	configDir := filepath.Join(home, ".privateer")
	configFile := filepath.Join(configDir, "config.yaml")

	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Error("operation_failed").Err(err).Send()
		return err
	}

	exampleConfig := `registries:
  - name: "my-registry"
    type: "ecr"  # ecr, harbor, ghcr, dockerhub
    region: "us-east-1"  # Para ECR
    # url: "harbor.company.com"  # Para Harbor

kubernetes:
  context: ""  # Deixe vazio para usar o contexto atual
  namespaces: []  # Deixe vazio para escanear todos os namespaces

github:
  token: ""  # Token do GitHub
  organization: ""  # Sua organização
  repositories: []  # Repos específicos ou vazio para todos

settings:
  language: "pt-BR"  # pt-BR, en-US, es-ES
  log_level: "info"  # debug, info, warn, error
  dry_run: false

# Configuração avançada para detecção de imagens
image_detection:
  # Registries que você considera públicos (além dos padrões)
  custom_public_registries:
    - "quay.io/prometheus"
    - "registry.k8s.io"
    - "public.ecr.aws"
    
  # Registries que você considera privados
  custom_private_registries:
    - "mycompany.azurecr.io"
    - "ghcr.io/mycompany"
    - "harbor.mycompany.com"
    
  # Registries para ignorar completamente
  ignore_registries:
    - "localhost"
    - "127.0.0.1"
`

	if _, err := os.Stat(configFile); err == nil {
		log.Warn("config_already_exists").Str("file", configFile).Send()
		return nil
	}

	if err := os.WriteFile(configFile, []byte(exampleConfig), 0644); err != nil {
		log.Error("operation_failed").Err(err).Send()
		return err
	}

	log.Info("config_created").Str("file", configFile).Send()
	log.Info("operation_completed").Str("operation", "init").Send()

	return nil
}
