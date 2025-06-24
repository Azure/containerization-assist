package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// PackageMigration defines a file/package movement operation
type PackageMigration struct {
	From        string
	To          string
	Description string
}

var (
	execute = flag.Bool("execute", false, "Execute the migration (default: dry-run)")
	verbose = flag.Bool("verbose", false, "Verbose output")
)

// Migration plan based on REORG.md
var migrations = []PackageMigration{
	// Interface consolidation - these will be handled by Team A
	{"pkg/mcp/internal/interfaces/", "pkg/mcp/interfaces.go", "Consolidate all interfaces into single file"},
	
	// Package restructuring - flattened structure
	{"pkg/mcp/internal/engine/", "pkg/mcp/internal/runtime/", "Rename engine to runtime"},
	{"pkg/mcp/internal/tools/atomic/build/", "pkg/mcp/internal/build/", "Flatten build tools"},
	{"pkg/mcp/internal/tools/atomic/deploy/", "pkg/mcp/internal/deploy/", "Flatten deploy tools"},
	{"pkg/mcp/internal/tools/security/", "pkg/mcp/internal/scan/", "Rename security to scan"},
	{"pkg/mcp/internal/tools/analysis/", "pkg/mcp/internal/analyze/", "Rename analysis to analyze"},
	
	// Session consolidation
	{"pkg/mcp/internal/store/session/", "pkg/mcp/internal/session/", "Consolidate session management"},
	{"pkg/mcp/internal/types/session/", "pkg/mcp/internal/session/", "Merge session types"},
	
	// Transport consolidation
	{"pkg/mcp/internal/transport/", "pkg/mcp/internal/transport/", "Keep transport structure"},
	
	// Workflow simplification
	{"pkg/mcp/internal/orchestration/workflow/", "pkg/mcp/internal/workflow/", "Simplify workflow package"},
	
	// Create observability package early
	{"pkg/logger/", "pkg/mcp/internal/observability/", "Move logging to observability"},
	{"pkg/mcp/internal/ops/", "pkg/mcp/internal/observability/", "Merge ops into observability"},
	
	// Validation package (shared/exported)
	{"pkg/mcp/internal/tools/validation/", "pkg/mcp/internal/validate/", "Create shared validation package"},
}

func main() {
	flag.Parse()
	
	fmt.Println("MCP Package Migration Tool")
	fmt.Println("==========================")
	
	if !*execute {
		fmt.Println("🔍 DRY RUN MODE - Use --execute to perform actual migration")
		fmt.Println()
	}
	
	// Check git status first
	if err := checkGitStatus(); err != nil {
		log.Fatalf("Git status check failed: %v", err)
	}
	
	// Analyze current structure
	if err := analyzeCurrentStructure(); err != nil {
		log.Fatalf("Structure analysis failed: %v", err)
	}
	
	// Execute migrations
	for i, migration := range migrations {
		fmt.Printf("Migration %d/%d: %s\n", i+1, len(migrations), migration.Description)
		if *verbose {
			fmt.Printf("  From: %s\n", migration.From)
			fmt.Printf("  To: %s\n", migration.To)
		}
		
		if *execute {
			if err := executeMigration(migration); err != nil {
				log.Printf("⚠️  Migration failed: %v", err)
				continue
			}
			fmt.Println("  ✅ Completed")
		} else {
			fmt.Println("  📋 Planned")
		}
		fmt.Println()
	}
	
	if *execute {
		fmt.Println("🎉 Migration completed!")
		fmt.Println("📝 Next steps:")
		fmt.Println("   1. Run 'go run tools/update_imports.go --all' to update import paths")
		fmt.Println("   2. Run 'go mod tidy' to clean up dependencies")
		fmt.Println("   3. Run tests to validate migration")
	} else {
		fmt.Println("📋 Migration plan complete. Use --execute to run.")
	}
}

func checkGitStatus() error {
	cmd := exec.Command("git", "status", "--porcelain")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to check git status: %w", err)
	}
	
	if len(strings.TrimSpace(string(output))) > 0 {
		fmt.Println("⚠️  Warning: Working directory has uncommitted changes")
		fmt.Println("   Consider committing or stashing changes before migration")
		
		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Continue anyway? (y/N): ")
		response, _ := reader.ReadString('\n')
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(response)), "y") {
			return fmt.Errorf("migration cancelled by user")
		}
	}
	
	return nil
}

func analyzeCurrentStructure() error {
	fmt.Println("🔍 Analyzing current package structure...")
	
	packageCount := 0
	fileCount := 0
	
	err := filepath.WalkDir("pkg/mcp", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if d.IsDir() {
			packageCount++
			if *verbose {
				fmt.Printf("  📁 %s\n", path)
			}
		} else if strings.HasSuffix(path, ".go") {
			fileCount++
		}
		
		return nil
	})
	
	if err != nil {
		return fmt.Errorf("failed to analyze structure: %w", err)
	}
	
	fmt.Printf("📊 Current structure: %d packages, %d Go files\n", packageCount, fileCount)
	fmt.Println()
	
	return nil
}

func executeMigration(migration PackageMigration) error {
	// Check if source exists
	if _, err := os.Stat(migration.From); os.IsNotExist(err) {
		return fmt.Errorf("source path does not exist: %s", migration.From)
	}
	
	// Create destination directory
	destDir := filepath.Dir(migration.To)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}
	
	// Use git mv to preserve history
	cmd := exec.Command("git", "mv", migration.From, migration.To)
	if output, err := cmd.CombinedOutput(); err != nil {
		// If git mv fails, try regular move (might be new files)
		if err := moveDirectory(migration.From, migration.To); err != nil {
			return fmt.Errorf("failed to move %s -> %s: %v\nOutput: %s", 
				migration.From, migration.To, err, string(output))
		}
	}
	
	return nil
}

func moveDirectory(src, dst string) error {
	// For non-git moves, use os.Rename
	return os.Rename(src, dst)
}

// findGoFiles recursively finds all .go files in a directory
func findGoFiles(dir string) ([]string, error) {
	var files []string
	
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		
		if !d.IsDir() && strings.HasSuffix(path, ".go") {
			files = append(files, path)
		}
		
		return nil
	})
	
	return files, err
}

// getPackageFromFile extracts the package name from a Go file
func getPackageFromFile(filename string) (string, error) {
	// Simplified version - just return empty for now
	// This function is not currently used in the migration logic
	return "", nil
}