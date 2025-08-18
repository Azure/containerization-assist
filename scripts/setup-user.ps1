# Containerization Assist User Setup Script for Windows
# This script sets up Containerization Assist MCP Server for non-technical Windows users

param(
    [switch]$Force,
    [switch]$Help
)

# Configuration
$RepoOwner = "Azure"
$RepoName = "containerization-assist"
$BinaryName = "containerization-assist-mcp"
$InstallDir = "$env:ProgramFiles\ContainerKit"
$FallbackDir = "$env:USERPROFILE\bin"

# Colors for output (Windows PowerShell compatible)
function Write-ColorText {
    param(
        [string]$Text,
        [string]$Color = "White"
    )
    
    $originalColor = $Host.UI.RawUI.ForegroundColor
    $Host.UI.RawUI.ForegroundColor = $Color
    Write-Host $Text
    $Host.UI.RawUI.ForegroundColor = $originalColor
}

function Write-Error-Custom {
    param([string]$Message)
    Write-ColorText "❌ Error: $Message" "Red"
}

function Write-Success {
    param([string]$Message)
    Write-ColorText "✅ $Message" "Green"
}

function Write-Info {
    param([string]$Message)
    Write-ColorText "ℹ️  $Message" "Cyan"
}

function Write-Warning-Custom {
    param([string]$Message)
    Write-ColorText "⚠️  $Message" "Yellow"
}

function Write-Step {
    param([string]$Message)
    Write-ColorText "🔧 $Message" "Yellow"
}

# Show help
function Show-Help {
    Write-Host @"
Containerization Assist User Setup Script for Windows

USAGE:
    .\setup-user.ps1 [OPTIONS]

OPTIONS:
    -Force      Skip all confirmation prompts
    -Help       Show this help message

DESCRIPTION:
    This script installs Containerization Assist and configures it for use with Claude Desktop.
    It will:
    1. Download and install Containerization Assist
    2. Configure Claude Desktop MCP settings
    3. Test the installation
    4. Show next steps

REQUIREMENTS:
    - Windows 10/11
    - PowerShell 5.1 or later
    - Administrator privileges (recommended)
    - Internet connection

EXAMPLES:
    .\setup-user.ps1                 # Interactive installation
    .\setup-user.ps1 -Force          # Non-interactive installation

"@
}

# Check if running as Administrator
function Test-Administrator {
    $currentUser = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = New-Object Security.Principal.WindowsPrincipal($currentUser)
    return $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)
}

# Check if command exists
function Test-Command {
    param([string]$CommandName)
    
    try {
        if (Get-Command $CommandName -ErrorAction SilentlyContinue) {
            return $true
        }
        return $false
    }
    catch {
        return $false
    }
}

# Check prerequisites
function Test-Prerequisites {
    Write-Step "Checking prerequisites..."
    
    $missingTools = @()
    
    # Check PowerShell version
    if ($PSVersionTable.PSVersion.Major -lt 5) {
        $missingTools += "PowerShell 5.1 or later"
    }
    
    # Check for internet connectivity
    try {
        $response = Invoke-WebRequest -Uri "https://github.com" -UseBasicParsing -TimeoutSec 10
        if ($response.StatusCode -ne 200) {
            $missingTools += "Internet connectivity"
        }
    }
    catch {
        $missingTools += "Internet connectivity"
    }
    
    # Check for Docker (warn if missing)
    if (-not (Test-Command "docker")) {
        Write-Warning-Custom "Docker is not installed. You'll need Docker to use Containerization Assist's containerization features."
        Write-Info "Install Docker from: https://www.docker.com/products/docker-desktop/"
    }
    else {
        Write-Success "Docker found"
    }
    
    # Check for Git (warn if missing)
    if (-not (Test-Command "git")) {
        Write-Warning-Custom "Git is not installed. Some Containerization Assist features may require Git."
        Write-Info "Install Git from: https://git-scm.com/downloads"
    }
    else {
        Write-Success "Git found"
    }
    
    if ($missingTools.Count -gt 0) {
        Write-Error-Custom "Missing required components: $($missingTools -join ', ')"
        Write-Info "Please install the missing components and run this script again."
        exit 1
    }
    
    Write-Success "Prerequisites check passed"
}

