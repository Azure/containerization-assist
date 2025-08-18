# Containerization Assist User Update Script for Windows
# This script updates Containerization Assist MCP Server to the latest version

param(
    [switch]$Force,
    [switch]$Help
)

# Configuration
$RepoOwner = "Azure"
$RepoName = "containerization-assist"
$BinaryName = "containerization-assist-mcp"

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
Containerization Assist Update Script for Windows

USAGE:
    .\update-user.ps1 [OPTIONS]

OPTIONS:
    -Force      Skip all confirmation prompts and force update
    -Help       Show this help message

DESCRIPTION:
    This script updates Containerization Assist to the latest version available on GitHub.
    It will:
    1. Check your current version
    2. Check for the latest version on GitHub
    3. Download and install the update if needed
    4. Verify the installation

REQUIREMENTS:
    - Windows 10/11
    - PowerShell 5.1 or later
    - Internet connection
    - Administrator privileges (recommended)

EXAMPLES:
    .\update-user.ps1                 # Interactive update
    .\update-user.ps1 -Force          # Non-interactive update

"@
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

# Get current version
function Get-CurrentVersion {
    if (Test-Command $BinaryName) {
        try {
            $version = & $BinaryName --version 2>$null | Select-Object -First 1
            return $version
        }
        catch {
            return "unknown"
        }
    }
    else {
        return "not installed"
    }
}

# Get latest version from GitHub
function Get-LatestVersion {
    try {
        $apiUrl = "https://api.github.com/repos/$RepoOwner/$RepoName/releases/latest"
        $release = Invoke-RestMethod -Uri $apiUrl -ErrorAction Stop
        return $release.tag_name
    }
    catch {
        Write-Error-Custom "Could not check for latest version: $($_.Exception.Message)"
        return $null
    }
}

# Compare versions (simple comparison)
function Compare-Versions {
    param(
        [string]$Current,
        [string]$Latest
    )
    
    if ($Current -eq $Latest) {
        return 0  # Same version
    }
    elseif ($Current -eq "unknown" -or $Current -eq "not installed") {
        return 1  # Need to install/update
    }
    else {
        # Simple string comparison (works for most semantic versioning)
        try {
            $currentVersion = [Version]($Current -replace '^v', '')
            $latestVersion = [Version]($Latest -replace '^v', '')
            
            if ($currentVersion -lt $latestVersion) {
                return 1  # Update available
            }
            elseif ($currentVersion -gt $latestVersion) {
                return 2  # Current is newer
            }
            else {
                return 0  # Same version
            }
        }
        catch {
            # Fallback to string comparison
            if ($Current -ne $Latest) {
                return 1  # Assume update needed
            }
            else {
                return 0
            }
        }
    }
}

# Check if Containerization Assist is running
function Test-ContainerizationAssistRunning {
    $processes = Get-Process -Name $BinaryName -ErrorAction SilentlyContinue
    
    if ($processes) {
        Write-Warning-Custom "Containerization Assist appears to be running (PIDs: $($processes.Id -join ', '))"
        Write-Info "Please close Claude Desktop and any running Containerization Assist processes"
        
        if (-not $Force) {
            $continue = Read-Host "Do you want to continue with the update anyway? (y/N)"
            if ($continue -notmatch '^[Yy]') {
                Write-Info "Update cancelled"
                exit 0
            }
        }
    }
}

# Backup current installation
function Backup-Current {
    if (Test-Command $BinaryName) {
        try {
            $binaryPath = (Get-Command $BinaryName).Source
            $backupPath = "$binaryPath.backup.$(Get-Date -Format 'yyyyMMdd-HHmmss')"
            
            Write-Step "Backing up current installation..."
            Copy-Item -Path $binaryPath -Destination $backupPath -ErrorAction Stop
            
            Write-Success "Backup created: $backupPath"
            return $backupPath
        }
        catch {
            Write-Warning-Custom "Could not backup current installation: $($_.Exception.Message)"
            return $null
        }
    }
    else {
        Write-Info "No existing installation found to backup"
        return $null
    }
}

# Update Containerization Assist
function Update-ContainerizationAssist {
    Write-Step "Updating Containerization Assist..."
    
    try {
        # Download and run the setup script
        $setupScriptUrl = "https://raw.githubusercontent.com/$RepoOwner/$RepoName/main/scripts/setup-user.ps1"
        $tempScript = New-TemporaryFile
        $tempScript = "$($tempScript.FullName).ps1"
        
        Write-Info "Downloading setup script..."
        Invoke-WebRequest -Uri $setupScriptUrl -OutFile $tempScript -ErrorAction Stop
        
        Write-Info "Running setup script..."
        & $tempScript -Force
        
        # Clean up
        Remove-Item $tempScript -Force -ErrorAction SilentlyContinue
        
        return $true
    }
    catch {
        Write-Error-Custom "Update failed: $($_.Exception.Message)"
        return $false
    }
}

