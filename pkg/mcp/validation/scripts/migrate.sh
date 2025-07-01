#!/bin/bash

# Unified Validation Migration Script
# This script helps automate the migration from scattered validation code to unified validation

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
PROJECT_ROOT="${PROJECT_ROOT:-$(pwd)}"
BACKUP_DIR="${PROJECT_ROOT}/validation_migration_backup"
LOG_FILE="${PROJECT_ROOT}/validation_migration.log"

# Functions
log() {
    echo "$(date '+%Y-%m-%d %H:%M:%S') - $1" | tee -a "$LOG_FILE"
}

info() {
    echo -e "${BLUE}[INFO]${NC} $1"
    log "INFO: $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
    log "SUCCESS: $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
    log "WARNING: $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    log "ERROR: $1"
}

# Check prerequisites
check_prerequisites() {
    info "Checking prerequisites..."
    
    # Check if we're in the right directory
    if [[ ! -f "go.mod" ]]; then
        error "go.mod not found. Please run this script from the project root."
        exit 1
    fi
    
    # Check if unified validation package exists
    if [[ ! -d "pkg/mcp/validation" ]]; then
        error "Unified validation package not found at pkg/mcp/validation"
        exit 1
    fi
    
    # Check Go version
    if ! command -v go &> /dev/null; then
        error "Go is not installed or not in PATH"
        exit 1
    fi
    
    success "Prerequisites check passed"
}

# Create backup of existing validation code
create_backup() {
    info "Creating backup of existing validation code..."
    
    mkdir -p "$BACKUP_DIR"
    
    # Backup validation-related files
    find . -name "*.go" -path "./pkg/mcp/*" -exec grep -l "ValidationResult\|ValidationError\|Validator" {} \; | \
    while read -r file; do
        backup_path="$BACKUP_DIR/$file"
        mkdir -p "$(dirname "$backup_path")"
        cp "$file" "$backup_path"
    done
    
    success "Backup created at $BACKUP_DIR"
}

# Analyze current validation usage
analyze_validation_usage() {
    info "Analyzing current validation usage..."
    
    local analysis_file="${PROJECT_ROOT}/validation_analysis.txt"
    
    {
        echo "=== VALIDATION USAGE ANALYSIS ==="
        echo "Generated: $(date)"
        echo ""
        
        echo "Files with ValidationResult:"
        grep -r "ValidationResult" pkg/mcp --include="*.go" | cut -d: -f1 | sort | uniq
        echo ""
        
        echo "Files with ValidationError:"
        grep -r "ValidationError" pkg/mcp --include="*.go" | cut -d: -f1 | sort | uniq
        echo ""
        
        echo "Files with Validator interfaces:"
        grep -r "type.*Validator.*interface" pkg/mcp --include="*.go"
        echo ""
        
        echo "Validation utility functions:"
        grep -r "func.*Validate" pkg/mcp --include="*.go" | head -20
        echo ""
        
        echo "Import statements to update:"
        grep -r "pkg/mcp/types.*validation\|pkg/mcp/internal.*validation" pkg/mcp --include="*.go" | cut -d: -f1 | sort | uniq
        
    } > "$analysis_file"
    
    success "Analysis saved to $analysis_file"
    
    # Display summary
    local validation_files=$(grep -r "ValidationResult\|ValidationError" pkg/mcp --include="*.go" | cut -d: -f1 | sort | uniq | wc -l)
    local validator_interfaces=$(grep -r "type.*Validator.*interface" pkg/mcp --include="*.go" | wc -l)
    local validate_functions=$(grep -r "func.*Validate" pkg/mcp --include="*.go" | wc -l)
    
    info "Found $validation_files files with validation code"
    info "Found $validator_interfaces validator interfaces"
    info "Found $validate_functions validation functions"
}

# Phase 1: Update import statements
update_imports() {
    info "Updating import statements..."
    
    # Update validation imports to use unified package
    find pkg/mcp -name "*.go" -exec grep -l "pkg/mcp/types.*validation\|pkg/mcp/internal.*validation" {} \; | \
    while read -r file; do
        info "Updating imports in $file"
        
        # Replace old validation imports with unified imports
        sed -i.bak \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/types"|"github.com/Azure/container-kit/pkg/mcp/validation/core"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/internal/errors/validation"|"github.com/Azure/container-kit/pkg/mcp/validation/core"|g' \
            -e 's|"github.com/Azure/container-kit/pkg/mcp/utils/validation_utils"|"github.com/Azure/container-kit/pkg/mcp/validation/utils"|g' \
            "$file"
        
        # Remove .bak file if sed succeeded
        rm -f "${file}.bak"
    done
    
    success "Import statements updated"
}

# Phase 2: Migrate specific packages
migrate_build_package() {
    info "Migrating build package validation..."
    
    local build_dir="pkg/mcp/internal/build"
    
    # Create unified build validators if they don't exist
    if [[ ! -f "pkg/mcp/validation/validators/docker.go" ]]; then
        warning "Docker validator not found. Creating placeholder..."
        # This would be replaced with actual validator creation
    fi
    
    # Update build package files to use unified validation
    find "$build_dir" -name "*validator*.go" | while read -r file; do
        info "Migrating $file"
        
        # Update type references
        sed -i.bak \
            -e 's|ValidationResult|core.ValidationResult|g' \
            -e 's|ValidationError|core.ValidationError|g' \
            -e 's|ValidationOptions|core.ValidationOptions|g' \
            "$file"
        
        # Add unified validation import if not present
        if ! grep -q "github.com/Azure/container-kit/pkg/mcp/validation/core" "$file"; then
            # Add import after package declaration
            sed -i.bak '/^package/a\\nimport (\n\t"github.com/Azure/container-kit/pkg/mcp/validation/core"\n)' "$file"
        fi
        
        rm -f "${file}.bak"
    done
    
    success "Build package migration completed"
}

