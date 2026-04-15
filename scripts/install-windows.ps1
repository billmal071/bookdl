# PowerShell install script for Windows
# Usage: iwr -useb https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-windows.ps1 | iex

param(
    [string]$InstallDir = "$env:USERPROFILE\AppData\Local\bookdl"
)

Write-Host "📥 Installing bookdl for Windows..." -ForegroundColor Cyan

# Create install directory
if (-not (Test-Path $InstallDir)) {
    New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    Write-Host "✓ Created install directory: $InstallDir" -ForegroundColor Green
}

# Download the latest release
$DownloadUrl = "https://github.com/billmal071/bookdl/releases/latest/download/bookdl-windows-amd64.exe"
$BinaryPath = Join-Path $InstallDir "bookdl.exe"

Write-Host "⬇️  Downloading..." -ForegroundColor Yellow
try {
    Invoke-WebRequest -Uri $DownloadUrl -OutFile $BinaryPath -UseBasicParsing
    Write-Host "✓ Download complete" -ForegroundColor Green
} catch {
    Write-Host "❌ Download failed: $_" -ForegroundColor Red
    exit 1
}

# Add to PATH if not already there
$UserPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ($UserPath -notlike "*$InstallDir*") {
    [Environment]::SetEnvironmentVariable("PATH", "$UserPath;$InstallDir", "User")
    Write-Host "✓ Added to PATH" -ForegroundColor Green
    Write-Host "⚠️  You may need to restart your terminal for PATH changes to take effect" -ForegroundColor Yellow
}

# Verify installation
Write-Host "✅ Verifying installation..." -ForegroundColor Yellow
try {
    $Version = & "$BinaryPath" --version 2>&1
    Write-Host "✓ bookdl installed successfully: $Version" -ForegroundColor Green
} catch {
    Write-Host "❌ Installation verification failed" -ForegroundColor Red
    exit 1
}

Write-Host ""
Write-Host "✨ Installation complete!" -ForegroundColor Green
Write-Host ""
Write-Host "Usage:" -ForegroundColor Cyan
Write-Host "  bookdl search `"clean code`""
Write-Host ""
Write-Host "For more information: bookdl --help"
