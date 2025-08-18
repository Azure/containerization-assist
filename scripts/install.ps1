# Containerization Assist Installation Script for Windows
# This script downloads and installs the latest version of containerization-assist

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\containerization-assist",
    [string]$Version = "latest"
)

$ErrorActionPreference = "Stop"

# Configuration
$RepoOwner = "Azure"
$RepoName = "containerization-assist"
$BinaryName = "containerization-assist.exe"

# Colors and output helpers
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    else {
        $input | Write-Output
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

function Write-Error-Message($message) {
    Write-ColorOutput Red "Error: $message"
}

function Write-Success($message) {
    Write-ColorOutput Green $message
}

function Write-Info($message) {
    Write-ColorOutput Yellow $message
}

# Detect architecture
function Get-Architecture {
    $arch = $env:PROCESSOR_ARCHITECTURE
    switch ($arch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default {
            Write-Error-Message "Unsupported architecture: $arch"
            exit 1
        }
    }
}

# Create installation directory
function Initialize-InstallDirectory {
    if (!(Test-Path $InstallDir)) {
        Write-Info "Creating installation directory: $InstallDir"
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
}

# Download the binary
function Download-Binary {
    $arch = Get-Architecture
    $platform = "windows_$arch"
    
    Write-Info "Downloading Containerization Assist for Windows ($arch)..."
    
    # Construct download URLs
    if ($Version -eq "latest") {
        $downloadUrl = "https://github.com/$RepoOwner/$RepoName/releases/latest/download/${RepoName}_${platform}.zip"
        $checksumUrl = "https://github.com/$RepoOwner/$RepoName/releases/latest/download/checksums.txt"
    }
    else {
        $downloadUrl = "https://github.com/$RepoOwner/$RepoName/releases/download/$Version/${RepoName}_${platform}.zip"
        $checksumUrl = "https://github.com/$RepoOwner/$RepoName/releases/download/$Version/checksums.txt"
    }
    
    $archivePath = Join-Path $env:TEMP "containerization-assist_${platform}.zip"
    $checksumPath = Join-Path $env:TEMP "checksums.txt"
    
    try {
        # Download archive
        Write-Info "Downloading from: $downloadUrl"
        Invoke-WebRequest -Uri $downloadUrl -OutFile $archivePath -UseBasicParsing
        
        # Download checksums
        try {
            Invoke-WebRequest -Uri $checksumUrl -OutFile $checksumPath -UseBasicParsing -ErrorAction SilentlyContinue
        }
        catch {
            Write-Info "Checksums not available for verification"
        }
        
        # Verify checksum if available
        if (Test-Path $checksumPath) {
            Write-Info "Verifying checksum..."
            $expectedChecksum = (Get-Content $checksumPath | Select-String "${RepoName}_${platform}.zip").Line.Split(' ')[0]
            $actualChecksum = (Get-FileHash -Path $archivePath -Algorithm SHA256).Hash.ToLower()
            
            if ($expectedChecksum -eq $actualChecksum) {
                Write-Success "Checksum verified"
            }
            else {
                Write-Error-Message "Checksum verification failed"
                Write-Error-Message "Expected: $expectedChecksum"
                Write-Error-Message "Actual: $actualChecksum"
                exit 1
            }
        }
        
        # Extract archive
        Write-Info "Extracting binary..."
        $extractPath = Join-Path $env:TEMP "containerization-assist-extract"
        if (Test-Path $extractPath) {
            Remove-Item -Path $extractPath -Recurse -Force
        }
        
        Expand-Archive -Path $archivePath -DestinationPath $extractPath -Force
        
        # Find the binary
        $binaryPath = Get-ChildItem -Path $extractPath -Filter "containerization-assist.exe" -Recurse | Select-Object -First 1
        if (!$binaryPath) {
            Write-Error-Message "Binary not found in archive"
            exit 1
        }
        
        # Copy to installation directory
        $destPath = Join-Path $InstallDir $BinaryName
        Copy-Item -Path $binaryPath.FullName -Destination $destPath -Force
        
        Write-Success "Successfully installed Containerization Assist to: $destPath"
        
        # Cleanup
        Remove-Item -Path $archivePath -Force -ErrorAction SilentlyContinue
        Remove-Item -Path $checksumPath -Force -ErrorAction SilentlyContinue
        Remove-Item -Path $extractPath -Recurse -Force -ErrorAction SilentlyContinue
        
        return $destPath
    }
    catch {
        Write-Error-Message "Failed to download or extract Containerization Assist: $_"
        exit 1
    }
}

# Add to PATH
function Add-ToPath {
    param($Directory)
    
    $currentPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($currentPath -notlike "*$Directory*") {
        Write-Info "Adding $Directory to PATH..."
        $newPath = "$currentPath;$Directory"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        $env:Path = "$env:Path;$Directory"
        
        Write-Info ""
        Write-Info "PATH has been updated. You may need to restart your terminal for changes to take effect."
    }
    else {
        Write-Info "$Directory is already in PATH"
    }
}

# Verify installation
function Test-Installation {
    $testPath = Join-Path $InstallDir $BinaryName
    if (Test-Path $testPath) {
        try {
            $version = & $testPath --version 2>$null
            Write-Success "✅ Containerization Assist is installed and accessible"
            Write-Info "Version: $version"
            Write-Info ""
            Write-Info "To get started, run:"
            Write-Info "  containerization-assist --help"
            
            if ($BinaryName -eq "containerization-assist.exe") {
                Write-Info ""
                Write-Info "For MCP server functionality, run:"
                Write-Info "  containerization-assist-mcp"
            }
        }
        catch {
            Write-Error-Message "Containerization Assist was installed but cannot be executed"
            Write-Error-Message $_
            exit 1
        }
    }
    else {
        Write-Error-Message "Installation verification failed"
        exit 1
    }
}

# Main installation flow
function Install-ContainerizationAssist {
    Write-Info "=== Containerization Assist Installation Script ==="
    Write-Info ""
    
    # Check for existing installation
    $existingPath = Get-Command containerization-assist -ErrorAction SilentlyContinue
    if ($existingPath) {
        Write-Info "Found existing installation at: $($existingPath.Path)"
        $response = Read-Host "Do you want to proceed with reinstallation? (y/N)"
        if ($response -ne 'y' -and $response -ne 'Y') {
            Write-Info "Installation cancelled"
            exit 0
        }
    }
    
    # Check if running as administrator
    $isAdmin = ([Security.Principal.WindowsPrincipal] [Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole] "Administrator")
    if ($isAdmin) {
        Write-Info "Running with administrator privileges"
    }
    else {
        Write-Info "Running without administrator privileges (installing to user directory)"
    }
    
    # Initialize installation directory
    Initialize-InstallDirectory
    
    # Download and install
    $installedPath = Download-Binary
    
    # Add to PATH
    Add-ToPath -Directory $InstallDir
    
    # Verify installation
    Test-Installation
    
    Write-Info ""
    Write-Success "🎉 Installation complete!"
}

# Run installation
try {
    Install-ContainerizationAssist
}
catch {
    Write-Error-Message "Installation failed: $_"
    exit 1
}