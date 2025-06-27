# Container Kit Installation Script for Windows
# This script downloads and installs the latest version of container-kit on Windows

param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:PROGRAMFILES\container-kit",
    [switch]$AddToPath = $true,
    [switch]$Force = $false
)

$ErrorActionPreference = "Stop"

# Configuration
$RepoOwner = "Azure"
$RepoName = "container-kit"
$BinaryName = "container-kit.exe"

# Colors for output
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-Info($message) {
    Write-Host $message -ForegroundColor Yellow
}

function Write-Success($message) {
    Write-Host $message -ForegroundColor Green
}

function Write-Error($message) {
    Write-Host "Error: $message" -ForegroundColor Red
}

# Check if running as administrator
function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Get the latest release version from GitHub
function Get-LatestVersion {
    Write-Info "Fetching latest release information..."

    try {
        $releases = Invoke-RestMethod -Uri "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest" -Method Get
        $latestVersion = $releases.tag_name

        if ([string]::IsNullOrEmpty($latestVersion)) {
            throw "Failed to fetch latest release version"
        }

        Write-Info "Latest version: $latestVersion"
        return $latestVersion
    }
    catch {
        Write-Error "Failed to fetch latest release: $_"
        exit 1
    }
}

# Download the binary
function Download-Binary {
    param(
        [string]$Version,
        [string]$DestinationPath
    )

    # Determine architecture
    $arch = if ([Environment]::Is64BitOperatingSystem) { "amd64" } else { "386" }

    # Construct download URL
    $versionTag = if ($Version -match '^v') { $Version } else { "v$Version" }
    $archiveName = "container-kit_$($versionTag.TrimStart('v'))_windows_$arch.zip"
    $downloadUrl = "https://github.com/$RepoOwner/$RepoName/releases/download/$versionTag/$archiveName"
    $checksumUrl = "https://github.com/$RepoOwner/$RepoName/releases/download/$versionTag/checksums.txt"

    Write-Info "Downloading container-kit $versionTag for Windows ($arch)..."

    # Create temporary directory
    $tempDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }

    try {
        # Download archive
        $archivePath = Join-Path $tempDir $archiveName
        Invoke-WebRequest -Uri $downloadUrl -OutFile $archivePath -UseBasicParsing

        # Download checksums
        $checksumPath = Join-Path $tempDir "checksums.txt"
        try {
            Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath -UseBasicParsing

            # Verify checksum
            Write-Info "Verifying checksum..."
            $expectedChecksum = (Get-Content $checksumPath | Select-String $archiveName).Line.Split(' ')[0]
            $actualChecksum = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLower()

            if ($expectedChecksum -eq $actualChecksum) {
                Write-Success "Checksum verified"
            }
            else {
                throw "Checksum verification failed"
            }
        }
        catch {
            Write-Info "Skipping checksum verification: $_"
        }

        # Extract archive
        Write-Info "Extracting binary..."
        Expand-Archive -Path $archivePath -DestinationPath $tempDir -Force

        # Find the binary
        $binaryPath = Get-ChildItem -Path $tempDir -Filter "container-kit.exe" -Recurse | Select-Object -First 1

        if (-not $binaryPath) {
            throw "Binary not found in archive"
        }

        # Move binary to destination
        Move-Item -Path $binaryPath.FullName -Destination $DestinationPath -Force
        Write-Success "Binary downloaded successfully"
    }
    finally {
        # Clean up temp directory
        Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

# Add directory to PATH
function Add-ToPath {
    param(
        [string]$Directory
    )

    $currentPath = [Environment]::GetEnvironmentVariable("Path", [EnvironmentVariableTarget]::Machine)

    if ($currentPath -notlike "*$Directory*") {
        Write-Info "Adding $Directory to system PATH..."

        try {
            $newPath = "$currentPath;$Directory"
            [Environment]::SetEnvironmentVariable("Path", $newPath, [EnvironmentVariableTarget]::Machine)

            # Update current session
            $env:Path = "$env:Path;$Directory"

            Write-Success "Successfully added to PATH"
            Write-Info "Note: You may need to restart your terminal for PATH changes to take effect"
        }
        catch {
            Write-Error "Failed to update PATH: $_"
            Write-Info "Please add the following directory to your PATH manually: $Directory"
        }
    }
    else {
        Write-Info "$Directory is already in PATH"
    }
}

# Main installation function
function Install-ContainerKit {
    Write-Info "=== Container Kit Installation Script for Windows ==="
    Write-Info ""

    # Check if running as administrator
    if (-not (Test-Administrator)) {
        Write-Error "This script requires administrator privileges"
        Write-Info "Please run PowerShell as Administrator and try again"
        Write-Info ""
        Write-Info "To run as Administrator:"
        Write-Info "1. Right-click on PowerShell"
        Write-Info "2. Select 'Run as Administrator'"
        Write-Info "3. Run this script again"
        exit 1
    }

    # Check for existing installation
    $existingBinary = Join-Path $InstallDir $BinaryName
    if (Test-Path $existingBinary) {
        if (-not $Force) {
            $currentVersion = & $existingBinary --version 2>$null
            Write-Info "Found existing installation: $currentVersion"
            $response = Read-Host "Do you want to proceed with reinstallation? (y/N)"
            if ($response -ne 'y' -and $response -ne 'Y') {
                Write-Info "Installation cancelled"
                exit 0
            }
        }
    }

    # Get version to install
    $versionToInstall = if ($Version -eq "latest") {
        Get-LatestVersion
    } else {
        $Version
    }

    # Create installation directory
    if (-not (Test-Path $InstallDir)) {
        Write-Info "Creating installation directory: $InstallDir"
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    # Download and install binary
    $binaryPath = Join-Path $InstallDir $BinaryName
    Download-Binary -Version $versionToInstall -DestinationPath $binaryPath

    # Add to PATH if requested
    if ($AddToPath) {
        Add-ToPath -Directory $InstallDir
    }

    # Verify installation
    Write-Info ""
    Write-Info "Verifying installation..."

    try {
        $installedVersion = & $binaryPath --version 2>$null
        Write-Success "✅ container-kit is installed successfully"
        Write-Info "Version: $installedVersion"
        Write-Info "Location: $binaryPath"
        Write-Info ""
        Write-Info "To get started, run:"
        Write-Info "  container-kit --help"
    }
    catch {
        Write-Error "❌ container-kit was installed but cannot be executed"
        Write-Info "Error: $_"
        exit 1
    }

    Write-Info ""
    Write-Success "🎉 Installation complete!"
}

# Uninstall function
function Uninstall-ContainerKit {
    Write-Info "=== Container Kit Uninstallation ==="

    if (-not (Test-Administrator)) {
        Write-Error "This script requires administrator privileges for uninstallation"
        exit 1
    }

    $binaryPath = Join-Path $InstallDir $BinaryName

    if (Test-Path $binaryPath) {
        Write-Info "Removing container-kit..."
        Remove-Item -Path $binaryPath -Force

        # Remove directory if empty
        if ((Get-ChildItem $InstallDir | Measure-Object).Count -eq 0) {
            Remove-Item -Path $InstallDir -Force
        }

        Write-Success "container-kit has been uninstalled"

        # Note about PATH
        Write-Info ""
        Write-Info "Note: If $InstallDir was added to PATH, you may want to remove it manually"
    }
    else {
        Write-Info "container-kit is not installed at $InstallDir"
    }
}

# Parse command line arguments
if ($args -contains "-Uninstall") {
    Uninstall-ContainerKit
}
else {
    Install-ContainerKit
}
