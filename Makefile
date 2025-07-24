VERSION ?= dev
BINARY_NAME = privateer
OUTPUT_DIR = bin

.PHONY: help build install clean test lint dev release

help:
	@echo "🚀 Privateer Build System"
	@echo ""
	@echo "Comandos disponíveis:"
	@echo "  build      - Compila o binário"
	@echo "  install    - Compila e instala no sistema"
	@echo "  dev        - Compila e instala versão de desenvolvimento"
	@echo "  clean      - Remove arquivos de build"
	@echo "  test       - Executa testes"
	@echo "  lint       - Executa linter"
	@echo "  release    - Cria release para múltiplas plataformas"
	@echo ""
	@echo "Exemplos:"
	@echo "  make install VERSION=v1.0.0"
	@echo "  make dev"
	@echo "  make release"

build:
	@echo "🔨 Compilando Privateer..."
	@chmod +x scripts/build.sh
	@./scripts/build.sh $(VERSION)

install: build
	@echo "✅ Build e instalação concluídos!"

dev:
	@echo "🚧 Instalando versão de desenvolvimento..."
	@chmod +x scripts/build.sh
	@./scripts/build.sh dev

clean:
	@echo "🧹 Limpando arquivos de build..."
	@rm -rf $(OUTPUT_DIR)
	@echo "✅ Limpeza concluída!"

test:
	@echo "🧪 Executando testes..."
	@go test -v ./...

lint:
	@echo "🔍 Executando linter..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run; \
	else \
		echo "⚠️  golangci-lint não encontrado, usando go vet"; \
		go vet ./...; \
	fi

release:
	@echo "📦 Criando release para múltiplas plataformas..."
	@mkdir -p $(OUTPUT_DIR)
	@echo "🔨 Compilando para Linux AMD64..."
	@GOOS=linux GOARCH=amd64 go build -ldflags="-X main.Version=$(VERSION)" -o $(OUTPUT_DIR)/$(BINARY_NAME)_linux_amd64 ./cmd/privateer
	@echo "🔨 Compilando para Linux ARM64..."
	@GOOS=linux GOARCH=arm64 go build -ldflags="-X main.Version=$(VERSION)" -o $(OUTPUT_DIR)/$(BINARY_NAME)_linux_arm64 ./cmd/privateer
	@echo "🔨 Compilando para macOS AMD64..."
	@GOOS=darwin GOARCH=amd64 go build -ldflags="-X main.Version=$(VERSION)" -o $(OUTPUT_DIR)/$(BINARY_NAME)_darwin_amd64 ./cmd/privateer
	@echo "🔨 Compilando para macOS ARM64..."
	@GOOS=darwin GOARCH=arm64 go build -ldflags="-X main.Version=$(VERSION)" -o $(OUTPUT_DIR)/$(BINARY_NAME)_darwin_arm64 ./cmd/privateer
	@echo "🔨 Compilando para Windows AMD64..."
	@GOOS=windows GOARCH=amd64 go build -ldflags="-X main.Version=$(VERSION)" -o $(OUTPUT_DIR)/$(BINARY_NAME)_windows_amd64.exe ./cmd/privateer
	@echo "✅ Release criado em $(OUTPUT_DIR)/"
	@ls -la $(OUTPUT_DIR)/

run:
	@go run cmd/privateer/main.go $(ARGS)

scan-cluster:
	@go run cmd/privateer/main.go scan cluster --dry-run

scan-github:
	@go run cmd/privateer/main.go scan github --dry-run

deps:
	@echo "📦 Atualizando dependências..."
	@go mod download
	@go mod tidy