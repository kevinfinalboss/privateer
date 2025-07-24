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
  # Registry Docker Local (prioridade alta)
  - name: "docker-local"
    type: "docker"
    enabled: true
    priority: 10  # Prioridade mais alta (0-100, maior = mais prioritário)
    url: "https://registry.example.com"  # ou http:// para insecure
    username: "admin"
    password: "password123"
    insecure: false  # true para HTTP sem SSL
    
  # Harbor Registry (prioridade média-alta)
  - name: "harbor-prod"
    type: "harbor"
    enabled: false
    priority: 8
    url: "https://harbor.company.com"
    username: "admin"
    password: "Harbor12345"
    project: "library"  # Projeto do Harbor
    insecure: false
    
  # AWS ECR com Credenciais Diretas (prioridade média)
  - name: "ecr-credentials"
    type: "ecr"
    enabled: true
    priority: 5
    region: "us-east-1"
    account_id: "123456789012"  # Opcional - descoberto automaticamente
    access_key: "AKIAIOSFODNN7EXAMPLE"
    secret_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
    
  # AWS ECR com Profiles (prioridade média-baixa)
  - name: "ecr-profiles"
    type: "ecr"
    enabled: false
    priority: 3
    region: "us-east-1"
    account_id: "503935937141"  # Obrigatório para filtrar profiles
    profiles:  # Lista de profiles para tentar em ordem
      - "production"
      - "default"
      - "company-aws"
    
  # AWS ECR com Credenciais Padrão (prioridade baixa)
  - name: "ecr-default"
    type: "ecr"
    enabled: false
    priority: 2
    region: "us-east-1"
    # account_id será descoberto automaticamente
    # Usa: ~/.aws/credentials, IAM roles, env vars
    
  # GitHub Container Registry (prioridade baixa)
  - name: "ghcr-company"
    type: "ghcr"
    enabled: false
    priority: 1
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
  # CONFIGURAÇÃO CRÍTICA: Define comportamento dos registries
  multiple_registries: false  # false = apenas 1 registry (maior prioridade)
                              # true = todos os registries habilitados

# Configuração de Webhooks
webhooks:
  discord:
    enabled: false  # true para habilitar notificações Discord
    url: ""        # URL do webhook Discord
    name: "Privateer 🏴‍☠️"  # Nome do bot (opcional)
    avatar: ""     # URL do avatar (opcional)

# Configuração avançada para detecção de imagens
image_detection:
  # Registries que você FORÇA como públicos (além dos padrões)
  custom_public_registries:
    - "quay.io/prometheus"
    - "registry.k8s.io"
    - "public.ecr.aws"
    - "docker.io/library"
    - "mcr.microsoft.com"
    - "gcr.io/google-containers"
    
  # Registries que você FORÇA como privados
  custom_private_registries:
    - "mycompany.azurecr.io"
    - "ghcr.io/mycompany"
    - "harbor.mycompany.com"
    - "registry.example.com"
    - "docker.io/mycompany"
    
  # Registries para IGNORAR completamente no scan
  ignore_registries:
    - "localhost"
    - "127.0.0.1"
    - "registry.local"
    - "kind-registry:5000"

