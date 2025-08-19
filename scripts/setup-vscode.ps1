# Containerization Assist VS Code Setup Script for Windows
# This script installs Containerization Assist and configures it for VS Code with MCP support

param(
    [string]$InstallDir = "$env:LOCALAPPDATA\Programs\containerization-assist"
)

$ErrorActionPreference = "Stop"

# Configuration
$RepoOwner = "Azure"
$RepoName = "containerization-assist"
$BinaryName = "containerization-assist-mcp.exe"
$CliBinaryName = "containerization-assist.exe"

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
    Write-ColorOutput Red "❌ Error: $message"
}

function Write-Success($message) {
    Write-ColorOutput Green "✅ $message"
}

function Write-Info($message) {
    Write-ColorOutput Cyan "ℹ️  $message"
}

function Write-Warning($message) {
    Write-ColorOutput Yellow "⚠️  $message"
}

function Write-Step($message) {
    Write-ColorOutput Yellow "🔧 $message"
}

# Check if running as administrator
function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Check prerequisites
function Test-Prerequisites {
    Write-Step "Checking prerequisites..."
    
    $missingPrereqs = $false
    
    # Check for VS Code
    $vscodePath = Get-Command code -ErrorAction SilentlyContinue
    if ($vscodePath) {
        Write-Success "VS Code CLI found"
    }
    else {
        # Check common VS Code installation paths
        $vscodeInstalled = $false
        $commonPaths = @(
            "$env:LOCALAPPDATA\Programs\Microsoft VS Code\bin\code.cmd",
            "$env:ProgramFiles\Microsoft VS Code\bin\code.cmd",
            "${env:ProgramFiles(x86)}\Microsoft VS Code\bin\code.cmd"
        )
        
        foreach ($path in $commonPaths) {
            if (Test-Path $path) {
                Write-Warning "VS Code found but 'code' command not in PATH"
                Write-Info "Add VS Code to PATH or restart your terminal"
                $vscodeInstalled = $true
                break
            }
        }
        
        if (-not $vscodeInstalled) {
            Write-Warning "VS Code not found"
            Write-Info "Please install VS Code from: https://code.visualstudio.com/"
            $missingPrereqs = $true
        }
    }
    
    # Check for Docker
    $dockerPath = Get-Command docker -ErrorAction SilentlyContinue
    if ($dockerPath) {
        Write-Success "Docker found"
    }
    else {
        Write-Warning "Docker not found"
        Write-Info "Docker is required for container operations"
        Write-Info "Install from: https://www.docker.com/products/docker-desktop/"
    }
    
    # Check for Git
    $gitPath = Get-Command git -ErrorAction SilentlyContinue
    if ($gitPath) {
        Write-Success "Git found"
    }
    else {
        Write-Warning "Git not found"
        Write-Info "Git is recommended for version control"
    }
    
    if ($missingPrereqs) {
        $response = Read-Host "Continue anyway? (y/N)"
        if ($response -ne 'y' -and $response -ne 'Y') {
            Write-Info "Installation cancelled"
            exit 0
        }
    }
}

# Get architecture
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

# Download and install Containerization Assist
function Install-ContainerizationAssist {
    Write-Step "Installing Containerization Assist..."
    
    $arch = Get-Architecture
    $platform = "windows_$arch"
    
    # Create installation directory
    if (!(Test-Path $InstallDir)) {
        Write-Info "Creating installation directory: $InstallDir"
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }
    
    # Download latest release
    $downloadUrl = "https://github.com/$RepoOwner/$RepoName/releases/latest/download/${RepoName}_${platform}.zip"
    $checksumUrl = "https://github.com/$RepoOwner/$RepoName/releases/latest/download/checksums.txt"
    
    $tempDir = Join-Path $env:TEMP "containerization-assist-install"
    if (Test-Path $tempDir) {
        Remove-Item -Path $tempDir -Recurse -Force
    }
    New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
    
    $archivePath = Join-Path $tempDir "${RepoName}_${platform}.zip"
    $checksumPath = Join-Path $tempDir "checksums.txt"
    
    try {
        Write-Info "Downloading Containerization Assist..."
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
                exit 1
            }
        }
        
        # Extract archive
        Write-Info "Extracting binaries..."
        Expand-Archive -Path $archivePath -DestinationPath $tempDir -Force
        
        # Copy binaries to installation directory
        $binaries = @($BinaryName, $CliBinaryName)
        foreach ($binary in $binaries) {
            $sourcePath = Join-Path $tempDir $binary
            if (Test-Path $sourcePath) {
                $destPath = Join-Path $InstallDir $binary
                Copy-Item -Path $sourcePath -Destination $destPath -Force
                Write-Success "Installed $binary"
            }
        }
        
        Write-Success "Containerization Assist installed to: $InstallDir"
    }
    catch {
        Write-Error-Message "Failed to install Containerization Assist: $_"
        exit 1
    }
    finally {
        # Cleanup
        if (Test-Path $tempDir) {
            Remove-Item -Path $tempDir -Recurse -Force -ErrorAction SilentlyContinue
        }
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
        
        Write-Info "PATH has been updated. You may need to restart your terminal."
    }
    else {
        Write-Info "$Directory is already in PATH"
    }
}

