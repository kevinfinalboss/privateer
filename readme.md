# 🏴‍☠️ Privateer

![Privateer Logo](.github/images/privateer-logo.png)

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/kevinfinalboss/privateer)](https://github.com/kevinfinalboss/privateer/releases)

> **Migre suas imagens Docker públicas para registries privados de forma automatizada**

Privateer é uma ferramenta CLI que escaneia clusters Kubernetes e repositórios GitHub para identificar imagens Docker públicas e migrá-las automaticamente para registries privados, garantindo maior segurança e controle sobre sua infraestrutura.

## 🎯 Objetivo

Com o crescimento das preocupações de segurança e compliance, muitas organizações precisam migrar suas imagens Docker de registries públicos (DockerHub, ECR Public, etc.) para registries privados. O Privateer automatiza esse processo complexo.

## ✨ Funcionalidades Implementadas

### 🔍 **Scanner Inteligente**
- ✅ Escaneia clusters Kubernetes (Deployments, StatefulSets, DaemonSets, Jobs, CronJobs)
- ✅ Detecta automaticamente imagens públicas vs privadas
- ✅ Suporte a init containers e multi-container pods
- ✅ Configuração customizada de registries públicos/privados
- ✅ Filtragem por namespace

### 🚀 **Engine de Migração**
- ✅ **Pull/Tag/Push automático** para registries privados
- ✅ **Docker Registry** (registry:2) - Funcional
- ✅ **Harbor** - Funcional
- ✅ **Dry-run mode** - Simulação sem alterações
- ✅ **Processamento concorrente** (configurável)
- ✅ **Health check** de registries
- ⚠️ **AWS ECR** - Planejado para v0.2.0
- ⚠️ **GitHub Container Registry** - Planejado para v0.2.0

### 🌐 **Multi-Registry Support**
- ✅ **Docker Registry** (registry:2)
- ✅ **Harbor** (self-hosted)
- ✅ **Autenticação htpasswd**
- ✅ **SSL/TLS** com certificados
- ✅ **HTTP insecure** mode
- 🚧 **AWS ECR** (em desenvolvimento)
- 🚧 **Azure Container Registry** (planejado)
- 🚧 **GitHub Container Registry** (planejado)

### 🌍 **Internacionalização**
- ✅ Português (pt-BR) - Nativo
- ✅ Inglês (en-US) 
- ✅ Espanhol (es-ES)
- ✅ Sistema de traduções baseado em arquivos YAML

### 🛠️ **DevOps Ready**
- ✅ CLI intuitivo com Cobra
- ✅ Logs estruturados com Zerolog
- ✅ Configuração YAML flexível
- ✅ Builds multi-plataforma (Linux, macOS, Windows)
- ✅ Scripts de instalação automatizada
- ✅ Namespace filtering
- ✅ Context switching

## 🚀 Instalação

### Via Script (Recomendado)

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/kevinfinalboss/privateer/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/kevinfinalboss/privateer/main/scripts/install.ps1 | iex
```

### Via Go Install
```bash
go install github.com/kevinfinalboss/privateer/cmd/privateer@latest
```

### Via Release
Baixe o binário para sua plataforma em [Releases](https://github.com/kevinfinalboss/privateer/releases)

## 📋 Uso Rápido

### 1. Inicializar Configuração
```bash
privateer init
```

### 2. Configurar Registry
Edite `~/.privateer/config.yaml`:
```yaml
registries:
  - name: "my-registry"
    type: "docker"
    url: "https://registry.example.com"
    username: "admin"
    password: "password123"
    insecure: false
```

### 3. Escanear Cluster
```bash
# Scan básico
privateer scan cluster

# Dry-run (sem modificações)
privateer scan cluster --dry-run

# Diferentes idiomas
privateer scan cluster --language=en-US
```

### 4. Migrar Imagens ✨
```bash
# Migração simulada (dry-run)
privateer migrate cluster --dry-run

# Migração real
privateer migrate cluster
```

## ⚙️ Configuração Completa

O arquivo de configuração é criado em `~/.privateer/config.yaml`:

```yaml
# Registries de destino para migração
registries:
  # Docker Registry (registry:2)
  - name: "my-docker-registry"
    type: "docker"
    url: "https://registry.example.com"
    username: "admin"
    password: "password123"
    insecure: false
    
  # Harbor registry
  - name: "harbor-prod"
    type: "harbor"
    url: "https://harbor.company.com"
    username: "admin"
    password: "Harbor12345"
    project: "library"
    insecure: false

# Configuração do Kubernetes
kubernetes:
  context: ""  # Deixe vazio para contexto atual
  namespaces: 
    - "production"
    - "staging"
  # Deixe vazio para todas as namespaces

# Configurações gerais
settings:
  language: "pt-BR"     # pt-BR, en-US, es-ES
  log_level: "info"     # debug, info, warn, error
  dry_run: false        # true para simular
  concurrency: 3        # Migrações simultâneas

# Detecção avançada de imagens
image_detection:
  # Forçar como públicos
  custom_public_registries:
    - "quay.io/prometheus"
    - "registry.k8s.io"
    - "public.ecr.aws"
    
  # Forçar como privados  
  custom_private_registries:
    - "registry.example.com"
    - "harbor.company.com"
    
  # Ignorar completamente
  ignore_registries:
    - "localhost"
    - "127.0.0.1"
```

## 🏗️ Arquitetura

```
privateer/
├── cmd/privateer/          # Ponto de entrada da aplicação
├── internal/
│   ├── cli/               # Comandos CLI (Cobra)
│   ├── config/            # Gerenciamento de configuração
│   ├── kubernetes/        # Cliente e scanner K8s
│   ├── logger/            # Sistema de logs i18n
│   ├── registry/          # Gerenciadores de registry
│   └── migration/         # Engine de migração
├── pkg/types/             # Tipos compartilhados
├── locales/               # Arquivos de tradução
├── scripts/               # Scripts de build e instalação
└── configs/               # Exemplos de configuração
```

## 🎯 Exemplos de Uso

### Migração de Namespace Específica
```bash
# Configure no config.yaml
kubernetes:
  namespaces: ["lavalink"]

# Execute migração
privateer migrate cluster --dry-run  # Simular
privateer migrate cluster            # Executar
```

### Resultado da Migração
```bash
# ANTES:
# alpine:latest (DockerHub)

# DEPOIS:  
# registry.example.com/alpine:latest (Seu registry)
```

### Verificar Imagens Migradas
```bash
# Listar repositórios
curl -u user:pass https://registry.example.com/v2/_catalog

# Listar tags
curl -u user:pass https://registry.example.com/v2/alpine/tags/list
```

## 📊 Status do Projeto

### ✅ **v0.1.0 - Implementado**
- [x] Core CLI com Cobra
- [x] Sistema de configuração YAML  
- [x] Scanner completo de Kubernetes
- [x] Detecção inteligente de imagens públicas/privadas
- [x] Sistema de i18n (3 idiomas)
- [x] Logs estruturados
- [x] Engine de migração
- [x] Suporte a Docker Registry
- [x] Suporte a Harbor
- [x] Processamento concorrente
- [x] Health checks de registry

### 🚧 **v0.2.0 - Em Desenvolvimento**
- [ ] Integração com AWS ECR
- [ ] Integração com Azure Container Registry
- [ ] Integração com GitHub Container Registry
- [ ] Sistema de retry com exponential backoff
- [ ] Cleanup automático de imagens locais
- [ ] Métricas de performance

### 📝 **v0.3.0 - Planejado**
- [ ] Scanner de repositórios GitHub
- [ ] Geração automática de manifests atualizados
- [ ] Sistema de Pull Requests automático
- [ ] Integração com ArgoCD/Flux
- [ ] Interface web (dashboard)

### 🎯 **v0.4.0 - Futuro**
- [ ] Métricas Prometheus
- [ ] Webhooks para CI/CD
- [ ] Políticas de retenção
- [ ] Scan de vulnerabilidades
- [ ] RBAC e multi-tenancy

## 💻 Desenvolvimento

### Pré-requisitos
- Go 1.24+
- Kubernetes cluster (para testes)
- Docker (para registry local)

### Build Local
```bash
# Clone o repositório
git clone https://github.com/kevinfinalboss/privateer.git
cd privateer

# Instalar dependências
go mod download

# Build desenvolvimento
make dev

# Build para produção
make build

# Executar testes
make test
```

### Comandos Úteis
```bash
# Executar diretamente
make run ARGS="scan cluster --dry-run"

# Build multi-plataforma
make release

# Limpar builds
make clean
```

## 🤝 Contribuição

Contribuições são muito bem-vindas! 

### Como Contribuir
1. Fork o projeto
2. Crie uma branch (`git checkout -b feature/nova-funcionalidade`)
3. Commit suas mudanças (`git commit -am 'Adiciona nova funcionalidade'`)
4. Push para a branch (`git push origin feature/nova-funcionalidade`)
5. Abra um Pull Request

### Adicionando Novos Registries
1. Implemente a interface `Registry` em `internal/registry/`
2. Adicione suporte no `NewEngine()` em `manager.go`
3. Teste com diferentes cenários
4. Atualize a documentação

## 📈 Roadmap Detalhado

### v0.2.0 - Registry Expansion
- ✅ Docker Registry (registry:2) 
- ✅ Harbor
- 🚧 AWS ECR
- 🚧 Azure Container Registry  
- 🚧 GitHub Container Registry
- 🚧 Google Container Registry

### v0.3.0 - GitOps Integration
- 🔄 Scanner de Dockerfiles
- 🔄 Detecção em docker-compose.yml
- 🔄 Pull Requests automáticos
- 🔄 Integração com GitHub Actions

### v0.4.0 - Enterprise Features
- 📊 Dashboard web
- 📈 Métricas avançadas
- 🔐 RBAC
- 📋 Audit logs

## 💾 Armazenamento Local

O Privateer armazena **minimamente** na máquina do usuário:

### ✅ Armazenado Permanentemente:
- **Binário**: `/usr/local/bin/privateer` (~10MB)
- **Config**: `~/.privateer/config.yaml` (~1KB)

### 🗑️ Cache Temporário (Durante Migração):
- **Imagens Docker**: Cache temporário durante pull/push
- **Logs**: Apenas no terminal (não persistidos)

## 📄 Licença

Este projeto está licenciado sob a Licença MIT - veja o arquivo [LICENSE](LICENSE) para detalhes.

## 🙋‍♂️ Suporte

- 📧 Email: [kevinmg50@gmail.com]
- 🐛 Issues: [GitHub Issues](https://github.com/kevinfinalboss/privateer/issues)
- 💬 Discussões: [GitHub Discussions](https://github.com/kevinfinalboss/privateer/discussions)

## 🎉 Agradecimentos

- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Zerolog](https://github.com/rs/zerolog) - Logging estruturado  
- [Kubernetes Client](https://github.com/kubernetes/client-go) - API do Kubernetes
- Comunidade Go e Kubernetes

---

<div align="center">

![Privateer Logo](.github/images/privateer-logo.png)

**[⭐ Star no GitHub](https://github.com/kevinfinalboss/privateer) • [📖 Documentação](docs/) • [🔄 Changelog](CHANGELOG.md)**

Made with ❤️ for the DevOps community

</div>