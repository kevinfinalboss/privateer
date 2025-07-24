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

	exampleConfig := `# 🏴‍☠️ Privateer Configuration
# Configuração dos registries de destino para migração

registries:
  # Registry Docker simples (como registry:2 no Kubernetes)
  - name: "my-docker-registry"
    type: "docker"
    enabled: true  # true para enviar imagens, false para ignorar
    url: "https://registry.example.com"  # Inclua https:// ou http://
    username: "admin"
    password: "password123"
    insecure: false  # true para HTTP sem SSL
    
  # Harbor registry (auto-hosted)
  - name: "harbor-prod"
    type: "harbor"
    enabled: false  # Desabilitado por padrão
    url: "https://harbor.company.com"  # Inclua https:// ou http://
    username: "admin"
    password: "Harbor12345"
    project: "library"  # Projeto padrão do Harbor
    insecure: false
    
  # AWS ECR - Opção 1: Usando credenciais diretas
  - name: "ecr-prod-credentials"
    type: "ecr"
    enabled: false  # Habilite conforme necessário
    region: "us-east-1"
    account_id: "123456789012"  # Opcional - será descoberto automaticamente
    access_key: "AKIAIOSFODNN7EXAMPLE"
    secret_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    
  # AWS ECR - Opção 2: Usando profiles AWS
  - name: "ecr-prod-profiles"
    type: "ecr"
    enabled: false  # Habilite conforme necessário
    region: "us-east-1"
    account_id: "123456789012"  # Obrigatório se usando múltiplos profiles
    profiles:  # Lista de profiles para tentar em ordem
      - "production"
      - "default"
      - "company-aws"
    
  # AWS ECR - Opção 3: Credenciais padrão do ambiente
  - name: "ecr-prod-default"
    type: "ecr"
    enabled: false  # Habilite conforme necessário
    region: "us-east-1"
    # account_id será descoberto automaticamente
    # Usa credenciais do ambiente: ~/.aws/credentials, IAM roles, etc.
    
  # GitHub Container Registry
  - name: "ghcr-company"
    type: "ghcr"
    enabled: false  # Ainda não implementado totalmente
    username: "your-github-user"
    password: "ghp_your_github_token"
    project: "your-organization"  # Nome da organização

# Configuração do Kubernetes
kubernetes:
  context: ""  # Deixe vazio para usar o contexto atual do kubectl
  namespaces: []  # Liste namespaces específicos ou deixe vazio para todos
  # Exemplo:
  # namespaces:
  #   - "default"
  #   - "production" 
  #   - "staging"

# Configuração do GitHub (futuro - v0.3.0)
github:
  token: ""  # Token do GitHub para scan de repositórios
  organization: ""  # Sua organização no GitHub
  repositories: []  # Repositórios específicos ou vazio para todos

# Configurações gerais da aplicação
settings:
  language: "pt-BR"     # pt-BR, en-US, es-ES
  log_level: "info"     # debug, info, warn, error
  dry_run: false        # true para simular sem fazer alterações
  concurrency: 3        # Número de migrações simultâneas (1-10)

# Configuração avançada para detecção de imagens
image_detection:
  # Registries que você FORÇA como públicos (além dos padrões)
  # Útil para casos especiais onde a detecção automática falha
  custom_public_registries:
    - "quay.io/prometheus"
    - "registry.k8s.io"
    - "public.ecr.aws"
    - "docker.io/library"
    - "mcr.microsoft.com"
    - "gcr.io/google-containers"
    
  # Registries que você FORÇA como privados
  # Útil para seus próprios registries ou registries da empresa
  custom_private_registries:
    - "mycompany.azurecr.io"
    - "ghcr.io/mycompany"
    - "harbor.mycompany.com"
    - "registry.example.com"
    - "docker.io/mycompany"
    
  # Registries para IGNORAR completamente no scan
  # Útil para registries locais ou de desenvolvimento
  ignore_registries:
    - "localhost"
    - "127.0.0.1"
    - "registry.local"
    - "kind-registry:5000"

# 📝 Exemplos de uso:
#
# 1. Escanear cluster:
#    privateer scan cluster
#
# 2. Migração dry-run:
#    privateer migrate cluster --dry-run
#
# 3. Migração real:
#    privateer migrate cluster
#
# 4. Debug verbose:
#    privateer scan cluster --log-level debug
#
# 5. Idioma específico:
#    privateer scan cluster --language en-US

# 🔧 Detecção Automática de Registries:
# 
# PÚBLICOS (detectados automaticamente):
# - docker.io/* (DockerHub)
# - quay.io/* (Red Hat Quay)
# - registry.k8s.io/* (Kubernetes)
# - public.ecr.aws/* (AWS ECR Public)
# - mcr.microsoft.com/* (Microsoft)
#
# PRIVADOS (detectados automaticamente):
# - *.dkr.ecr.*.amazonaws.com/* (AWS ECR Private)
# - *.azurecr.io/* (Azure Container Registry)
# - *.gcr.io/* e *.pkg.dev/* (Google Container Registry)
# - ghcr.io/*/* (GitHub Container Registry com org)
# - qualquer.dominio.com/* (registries com domínio customizado)
#
# Use as configurações custom_* acima para sobrescrever a detecção automática.

# 🚀 AWS ECR - Configuração Avançada:
#
# O Privateer suporta 3 formas de autenticação com AWS ECR:
#
# 1. CREDENCIAIS DIRETAS (menos seguro, só para testes):
#    - access_key e secret_key diretamente no config
#
# 2. PROFILES AWS (recomendado):
#    - Configure múltiplos profiles em ~/.aws/credentials
#    - O Privateer tenta cada profile em ordem até encontrar um válido
#    - Útil para organizações com múltiplas contas AWS
#
# 3. CREDENCIAIS PADRÃO (mais seguro):
#    - IAM Roles (para EC2/ECS/Lambda)
#    - Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
#    - ~/.aws/credentials com profile [default]
#
# ACCOUNT_ID:
# - Obrigatório apenas se você tem múltiplos profiles para contas diferentes
# - O Privateer filtra automaticamente qual profile usar baseado no account_id
# - Se não fornecido, será descoberto automaticamente via STS GetCallerIdentity

# ⚠️  VALIDAÇÃO DE DUPLICATAS:
#
# O Privateer verifica automaticamente se uma imagem já existe no registry
# de destino antes de fazer a migração. Se encontrar duplicata:
# - ALERTA no log
# - PULA a migração dessa imagem para esse registry
# - CONTINUA com outros registries habilitados
# - NÃO FALHA a operação geral
#
# Isso evita:
# - Sobrescrever imagens existentes acidentalmente
# - Desperdiçar tempo/banda fazendo upload desnecessário
# - Conflitos de versioning
`

	if _, err := os.Stat(configFile); err == nil {
		log.Warn("config_already_exists").Str("file", configFile).Send()
		log.Info("config_location").Str("file", configFile).Send()
		log.Info("config_edit_tip").
			Str("message", "Edite o arquivo para configurar seus registries").
			Send()
		return nil
	}

	if err := os.WriteFile(configFile, []byte(exampleConfig), 0644); err != nil {
		log.Error("operation_failed").Err(err).Send()
		return err
	}

	log.Info("config_created").Str("file", configFile).Send()
	log.Info("config_next_steps").
		Str("message", "1. Edite o arquivo de configuração").
		Send()
	log.Info("config_next_steps").
		Str("message", "2. Configure seus registries de destino").
		Send()
	log.Info("config_next_steps").
		Str("message", "3. Habilite os registries (enabled: true)").
		Send()
	log.Info("config_next_steps").
		Str("message", "4. Execute: privateer scan cluster --dry-run").
		Send()
	log.Info("operation_completed").Str("operation", "init").Send()

	return nil
}
