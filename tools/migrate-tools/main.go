package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var (
	execute = flag.Bool("execute", false, "Execute the migration (default: dry-run)")
	verbose = flag.Bool("verbose", false, "Verbose output")
)

// ToolMigration defines a tool file movement
type ToolMigration struct {
	From        string
	To          string
	Description string
}

// Tool file migrations based on REORG.md domain organization
var toolMigrations = []ToolMigration{
	// Build tools
	{"pkg/mcp/internal/tools/build_image.go", "pkg/mcp/internal/build/build_image.go", "Move build_image tool"},
	{"pkg/mcp/internal/tools/build_image_atomic.go", "pkg/mcp/internal/build/build_image_atomic.go", "Move build_image_atomic tool"},
	{"pkg/mcp/internal/tools/build_image_atomic_test.go", "pkg/mcp/internal/build/build_image_atomic_test.go", "Move build_image_atomic test"},
	{"pkg/mcp/internal/tools/build_context.go", "pkg/mcp/internal/build/build_context.go", "Move build_context"},
	{"pkg/mcp/internal/tools/build_executor.go", "pkg/mcp/internal/build/build_executor.go", "Move build_executor"},
	{"pkg/mcp/internal/tools/build_fixer.go", "pkg/mcp/internal/build/build_fixer.go", "Move build_fixer"},
	{"pkg/mcp/internal/tools/build_validator.go", "pkg/mcp/internal/build/build_validator.go", "Move build_validator"},
	{"pkg/mcp/internal/tools/tag_image_atomic.go", "pkg/mcp/internal/build/tag_image.go", "Move tag_image tool"},
	{"pkg/mcp/internal/tools/push_image.go", "pkg/mcp/internal/build/push_image.go", "Move push_image tool"},
	{"pkg/mcp/internal/tools/push_image_atomic.go", "pkg/mcp/internal/build/push_image_atomic.go", "Move push_image_atomic tool"},
	{"pkg/mcp/internal/tools/push_image_atomic_test.go", "pkg/mcp/internal/build/push_image_atomic_test.go", "Move push_image_atomic test"},
	{"pkg/mcp/internal/tools/pull_image_atomic.go", "pkg/mcp/internal/build/pull_image.go", "Move pull_image tool"},

	// Deploy tools
	{"pkg/mcp/internal/tools/deploy_kubernetes_atomic.go", "pkg/mcp/internal/deploy/deploy_kubernetes.go", "Move deploy_kubernetes tool"},
	{"pkg/mcp/internal/tools/generate_manifests.go", "pkg/mcp/internal/deploy/generate_manifests.go", "Move generate_manifests tool"},
	{"pkg/mcp/internal/tools/generate_manifests_atomic.go", "pkg/mcp/internal/deploy/generate_manifests_atomic.go", "Move generate_manifests_atomic"},
	{"pkg/mcp/internal/tools/generate_manifests_resources.go", "pkg/mcp/internal/deploy/generate_manifests_resources.go", "Move manifest resources"},
	{"pkg/mcp/internal/tools/generate_manifests_secrets.go", "pkg/mcp/internal/deploy/generate_manifests_secrets.go", "Move manifest secrets"},
	{"pkg/mcp/internal/tools/generate_manifests_strategy.go", "pkg/mcp/internal/deploy/generate_manifests_strategy.go", "Move manifest strategy"},
	{"pkg/mcp/internal/tools/generate_manifests_types.go", "pkg/mcp/internal/deploy/generate_manifests_types.go", "Move manifest types"},
	{"pkg/mcp/internal/tools/generate_manifests_yaml.go", "pkg/mcp/internal/deploy/generate_manifests_yaml.go", "Move manifest yaml"},
	{"pkg/mcp/internal/tools/check_health_atomic.go", "pkg/mcp/internal/deploy/check_health.go", "Move check_health tool"},

	// Scan tools (security)
	{"pkg/mcp/internal/tools/scan_image_security_atomic.go", "pkg/mcp/internal/scan/scan_image_security.go", "Move security scan tool"},
	{"pkg/mcp/internal/tools/scan_secrets_atomic.go", "pkg/mcp/internal/scan/scan_secrets.go", "Move secrets scan tool"},

	// Analysis tools
	{"pkg/mcp/internal/tools/analyze_repository.go", "pkg/mcp/internal/analyze/analyze_repository.go", "Move analyze_repository"},
	{"pkg/mcp/internal/tools/analyze_repository_atomic.go", "pkg/mcp/internal/analyze/analyze_repository_atomic.go", "Move analyze_repository_atomic"},
	{"pkg/mcp/internal/tools/analyze_repository_atomic_test.go", "pkg/mcp/internal/analyze/analyze_repository_atomic_test.go", "Move analyze test"},
	{"pkg/mcp/internal/tools/analyze_simple.go", "pkg/mcp/internal/analyze/analyze_simple.go", "Move analyze_simple"},
	{"pkg/mcp/internal/tools/validate_dockerfile_atomic.go", "pkg/mcp/internal/analyze/validate_dockerfile.go", "Move dockerfile validator"},
	{"pkg/mcp/internal/tools/generate_dockerfile.go", "pkg/mcp/internal/analyze/generate_dockerfile.go", "Move dockerfile generator"},
	{"pkg/mcp/internal/tools/generate_dockerfile_enhanced.go", "pkg/mcp/internal/analyze/generate_dockerfile_enhanced.go", "Move enhanced generator"},

	// Validation helpers (to validate package)
	{"pkg/mcp/internal/tools/validation_helpers.go", "pkg/mcp/internal/validate/validation_helpers.go", "Move validation helpers"},
	{"pkg/mcp/internal/tools/validation_test.go", "pkg/mcp/internal/validate/validation_test.go", "Move validation tests"},
	{"pkg/mcp/internal/services/validation/service.go", "pkg/mcp/internal/validate/service.go", "Move validation service"},
}

