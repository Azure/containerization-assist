#!/bin/bash

# Install pre-commit hooks for quality checks

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
HOOKS_DIR="$ROOT_DIR/.git/hooks"

echo "🪝 Installing quality pre-commit hooks..."
echo "========================================"

# Create hooks directory if it doesn't exist
mkdir -p "$HOOKS_DIR"

# Create pre-commit hook
cat > "$HOOKS_DIR/pre-commit" << 'EOF'
#!/bin/bash

# MCP Quality Pre-commit Hook
# This hook runs quality checks before allowing commits

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🔍 Running pre-commit quality checks..."
echo "======================================"

# Get the root directory
ROOT_DIR=$(git rev-parse --show-toplevel)

# Function to check if tool exists
check_tool() {
    if ! command -v $1 &> /dev/null; then
        echo -e "${RED}❌ $1 is not installed${NC}"
        return 1
    fi
    return 0
}

# 1. Check for interface validation errors
echo -e "\n${YELLOW}1. Validating interfaces...${NC}"
if go run "$ROOT_DIR/tools/validate-interfaces/main.go" --check-only 2>/dev/null; then
    echo -e "${GREEN}✅ Interface validation passed${NC}"
else
    echo -e "${RED}❌ Interface validation failed${NC}"
    echo "Run: go run tools/validate-interfaces/main.go --verbose"
    exit 1
fi

# 2. Quick quality metrics check
echo -e "\n${YELLOW}2. Checking quality metrics...${NC}"
METRICS_OUTPUT=$(go run "$ROOT_DIR/tools/quality-dashboard/main.go" -format json -output - 2>/dev/null || echo "{}")

if command -v jq &> /dev/null && [ "$METRICS_OUTPUT" != "{}" ]; then
    ERROR_RATE=$(echo "$METRICS_OUTPUT" | jq '.error_handling.adoption_rate // 0')
    COVERAGE=$(echo "$METRICS_OUTPUT" | jq '.test_coverage.overall_coverage // 0')
    
    # Warning thresholds (not blocking)
    if (( $(echo "$ERROR_RATE < 30" | bc -l 2>/dev/null || echo 0) )); then
        echo -e "${YELLOW}⚠️  Warning: Error handling adoption is low (${ERROR_RATE}%)${NC}"
    else
        echo -e "${GREEN}✅ Error handling adoption: ${ERROR_RATE}%${NC}"
    fi
    
    if (( $(echo "$COVERAGE < 40" | bc -l 2>/dev/null || echo 0) )); then
        echo -e "${YELLOW}⚠️  Warning: Test coverage is low (${COVERAGE}%)${NC}"
    else
        echo -e "${GREEN}✅ Test coverage: ${COVERAGE}%${NC}"
    fi
fi

# 3. Check for common issues in staged files
echo -e "\n${YELLOW}3. Checking staged files...${NC}"
STAGED_GO_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep '\.go$' || true)

if [ -n "$STAGED_GO_FILES" ]; then
    # Check for fmt.Errorf in new code (warning only)
    FMT_ERRORS=$(git diff --cached -U0 | grep -E '^\+.*fmt\.Errorf' | wc -l || true)
    if [ "$FMT_ERRORS" -gt 0 ]; then
        echo -e "${YELLOW}⚠️  Found $FMT_ERRORS new fmt.Errorf calls - consider using RichError${NC}"
    fi
    
    # Check for missing error handling
    UNCHECKED_ERRORS=$(git diff --cached -U0 | grep -E '^\+.*err\s*:=.*' | grep -v 'if err' | wc -l || true)
    if [ "$UNCHECKED_ERRORS" -gt 0 ]; then
        echo -e "${YELLOW}⚠️  Found $UNCHECKED_ERRORS potential unchecked errors${NC}"
    fi
    
    # Run gofmt check
    echo -e "\n${YELLOW}4. Running gofmt...${NC}"
    UNFORMATTED=$(echo "$STAGED_GO_FILES" | xargs gofmt -l 2>/dev/null || true)
    if [ -n "$UNFORMATTED" ]; then
        echo -e "${RED}❌ The following files are not gofmt'd:${NC}"
        echo "$UNFORMATTED"
        echo -e "${YELLOW}Run: gofmt -w <file>${NC}"
        exit 1
    else
        echo -e "${GREEN}✅ All files are properly formatted${NC}"
    fi
    
    # Run go vet on staged files
    echo -e "\n${YELLOW}5. Running go vet...${NC}"
    if echo "$STAGED_GO_FILES" | xargs go vet 2>&1; then
        echo -e "${GREEN}✅ go vet passed${NC}"
    else
        echo -e "${RED}❌ go vet found issues${NC}"
        exit 1
    fi