# Download and install Containerization Assist
function Install-ContainerKit {
    Write-Step "Installing Containerization Assist..."
    
    try {
        # Get the latest release
        $apiUrl = "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest"
        $release = Invoke-RestMethod -Uri $apiUrl
        
        # Find Windows binary
        $asset = $release.assets | Where-Object { $_.name -like "*windows_amd64*" }
        if (-not $asset) {
            Write-Error-Custom "Windows binary not found in latest release"
            exit 1
        }
        
        # Create temporary directory
        $tempDir = New-TemporaryFile | ForEach-Object { Remove-Item $_; New-Item -ItemType Directory -Path $_ }
        $downloadPath = Join-Path $tempDir $asset.name
        
        Write-Info "Downloading $($asset.name)..."
        Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $downloadPath
        
        # Extract the archive
        Write-Info "Extracting binary..."
        if ($asset.name.EndsWith('.zip')) {
            Expand-Archive -Path $downloadPath -DestinationPath $tempDir -Force
        }
        else {
            # Handle .tar.gz (requires additional tools or PowerShell 7+)
            Write-Error-Custom "Unsupported archive format. Please download manually from GitHub releases."
            exit 1
        }
        
        # Find the binary
        $binaryPath = Get-ChildItem -Path $tempDir -Name "$BinaryName.exe" -Recurse | Select-Object -First 1
        if (-not $binaryPath) {
            Write-Error-Custom "Binary not found in extracted archive"
            exit 1
        }
        
        $fullBinaryPath = Join-Path $tempDir $binaryPath
        
        # Install to system directory or user directory
        $installPath = ""
        
        if ((Test-Administrator) -and (-not (Test-Path $InstallDir))) {
            New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
            $installPath = Join-Path $InstallDir "$BinaryName.exe"
        }
        elseif (Test-Administrator) {
            $installPath = Join-Path $InstallDir "$BinaryName.exe"
        }
        else {
            Write-Warning-Custom "Not running as Administrator. Installing to user directory."
            if (-not (Test-Path $FallbackDir)) {
                New-Item -ItemType Directory -Path $FallbackDir -Force | Out-Null
            }
            $installPath = Join-Path $FallbackDir "$BinaryName.exe"
        }
        
        # Copy binary
        Copy-Item -Path $fullBinaryPath -Destination $installPath -Force
        
        # Add to PATH if needed
        $pathDirectories = $env:PATH -split ';'
        $binaryDir = Split-Path $installPath -Parent
        
        if ($pathDirectories -notcontains $binaryDir) {
            Write-Info "Adding Containerization Assist to PATH..."
            $newPath = "$binaryDir;$env:PATH"
            [Environment]::SetEnvironmentVariable("PATH", $newPath, [EnvironmentVariableTarget]::User)
            $env:PATH = $newPath
        }
        
        # Clean up
        Remove-Item -Path $tempDir -Recurse -Force
        
        # Verify installation
        if (Test-Command $BinaryName) {
            try {
                $version = & $BinaryName --version 2>$null
                Write-Success "Containerization Assist installed successfully"
                Write-Info "Version: $version"
            }
            catch {
                Write-Success "Containerization Assist installed (version check failed)"
            }
        }
        else {
            Write-Error-Custom "Containerization Assist installation failed - binary not accessible"
            Write-Info "You may need to restart PowerShell or add $binaryDir to your PATH"
            exit 1
        }
    }
    catch {
        Write-Error-Custom "Installation failed: $($_.Exception.Message)"
        exit 1
    }
}

# Setup Claude Desktop configuration
function Set-ClaudeConfig {
    Write-Step "Setting up Claude Desktop configuration..."
    
    $configDir = "$env:APPDATA\Claude"
    $configFile = Join-Path $configDir "claude_desktop_config.json"
    
    # Check if Claude Desktop is installed
    $claudeInstalled = $false
    $claudePaths = @(
        "$env:LOCALAPPDATA\Programs\Claude\Claude.exe",
        "$env:ProgramFiles\Claude\Claude.exe",
        "$env:ProgramFiles(x86)\Claude\Claude.exe"
    )
    
    foreach ($path in $claudePaths) {
        if (Test-Path $path) {
            $claudeInstalled = $true
            break
        }
    }
    
    if (-not $claudeInstalled) {
        Write-Warning-Custom "Claude Desktop not found"
        Write-Info "Please install Claude Desktop from: https://claude.ai/download"
        Write-Info "Then run this script again, or manually configure the MCP server"
        return
    }
    
    # Create config directory if it doesn't exist
    if (-not (Test-Path $configDir)) {
        New-Item -ItemType Directory -Path $configDir -Force | Out-Null
    }
    
    # Create configuration content
    $configContent = @{
        mcpServers = @{
            "containerization-assist" = @{
                command = $BinaryName
                args = @()
            }
        }
    } | ConvertTo-Json -Depth 3
    
    # Backup existing configuration
    if (Test-Path $configFile) {
        $backupFile = "$configFile.backup.$(Get-Date -Format 'yyyyMMdd-HHmmss')"
        Copy-Item -Path $configFile -Destination $backupFile
        Write-Info "Backed up existing configuration to: $backupFile"
    }
    
    # Write new configuration
    $configContent | Out-File -FilePath $configFile -Encoding UTF8
    Write-Success "Claude Desktop configuration created"
    Write-Info "Configuration file: $configFile"
}

