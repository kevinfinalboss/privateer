param(
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"

$BINARY_NAME = "privateer"
$REPO_URL = "https://github.com/kevinfinalboss/privateer"

Write-Host "🚀 Instalando Privateer..." -ForegroundColor Green

function Get-Architecture {
    $arch = [System.Environment]::GetEnvironmentVariable("PROCESSOR_ARCHITECTURE")
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { 
            Write-Host "❌ Arquitetura não suportada: $arch" -ForegroundColor Red
            exit 1 
        }
    }
}

function Install-FromRelease {
    $arch = Get-Architecture
    $filename = "${BINARY_NAME}_windows_${arch}.exe"
    
    Write-Host "📥 Baixando release $Version para windows/$arch..." -ForegroundColor Yellow
    
    $tempDir = [System.IO.Path]::GetTempPath()
    $tempFile = Join-Path $tempDir "${BINARY_NAME}.exe"
    
    if ($Version -eq "latest") {
        $downloadUrl = "$REPO_URL/releases/latest/download/$filename"
    } else {
        $downloadUrl = "$REPO_URL/releases/download/$Version/$filename"
    }
    
    try {
        Invoke-WebRequest -Uri $downloadUrl -OutFile $tempFile
    } catch {
        Write-Host "⚠️ Release não encontrado, tentando compilar do código fonte..." -ForegroundColor Yellow
        Install-FromSource
        return
    }
    
    # Determinar diretório de instalação
    $installDir = "${env:USERPROFILE}\.local\bin"
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        Write-Host "📁 Criado diretório: $installDir" -ForegroundColor Blue
    }
    
    $installPath = Join-Path $installDir "${BINARY_NAME}.exe"
    Copy-Item $tempFile $installPath -Force
    
    # Adicionar ao PATH se necessário
    $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
    if ($userPath -notlike "*$installDir*") {
        Write-Host "🔧 Adicionando $installDir ao PATH..." -ForegroundColor Blue
        [Environment]::SetEnvironmentVariable("PATH", "$userPath;$installDir", "User")
        Write-Host "⚠️ Reinicie o terminal ou execute: `$env:PATH += ';$installDir'" -ForegroundColor Yellow
    }
    
    Write-Host "✅ Privateer instalado em $installPath" -ForegroundColor Green
    Remove-Item $tempFile -Force
}

function Install-FromSource {
    Write-Host "📥 Clonando repositório..." -ForegroundColor Yellow
    
    $tempDir = Join-Path ([System.IO.Path]::GetTempPath()) "privateer-build"
    if (Test-Path $tempDir) {
        Remove-Item $tempDir -Recurse -Force
    }
    
    git clone $REPO_URL $tempDir
    Set-Location $tempDir
    
    Write-Host "🔨 Compilando..." -ForegroundColor Yellow
    
    $arch = Get-Architecture
    $env:GOOS = "windows"
    $env:GOARCH = $arch
    
    go build -ldflags="-X main.Version=$Version" -o "${BINARY_NAME}.exe" ./cmd/privateer
    
    $installDir = "${env:USERPROFILE}\.local\bin"
    if (-not (Test-Path $installDir)) {
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
    }
    
    $installPath = Join-Path $installDir "${BINARY_NAME}.exe"
    Copy-Item "${BINARY_NAME}.exe" $installPath -Force
    
    Set-Location $env:USERPROFILE
    Remove-Item $tempDir -Recurse -Force
    
    Write-Host "✅ Privateer compilado e instalado em $installPath" -ForegroundColor Green
}

# Verificar se Go está instalado para builds do código fonte
$goInstalled = Get-Command go -ErrorAction SilentlyContinue
if (-not $goInstalled -and $Version -eq "dev") {
    Write-Host "❌ Go não encontrado. Instale Go ou use uma release." -ForegroundColor Red
    exit 1
}

if ($Version -eq "dev" -or $Version -eq "source") {
    Install-FromSource
} else {
    Install-FromRelease
}

Write-Host "" 
Write-Host "🧪 Testando instalação..." -ForegroundColor Blue
$privateerPath = Get-Command privateer -ErrorAction SilentlyContinue
if ($privateerPath) {
    Write-Host "✅ Privateer instalado com sucesso!" -ForegroundColor Green
    & privateer --version
} else {
    Write-Host "⚠️ Privateer instalado, mas não encontrado no PATH" -ForegroundColor Yellow
    Write-Host "💡 Reinicie o terminal ou adicione manualmente ao PATH" -ForegroundColor Blue
}

Write-Host ""
Write-Host "🚀 Próximos passos:" -ForegroundColor Green
Write-Host "   privateer init"
Write-Host "   privateer scan cluster"