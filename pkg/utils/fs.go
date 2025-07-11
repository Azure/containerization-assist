// Package utils provides common utility functions for file system operations,
// directory tree generation, and other shared functionality used throughout
// the container-kit application.
package utils

import (
	"os"
	"path/filepath"
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
