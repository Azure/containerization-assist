#!/bin/bash
# Generate changelog from GitHub pull requests and commits

set -e

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Configuration
REPO_OWNER="Azure"
REPO_NAME="container-kit"

# Print colored messages
print_info() {
    echo -e "${YELLOW}$1${NC}"
}

print_success() {
    echo -e "${GREEN}$1${NC}"
}

print_error() {
    echo -e "${RED}Error: $1${NC}" >&2
}

# Check if gh CLI is installed
check_gh_cli() {
    if ! command -v gh &> /dev/null; then
        print_error "GitHub CLI (gh) is not installed"
        print_info "Install it from: https://cli.github.com/"
        exit 1
    fi

    # Check if authenticated
    if ! gh auth status &> /dev/null; then
        print_error "GitHub CLI is not authenticated"
        print_info "Run: gh auth login"
        exit 1
    fi
}

# Get the previous tag
get_previous_tag() {
    local current_tag=$1
    local previous_tag

    if [ -z "$current_tag" ]; then
        # Get the latest tag
        current_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
    fi

    if [ -z "$current_tag" ]; then
        print_error "No tags found in the repository"
        exit 1
    fi

    # Get the previous tag
    previous_tag=$(git describe --tags --abbrev=0 "${current_tag}^" 2>/dev/null || echo "")

    echo "$previous_tag"
}

# Fetch PRs between two tags
fetch_prs_between_tags() {
    local since_tag=$1
    local until_tag=$2
    local prs=()

    print_info "Fetching PRs between $since_tag and $until_tag..."

    # Get commit SHAs for the tags
    local since_sha=$(git rev-list -n 1 "$since_tag" 2>/dev/null || echo "")
    local until_sha=$(git rev-list -n 1 "$until_tag" 2>/dev/null || echo "HEAD")

    # Get all merged PRs
    local pr_numbers=$(git log --merges --pretty=format:"%s" "$since_sha..$until_sha" | grep -oE '#[0-9]+' | sed 's/#//' | sort -u)

    echo "$pr_numbers"
}

