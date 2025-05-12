package filetree

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"strings"

	ignore "github.com/sabhiram/go-gitignore"
)

var defaultIgnores = []string{
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
	"*.mp4",
	"*.ico",
	"*.svg",
	"*.log",
	"*.exe",
}

func ReadFileTree(root string, maxDepth int) (string, error) {
	// Create a map to represent the file tree structure
	fileTree := make(map[string]interface{})

	// Load .gitignore if it exists
	gitignorePath := filepath.Join(root, ".gitignore")
	var gitIgnoreMatcher *ignore.GitIgnore
	ignorePatterns := defaultIgnores

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
				// Move deeper into the tree
				current = current[part].(map[string]interface{})
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	// Format the tree as a string
	var buffer bytes.Buffer
	formatTree(fileTree, &buffer, 0)
	return buffer.String(), nil
}

// formatTree recursively formats the tree map into a string representation
// of a directory structure in JSON-like format
func formatTree(tree map[string]interface{}, buffer *bytes.Buffer, indent int) {
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
			formatTree(subdirectory, buffer, indent+1)
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