# Find VS Code settings.json location
function Get-VSCodeSettingsPath {
    $settingsPaths = @(
        "$env:APPDATA\Code\User\settings.json",
        "$env:APPDATA\Code - Insiders\User\settings.json"
    )
    
    foreach ($path in $settingsPaths) {
        if (Test-Path (Split-Path $path -Parent)) {
            return $path
        }
    }
    
    # Default to regular VS Code path
    return $settingsPaths[0]
}

# Configure VS Code for MCP
function Set-VSCodeConfiguration {
    Write-Step "Configuring VS Code for Containerization Assist MCP..."
    
    $settingsPath = Get-VSCodeSettingsPath
    $settingsDir = Split-Path $settingsPath -Parent
    
    # Create settings directory if it doesn't exist
    if (!(Test-Path $settingsDir)) {
        New-Item -ItemType Directory -Path $settingsDir -Force | Out-Null
    }
    
    # MCP configuration to add
    $mcpConfig = @{
        "mcp.servers" = @{
            "containerization-assist" = @{
                "command" = "containerization-assist-mcp"
                "args" = @()
                "transport" = "stdio"
            }
        }
        "github.copilot.chat.experimental.mcp.enabled" = $true
    }
    
    try {
        if (Test-Path $settingsPath) {
            # Backup existing settings
            Write-Info "Backing up existing VS Code settings..."
            $backupPath = "$settingsPath.backup.$(Get-Date -Format 'yyyyMMdd_HHmmss')"
            Copy-Item -Path $settingsPath -Destination $backupPath -Force
            
            # Read existing settings
            $existingSettings = Get-Content $settingsPath -Raw | ConvertFrom-Json -AsHashtable
            
            # Check if MCP is already configured
            if ($existingSettings.ContainsKey("mcp.servers") -and $existingSettings["mcp.servers"].ContainsKey("containerization-assist")) {
                Write-Warning "Containerization Assist MCP configuration already exists"
                Write-Info "Updating existing configuration..."
            }
            
            # Merge configurations
            foreach ($key in $mcpConfig.Keys) {
                $existingSettings[$key] = $mcpConfig[$key]
            }
            
            # Write updated settings
            $existingSettings | ConvertTo-Json -Depth 10 | Set-Content $settingsPath -Encoding UTF8
        }
        else {
            # Create new settings file
            $mcpConfig | ConvertTo-Json -Depth 10 | Set-Content $settingsPath -Encoding UTF8
        }
        
        Write-Success "VS Code configuration updated"
    }
    catch {
        Write-Error-Message "Failed to update VS Code settings: $_"
        Write-Info "Please add the following to your VS Code settings.json manually:"
        Write-Info ($mcpConfig | ConvertTo-Json -Depth 10)
    }
}