# Categorize PR by title
categorize_pr() {
    local title=$1
    local category="other"

    # Convert to lowercase for matching
    local lower_title=$(echo "$title" | tr '[:upper:]' '[:lower:]')

    # Match conventional commit prefixes
    if [[ "$lower_title" =~ ^feat(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ feature ]]; then
        category="feature"
    elif [[ "$lower_title" =~ ^fix(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ bugfix|bug.fix ]]; then
        category="bugfix"
    elif [[ "$lower_title" =~ ^docs(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ documentation ]]; then
        category="documentation"
    elif [[ "$lower_title" =~ ^chore(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ chore ]]; then
        category="chore"
    elif [[ "$lower_title" =~ ^refactor(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ refactor ]]; then
        category="refactor"
    elif [[ "$lower_title" =~ ^perf(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ performance ]]; then
        category="performance"
    elif [[ "$lower_title" =~ ^test(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ test ]]; then
        category="test"
    elif [[ "$lower_title" =~ ^ci(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ ci/cd ]]; then
        category="ci"
    elif [[ "$lower_title" =~ ^security(\(.+\))?:.*$ ]] || [[ "$lower_title" =~ security ]]; then
        category="security"
    elif [[ "$lower_title" =~ breaking.change|breaking ]]; then
        category="breaking"
    fi

    echo "$category"
}

# Generate changelog
generate_changelog() {
    local since_tag=$1
    local until_tag=$2
    local pr_numbers=$3

    # Initialize arrays for categories
    declare -A categories
    categories["breaking"]=""
    categories["security"]=""
    categories["feature"]=""
    categories["bugfix"]=""
    categories["performance"]=""
    categories["refactor"]=""
    categories["documentation"]=""
    categories["test"]=""
    categories["ci"]=""
    categories["chore"]=""
    categories["other"]=""

    # Process each PR
    while IFS= read -r pr_num; do
        if [ -z "$pr_num" ]; then
            continue
        fi

        print_info "Fetching PR #$pr_num..."

        # Fetch PR details using gh CLI
        local pr_json=$(gh pr view "$pr_num" --repo "$REPO_OWNER/$REPO_NAME" --json title,author,url 2>/dev/null || echo "")

        if [ -z "$pr_json" ]; then
            continue
        fi

        local title=$(echo "$pr_json" | jq -r .title)
        local author=$(echo "$pr_json" | jq -r .author.login)
        local url=$(echo "$pr_json" | jq -r .url)

        # Categorize the PR
        local category=$(categorize_pr "$title")

        # Add to appropriate category
        categories["$category"]+="- $title ([#$pr_num]($url)) by @$author\n"
    done <<< "$pr_numbers"

    # Generate the changelog
    echo "# Changelog"
    echo ""
    echo "## $until_tag"
    echo ""

    if [ -n "$since_tag" ]; then
        echo "### Changes since $since_tag"
    else
        echo "### Changes"
    fi
    echo ""

    # Print categories in order
    if [ -n "${categories[breaking]}" ]; then
        echo "#### ⚠️ Breaking Changes"
        echo -e "${categories[breaking]}"
    fi

    if [ -n "${categories[security]}" ]; then
        echo "#### 🔐 Security"
        echo -e "${categories[security]}"
    fi

    if [ -n "${categories[feature]}" ]; then
        echo "#### 🚀 Features"
        echo -e "${categories[feature]}"
    fi

    if [ -n "${categories[bugfix]}" ]; then
        echo "#### 🐛 Bug Fixes"
        echo -e "${categories[bugfix]}"
    fi

    if [ -n "${categories[performance]}" ]; then
        echo "#### ⚡ Performance Improvements"
        echo -e "${categories[performance]}"
    fi

    if [ -n "${categories[refactor]}" ]; then
        echo "#### 🔨 Refactoring"
        echo -e "${categories[refactor]}"
    fi

    if [ -n "${categories[documentation]}" ]; then
        echo "#### 📚 Documentation"
        echo -e "${categories[documentation]}"
    fi

    if [ -n "${categories[test]}" ]; then
        echo "#### 🧪 Tests"
        echo -e "${categories[test]}"
    fi

    if [ -n "${categories[ci]}" ]; then
        echo "#### 👷 CI/CD"
        echo -e "${categories[ci]}"
    fi

    if [ -n "${categories[chore]}" ]; then
        echo "#### 🧹 Chores"
        echo -e "${categories[chore]}"
    fi

    if [ -n "${categories[other]}" ]; then
        echo "#### 📦 Other Changes"
        echo -e "${categories[other]}"
    fi

    # Add commit range link
    echo "### Full Changelog"
    if [ -n "$since_tag" ]; then
        echo ""
        echo "**Full Changelog**: https://github.com/$REPO_OWNER/$REPO_NAME/compare/$since_tag...$until_tag"
    fi
}

# Main function
main() {
    local since_tag=""
    local until_tag=""
    local output_file=""

    # Parse command line arguments
    while [[ $# -gt 0 ]]; do
        case $1 in
            --since)
                since_tag="$2"
                shift 2
                ;;
            --until)
                until_tag="$2"
                shift 2
                ;;
            --output)
                output_file="$2"
                shift 2
                ;;
            --help)
                echo "Usage: $0 [options]"
                echo ""
                echo "Options:"
                echo "  --since TAG    Generate changelog since this tag (default: previous tag)"
                echo "  --until TAG    Generate changelog until this tag (default: HEAD)"
                echo "  --output FILE  Write changelog to file (default: stdout)"
                echo "  --help         Show this help message"
                echo ""
                echo "Examples:"
                echo "  $0                                    # Changelog since last tag"
                echo "  $0 --since v1.0.0 --until v1.1.0    # Changelog between tags"
                echo "  $0 --output CHANGELOG.md             # Write to file"
                exit 0
                ;;
            *)
                print_error "Unknown option: $1"
                echo "Run '$0 --help' for usage"
                exit 1
                ;;
        esac
    done

    # Check prerequisites
    check_gh_cli

    # Set defaults
    if [ -z "$until_tag" ]; then
        until_tag="HEAD"
    fi

    if [ -z "$since_tag" ]; then
        if [ "$until_tag" = "HEAD" ]; then
            since_tag=$(git describe --tags --abbrev=0 2>/dev/null || echo "")
        else
            since_tag=$(get_previous_tag "$until_tag")
        fi
    fi

    # Fetch PRs
    local pr_numbers=$(fetch_prs_between_tags "$since_tag" "$until_tag")

    if [ -z "$pr_numbers" ]; then
        print_info "No pull requests found between $since_tag and $until_tag"
    fi

    # Generate changelog
    local changelog=$(generate_changelog "$since_tag" "$until_tag" "$pr_numbers")

    # Output changelog
    if [ -n "$output_file" ]; then
        echo "$changelog" > "$output_file"
        print_success "Changelog written to $output_file"
    else
        echo "$changelog"
    fi
}

# Run main function
main "$@"
