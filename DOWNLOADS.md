# Download bookdl

Choose your platform to download the latest version:

## macOS

### Installers (Recommended - Double-click to install)

[![Download for Mac (Intel) - .pkg](https://img.shields.io/badge/Download-Mac%20(Intel)%20.pkg-blue?logo=apple&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-darwin-amd64.pkg)

[![Download for Mac (Apple Silicon) - .pkg](https://img.shields.io/badge/Download-Mac%20(Apple%20Silicon)%20.pkg-blue?logo=apple&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-darwin-arm64.pkg)

> **Note:** Apple Silicon includes M1, M2, and M3 chips. Just double-click the .pkg file and follow the installer.

### Binaries

[![Download for Mac (Intel)](https://img.shields.io/badge/Download-Mac%20(Intel)%20Binary-lightblue?logo=apple&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-darwin-amd64)

[![Download for Mac (Apple Silicon)](https://img.shields.io/badge/Download-Mac%20(Apple%20Silicon)%20Binary-lightblue?logo=apple&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-darwin-arm64)

## Linux

### Debian/Ubuntu (.deb Package)

[![Download for Linux (x86_64) - .deb](https://img.shields.io/badge/Download-Linux%20(x86__64)%20.deb-orange?logo=linux&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl_linux_amd64.deb)

[![Download for Linux (ARM64) - .deb](https://img.shields.io/badge/Download-Linux%20(ARM64)%20.deb-orange?logo=linux&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl_linux_arm64.deb)

Install with: `sudo dpkg -i bookdl_*.deb`

### Fedora/CentOS/RHEL (.rpm Package)

[![Download for Linux (x86_64) - .rpm](https://img.shields.io/badge/Download-Linux%20(x86__64)%20.rpm-orange?logo=linux&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-*.x86_64.rpm)

[![Download for Linux (ARM64) - .rpm](https://img.shields.io/badge/Download-Linux%20(ARM64)%20.rpm-orange?logo=linux&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-*.aarch64.rpm)

Install with: `sudo rpm -i bookdl-*.rpm`

### Binaries

[![Download for Linux (x86_64)](https://img.shields.io/badge/Download-Linux%20(x86__64)%20Binary-lightorange?logo=linux&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-linux-amd64)

[![Download for Linux (ARM64)](https://img.shields.io/badge/Download-Linux%20(ARM64)%20Binary-lightorange?logo=linux&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-linux-arm64)

## Windows

### Binary (Recommended for Windows)

[![Download for Windows](https://img.shields.io/badge/Download-Windows%20.exe-blue?logo=windows&style=for-the-badge)](https://github.com/billmal071/bookdl/releases/latest/download/bookdl-windows-amd64.exe)

> **Note:** The Windows installer requires Inno Setup to build. For now, use the binary or PowerShell install script.

## Installation

After downloading, follow the instructions for your platform in [INSTALL.md](INSTALL.md).

## Quick Install Scripts

Prefer command-line installation? Use our install scripts:

**macOS:**
```bash
curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-mac.sh | bash
```

**Linux:**
```bash
curl -fsSL https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-linux.sh | bash
```

**Windows (PowerShell):**
```powershell
iwr -useb https://raw.githubusercontent.com/billmal071/bookdl/main/scripts/install-windows.ps1 | iex
```

## Verification

Verify your download with checksums from the [Releases page](https://github.com/billmal071/bookdl/releases/latest).

```bash
# macOS/Linux
sha256sum bookdl-*

# Windows (PowerShell)
Get-FileHash bookdl-*.exe -Algorithm SHA256
```

## Need Help?

- 📖 [Full installation guide](INSTALL.md)
- 📚 [User guide](README.md)
- 🐛 [Report an issue](https://github.com/billmal071/bookdl/issues)