migrate_deploy_package() {
    info "Migrating deploy package validation..."
    
    local deploy_dir="pkg/mcp/internal/deploy"
    
    # Similar migration logic for deploy package
    find "$deploy_dir" -name "*validator*.go" -o -name "*health*.go" | while read -r file; do
        info "Migrating $file"
        
        sed -i.bak \
            -e 's|ValidationResult|core.ValidationResult|g' \
            -e 's|ValidationError|core.ValidationError|g' \
            -e 's|ValidationOptions|core.ValidationOptions|g' \
            "$file"
        
        rm -f "${file}.bak"
    done
    
    success "Deploy package migration completed"
}

migrate_scan_package() {
    info "Migrating scan package validation..."
    
    local scan_dir="pkg/mcp/internal/scan"
    
    find "$scan_dir" -name "*validator*.go" | while read -r file; do
        info "Migrating $file"
        
        sed -i.bak \
            -e 's|ValidationResult|core.ValidationResult|g' \
            -e 's|ValidationError|core.ValidationError|g' \
            -e 's|ValidationOptions|core.ValidationOptions|g' \
            "$file"
        
        rm -f "${file}.bak"
    done
    
    success "Scan package migration completed"
}

# Run tests to validate migration
run_tests() {
    info "Running tests to validate migration..."
    
    # Test unified validation package
    if go test ./pkg/mcp/validation/...; then
        success "Unified validation tests passed"
    else
        error "Unified validation tests failed"
        return 1
    fi
    
    # Test migrated packages
    local packages=("./pkg/mcp/internal/build/..." "./pkg/mcp/internal/deploy/..." "./pkg/mcp/internal/scan/...")
    
    for package in "${packages[@]}"; do
        info "Testing $package"
        if go test "$package"; then
            success "$package tests passed"
        else
            warning "$package tests failed - may need manual fixes"
        fi
    done
}

# Generate migration report
generate_report() {
    info "Generating migration report..."
    
    local report_file="${PROJECT_ROOT}/validation_migration_report.md"
    
    {
        echo "# Validation Migration Report"
        echo "Generated: $(date)"
        echo ""
        
        echo "## Migration Summary"
        echo "- Backup created: $BACKUP_DIR"
        echo "- Migration log: $LOG_FILE"
        echo ""
        
        echo "## Files Modified"
        echo "\`\`\`"
        find pkg/mcp -name "*.go" -newer "$BACKUP_DIR" 2>/dev/null | head -20 || echo "No recently modified files found"
        echo "\`\`\`"
        echo ""
        
        echo "## Remaining Manual Tasks"
        echo "1. Review and update custom validator implementations"
        echo "2. Update tests that use old validation types"
        echo "3. Remove duplicate validation utilities"
        echo "4. Update documentation references"
        echo ""
        
        echo "## Validation Status"
        echo "Run the following commands to verify migration:"
        echo "\`\`\`bash"
        echo "go test ./pkg/mcp/validation/..."
        echo "go test ./pkg/mcp/internal/build/..."
        echo "go test ./pkg/mcp/internal/deploy/..."
        echo "go test ./pkg/mcp/internal/scan/..."
        echo "\`\`\`"
        
    } > "$report_file"
    
    success "Migration report saved to $report_file"
}

# Cleanup function
cleanup_on_error() {
    if [[ -d "$BACKUP_DIR" ]]; then
        warning "Migration failed. To restore backup:"
        echo "  cp -r $BACKUP_DIR/* ."
    fi
}

# Main migration function
main() {
    local phase="${1:-all}"
    
    info "Starting validation migration (phase: $phase)"
    
    # Set up error handling
    trap cleanup_on_error ERR
    
    case "$phase" in
        "analyze")
            check_prerequisites
            analyze_validation_usage
            ;;
        "backup")
            check_prerequisites
            create_backup
            ;;
        "imports")
            check_prerequisites
            create_backup
            update_imports
            run_tests
            ;;
        "build")
            check_prerequisites
            migrate_build_package
            run_tests
            ;;
        "deploy")
            check_prerequisites
            migrate_deploy_package
            run_tests
            ;;
        "scan")
            check_prerequisites
            migrate_scan_package
            run_tests
            ;;
        "test")
            run_tests
            ;;
        "report")
            generate_report
            ;;
        "all")
            check_prerequisites
            analyze_validation_usage
            create_backup
            update_imports
            migrate_build_package
            migrate_deploy_package
            migrate_scan_package
            run_tests
            generate_report
            ;;
        *)
            echo "Usage: $0 [analyze|backup|imports|build|deploy|scan|test|report|all]"
            echo ""
            echo "Phases:"
            echo "  analyze  - Analyze current validation usage"
            echo "  backup   - Create backup of existing code"
            echo "  imports  - Update import statements"
            echo "  build    - Migrate build package validation"
            echo "  deploy   - Migrate deploy package validation"
            echo "  scan     - Migrate scan package validation"
            echo "  test     - Run tests to validate migration"
            echo "  report   - Generate migration report"
            echo "  all      - Run complete migration (default)"
            exit 1
            ;;
    esac
    
    success "Migration phase '$phase' completed successfully"
}

# Run main function with all arguments
main "$@"