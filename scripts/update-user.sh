#!/bin/bash
# Containerization Assist User Update Script
# This script updates Containerization Assist MCP Server to the latest version

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="Azure"
REPO_NAME="containerization-assist"
BINARY_NAME="containerization-assist-mcp"

# Print colored messages
print_error() {
    echo -e "${RED}❌ Error: $1${NC}" >&2
}

print_success() {
    echo -e "${GREEN}✅ $1${NC}"
}

print_info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

print_warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

print_step() {
    echo -e "${YELLOW}🔧 $1${NC}"
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Get current version
get_current_version() {
    if command_exists "$BINARY_NAME"; then
        local version
        version=$("$BINARY_NAME" --version 2>/dev/null | head -n1 || echo "unknown")
        echo "$version"
    else
        echo "not installed"
    fi
}

# Get latest version from GitHub
get_latest_version() {
    local api_url="https://api.github.com/repos/$REPO_OWNER/$REPO_NAME/releases/latest"
    
    if command_exists curl; then
        curl -s "$api_url" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/'
    elif command_exists wget; then
        wget -qO- "$api_url" | grep '"tag_name"' | sed -E 's/.*"tag_name": "([^"]+)".*/\1/'
    else
        print_error "Neither curl nor wget found. Cannot check for updates."
        exit 1
    fi
}

# Compare versions
version_compare() {
    local current="$1"
    local latest="$2"
    
    # Simple version comparison (works for semantic versioning)
    if [ "$current" = "$latest" ]; then
        return 0  # Same version
    elif [ "$current" = "unknown" ] || [ "$current" = "not installed" ]; then
        return 1  # Need to install/update
    else
        # Use sort -V for version comparison if available
        if command_exists sort; then
            local sorted
            sorted=$(printf '%s\n%s\n' "$current" "$latest" | sort -V | head -n1)
            if [ "$sorted" = "$latest" ]; then
                return 2  # Current is newer (shouldn't happen)
            else
                return 1  # Latest is newer
            fi
        else
            # Fallback: assume update is needed
            return 1
        fi
    fi
}

# Check if Containerization Assist is running
check_if_running() {
    local pids
    pids=$(pgrep -f "$BINARY_NAME" 2>/dev/null || true)
    
    if [ -n "$pids" ]; then
        print_warning "Containerization Assist appears to be running (PIDs: $pids)"
        print_info "Please close Claude Desktop and any running Containerization Assist processes"
        
        read -p "Do you want to continue with the update anyway? (y/N) " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Update cancelled"
            exit 0
        fi
    fi
}

# Backup current installation
backup_current() {
    local binary_path
    
    if command_exists "$BINARY_NAME"; then
        binary_path=$(which "$BINARY_NAME")
        local backup_path="${binary_path}.backup.$(date +%Y%m%d-%H%M%S)"
        
        print_step "Backing up current installation..."
        cp "$binary_path" "$backup_path" 2>/dev/null || {
            print_warning "Could not backup current installation (insufficient permissions)"
            return 1
        }
        
        print_success "Backup created: $backup_path"
        echo "$backup_path"  # Return backup path
    else
        print_info "No existing installation found to backup"
        echo ""
    fi
}

# Update Containerization Assist
update_containerization_assist() {
    print_step "Updating Containerization Assist..."
    
    # Use the setup script to install the latest version
    if command_exists curl; then
        curl -sSL https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/scripts/setup-user.sh | bash -s -- --force
    elif command_exists wget; then
        wget -qO- https://raw.githubusercontent.com/$REPO_OWNER/$REPO_NAME/main/scripts/setup-user.sh | bash -s -- --force
    else
        print_error "Neither curl nor wget found. Cannot download update."
        exit 1
    fi
}

# Verify update
verify_update() {
    print_step "Verifying update..."
    
    if command_exists "$BINARY_NAME"; then
        local new_version
        new_version=$("$BINARY_NAME" --version 2>/dev/null | head -n1 || echo "unknown")
        print_success "Update completed successfully"
        print_info "New version: $new_version"
        return 0
    else
        print_error "Update verification failed - binary not accessible"
        return 1
    fi
}

# Restore from backup
restore_backup() {
    local backup_path="$1"
    
    if [ -n "$backup_path" ] && [ -f "$backup_path" ]; then
        print_step "Restoring from backup..."
        local original_path
        original_path=$(echo "$backup_path" | sed 's/\.backup\.[0-9-]*$//')
        
        if cp "$backup_path" "$original_path" 2>/dev/null; then
            print_success "Restored from backup"
        else
            print_error "Failed to restore from backup"
            print_info "Manual restore may be needed: $backup_path -> $original_path"
        fi
    fi
}

# Show update summary
show_summary() {
    local old_version="$1"
    local new_version="$2"
    
    echo
    print_success "🎉 Containerization Assist Update Complete!"
    echo
    print_info "Version Update:"
    print_info "  • From: $old_version"
    print_info "  • To:   $new_version"
    echo
    print_info "Next Steps:"
    print_info "1. 🔄 Restart Claude Desktop (if it was running)"
    print_info "2. 🧪 Test the connection by asking Claude about Containerization Assist tools"
    print_info "3. 📖 Check the changelog for new features: https://github.com/$REPO_OWNER/$REPO_NAME/releases/latest"
    echo
    print_info "If you encounter any issues:"
    print_info "• Check the troubleshooting guide in USER_GUIDE.md"
    print_info "• Report bugs at: https://github.com/$REPO_OWNER/$REPO_NAME/issues"
    echo
}

# Main update flow
main() {
    echo
    print_info "=== Containerization Assist Update Script ==="
    print_info "This script will update Containerization Assist to the latest version"
    echo
    
    # Get current and latest versions
    print_step "Checking current version..."
    local current_version
    current_version=$(get_current_version)
    print_info "Current version: $current_version"
    
    print_step "Checking for updates..."
    local latest_version
    latest_version=$(get_latest_version)
    
    if [ -z "$latest_version" ] || [ "$latest_version" = "null" ]; then
        print_error "Could not determine latest version"
        print_info "Please check your internet connection and try again"
        exit 1
    fi
    
    print_info "Latest version: $latest_version"
    
    # Compare versions
    if version_compare "$current_version" "$latest_version"; then
        print_success "You already have the latest version ($current_version)"
        print_info "No update needed."
        exit 0
    fi
    
    print_info "Update available: $current_version → $latest_version"
    echo
    
    # Ask for confirmation
    read -p "Do you want to update now? (Y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Nn]$ ]]; then
        print_info "Update cancelled"
        exit 0
    fi
    
    # Check if running
    check_if_running
    
    # Backup current installation
    local backup_path
    backup_path=$(backup_current)
    
    # Perform update
    if update_containerization_assist; then
        if verify_update; then
            local new_version
            new_version=$(get_current_version)
            show_summary "$current_version" "$new_version"
            
            # Clean up backup if update was successful
            if [ -n "$backup_path" ] && [ -f "$backup_path" ]; then
                print_info "Cleaning up backup file..."
                rm -f "$backup_path" 2>/dev/null || print_warning "Could not remove backup file: $backup_path"
            fi
        else
            print_error "Update verification failed"
            restore_backup "$backup_path"
            exit 1
        fi
    else
        print_error "Update failed"
        restore_backup "$backup_path"
        exit 1
    fi
}

# Handle interruption
trap 'echo; print_warning "Update interrupted by user"; exit 1' INT

# Run main function
main "$@"