# 📝 DOCUMENTAÇÃO COMPLETA DE USO:
#
# 🎯 SISTEMA DE PRIORIDADE:
# - priority: 0-100 (maior número = maior prioridade)
# - Apenas registries com enabled: true são considerados
# - Se multiple_registries: false → apenas o de maior prioridade recebe
# - Se multiple_registries: true → TODOS os habilitados recebem
#
# Exemplo de cenário com sua configuração:
# - docker-local: enabled=true, priority=10
# - ecr-credentials: enabled=true, priority=5  
# - harbor-prod: enabled=false, priority=8
#
# Resultado com multiple_registries: false → apenas docker-local
# Resultado com multiple_registries: true → docker-local + ecr-credentials
#
# 🔧 CAMPOS DE CONFIGURAÇÃO POR TIPO:
#
# DOCKER REGISTRY:
# - url: URL completa (https://registry.example.com)
# - username: Nome de usuário
# - password: Senha
# - insecure: true para HTTP sem SSL
#
# HARBOR REGISTRY:
# - url: URL completa (https://harbor.company.com)
# - username: Nome de usuário do Harbor
# - password: Senha do Harbor
# - project: Projeto do Harbor (padrão: "library")
# - insecure: true para HTTP sem SSL
#
# AWS ECR:
# - region: Região AWS (us-east-1, eu-west-1, etc.)
# - account_id: ID da conta AWS (opcional, descoberto automaticamente)
# - access_key: Chave de acesso AWS (método 1)
# - secret_key: Chave secreta AWS (método 1)
# - profiles: Lista de profiles ~/.aws/credentials (método 2)
# - (método 3: credenciais padrão do ambiente - sem campos extras)
#
# GITHUB CONTAINER REGISTRY:
# - username: Seu usuário GitHub
# - password: Token GitHub (ghp_...)
# - project: Nome da organização GitHub
#
# 🔔 WEBHOOKS DISCORD:
# 1. Crie um webhook no seu servidor Discord:
#    - Configurações do Servidor → Integrações → Webhooks → Novo Webhook
# 2. Copie a URL do webhook
# 3. Configure enabled: true e cole a URL
# 4. O Privateer enviará notificações de início, fim e erros
#
# 📊 NOTIFICAÇÕES INCLUEM:
# - Início da migração (quantas imagens, quais registries)
# - Progresso em tempo real 
# - Resultado final (sucessos, falhas, ignoradas)
# - Exemplos de migrações realizadas
# - Detalhes de erros quando ocorrem
#
# 🚀 COMANDOS ESSENCIAIS:
# privateer init                          # Gerar esta configuração
# privateer scan cluster                  # Listar imagens públicas
# privateer scan cluster --dry-run        # Simular scan
# privateer migrate cluster --dry-run     # Simular migração + Discord
# privateer migrate cluster               # Executar migração + Discord
# privateer migrate cluster --log-level debug  # Logs detalhados
#
# ⚙️ AWS ECR - 3 MÉTODOS DE AUTENTICAÇÃO:
#
# MÉTODO 1: Credenciais Diretas (menos seguro, para testes)
# - access_key: "AKIAIOSFODNN7EXAMPLE"
# - secret_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
#
# MÉTODO 2: Profiles AWS (recomendado para múltiplas contas)
# - profiles: ["production", "default", "company-aws"]
# - account_id: "503935937141"  # Obrigatório para filtrar profiles
#
# MÉTODO 3: Credenciais Padrão (mais seguro para produção)
# - IAM Roles (EC2/ECS/Lambda)
# - Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
# - ~/.aws/credentials com profile [default]
#
# 🔍 VALIDAÇÃO ANTI-DUPLICAÇÃO:
# - Verifica se imagem já existe antes de migrar
# - ECR: cria repositórios automaticamente se necessário
# - Docker/Harbor: verifica manifests via API
# - Evita sobrescrever imagens existentes
# - Logs detalhados de duplicações detectadas
#
# 🎮 CENÁRIOS DE TESTE PRÁTICOS:
#
# CENÁRIO 1: Registry único (modo conservador)
# multiple_registries: false
# enabled: docker-local=true, ecr-credentials=true
# priority: docker-local=10, ecr-credentials=5
# Resultado: apenas docker-local recebe (maior prioridade)
#
# CENÁRIO 2: Múltiplos registries (modo backup/redundância)
# multiple_registries: true
# enabled: docker-local=true, ecr-credentials=true
# Resultado: AMBOS recebem a mesma imagem
#
# CENÁRIO 3: Failover automático
# enabled: docker-local=false, ecr-credentials=true
# Resultado: ecr-credentials recebe (único habilitado)
#
# CENÁRIO 4: Teste com Harbor
# enabled: harbor-prod=true, priority=8
# multiple_registries: false
# Resultado: harbor-prod recebe se for o de maior prioridade habilitado
#
# 💡 DICAS DE CONFIGURAÇÃO:
# - Use priority 10 para registry principal de produção
# - Use priority 5-8 para registries secundários  
# - Use priority 1-3 para registries de backup/teste
# - Deixe enabled=false em registries não utilizados no momento
# - Configure Discord para monitorar migrações importantes em produção
# - Use insecure=true apenas em ambientes de desenvolvimento local
# - Para ECR, prefira profiles ou IAM roles em vez de credenciais diretas
#
# 🔒 SEGURANÇA E BOAS PRÁTICAS:
# - Senhas em texto plano apenas para desenvolvimento/teste
# - Use profiles AWS ou IAM roles em produção
# - Configure HTTPS (insecure=false) sempre que possível
# - Monitore logs para tentativas de acesso não autorizadas
# - Rotacione credenciais regularmente
# - Use tokens GitHub com escopo mínimo necessário
# - Mantenha backups das configurações importantes
#
# 🔧 DETECÇÃO AUTOMÁTICA DE REGISTRIES:
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
		Str("message", "3. Defina prioridades (priority: 0-100)").
		Send()
	log.Info("config_next_steps").
		Str("message", "4. Habilite os registries (enabled: true)").
		Send()
	log.Info("config_next_steps").
		Str("message", "5. Configure multiple_registries (true/false)").
		Send()
	log.Info("config_next_steps").
		Str("message", "6. Configure Discord webhook (opcional)").
		Send()
	log.Info("config_next_steps").
		Str("message", "7. Execute: privateer migrate cluster --dry-run").
		Send()
	log.Info("operation_completed").Str("operation", "init").Send()

	return nil
}
