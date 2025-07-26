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

# Configuração do GitHub para GitOps
github:
  enabled: false  # true para habilitar migração de repositórios GitHub
  token: ""  # Token do GitHub (ghp_..., fine-grained token ou classic)
  repositories:
    # Repositório principal de manifests
    - name: "company/app-manifests"
      enabled: true
      priority: 10  # Maior prioridade para processar primeiro
      paths:  # Caminhos específicos para buscar imagens
        - "apps/"
        - "manifests/"
        - "k8s/"
        - "charts/*/values*.yaml"
      excluded_paths:  # Caminhos para ignorar
        - ".git/"
        - "node_modules/"
        - "vendor/"
        - "docs/"
      branch_strategy: "create_new"  # create_new ou use_main
      pr_settings:
        auto_merge: false  # true para auto-merge (cuidado!)
        reviewers: ["devops-team", "platform-team"]  # Revisores obrigatórios
        labels: ["privateer", "security", "automated"]  # Labels do PR
        template: ".github/pr-templates/privateer.md"  # Template personalizado
        draft: false  # true para criar como draft
        commit_prefix: "🏴‍☠️ Privateer:"  # Prefixo dos commits
        
    # Repositório de Helm Charts
    - name: "company/helm-charts"
      enabled: true
      priority: 8
      paths:
        - "charts/*/values.yaml"
        - "charts/*/values-*.yaml" 
        - "charts/*/templates/"
      excluded_paths:
        - ".git/"
        - "docs/"
      branch_strategy: "create_new"
      pr_settings:
        auto_merge: false
        reviewers: ["helm-maintainers"]
        labels: ["privateer", "helm-charts"]
        draft: false
        
    # Repositório ArgoCD Applications (exemplo)
    - name: "company/argocd-apps"
      enabled: false  # Desabilitado por padrão
      priority: 5
      paths:
        - "applications/"
        - "projects/"
        - "app-of-apps/"
      excluded_paths:
        - ".git/"
      branch_strategy: "create_new"
      pr_settings:
        auto_merge: false
        reviewers: ["argocd-admins"]
        labels: ["privateer", "argocd"]
        draft: true  # Draft por segurança

