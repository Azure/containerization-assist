package main

import (
	"bytes"
	"os"
	"path/filepath"

	ignore "github.com/sabhiram/go-gitignore"
)

var defaultIgnores = []string{
	"node_modules/",
	"vendor/",
	"go.mod",
	"go.sum",
	"bin/",
	"obj/",
	".git/",
	".DS_Store",
}

func readFileTree(root string) (string, error) {
	var buffer bytes.Buffer

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

	// Walk the directory tree using Walk as suggested which is more efficient
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}

		if gitIgnoreMatcher != nil && gitIgnoreMatcher.MatchesPath(relPath) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		buffer.WriteString(relPath + "\n")
		return nil
	})

	if err != nil {
		return "", err
	}

	return buffer.String(), nil
}