func main() {
	flag.Parse()

	fmt.Println("MCP Tool File Migration")
	fmt.Println("=======================")

	if !*execute {
		fmt.Println("🔍 DRY RUN MODE - Use --execute to perform actual migration")
		fmt.Println()
	}

	successCount := 0
	skipCount := 0
	failCount := 0

	for i, migration := range toolMigrations {
		fmt.Printf("Migration %d/%d: %s\n", i+1, len(toolMigrations), migration.Description)

		if *verbose {
			fmt.Printf("  From: %s\n", migration.From)
			fmt.Printf("  To: %s\n", migration.To)
		}

		// Check if source exists
		if _, err := os.Stat(migration.From); os.IsNotExist(err) {
			fmt.Printf("  ⏭️  Skipped (source not found)\n")
			skipCount++
			continue
		}

		if *execute {
			if err := executeMigration(migration); err != nil {
				fmt.Printf("  ❌ Failed: %v\n", err)
				failCount++
			} else {
				fmt.Printf("  ✅ Completed\n")
				successCount++
			}
		} else {
			fmt.Printf("  📋 Planned\n")
		}
		fmt.Println()
	}

	fmt.Printf("\n📊 Summary:\n")
	fmt.Printf("   Total migrations: %d\n", len(toolMigrations))
	if *execute {
		fmt.Printf("   Successful: %d\n", successCount)
		fmt.Printf("   Skipped: %d\n", skipCount)
		fmt.Printf("   Failed: %d\n", failCount)
	}

	if !*execute {
		fmt.Println("\n📋 Migration plan complete. Use --execute to run.")
	}
}

func executeMigration(migration ToolMigration) error {
	// Create destination directory
	destDir := filepath.Dir(migration.To)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Use git mv to preserve history
	cmd := exec.Command("git", "mv", migration.From, migration.To)
	if output, err := cmd.CombinedOutput(); err != nil {
		// If git mv fails, try regular move
		if err := os.Rename(migration.From, migration.To); err != nil {
			return fmt.Errorf("failed to move: %v (git output: %s)", err, string(output))
		}
	}

	return nil
}