# Configuração avançada do GitOps
gitops:
  enabled: false  # true para habilitar funcionalidade GitOps
  strategy: "smart_search"  # smart_search, annotation_based, manual_mapping
  auto_pr: true  # false para apenas preparar mudanças sem criar PR
  branch_prefix: "privateer/migrate-"  # Prefixo das branches criadas
  commit_message: "🏴‍☠️ Migrate {image} to private registry"  # Template da mensagem
  
  # Padrões de busca personalizados
  search_patterns:
    - pattern: "image:\\s*([^\\s]+)"  # YAML: image: nginx:latest
      file_types: ["yaml", "yml"]
      description: "YAML image field"
      enabled: true
      
    - pattern: "repository:\\s*([^\\s]+)"  # Helm: repository: nginx
      file_types: ["yaml", "yml"]
      description: "Helm repository field"
      enabled: true
      
    - pattern: "newName:\\s*([^\\s]+)"  # Kustomize: newName: nginx
      file_types: ["yaml", "yml"]
      description: "Kustomize newName field"
      enabled: true
  
  # Regras de mapeamento para casos específicos
  mapping_rules:
    # Mapeamento direto namespace → repositório
    - namespace: "production"
      repository: "company/production-manifests"
      path: "apps/"
      mapping_type: "direct"
      confidence: 1.0
      source: "manual"
      
    # Mapeamento por nome da aplicação
    - app_name: "frontend"
      repository: "company/frontend-config"
      path: "k8s/"
      mapping_type: "app_name"
      confidence: 0.9
      source: "heuristic"
  
  # Configurações de validação
  validation:
    validate_yaml: true     # Validar sintaxe YAML após mudanças
    validate_helm: true     # Validar charts Helm se disponível
    validate_brackets: false # Validar chaves {} e colchetes [] balanceados (pode causar problemas com Helm templates)
    check_image_exists: true # Verificar se imagem existe no registry privado
    dry_run_kubernetes: false # Fazer dry-run no Kubernetes (futuro)

  tag_resolution:
    enabled: true                    # Habilitar resolução de tags vazias
    auto_fill_empty_tags: false      # Preencher automaticamente tags vazias nos values.yaml
    prefer_cluster_tags: true        # Preferir tags encontradas no cluster vs registry
    consider_latest_empty: false     # Considerar "latest" como tag vazia
    fallback_tag: "latest"           # Tag padrão se não encontrar nenhuma
    require_private_exists: true     # Só resolver se a imagem existir no registry privado
    common_tags_to_try:              # Tags comuns para tentar quando não encontrar no cluster
      - "latest"
      - "stable" 
      - "main"
      - "v1"

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
# 🎯 NOVO: GITOPS E GITHUB INTEGRATION
#
# 🚀 COMANDOS GITOPS:
# privateer migrate github --dry-run      # Simular migração GitHub
# privateer migrate github                # Executar migração GitHub + PRs
# privateer migrate all --dry-run         # Simular cluster + GitHub
# privateer migrate all                   # Executar cluster + GitHub
#
# 🔧 CONFIGURAÇÃO GITHUB:
# 1. Crie um token GitHub:
#    - Settings → Developer settings → Personal access tokens
#    - Ou use fine-grained tokens para repositórios específicos
#    - Permissões necessárias: repo, pull_requests, contents
#
# 2. Configure repositories:
#    - name: "owner/repository" (obrigatório)
#    - paths: onde buscar por imagens
#    - excluded_paths: o que ignorar
#    - pr_settings: como criar os PRs
#
# 3. Configure GitOps:
#    - strategy: como mapear cluster → repositórios
#    - search_patterns: regex para encontrar imagens
#    - validation: validações antes de criar PR
#
# 🎯 ESTRATÉGIAS DE MAPEAMENTO:
#
# SMART_SEARCH (padrão):
# - Busca por anotações ArgoCD/Helm no Kubernetes
# - Mapeia por nome da aplicação
# - Busca heurística em repositórios configurados
#
# ANNOTATION_BASED:
# - Apenas usa anotações específicas do Kubernetes
# - Mais conservador, menos false positives
#
# MANUAL_MAPPING:
# - Apenas usa mapping_rules configuradas manualmente
# - Controle total, mas requer configuração completa
#
# 📊 TIPOS DE ARQUIVO SUPORTADOS:
# - Kubernetes Manifests (.yaml/.yml)
# - Helm Values (values*.yaml)
# - ArgoCD Applications (.yaml/.yml)
# - Kustomization (kustomization.yaml)
# - Docker Compose (compose*.yaml) - futuro
#
# 🔍 PADRÕES DE IMAGEM DETECTADOS:
# image: nginx:latest                    # Kubernetes containers
# repository: nginx / tag: latest        # Helm separated values
# newName: nginx / newTag: latest        # Kustomize
# values: |                              # ArgoCD inline values
#   image: nginx:latest
#
# ⚙️ EXEMPLO DE WORKFLOW COMPLETO:
#
# 1. Migrar imagens no cluster:
#    privateer migrate cluster
#
# 2. Atualizar repositórios GitHub:
#    privateer migrate github --dry-run  # Verificar mudanças
#    privateer migrate github            # Criar PRs
#
# 3. Revisar e aprovar PRs criados
#
# 4. Deploy das mudanças (ArgoCD/Flux automático)
#
# 🎮 CENÁRIOS DE TESTE:
#
# CENÁRIO 1: Monorepo
# - Um repositório com múltiplas aplicações
# - paths: ["apps/", "services/", "infra/"]
# - O Privateer busca em todos os paths
#
# CENÁRIO 2: Repositórios separados
# - Cada aplicação tem seu próprio repositório
# - Configure um repository config para cada um
# - mapping_rules para casos específicos
#
# CENÁRIO 3: Helm Charts centralizados
# - Repository separado só para charts
# - paths: ["charts/*/values*.yaml"]
# - pr_settings específicos para helm-maintainers
#
# 🔒 SEGURANÇA E BOAS PRÁTICAS:
# - Use fine-grained tokens quando possível
# - Configure reviewers obrigatórios
# - Use draft: true para PRs críticos
# - Sempre teste com --dry-run primeiro
# - Configure excluded_paths para evitar falsos positivos
# - Use branch_strategy: "create_new" sempre
# - Monitore os logs para problemas de autenticação
#
# 💡 TROUBLESHOOTING COMUM:
# - Token sem permissões: verificar scopes
# - Repository não encontrado: verificar name format
# - PRs não criados: verificar pr_settings.reviewers existem
# - Imagens não encontradas: verificar search_patterns
# - YAML inválido após mudança: habilitar validation.validate_yaml
#
# 🎉 INTEGRAÇÃO COM DISCORD:
# As notificações Discord agora incluem:
# - Repositórios processados
# - PRs criados com links diretos
# - Arquivos modificados por tipo
# - Resumo de sucessos/falhas
# - Links para revisar mudanças
`

	if _, err := os.Stat(configFile); err == nil {
		log.Warn("config_already_exists").Str("file", configFile).Send()
		log.Info("config_location").Str("file", configFile).Send()
		log.Info("config_edit_tip").
			Str("message", "Edite o arquivo para configurar seus registries e GitHub").
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
		Str("message", "3. Configure GitHub token e repositórios").
		Send()
	log.Info("config_next_steps").
		Str("message", "4. Habilite GitOps (gitops.enabled: true)").
		Send()
	log.Info("config_next_steps").
		Str("message", "5. Execute: privateer migrate cluster --dry-run").
		Send()
	log.Info("config_next_steps").
		Str("message", "6. Execute: privateer migrate github --dry-run").
		Send()
	log.Info("operation_completed").Str("operation", "init").Send()

	return nil
}