# Verify update
function Test-Update {
    Write-Step "Verifying update..."
    
    if (Test-Command $BinaryName) {
        try {
            $newVersion = & $BinaryName --version 2>$null | Select-Object -First 1
            Write-Success "Update completed successfully"
            Write-Info "New version: $newVersion"
            return $true
        }
        catch {
            Write-Success "Update completed (version check failed)"
            return $true
        }
    }
    else {
        Write-Error-Custom "Update verification failed - binary not accessible"
        return $false
    }
}

# Restore from backup
function Restore-Backup {
    param([string]$BackupPath)
    
    if ($BackupPath -and (Test-Path $BackupPath)) {
        Write-Step "Restoring from backup..."
        
        try {
            $originalPath = $BackupPath -replace '\.backup\.\d{8}-\d{6}$', ''
            Copy-Item -Path $BackupPath -Destination $originalPath -Force -ErrorAction Stop
            Write-Success "Restored from backup"
        }
        catch {
            Write-Error-Custom "Failed to restore from backup: $($_.Exception.Message)"
            Write-Info "Manual restore may be needed: $BackupPath"
        }
    }
}

# Show update summary
function Show-Summary {
    param(
        [string]$OldVersion,
        [string]$NewVersion
    )
    
    Write-Host ""
    Write-Success "🎉 Containerization Assist Update Complete!"
    Write-Host ""
    Write-Info "Version Update:"
    Write-Info "  • From: $OldVersion"
    Write-Info "  • To:   $NewVersion"
    Write-Host ""
    Write-Info "Next Steps:"
    Write-Info "1. 🔄 Restart Claude Desktop (if it was running)"
    Write-Info "2. 🧪 Test the connection by asking Claude about Containerization Assist tools"
    Write-Info "3. 📖 Check the changelog for new features: https://github.com/$RepoOwner/$RepoName/releases/latest"
    Write-Host ""
    Write-Info "If you encounter any issues:"
    Write-Info "• Check the troubleshooting guide in USER_GUIDE.md"
    Write-Info "• Report bugs at: https://github.com/$RepoOwner/$RepoName/issues"
    Write-Host ""
}

# Main function
function Main {
    if ($Help) {
        Show-Help
        return
    }
    
    Write-Host ""
    Write-Info "=== Containerization Assist Update Script for Windows ==="
    Write-Info "This script will update Containerization Assist to the latest version"
    Write-Host ""
    
    # Get current version
    Write-Step "Checking current version..."
    $currentVersion = Get-CurrentVersion
    Write-Info "Current version: $currentVersion"
    
    # Get latest version
    Write-Step "Checking for updates..."
    $latestVersion = Get-LatestVersion
    
    if (-not $latestVersion) {
        Write-Error-Custom "Could not determine latest version"
        Write-Info "Please check your internet connection and try again"
        exit 1
    }
    
    Write-Info "Latest version: $latestVersion"
    
    # Compare versions
    $comparison = Compare-Versions -Current $currentVersion -Latest $latestVersion
    
    if ($comparison -eq 0) {
        Write-Success "You already have the latest version ($currentVersion)"
        Write-Info "No update needed."
        return
    }
    elseif ($comparison -eq 2) {
        Write-Info "Your version ($currentVersion) is newer than the latest release ($latestVersion)"
        Write-Info "You might be using a development version."
        return
    }
    
    Write-Info "Update available: $currentVersion → $latestVersion"
    Write-Host ""
    
    # Ask for confirmation
    if (-not $Force) {
        $continue = Read-Host "Do you want to update now? (Y/n)"
        if ($continue -match '^[Nn]') {
            Write-Info "Update cancelled"
            return
        }
    }
    
    # Check if running
    Test-ContainerizationAssistRunning
    
    # Backup current installation
    $backupPath = Backup-Current
    
    # Perform update
    try {
        if (Update-ContainerizationAssist) {
            if (Test-Update) {
                $newVersion = Get-CurrentVersion
                Show-Summary -OldVersion $currentVersion -NewVersion $newVersion
                
                # Clean up backup if update was successful
                if ($backupPath -and (Test-Path $backupPath)) {
                    Write-Info "Cleaning up backup file..."
                    Remove-Item -Path $backupPath -Force -ErrorAction SilentlyContinue
                }
            }
            else {
                Write-Error-Custom "Update verification failed"
                Restore-Backup -BackupPath $backupPath
                exit 1
            }
        }
        else {
            Write-Error-Custom "Update failed"
            Restore-Backup -BackupPath $backupPath
            exit 1
        }
    }
    catch {
        Write-Error-Custom "Update process failed: $($_.Exception.Message)"
        Restore-Backup -BackupPath $backupPath
        exit 1
    }
}

# Handle Ctrl+C
trap {
    Write-Host ""
    Write-Warning-Custom "Update interrupted by user"
    exit 1
}

# Run main function
Main