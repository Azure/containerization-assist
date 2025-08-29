#!/bin/bash

# Complete publishing workflow for Containerization Assist MCP
# This script handles building, packaging, and publishing all packages

set -e

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
NPM_DIR="$SCRIPT_DIR/.."
PACKAGES_DIR="$NPM_DIR/platform-packages"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_step() {
  echo -e "\n${BLUE}==>${NC} ${1}"
}

print_success() {
  echo -e "${GREEN}✓${NC} ${1}"
}

print_warning() {
  echo -e "${YELLOW}⚠${NC} ${1}"
}

print_error() {
  echo -e "${RED}✗${NC} ${1}"
}

# Header
echo -e "${BLUE}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${BLUE}║     Containerization Assist MCP - Publishing Workflow      ║${NC}"
echo -e "${BLUE}╚════════════════════════════════════════════════════════════╝${NC}"

# Check if we're logged in to npm
print_step "Checking npm authentication"
if ! npm whoami &> /dev/null; then
  print_error "Not logged in to npm. Please run: npm login"
  exit 1
fi
NPM_USER=$(npm whoami)
print_success "Logged in as: $NPM_USER"

# Get version
cd "$NPM_DIR"
VERSION=$(node -p "require('./package.json').version")
print_step "Publishing version: $VERSION"

# Check for uncommitted changes
print_step "Checking for uncommitted changes"
if [ -n "$(git status --porcelain)" ]; then
  print_warning "You have uncommitted changes. Consider committing before publishing."
  echo "Continue anyway? (y/N)"
  read -r response
  if [[ ! "$response" =~ ^[Yy]$ ]]; then
    echo "Publishing cancelled"
    exit 1
  fi
fi

# Step 1: Clean previous builds
print_step "Cleaning previous builds"
rm -rf "$PACKAGES_DIR"
print_success "Cleaned platform-packages directory"

# Step 2: Build optimized binaries
print_step "Building optimized binaries for all platforms"
bash "$SCRIPT_DIR/build.sh"
print_success "Binaries built successfully"

# Step 3: Create platform packages
print_step "Creating platform-specific packages"
bash "$SCRIPT_DIR/create-platform-packages.sh"
print_success "Platform packages created"

# Step 4: Sync versions
print_step "Syncing versions across packages"
node "$SCRIPT_DIR/sync-versions.js"
print_success "Versions synchronized"

# Step 5: Display package sizes
print_step "Package sizes"
echo -e "\n  Main package: $(du -sh "$NPM_DIR" --exclude=bin --exclude=platform-packages --exclude=.git | cut -f1)"
for dir in "$PACKAGES_DIR"/*; do
  if [ -d "$dir" ]; then
    size=$(du -sh "$dir" | cut -f1)
    name=$(basename "$dir")
    echo "  Platform $name: $size"
  fi
done

# Step 6: Confirmation
echo -e "\n${YELLOW}Ready to publish:${NC}"
echo "  - 6 platform packages"
echo "  - 1 main package"
echo "  - Version: $VERSION"
echo -e "\n${YELLOW}This will publish to npm registry. Continue? (y/N)${NC}"
read -r response
if [[ ! "$response" =~ ^[Yy]$ ]]; then
  echo "Publishing cancelled"
  exit 0
fi

# Step 7: Publish platform packages
print_step "Publishing platform packages"
PUBLISHED_COUNT=0
FAILED_COUNT=0

for dir in "$PACKAGES_DIR"/*; do
  if [ -d "$dir" ]; then
    package_name=$(node -p "require('$dir/package.json').name")
    echo -n "  Publishing $package_name... "
    
    cd "$dir"
    if npm publish --access public 2>/dev/null; then
      echo -e "${GREEN}✓${NC}"
      ((PUBLISHED_COUNT++))
    else
      # Check if already published
      if npm view "$package_name@$VERSION" version &>/dev/null; then
        echo -e "${YELLOW}already published${NC}"
        ((PUBLISHED_COUNT++))
      else
        echo -e "${RED}failed${NC}"
        ((FAILED_COUNT++))
      fi
    fi
  fi
done

if [ $FAILED_COUNT -gt 0 ]; then
  print_error "$FAILED_COUNT platform packages failed to publish"
  exit 1
fi

print_success "All $PUBLISHED_COUNT platform packages published"

# Step 8: Publish main package
print_step "Publishing main package"
cd "$NPM_DIR"

if npm publish --access public; then
  print_success "Main package published successfully"
else
  if npm view "@thgamble/containerization-assist-mcp@$VERSION" version &>/dev/null; then
    print_warning "Main package already published at version $VERSION"
  else
    print_error "Failed to publish main package"
    exit 1
  fi
fi

# Step 9: Verification
print_step "Verifying publication"
sleep 2  # Give npm registry a moment to update

# Check main package
if npm view "@thgamble/containerization-assist-mcp@$VERSION" version &>/dev/null; then
  print_success "Main package verified: @thgamble/containerization-assist-mcp@$VERSION"
else
  print_warning "Main package not yet visible in registry (may take a few minutes)"
fi

# Summary
echo -e "\n${GREEN}╔════════════════════════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║                  Publishing Complete! 🎉                   ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════════════════════════╝${NC}"
echo ""
echo "Published packages:"
echo "  Main: @thgamble/containerization-assist-mcp@$VERSION"
echo ""
echo "Users can now install with:"
echo -e "  ${YELLOW}npm install @thgamble/containerization-assist-mcp@$VERSION${NC}"
echo ""
echo ""
echo "View online:"
echo "  https://www.npmjs.com/package/@thgamble/containerization-assist-mcp"
echo "  https://packagephobia.com/result?p=@thgamble/containerization-assist-mcp"