# Install VS Code extensions
function Install-VSCodeExtensions {
    Write-Step "Installing recommended VS Code extensions..."
    
    $codePath = Get-Command code -ErrorAction SilentlyContinue
    if (-not $codePath) {
        Write-Warning "VS Code CLI not found, skipping extension installation"
        Write-Info "Install these extensions manually:"
        Write-Info "  - GitHub Copilot (GitHub.copilot)"
        Write-Info "  - GitHub Copilot Chat (GitHub.copilot-chat)"
        Write-Info "  - Docker (ms-azuretools.vscode-docker)"
        return
    }
    
    $extensions = @(
        "GitHub.copilot",
        "GitHub.copilot-chat", 
        "ms-azuretools.vscode-docker"
    )
    
    foreach ($ext in $extensions) {
        Write-Info "Installing $ext..."
        try {
            & code --install-extension $ext --force 2>$null
            Write-Success "Installed $ext"
        }
        catch {
            Write-Warning "Failed to install $ext"
        }
    }
}

# Verify installation
function Test-Installation {
    Write-Step "Verifying installation..."
    
    $mcpPath = Join-Path $InstallDir $BinaryName
    if (Test-Path $mcpPath) {
        try {
            $version = & $mcpPath --version 2>$null
            Write-Success "Containerization Assist MCP is installed"
            Write-Info "Version: $version"
        }
        catch {
            Write-Warning "Containerization Assist MCP installed but cannot get version"
        }
    }
    else {
        Write-Error-Message "Containerization Assist MCP not found at: $mcpPath"
        return $false
    }
    
    # Check VS Code configuration
    $settingsPath = Get-VSCodeSettingsPath
    if (Test-Path $settingsPath) {
        $content = Get-Content $settingsPath -Raw
        if ($content -match "containerization-assist") {
            Write-Success "VS Code MCP configuration found"
        }
        else {
            Write-Warning "VS Code MCP configuration not found"
        }
    }
    
    return $true
}

# Print final instructions
function Show-FinalInstructions {
    Write-Host ""
    Write-Success "🎉 Setup complete!"
    Write-Host ""
    Write-Info "Next steps:"
    Write-Info "1. Restart VS Code (or open a new VS Code window)"
    Write-Info "2. Open GitHub Copilot Chat (Ctrl+Alt+I)"
    Write-Info "3. Ask: 'What Containerization Assist tools are available?'"
    Write-Host ""
    Write-Info "To use Containerization Assist:"
    Write-Info "• Ask Copilot to analyze your repository"
    Write-Info "• Request help containerizing your application"
    Write-Info "• Use specific tools like 'generate_dockerfile' or 'build_image'"
    Write-Host ""
    
    $dockerPath = Get-Command docker -ErrorAction SilentlyContinue
    if (-not $dockerPath) {
        Write-Warning "Remember to install Docker Desktop for container operations"
        Write-Info "Download from: https://www.docker.com/products/docker-desktop/"
    }
    
    Write-Info "For help, visit: https://github.com/Azure/containerization-assist"
}

# Main installation flow
function Install-ContainerizationAssistVSCode {
    Write-Host ""
    Write-Info "=== Containerization Assist VS Code Setup Script ==="
    Write-Info "This script will install Containerization Assist and configure it for VS Code"
    Write-Host ""
    
    # Check if already installed
    $existingMcp = Get-Command containerization-assist-mcp -ErrorAction SilentlyContinue
    if ($existingMcp) {
        try {
            $currentVersion = & containerization-assist-mcp --version 2>$null
            Write-Info "Found existing Containerization Assist installation: $currentVersion"
        }
        catch {
            Write-Info "Found existing Containerization Assist installation"
        }
        
        $response = Read-Host "Do you want to reinstall? (y/N)"
        if ($response -ne 'y' -and $response -ne 'Y') {
            Write-Info "Skipping Containerization Assist installation"
            # Still configure VS Code
            Set-VSCodeConfiguration
            Install-VSCodeExtensions
            Test-Installation | Out-Null
            Show-FinalInstructions
            return
        }
    }
    
    # Run installation steps
    Test-Prerequisites
    Install-ContainerizationAssist
    Add-ToPath -Directory $InstallDir
    Set-VSCodeConfiguration
    Install-VSCodeExtensions
    
    if (Test-Installation) {
        Show-FinalInstructions
    }
    else {
        Write-Error-Message "Installation completed with errors. Please check the output above."
    }
}

# Run main function
try {
    Install-ContainerizationAssistVSCode
}
catch {
    Write-Error-Message "Installation failed: $_"
    exit 1
}