# Test the installation
function Test-Installation {
    Write-Step "Testing installation..."
    
    # Test Containerization Assist
    if (Test-Command $BinaryName) {
        Write-Success "Containerization Assist is accessible from command line"
        
        # Test version command
        try {
            $version = & $BinaryName --version 2>$null
            Write-Success "Version check passed: $version"
        }
        catch {
            Write-Warning-Custom "Version check failed, but binary is accessible"
        }
    }
    else {
        Write-Error-Custom "Containerization Assist is not accessible from command line"
        Write-Info "You may need to restart PowerShell or check your PATH"
    }
    
    # Test Docker if available
    if (Test-Command "docker") {
        try {
            $dockerVersion = docker version 2>$null
            Write-Success "Docker is running"
        }
        catch {
            Write-Warning-Custom "Docker is installed but not running"
            Write-Info "Please start Docker Desktop"
        }
    }
}

# Show next steps
function Show-NextSteps {
    Write-Host ""
    Write-Success "🎉 Containerization Assist User Setup Complete!"
    Write-Host ""
    Write-Info "Next Steps:"
    Write-Host ""
    Write-Info "1. 📱 Open Claude Desktop (restart if it was running)"
    Write-Info "2. 💬 Start a new conversation"
    Write-Info "3. 🗣️  Ask: 'What Containerization Assist tools are available?'"
    Write-Info "4. 🚀 Try: 'Help me containerize my application at [your-repo-url]'"
    Write-Host ""
    Write-Info "📚 Documentation:"
    Write-Info "   • User Guide: USER_GUIDE.md in the Containerization Assist repository"
    Write-Info "   • GitHub: https://github.com/$RepoOwner/$RepoName"
    Write-Host ""
    Write-Info "🆘 Need Help?"
    Write-Info "   • Issues: https://github.com/$RepoOwner/$RepoName/issues"
    Write-Info "   • Discussions: https://github.com/$RepoOwner/$RepoName/discussions"
    Write-Host ""
    Write-Info "🔧 Advanced Configuration:"
    Write-Info "   • Claude Config: $env:APPDATA\Claude\claude_desktop_config.json"
    Write-Info "   • Add debug logging by adding 'env': {'CONTAINER_KIT_LOG_LEVEL': 'debug'}"
    Write-Host ""
}

# Main function
function Main {
    if ($Help) {
        Show-Help
        return
    }
    
    Write-Host ""
    Write-Info "=== Containerization Assist User Setup Script for Windows ==="
    Write-Info "This script will install Containerization Assist and configure it for Claude Desktop"
    Write-Host ""
    
    # Check if user wants to continue
    if (-not $Force) {
        $continue = Read-Host "Do you want to continue? (y/N)"
        if ($continue -notmatch '^[Yy]') {
            Write-Info "Setup cancelled"
            return
        }
    }
    
    # Warn about Administrator privileges
    if (-not (Test-Administrator)) {
        Write-Warning-Custom "Not running as Administrator"
        Write-Info "Some features may not work correctly. Consider running as Administrator."
        
        if (-not $Force) {
            $continue = Read-Host "Continue anyway? (y/N)"
            if ($continue -notmatch '^[Yy]') {
                Write-Info "Setup cancelled"
                return
            }
        }
    }
    
    try {
        Test-Prerequisites
        Install-ContainerKit
        Set-ClaudeConfig
        Test-Installation
        Show-NextSteps
    }
    catch {
        Write-Error-Custom "Setup failed: $($_.Exception.Message)"
        Write-Info "Please check the error messages above and try again"
        exit 1
    }
}

# Handle Ctrl+C
trap {
    Write-Host ""
    Write-Warning-Custom "Setup interrupted by user"
    exit 1
}

# Run main function
Main