#!/bin/bash

set -e

VERSION=${1:-"dev"}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

BINARY_NAME="privateer"
OUTPUT_DIR="bin"
INSTALL_DIR="/usr/local/bin"

echo "🚀 Construindo Privateer..."
echo "📦 Versão: $VERSION"
echo "⏰ Build: $BUILD_TIME"
echo "🔗 Commit: $GIT_COMMIT"

mkdir -p $OUTPUT_DIR

LDFLAGS="-X main.Version=$VERSION -X main.BuildTime=$BUILD_TIME -X main.GitCommit=$GIT_COMMIT"

echo "🔨 Compilando para Linux AMD64..."
GOOS=linux GOARCH=amd64 go build \
    -ldflags="$LDFLAGS" \
    -o $OUTPUT_DIR/${BINARY_NAME}_linux_amd64 \
    ./cmd/privateer

echo "🔨 Compilando para Linux ARM64..."
GOOS=linux GOARCH=arm64 go build \
    -ldflags="$LDFLAGS" \
    -o $OUTPUT_DIR/${BINARY_NAME}_linux_arm64 \
    ./cmd/privateer

echo "🔨 Compilando para Windows AMD64..."
GOOS=windows GOARCH=amd64 go build \
    -ldflags="$LDFLAGS" \
    -o $OUTPUT_DIR/${BINARY_NAME}_windows_amd64.exe \
    ./cmd/privateer

echo "🔨 Compilando para macOS AMD64..."
GOOS=darwin GOARCH=amd64 go build \
    -ldflags="$LDFLAGS" \
    -o $OUTPUT_DIR/${BINARY_NAME}_darwin_amd64 \
    ./cmd/privateer

echo "🔨 Compilando para macOS ARM64..."
GOOS=darwin GOARCH=arm64 go build \
    -ldflags="$LDFLAGS" \
    -o $OUTPUT_DIR/${BINARY_NAME}_darwin_arm64 \
    ./cmd/privateer

CURRENT_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
CURRENT_ARCH=$(uname -m)

if [ "$CURRENT_OS" = "linux" ]; then
    if [ "$CURRENT_ARCH" = "x86_64" ]; then
        BINARY_PATH="$OUTPUT_DIR/${BINARY_NAME}_linux_amd64"
    elif [ "$CURRENT_ARCH" = "aarch64" ] || [ "$CURRENT_ARCH" = "arm64" ]; then
        BINARY_PATH="$OUTPUT_DIR/${BINARY_NAME}_linux_arm64"
    fi
elif [ "$CURRENT_OS" = "darwin" ]; then
    if [ "$CURRENT_ARCH" = "x86_64" ]; then
        BINARY_PATH="$OUTPUT_DIR/${BINARY_NAME}_darwin_amd64"
    elif [ "$CURRENT_ARCH" = "arm64" ]; then
        BINARY_PATH="$OUTPUT_DIR/${BINARY_NAME}_darwin_arm64"
    fi
else
    echo "❌ Sistema operacional não suportado para instalação automática: $CURRENT_OS"
    echo "📦 Binários disponíveis em $OUTPUT_DIR/"
    ls -la $OUTPUT_DIR/
    exit 0
fi

echo "📁 Binário construído: $BINARY_PATH"

if [ "$EUID" -eq 0 ]; then
    echo "🔧 Instalando no sistema como root..."
    cp $BINARY_PATH $INSTALL_DIR/$BINARY_NAME
    chmod +x $INSTALL_DIR/$BINARY_NAME
    echo "✅ Privateer instalado em $INSTALL_DIR/$BINARY_NAME"
else
    echo "🔧 Instalando para o usuário atual..."
    
    if [ ! -d "$HOME/.local/bin" ]; then
        mkdir -p "$HOME/.local/bin"
        echo "📁 Criado diretório: $HOME/.local/bin"
    fi
    
    cp $BINARY_PATH "$HOME/.local/bin/$BINARY_NAME"
    chmod +x "$HOME/.local/bin/$BINARY_NAME"
    
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        echo "⚠️  ATENÇÃO: $HOME/.local/bin não está no PATH"
        echo "💡 Adicione esta linha ao seu ~/.bashrc ou ~/.zshrc:"
        echo "   export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo ""
        echo "🔄 Ou execute agora:"
        echo "   export PATH=\"\$HOME/.local/bin:\$PATH\""
        echo "   source ~/.bashrc"
    fi
    
    echo "✅ Privateer instalado em $HOME/.local/bin/$BINARY_NAME"
fi

echo ""
echo "🎉 Build completo!"
echo "📋 Arquivos gerados:"
ls -la $OUTPUT_DIR/

echo ""
echo "🧪 Testando instalação..."
if command -v $BINARY_NAME &> /dev/null; then
    echo "✅ Comando 'privateer' está disponível"
    $BINARY_NAME --version 2>/dev/null || echo "📌 Versão: $VERSION"
else
    echo "⚠️  Comando 'privateer' não encontrado no PATH"
    echo "💡 Você pode executar diretamente: ./$BINARY_PATH"
fi

echo ""
echo "🚀 Para começar:"
echo "   privateer init"
echo "   privateer scan cluster"