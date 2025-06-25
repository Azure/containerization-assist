package mcp

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMCPNoExternalAIDependencies(t *testing.T) {
	// Get the path to pkg/mcp/ relative to this test file
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok, "Failed to get current file path")

	// We're already in the pkg/mcp directory
	mcpPath := filepath.Dir(currentFile)
	mcpPath, err := filepath.Abs(mcpPath)
	require.NoError(t, err)

	// Navigate to repo root
	repoRoot := filepath.Join(mcpPath, "..", "..")
	repoRoot, err = filepath.Abs(repoRoot)
	require.NoError(t, err)

	t.Run("NoAzureOpenAIImports", func(t *testing.T) {
		// Test 1: No Azure OpenAI imports in MCP package only
		err := filepath.Walk(mcpPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}

			content, err := os.ReadFile(path)
			if err != nil {
				return err
			}

			// Skip this test file itself
			if strings.Contains(path, "no_external_ai_test.go") {
				return nil
			}

			contentStr := string(content)
			if strings.Contains(contentStr, "azure-openai") ||
				strings.Contains(contentStr, "github.com/Azure/azure-openai") ||
				strings.Contains(contentStr, "github.com/Azure/azure-sdk-for-go") {
				t.Errorf("Found Azure OpenAI import in MCP code: %s", path)
			}

			return nil
		})
		require.NoError(t, err)
	})

	t.Run("NoHTTPImportsInTools", func(t *testing.T) {
		// Test 2: No external HTTP calls from MCP tool code
		toolsPath := filepath.Join(mcpPath, "tools")
		if _, err := os.Stat(toolsPath); os.IsNotExist(err) {
			t.Skip("tools directory does not exist")
			return
		}

		err := filepath.Walk(toolsPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}

			fset := token.NewFileSet()
			node, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
			if err != nil {
				return err
			}

			ast.Inspect(node, func(n ast.Node) bool {
				if importSpec, ok := n.(*ast.ImportSpec); ok {
					if importSpec.Path.Value == `"net/http"` {
						t.Errorf("MCP Tool %s imports net/http - MCP tools should not make external calls", path)
					}
				}
				return true
			})

			return nil
		})
		require.NoError(t, err)
	})

	t.Run("NoAzureEnvVars", func(t *testing.T) {
		// Test 3: No AZURE_OPENAI environment variable references in MCP code or tests
		checkDirs := []string{
			mcpPath,
			filepath.Join(repoRoot, "test"),
			filepath.Join(repoRoot, "hack"),
			filepath.Join(repoRoot, "scripts"),
		}

		for _, dir := range checkDirs {
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				continue // Skip if directory doesn't exist
			}

			// Skip checking hack/ and scripts/ dirs as they're CLI-related
			if strings.Contains(dir, "hack") || strings.Contains(dir, "scripts") {
				continue
			}

			cmd := exec.Command("grep", "-r", "AZURE_OPENAI_", dir)
			cmd.Dir = repoRoot
			output, err := cmd.Output()
			if err == nil && len(output) > 0 {
				// Filter out references in this test file itself
				lines := strings.Split(string(output), "\n")
				filteredLines := []string{}
				for _, line := range lines {
					if !strings.Contains(line, "no_external_ai_test.go") && strings.TrimSpace(line) != "" {
						filteredLines = append(filteredLines, line)
					}
				}
				if len(filteredLines) > 0 {
					t.Errorf("Found AZURE_OPENAI_ environment variable references in %s:\\n%s", dir, strings.Join(filteredLines, "\n"))
				}
			}
		}
	})

	t.Run("NoAzureReplaceBlocks", func(t *testing.T) {
		// Test 4: No go replace blocks pointing to Azure SDK
		goModPath := filepath.Join(repoRoot, "go.mod")
		if _, err := os.Stat(goModPath); os.IsNotExist(err) {
			t.Skip("go.mod file does not exist")
			return
		}

		cmd := exec.Command("grep", "replace.*azure", goModPath)
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			t.Errorf("Found go replace blocks pointing to Azure SDK:\\n%s", output)
		}
	})

	t.Run("BuildTagsSeparation", func(t *testing.T) {
		// Test 5: Verify build tags are properly used
		// Check that analyzer_cli.go has //go:build cli
		cliAnalyzerPath := filepath.Join(repoRoot, "pkg", "ai", "analyzer_cli.go")
		if _, err := os.Stat(cliAnalyzerPath); err == nil {
			content, err := os.ReadFile(cliAnalyzerPath)
			require.NoError(t, err)
			if !strings.Contains(string(content), "//go:build cli") {
				t.Error("analyzer_cli.go should have //go:build cli tag")
			}
		}

		// MCP analyzer doesn't need build tags since it's standalone
		// Check that MCP analyzer exists and is separate from CLI analyzer
		mcpAnalyzerPath := filepath.Join(mcpPath, "internal", "analyze", "analyzer.go")
		if _, err := os.Stat(mcpAnalyzerPath); os.IsNotExist(err) {
			t.Error("MCP analyzer.go should exist in pkg/mcp/internal/analyze/")
		}
	})
}