fi

# 4. Check commit message format (if COMMIT_MSG is set)
if [ -n "$COMMIT_MSG" ]; then
    echo -e "\n${YELLOW}6. Checking commit message...${NC}"
    # Simple check for conventional commits
    if echo "$COMMIT_MSG" | grep -qE '^(feat|fix|docs|style|refactor|test|chore|perf|ci|build|revert)(\(.+\))?: .+'; then
        echo -e "${GREEN}✅ Commit message follows conventional format${NC}"
    else
        echo -e "${YELLOW}⚠️  Consider using conventional commit format: type(scope): message${NC}"
    fi
fi

echo -e "\n${GREEN}✅ All pre-commit checks passed!${NC}"
echo "======================================"
EOF

# Create pre-push hook
cat > "$HOOKS_DIR/pre-push" << 'EOF'
#!/bin/bash

# MCP Quality Pre-push Hook
# This hook runs comprehensive quality checks before pushing

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "🚀 Running pre-push quality checks..."
echo "===================================="

ROOT_DIR=$(git rev-parse --show-toplevel)

# 1. Run tests
echo -e "\n${YELLOW}1. Running tests...${NC}"
if go test ./... -short -timeout 30s >/dev/null 2>&1; then
    echo -e "${GREEN}✅ Tests passed${NC}"
else
    echo -e "${RED}❌ Tests failed${NC}"
    echo "Run: go test ./... -v"
    exit 1
fi

# 2. Check build
echo -e "\n${YELLOW}2. Checking build...${NC}"
if go build ./... >/dev/null 2>&1; then
    echo -e "${GREEN}✅ Build successful${NC}"
else
    echo -e "${RED}❌ Build failed${NC}"
    exit 1
fi

# 3. Run comprehensive quality check
echo -e "\n${YELLOW}3. Running quality gates...${NC}"
METRICS=$(go run "$ROOT_DIR/tools/quality-dashboard/main.go" -format json -output - 2>/dev/null || echo "{}")

if [ "$METRICS" != "{}" ] && command -v jq &> /dev/null; then
    ERROR_RATE=$(echo "$METRICS" | jq '.error_handling.adoption_rate // 0')
    INTERFACE_ERRORS=$(go run "$ROOT_DIR/tools/validate-interfaces/main.go" --check-only 2>&1 | grep -c "error" || echo 0)
    
    # Strict checks for push
    if [ "$INTERFACE_ERRORS" -gt 0 ]; then
        echo -e "${RED}❌ Cannot push with interface validation errors${NC}"
        exit 1
    fi
    
    if (( $(echo "$ERROR_RATE < 20" | bc -l 2>/dev/null || echo 0) )); then
        echo -e "${RED}❌ Error handling adoption too low for push (${ERROR_RATE}% < 20%)${NC}"
        echo "Please improve error handling before pushing"
        exit 1
    fi
fi

echo -e "\n${GREEN}✅ All pre-push checks passed!${NC}"
echo "===================================="
EOF

# Make hooks executable
chmod +x "$HOOKS_DIR/pre-commit"
chmod +x "$HOOKS_DIR/pre-push"

# Create commit message template
cat > "$ROOT_DIR/.gitmessage" << 'EOF'
# <type>(<scope>): <subject>
#
# <body>
#
# <footer>

# Type should be one of the following:
# - feat: A new feature
# - fix: A bug fix
# - docs: Documentation only changes
# - style: Changes that do not affect the meaning of the code
# - refactor: A code change that neither fixes a bug nor adds a feature
# - perf: A code change that improves performance
# - test: Adding missing tests or correcting existing tests
# - build: Changes that affect the build system or external dependencies
# - ci: Changes to our CI configuration files and scripts
# - chore: Other changes that don't modify src or test files
# - revert: Reverts a previous commit
EOF

# Configure git to use the commit template
git config --local commit.template .gitmessage

echo ""
echo "✅ Pre-commit hooks installed successfully!"
echo ""
echo "Hooks installed:"
echo "  - pre-commit: Runs quick quality checks before each commit"
echo "  - pre-push: Runs comprehensive checks before pushing"
echo ""
echo "To bypass hooks (emergency only):"
echo "  - git commit --no-verify"
echo "  - git push --no-verify"
echo ""
echo "To uninstall hooks:"
echo "  - rm .git/hooks/pre-commit .git/hooks/pre-push"
EOF