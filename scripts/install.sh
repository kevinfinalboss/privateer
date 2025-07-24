#!/bin/bash

set -e

BINARY_NAME="privateer"
REPO_URL="https://github.com/kevinfinalboss/privateer"
VERSION=${1:-"latest"}

echo "🚀 Instalando Privateer..."

detect_arch() {
    local arch=$(uname -m)
    case $arch in
        x86_64)
            echo "amd64"
            ;;
        aarch64|arm64)
            echo "arm64"
            ;;
        *)
            echo "❌ Arquitetura não suportada: $arch"
            exit 1
            ;;
    esac
}

detect_os() {
    local os=$(uname -s | tr '[:upper:]' '[:lower:]')
    case $os in
        linux)
            echo "linux"
            ;;
        darwin)
            echo "darwin"
            ;;
        *)
            echo "❌ Sistema operacional não suportado: $os"
            exit 1
            ;;
    esac
}

install_from_source() {
    echo "📥 Clonando repositório..."
    
    TEMP_DIR=$(mktemp -d)
    cd $TEMP_DIR
    
    git clone $REPO_URL .
    
    echo "🔨 Compilando..."
    chmod +x scripts/build.sh
    ./scripts/build.sh $VERSION
    
    cd -
    rm -rf $TEMP_DIR
}

install_from_release() {
    local os=$(detect_os)
    local arch=$(detect_arch)
    local filename="${BINARY_NAME}_${os}_${arch}"
    
    echo "📥 Baixando release $VERSION para $os/$arch..."
    
    TEMP_DIR=$(mktemp -d)
    cd $TEMP_DIR
    
    if [ "$VERSION" = "latest" ]; then
        DOWNLOAD_URL="$REPO_URL/releases/latest/download/$filename"
    else
        DOWNLOAD_URL="$REPO_URL/releases/download/$VERSION/$filename"
    fi
    
    if command -v curl &> /dev/null; then
        curl -L -o $BINARY_NAME $DOWNLOAD_URL
    elif command -v wget &> /dev/null; then
        wget -O $BINARY_NAME $DOWNLOAD_URL
    else
        echo "❌ curl ou wget não encontrado"
        exit 1
    fi
    
    chmod +x $BINARY_NAME
    
    if [ "$EUID" -eq 0 ]; then
        mv $BINARY_NAME /usr/local/bin/
        echo "✅ Instalado em /usr/local/bin/$BINARY_NAME"
    else
        mkdir -p "$HOME/.local/bin"
        mv $BINARY_NAME "$HOME/.local/bin/"
        echo "✅ Instalado em $HOME/.local/bin/$BINARY_NAME"
        
        if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
            echo "⚠️  Adicione $HOME/.local/bin ao PATH:"
            echo "   export PATH=\"\$HOME/.local/bin:\$PATH\""
        fi
    fi
    
    cd -
    rm -rf $TEMP_DIR
}

if [ "$VERSION" = "dev" ] || [ "$VERSION" = "source" ]; then
    install_from_source
else
    if install_from_release; then
        echo "✅ Instalação via release concluída"
    else
        echo "⚠️  Release não encontrado, compilando do código fonte..."
        install_from_source
    fi
fi

echo ""
echo "🧪 Testando instalação..."
if command -v $BINARY_NAME &> /dev/null; then
    echo "✅ Privateer instalado com sucesso!"
    $BINARY_NAME --version 2>/dev/null || echo "📦 Versão instalada"
else
    echo "❌ Instalação falhou"
    exit 1
fi

echo ""
echo "🚀 Próximos passos:"
echo "   privateer init"
echo "   privateer scan cluster"