# 🏴‍☠️ Privateer

<p align="center">
  <img src=".github/images/privateer-logo.png" alt="Privateer Logo" width="300"/>
</p>

[![Go](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/kevinfinalboss/privateer)](https://github.com/kevinfinalboss/privateer/releases)

> **Automate migration of Docker images from public to private registries**

Privateer is a powerful CLI tool that scans Kubernetes clusters and GitHub repositories to identify public Docker images and automatically migrates them to private registries, ensuring enhanced security and control over your infrastructure.

## 🎯 Mission

As security and compliance concerns grow, organizations need to migrate their Docker images from public registries (DockerHub, ECR Public, etc.) to private registries. Privateer automates this complex process with enterprise-grade reliability.

## ✨ Features

### 🔍 **Intelligent Scanner**
- ✅ **Kubernetes cluster scanning** (Deployments, StatefulSets, DaemonSets, Jobs, CronJobs)
- ✅ **Automatic public/private image detection** with smart algorithms
- ✅ **Init containers and multi-container pods** support
- ✅ **Custom registry classification** rules
- ✅ **Namespace filtering** and context switching

### 🚀 **Migration Engine**
- ✅ **Automated Pull/Tag/Push workflow** to private registries
- ✅ **Multiple registry support** with priority-based selection
- ✅ **Concurrent processing** (configurable 1-10 threads)
- ✅ **Dry-run mode** for safe testing and validation
- ✅ **Health checks** and authentication validation
- ✅ **Duplicate detection** and anti-collision system

### 🎯 **Registry Support**
- ✅ **Docker Registry** (registry:2) - Production ready
- ✅ **Harbor** (self-hosted) - Production ready  
- ✅ **AWS ECR** (Elastic Container Registry) - Production ready
- 🚧 **GitHub Container Registry** - Coming in v0.3.0
- 🚧 **Azure Container Registry** - Coming in v0.3.0
- 🚧 **Google Container Registry** - Coming in v0.3.0

### 🔧 **Advanced Configuration**
- ✅ **Priority-based registry selection** (0-100 scale)
- ✅ **Multiple registries mode** vs single registry mode
- ✅ **AWS ECR authentication** (3 methods: credentials, profiles, IAM)
- ✅ **SSL/TLS support** with insecure mode option
- ✅ **Custom detection rules** for public/private classification

### 📊 **Professional Reporting**
- ✅ **HTML reports** with visual charts and statistics
- ✅ **Discord webhook integration** for real-time notifications
- ✅ **Detailed migration logs** with structured JSON output
- ✅ **Success/failure metrics** and performance analytics

## 🚀 Installation

### Quick Installation (Recommended)

**Linux/macOS:**
```bash
curl -sSL https://raw.githubusercontent.com/kevinfinalboss/privateer/main/scripts/install.sh | bash
```

**Windows (PowerShell):**
```powershell
irm https://raw.githubusercontent.com/kevinfinalboss/privateer/main/scripts/install.ps1 | iex
```

### Alternative Methods

**Go Install:**
```bash
go install github.com/kevinfinalboss/privateer/cmd/privateer@latest
```

**Manual Download:**
Download binaries from [GitHub Releases](https://github.com/kevinfinalboss/privateer/releases)

## 📋 Quick Start

### 1. Initialize Configuration
```bash
privateer init
```

### 2. Configure Registries
Edit `~/.privateer/config.yaml`:
```yaml
registries:
  # Docker Registry with highest priority
  - name: "docker-local"
    type: "docker"
    enabled: true
    priority: 10  # Highest priority (0-100)
    url: "registry.company.com"
    username: "admin"
    password: "secure-password"
    insecure: false
    
  # AWS ECR with medium priority
  - name: "ecr-prod"
    type: "ecr"
    enabled: true
    priority: 5
    region: "us-east-1"
    account_id: "123456789012"
    # Uses default AWS credentials

settings:
  multiple_registries: false  # true = all enabled registries
                              # false = highest priority only
  concurrency: 3
  
webhooks:
  discord:
    enabled: true
    url: "https://discord.com/api/webhooks/..."
```

### 3. Scan Cluster
```bash
# Basic scan
privateer scan cluster

# Dry-run simulation
privateer scan cluster --dry-run

# Different language
privateer scan cluster --language=en-US
```

### 4. Migrate Images
```bash
# Simulate migration (generates HTML report)
privateer migrate cluster --dry-run

# Execute migration (generates HTML report + Discord notifications)
privateer migrate cluster
```

## ⚙️ Advanced Configuration

### Registry Priority System
```yaml
registries:
  - name: "primary-registry"
    priority: 10  # Highest priority
    enabled: true
    
  - name: "backup-registry"  
    priority: 5   # Medium priority
    enabled: true
    
  - name: "archive-registry"
    priority: 1   # Lowest priority
    enabled: false  # Disabled
```

**Behavior:**
- `multiple_registries: false` → Only highest priority registry receives images
- `multiple_registries: true` → All enabled registries receive images

### AWS ECR Authentication Methods

**Method 1: Direct Credentials**
```yaml
- name: "ecr-direct"
  type: "ecr"
  region: "us-east-1"
  access_key: "AKIAIOSFODNN7EXAMPLE"
  secret_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
```

**Method 2: AWS Profiles**
```yaml
- name: "ecr-profiles"
  type: "ecr"
  region: "us-east-1"
  account_id: "123456789012"  # Required for profile filtering
  profiles:
    - "production"
    - "default"
```

**Method 3: Default Credentials (Recommended)**
```yaml
- name: "ecr-default"
  type: "ecr"
  region: "us-east-1"
  # Uses IAM roles, environment variables, or ~/.aws/credentials
```

## 📊 HTML Reports

Privateer automatically generates professional HTML reports for every migration:

### Report Locations
```
~/.privateer/reports/
├── privateer-dryrun-2025-01-24_15-30-45.html   # Simulation reports
├── privateer-report-2025-01-24_16-45-12.html   # Production reports
└── privateer-dryrun-2025-01-24_17-20-33.html   # Historical reports
```

### Report Contents
- 📈 **Executive dashboard** with key metrics and success rates
- 🎯 **Registry-specific statistics** with priority information
- 📋 **Detailed migration list** with source/target mappings
- ⚙️ **Configuration summary** showing enabled registries
- 🏴‍☠️ **Professional design** optimized for sharing and archival

## 🏗️ Architecture

```
privateer/
├── cmd/privateer/          # Application entry point
├── internal/
│   ├── cli/               # CLI commands (Cobra)
│   ├── config/            # Configuration management
│   ├── kubernetes/        # K8s client and scanner
│   ├── logger/            # Structured logging with i18n
│   ├── registry/          # Registry implementations
│   ├── migration/         # Migration engine
│   ├── reporter/          # HTML report generation
│   └── webhook/           # Discord notifications
├── pkg/types/             # Shared type definitions
├── locales/               # Translation files
└── scripts/               # Installation and build scripts
```

## 🎯 Use Cases

### Enterprise Migration Scenario
```bash
# Configuration for enterprise with multiple registries
registries:
  - name: "production-ecr"
    type: "ecr"
    enabled: true
    priority: 10
    region: "us-east-1"
    
  - name: "backup-harbor"
    type: "harbor"
    enabled: true
    priority: 5
    url: "harbor.company.com"
    project: "backup"

# Execute with notifications
privateer migrate cluster
```

**Result:**
- **Single mode**: Only production-ecr receives images (priority 10 > 5)
- **Multiple mode**: Both registries receive identical copies
- **Reports**: HTML report saved to `~/.privateer/reports/`
- **Notifications**: Discord alerts with migration status

### Compliance and Audit
```bash
# Generate compliance report
privateer migrate cluster --dry-run --log-level debug

# View detailed HTML report
open ~/.privateer/reports/privateer-dryrun-*.html
```

## 📈 Roadmap

### ✅ **v0.2.0 - Current (Registry Expansion)**
- [x] AWS ECR integration with 3 authentication methods
- [x] Priority-based registry selection system
- [x] Multiple registries support with intelligent routing
- [x] HTML reporting with visual charts and statistics
- [x] Discord webhook integration for real-time notifications
- [x] Advanced duplicate detection and anti-collision system

### 🚧 **v0.3.0 - GitOps Integration (Q2 2025)**
- [ ] GitHub Container Registry support
- [ ] Azure Container Registry integration
- [ ] GitHub repository scanning for Dockerfiles
- [ ] Automated Pull Request generation with updated images
- [ ] ArgoCD/Flux integration for GitOps workflows

### 🎯 **v0.4.0 - Enterprise Features (Q3 2025)**
- [ ] Web dashboard with real-time monitoring
- [ ] Prometheus metrics and alerting
- [ ] RBAC and multi-tenancy support
- [ ] Compliance reporting and audit trails
- [ ] Advanced scheduling and batch processing

## 💻 Development

### Prerequisites
- Go 1.24+
- Kubernetes cluster (for testing)
- Docker (for local registry testing)

### Local Development
```bash
# Clone repository
git clone https://github.com/kevinfinalboss/privateer.git
cd privateer

# Install dependencies
go mod download

# Development build
make dev

# Run tests
make test

# Local execution
make run ARGS="scan cluster --dry-run"
```

### Build System
```bash
# Multi-platform release
make release

# Clean build artifacts
make clean

# Code quality checks
make lint
```

## 🤝 Contributing

We welcome contributions from the community!

### How to Contribute
1. **Fork** the repository
2. **Create** a feature branch (`git checkout -b feature/amazing-feature`)
3. **Commit** your changes (`git commit -m 'Add amazing feature'`)
4. **Push** to the branch (`git push origin feature/amazing-feature`)
5. **Open** a Pull Request

### Adding New Registries
1. Implement the `Registry` interface in `internal/registry/`
2. Add support in `NewEngine()` in `manager.go`
3. Update configuration examples and documentation
4. Add comprehensive tests

### Storage Requirements
```
~/.privateer/
├── config.yaml          # <1KB - Configuration
└── reports/              # ~100KB per report
    └── *.html           # Self-contained HTML files
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙋‍♂️ Support & Community

- 📧 **Email**: [kevinmg50@gmail.com](mailto:kevinmg50@gmail.com)
- 🐛 **Issues**: [GitHub Issues](https://github.com/kevinfinalboss/privateer/issues)
- 💬 **Discussions**: [GitHub Discussions](https://github.com/kevinfinalboss/privateer/discussions)
- 📖 **Documentation**: [docs/](docs/)

## 🎉 Acknowledgments

- [Cobra](https://github.com/spf13/cobra) - Powerful CLI framework
- [Zerolog](https://github.com/rs/zerolog) - High-performance structured logging
- [Kubernetes Client](https://github.com/kubernetes/client-go) - Official Kubernetes API client
- [AWS SDK](https://github.com/aws/aws-sdk-go-v2) - AWS integration

---

<div align="center">

![Privateer Logo](.github/images/privateer-logo.png)

**[⭐ Star on GitHub](https://github.com/kevinfinalboss/privateer) • [📖 Documentation](docs/) • [🔄 Changelog](CHANGELOG.md)**

*Built with ❤️ for the DevOps and Platform Engineering community*

</div>