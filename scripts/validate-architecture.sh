#!/bin/bash
# Architecture Validation Script
# Enforces three-layer architecture (domain/application/infra) boundaries

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Allow custom directory via argument or default to repo
if [[ $# -gt 0 ]]; then
    PKG_MCP_DIR="$1/pkg/mcp"
else
    PKG_MCP_DIR="$REPO_ROOT/pkg/mcp"
fi

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters for violations
VIOLATIONS=0
WARNINGS=0

echo -e "${BLUE}🏗️  Validating Three-Layer Architecture${NC}"
echo "================================================"

# Function to report violation
violation() {
    echo -e "${RED}❌ VIOLATION:${NC} $1"
    ((VIOLATIONS++))
}

# Function to report warning
warning() {
    echo -e "${YELLOW}⚠️  WARNING:${NC} $1"
    ((WARNINGS++))
}

# Function to report success
success() {
    echo -e "${GREEN}✅${NC} $1"
}

# Check if pkg/mcp exists
if [[ ! -d "$PKG_MCP_DIR" ]]; then
    echo -e "${YELLOW}⚠️  pkg/mcp directory not found - skipping architecture validation${NC}"
    exit 0
fi

echo "Validating architecture in: $PKG_MCP_DIR"
echo ""

# 1. LAYER STRUCTURE VALIDATION
echo -e "${BLUE}1. Layer Structure Validation${NC}"
echo "-----------------------------"

# Check for required three-layer structure
if [[ -d "$PKG_MCP_DIR/domain" ]]; then
    success "Domain layer exists"
else
    violation "Domain layer missing (pkg/mcp/domain/)"
fi

if [[ -d "$PKG_MCP_DIR/application" ]]; then
    success "Application layer exists"
else
    violation "Application layer missing (pkg/mcp/application/)"
fi

if [[ -d "$PKG_MCP_DIR/infra" ]]; then
    success "Infrastructure layer exists"
else
    violation "Infrastructure layer missing (pkg/mcp/infra/)"
fi

# Check for legacy package structure (should be cleaned up)
LEGACY_PACKAGES=("tools" "core" "internal" "services")
for pkg in "${LEGACY_PACKAGES[@]}"; do
    if [[ -d "$PKG_MCP_DIR/$pkg" ]]; then
        violation "Legacy package still exists: pkg/mcp/$pkg/ (should be migrated)"
    fi
done

# Check for excessive nesting (max 3 levels deep)
echo ""
echo -e "${BLUE}2. Package Depth Validation${NC}"
echo "----------------------------"

MAX_DEPTH=3
DEEP_DIRS=$(find "$PKG_MCP_DIR" -type d | awk -F'/' '{print NF-1}' | sort -nr | head -1)
BASE_DEPTH=$(echo "$PKG_MCP_DIR" | awk -F'/' '{print NF-1}')
ACTUAL_DEPTH=$((DEEP_DIRS - BASE_DEPTH))

if [[ $ACTUAL_DEPTH -le $MAX_DEPTH ]]; then
    success "Package depth OK (max depth: $ACTUAL_DEPTH levels)"
else
    violation "Package depth too deep (max: $MAX_DEPTH, actual: $ACTUAL_DEPTH levels)"
    echo "  Deep directories:"
    find "$PKG_MCP_DIR" -type d -mindepth $((MAX_DEPTH + 1)) | head -5
fi

# 3. DEPENDENCY DIRECTION VALIDATION
echo ""
echo -e "${BLUE}3. Dependency Direction Validation${NC}"
echo "-----------------------------------"

# Domain layer should not import application or infra
if [[ -d "$PKG_MCP_DIR/domain" ]]; then
    DOMAIN_VIOLATIONS=$(grep -r "pkg/mcp/application\|pkg/mcp/infra" "$PKG_MCP_DIR/domain/" 2>/dev/null | wc -l || echo "0")
    if [[ $DOMAIN_VIOLATIONS -eq 0 ]]; then
        success "Domain layer has no upward dependencies"
    else
        violation "Domain layer imports application/infra ($DOMAIN_VIOLATIONS violations)"
        echo "  Examples:"
        grep -r "pkg/mcp/application\|pkg/mcp/infra" "$PKG_MCP_DIR/domain/" 2>/dev/null | head -3 | sed 's/^/    /'
    fi
fi

# Application layer should not import infra
if [[ -d "$PKG_MCP_DIR/application" ]]; then
    APP_VIOLATIONS=$(grep -r "pkg/mcp/infra" "$PKG_MCP_DIR/application/" 2>/dev/null | wc -l || echo "0")
    if [[ $APP_VIOLATIONS -eq 0 ]]; then
        success "Application layer doesn't import infrastructure"
    else
        violation "Application layer imports infrastructure ($APP_VIOLATIONS violations)"
        echo "  Examples:"
        grep -r "pkg/mcp/infra" "$PKG_MCP_DIR/application/" 2>/dev/null | head -3 | sed 's/^/    /'
    fi
fi

# 4. EXTERNAL DEPENDENCY ISOLATION
echo ""
echo -e "${BLUE}4. External Dependency Isolation${NC}"
echo "----------------------------------"

# Check that external dependencies are only in infra layer
EXTERNAL_DEPS=("docker\.Client" "kubernetes\.Interface" "database/sql" "github\.com/aws/aws-sdk")

for dep in "${EXTERNAL_DEPS[@]}"; do
    # Check domain layer
    if [[ -d "$PKG_MCP_DIR/domain" ]]; then
        DOMAIN_EXT=$(grep -r "$dep" "$PKG_MCP_DIR/domain/" 2>/dev/null | wc -l || echo "0")
        if [[ $DOMAIN_EXT -gt 0 ]]; then
            violation "Domain layer has external dependency: $dep"
        fi
    fi

    # Check application layer
    if [[ -d "$PKG_MCP_DIR/application" ]]; then
        APP_EXT=$(grep -r "$dep" "$PKG_MCP_DIR/application/" 2>/dev/null | wc -l || echo "0")
        if [[ $APP_EXT -gt 0 ]]; then
            violation "Application layer has external dependency: $dep"
        fi
    fi
done

success "External dependencies properly isolated"

# 5. ANTI-PATTERN DETECTION
echo ""
echo -e "${BLUE}5. Anti-Pattern Detection${NC}"
echo "--------------------------"

# Check for manager pattern files
MANAGER_FILES=$(find "$PKG_MCP_DIR" -name "*manager*.go" 2>/dev/null | wc -l || echo "0")
if [[ $MANAGER_FILES -eq 0 ]]; then
    success "No manager pattern files found"
else
    warning "$MANAGER_FILES manager pattern files found (consider refactoring to services)"
    find "$PKG_MCP_DIR" -name "*manager*.go" 2>/dev/null | head -3 | sed 's/^/    /'
fi

# Check for adapter/wrapper pattern files
ADAPTER_FILES=$(find "$PKG_MCP_DIR" -name "*adapter*.go" -o -name "*wrapper*.go" 2>/dev/null | wc -l || echo "0")
if [[ $ADAPTER_FILES -eq 0 ]]; then
    success "No adapter/wrapper pattern files found"
else
    violation "$ADAPTER_FILES adapter/wrapper pattern files found"
    find "$PKG_MCP_DIR" -name "*adapter*.go" -o -name "*wrapper*.go" 2>/dev/null | head -3 | sed 's/^/    /'
fi

# 6. INTERFACE ORGANIZATION VALIDATION
echo ""
echo -e "${BLUE}6. Interface Organization Validation${NC}"
echo "-------------------------------------"

# Check that interfaces are in ports package (not scattered)
if [[ -f "$PKG_MCP_DIR/application/ports/interfaces.go" ]]; then
    success "Canonical interfaces in application/ports/"
else
    if [[ -d "$PKG_MCP_DIR/application" ]]; then
        warning "Interfaces not found in application/ports/ (may not be migrated yet)"
    fi
fi

# Check for scattered interface definitions
INTERFACE_FILES=$(find "$PKG_MCP_DIR" -name "*interface*.go" ! -path "*/ports/*" 2>/dev/null | wc -l || echo "0")
if [[ $INTERFACE_FILES -gt 0 ]]; then
    warning "$INTERFACE_FILES interface files outside ports package"
    find "$PKG_MCP_DIR" -name "*interface*.go" ! -path "*/ports/*" 2>/dev/null | head -3 | sed 's/^/    /'
fi

# 7. BUILD TAG VALIDATION
echo ""
echo -e "${BLUE}7. Build Tag Validation${NC}"
echo "------------------------"

# Check for proper build tag usage in infra layer
if [[ -d "$PKG_MCP_DIR/infra" ]]; then
    BUILD_TAG_FILES=$(find "$PKG_MCP_DIR/infra" -name "*.go" -exec grep -l "//go:build.*\(docker\|k8s\|cloud\)" {} \; 2>/dev/null | wc -l || echo "0")
    INFRA_GO_FILES=$(find "$PKG_MCP_DIR/infra" -name "*.go" ! -name "*_test.go" 2>/dev/null | wc -l || echo "0")

    if [[ $BUILD_TAG_FILES -gt 0 ]]; then
        success "Infrastructure uses build tags for optional dependencies"
    elif [[ $INFRA_GO_FILES -gt 0 ]]; then
        # Only warn for files that might need build tags (those with external deps)
        EXTERNAL_DEP_FILES=$(grep -r "docker\|kubernetes\|aws\|azure\|gcp" "$PKG_MCP_DIR/infra" 2>/dev/null | cut -d: -f1 | sort -u | wc -l || echo "0")
        if [[ $EXTERNAL_DEP_FILES -gt 0 ]]; then
            warning "Infrastructure files with external dependencies should use build tags"
        else
            success "Infrastructure build tag usage appropriate"
        fi
    else
        success "No infrastructure files to validate"
    fi
fi

# 8. IMPORT CYCLE DETECTION
echo ""
echo -e "${BLUE}8. Import Cycle Detection${NC}"
echo "--------------------------"

if command -v go >/dev/null 2>&1; then
    CYCLES=$(go list -deps "$PKG_MCP_DIR/..." 2>&1 | grep -i cycle | wc -l || echo "0")
    if [[ $CYCLES -eq 0 ]]; then
        success "No import cycles detected"
    else
        violation "$CYCLES import cycles detected"
        go list -deps "$PKG_MCP_DIR/..." 2>&1 | grep -i cycle | head -3 | sed 's/^/    /'
    fi
else
    warning "Go not available - skipping import cycle detection"
fi

# SUMMARY
echo ""
echo "================================================"
echo -e "${BLUE}📊 Architecture Validation Summary${NC}"
echo "================================================"

if [[ $VIOLATIONS -eq 0 ]]; then
    echo -e "${GREEN}✅ SUCCESS:${NC} No architecture violations found!"
    if [[ $WARNINGS -gt 0 ]]; then
        echo -e "${YELLOW}⚠️  Warnings:${NC} $WARNINGS (consider addressing)"
    fi
    echo ""
    echo -e "${GREEN}🏗️  Three-layer architecture properly maintained!${NC}"
    exit 0
else
    echo -e "${RED}❌ FAILURE:${NC} $VIOLATIONS architecture violations found"
    if [[ $WARNINGS -gt 0 ]]; then
        echo -e "${YELLOW}⚠️  Warnings:${NC} $WARNINGS"
    fi
    echo ""
    echo -e "${RED}🚨 Please fix violations before committing${NC}"
    echo ""
    echo "Architecture Guidelines:"
    echo "  • Domain layer: Pure business logic, no external dependencies"
    echo "  • Application layer: Use cases, orchestration, calls domain only"
    echo "  • Infrastructure layer: External integrations, implements interfaces"
    echo "  • Dependency direction: infra → application → domain"
    echo ""
    exit 1
fi
