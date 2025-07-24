#!/bin/bash

set -e

BINARY_NAME="privateer"
CURRENT_VERSION=""

echo "🔄 Atualizando Privateer..."

if command -v $BINARY_NAME &> /dev/null; then
    CURRENT_VERSION=$($BINARY_NAME --version 2>/dev/null | head -n1 | awk '{print $2}' || echo "unknown")
    echo "📦 Versão atual: $CURRENT_VERSION"
else
    echo "📦 Privateer não encontrado, instalando pela primeira vez..."
fi

echo "📥 Baixando versão mais recente..."

TEMP_DIR=$(mktemp -d)
cd $TEMP_DIR

curl -s https://api.github.com/repos/kevinfinalboss/privateer/releases/latest | \
    grep "tag_name" | \
    cut -d '"' -f 4 > latest_version.txt

LATEST_VERSION=$(cat latest_version.txt)

if [ "$CURRENT_VERSION" = "$LATEST_VERSION" ]; then
    echo "✅ Você já tem a versão mais recente: $CURRENT_VERSION"
    cd -
    rm -rf $TEMP_DIR
    exit 0
fi

echo "🆙 Atualizando de $CURRENT_VERSION para $LATEST_VERSION..."

detect_arch() {
    local arch=$(uname -m)
    case $arch in
        x86_64) echo "amd64" ;;
        aarch64|arm64) echo "arm64" ;;
        *) echo "❌ Arquitetura não suportada: $arch"; exit 1 ;;
    esac
}

detect_os() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case $os in
        linux) echo "linux" ;;
        darwin) echo "darwin" ;;
        *) echo "❌ SO não suportado: $os"; exit 1 ;;
    esac
}

OS=$(detect_os)
ARCH=$(detect_arch)
FILENAME="${BINARY_NAME}_${OS}_${ARCH}"

DOWNLOAD_URL="https://github.com/kevinfinalboss/privateer/releases/download/$LATEST_VERSION/$FILENAME"

echo "📥 Baixando $DOWNLOAD_URL..."

if command -v curl &> /dev/null; then
    curl -L -o $BINARY_NAME $DOWNLOAD_URL
elif command -v wget &> /dev/null; then
    wget -O $BINARY_NAME $DOWNLOAD_URL
else
    echo "❌ curl ou wget não encontrado"
    exit 1
fi

chmod +x $BINARY_NAME

INSTALL_PATH=""
if [ -f "/usr/local/bin/$BINARY_NAME" ]; then
    INSTALL_PATH="/usr/local/bin/$BINARY_NAME"
elif [ -f "$HOME/.local/bin/$BINARY_NAME" ]; then
    INSTALL_PATH="$HOME/.local/bin/$BINARY_NAME"
else
    if [ "$EUID" -eq 0 ]; then
        INSTALL_PATH="/usr/local/bin/$BINARY_NAME"
    else
        mkdir -p "$HOME/.local/bin"
        INSTALL_PATH="$HOME/.local/bin/$BINARY_NAME"
    fi
fi

echo "🔧 Instalando em $INSTALL_PATH..."

if [[ "$INSTALL_PATH" == "/usr/local/bin/"* ]] && [ "$EUID" -ne 0 ]; then
    sudo cp $BINARY_NAME $INSTALL_PATH
else
    cp $BINARY_NAME $INSTALL_PATH
fi

cd -
rm -rf $TEMP_DIR

echo "✅ Privateer atualizado com sucesso!"
echo "📦 Nova versão: $($BINARY_NAME --version | head -n1 | awk '{print $2}')"

echo ""
echo "🎉 Atualização concluída!"
echo "🚀 Execute: privateer --help"