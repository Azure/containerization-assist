// Package filesystem provides common utility functions for file system operations,
// directory tree generation, and other shared functionality used throughout
// the containerization-assist application.
package filesystem

import (
"os"
"path/filepath"
"strings"

ignore "github.com/sabhiram/go-gitignore"
)

// DefaultIgnorePatterns contains common directories and files to ignore when generating file trees
var DefaultIgnorePatterns = []string{
	"node_modules/",
	"vendor/",
	"go.mod",
	"go.sum",
	"target/",
	"build/",
	"out/",
	"dist/",
	"bin/",
	"obj/",
	".git/",
	".DS_Store",
	".idea/",
	".vscode/",
	"*.class",
	"*.png",
	"*.jpg",
	"*.jpeg",
	"*.gif",
	"*.ico",
	"*.svg",
	"*.woff",
	"*.woff2",
	"*.ttf",
	"*.eot",
	"__pycache__/",
	"*.pyc",
	"*.pyo",
	".pytest_cache/",
	"coverage/",
}

// FileTreeOptions configures how file trees are generated, including
// depth limits, ignore patterns, and visibility settings.
type FileTreeOptions struct {
	// MaxDepth limits how deep into the directory structure to traverse
	MaxDepth int
	// IgnorePatterns is a list of glob patterns for files/directories to skip
	IgnorePatterns []string
	// UseGitIgnore determines whether to respect .gitignore files
	UseGitIgnore bool
	// ShowHidden determines whether to include hidden files/directories
	ShowHidden bool
}

// GenerateFileTreeMap creates a structured map representation of the directory structure
// This builds on GenerateJSONFileTree but returns the actual map instead of a formatted string
func GenerateFileTreeMap(root string, maxDepth int) (map[string]interface{}, error) {
	// Create a map to represent the file tree structure
	fileTree := make(map[string]interface{})

	// Load .gitignore if it exists
	gitignorePath := filepath.Join(root, ".gitignore")
	var gitIgnoreMatcher *ignore.GitIgnore
	ignorePatterns := DefaultIgnorePatterns

	if _, err := os.Stat(gitignorePath); err == nil {
		gitignoreContent, err := os.ReadFile(gitignorePath)
		if err == nil {
			gitignoreLines := strings.Split(string(gitignoreContent), "\n")
			ignorePatterns = append(ignorePatterns, gitignoreLines...)
		}
	}

	gitIgnoreMatcher = ignore.CompileIgnoreLines(ignorePatterns...)

	// Walk the directory tree with depth limit
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil || relPath == "." {
			return nil
		}

		// count separators to determine depth
		depth := strings.Count(relPath, string(filepath.Separator))
		if maxDepth >= 0 && depth > maxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if the path is ignored
		if gitIgnoreMatcher != nil {
			// Append slash for directories so patterns ending in '/' match
			pathToMatch := relPath
			if info.IsDir() {
				pathToMatch = relPath + string(filepath.Separator)
			}
			if gitIgnoreMatcher.MatchesPath(pathToMatch) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Split the path into components
		parts := strings.Split(relPath, string(filepath.Separator))

		// Traverse/build the tree
		current := fileTree
		for i, part := range parts {
			isLast := i == len(parts)-1

			if isLast {
				if !info.IsDir() {
					// Add file as a null value
					current[part] = nil
				} else {
					// Create an empty map for directories
					if _, exists := current[part]; !exists {
						current[part] = make(map[string]interface{})
					}
				}
			} else {
				// Ensure parent directory exists in the tree
				if _, exists := current[part]; !exists {
					current[part] = make(map[string]interface{})
				}
				// Move deeper into the tree - safely check type assertion
				if nextLevel, ok := current[part].(map[string]interface{}); ok {
					current = nextLevel
				} else {
					// Handle unexpected type - create a new map
					current[part] = make(map[string]interface{})
					current = current[part].(map[string]interface{})
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return fileTree, nil
}
