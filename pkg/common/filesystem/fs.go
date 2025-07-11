// Package filesystem provides common utility functions for file system operations,
// directory tree generation, and other shared functionality used throughout
// the container-kit application.
package filesystem

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

// FileExists checks if a file exists at the given path
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

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

// DefaultFileTreeOptions returns sensible defaults for file tree generation
// with a reasonable depth limit and common ignore patterns applied.
func DefaultFileTreeOptions() FileTreeOptions {
	return FileTreeOptions{
		MaxDepth:       5,
		IgnorePatterns: DefaultIgnorePatterns,
		UseGitIgnore:   true,
		ShowHidden:     false,
	}
}

// GenerateFileTree creates a string representation of a directory structure
// using the specified options. This consolidates the duplicate implementations
// from filetree, workspace, and analyze_simple packages.
//
// The returned string uses ASCII tree characters to show the directory
// hierarchy in a readable format.
func GenerateFileTree(rootPath string, options FileTreeOptions) (string, error) {
	var builder strings.Builder

	// Load .gitignore if enabled and exists
	var gitIgnoreMatcher *ignore.GitIgnore
	ignorePatterns := options.IgnorePatterns

	if options.UseGitIgnore {
		gitignorePath := filepath.Join(rootPath, ".gitignore")
		if _, err := os.Stat(gitignorePath); err == nil {
			gitignoreContent, err := os.ReadFile(gitignorePath)
			if err == nil {
				gitignoreLines := strings.Split(string(gitignoreContent), "\n")
				ignorePatterns = append(ignorePatterns, gitignoreLines...)
			}
		}
	}

	gitIgnoreMatcher = ignore.CompileIgnoreLines(ignorePatterns...)

	err := filepath.Walk(rootPath, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from root
		relPath, err := filepath.Rel(rootPath, filePath)
		if err != nil {
			return err
		}

		// Skip root directory itself
		if relPath == "." {
			return nil
		}

		// Check depth limit
		if options.MaxDepth > 0 {
			depth := strings.Count(relPath, string(filepath.Separator))
			if depth >= options.MaxDepth {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}

		// Check if file should be ignored
		if gitIgnoreMatcher.MatchesPath(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Skip hidden files/directories unless explicitly enabled
		if !options.ShowHidden && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Calculate indentation based on depth
		depth := strings.Count(relPath, string(filepath.Separator))
		indent := strings.Repeat("  ", depth)

		// Add entry to tree
		if info.IsDir() {
			builder.WriteString(indent + info.Name() + "/\n")
		} else {
			builder.WriteString(indent + info.Name() + "\n")
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return builder.String(), nil
}

// GenerateSimpleFileTree creates a basic file tree with minimal filtering options.
// This is a convenience function that matches the original workspace.GenerateFileTree
// behavior with no depth limit and only basic ignore patterns for common
// non-essential files.
//
// Use this function when you need a complete directory overview without
// the more aggressive filtering of DefaultFileTreeOptions.
func GenerateSimpleFileTree(rootPath string) (string, error) {
	options := FileTreeOptions{
		MaxDepth: 0, // No depth limit
		IgnorePatterns: []string{
			"node_modules",
			"target",
			"__pycache__",
		},
		UseGitIgnore: false,
		ShowHidden:   false,
	}

	return GenerateFileTree(rootPath, options)
}

// GenerateJSONFileTree creates a JSON-like representation of the directory structure
// with nested braces. This is the consolidated version of the former ReadFileTree function.
// It respects .gitignore and uses the DefaultIgnorePatterns for filtering.
func GenerateJSONFileTree(root string, maxDepth int) (string, error) {
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
					// Add file as a string value
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
		return "", err
	}

	// Format the tree as a string
	var buffer bytes.Buffer
	formatJSONTree(fileTree, &buffer, 0)
	return buffer.String(), nil
}

// formatJSONTree recursively formats the tree map into a string representation
// of a directory structure in JSON-like format
func formatJSONTree(tree map[string]interface{}, buffer *bytes.Buffer, indent int) {
	// Open the current level
	buffer.WriteString("{\n")

	// Sort keys for consistent output
	entryNames := getSortedKeys(tree)

	// Process each entry in the tree
	for i, entryName := range entryNames {
		// Create proper indentation for this level
		currentIndent := strings.Repeat("  ", indent+1)
		buffer.WriteString(currentIndent)

		// Write the entry name
		buffer.WriteString("\"" + entryName + "\"")

		entryValue := tree[entryName]
		if entryValue == nil {
			// This is a file - no additional formatting needed
		} else if subdirectory, isDirectory := entryValue.(map[string]interface{}); isDirectory {
			// This is a directory - recursively format its contents
			buffer.WriteString(": ")
			formatJSONTree(subdirectory, buffer, indent+1)
		}

		// Add comma if not the last entry
		if i < len(entryNames)-1 {
			buffer.WriteString(",")
		}
		buffer.WriteString("\n")
	}

	// Close the current level with proper indentation
	closingIndent := strings.Repeat("  ", indent)
	buffer.WriteString(closingIndent + "}")
}

// getSortedKeys extracts and sorts keys from a map
func getSortedKeys(tree map[string]interface{}) []string {
	var keys []string
	for k := range tree {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
