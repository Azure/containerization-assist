package version

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectGoVersion detects Go version from go.mod
func (d *Detector) detectGoVersion(repoPath string) string {
	goModPath := filepath.Join(repoPath, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	goRegex := regexp.MustCompile(`go\s+(\d+\.\d+(?:\.\d+)?)`)
	if matches := goRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
		version := matches[1]
		d.logger.Debug("Found Go version in go.mod", "version", version)
		return version
	}

	return ""
}

// detectGoFrameworkVersion detects version of Go frameworks
func (d *Detector) detectGoFrameworkVersion(repoPath, framework string) string {
	goModPath := filepath.Join(repoPath, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	frameworkModules := map[string]string{
		"gin":   "github.com/gin-gonic/gin",
		"echo":  "github.com/labstack/echo",
		"fiber": "github.com/gofiber/fiber",
	}

	if module, exists := frameworkModules[framework]; exists {
		moduleRegex := regexp.MustCompile(fmt.Sprintf(`%s\s+v([^\s]+)`, regexp.QuoteMeta(module)))
		if matches := moduleRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Go framework version in go.mod", "framework", framework, "version", version)
			return version
		}
	}

	return ""
}

// detectGoFrameworkFromMod detects Go frameworks from go.mod dependencies
func (d *Detector) detectGoFrameworkFromMod(repoPath string) string {
	goModPath := filepath.Join(repoPath, "go.mod")
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return ""
	}

	contentStr := string(content)

	// Framework detection patterns
	frameworks := []struct {
		name    string
		pattern string
	}{
		{"gin", "github.com/gin-gonic/gin"},
		{"echo", "github.com/labstack/echo"},
		{"fiber", "github.com/gofiber/fiber"},
		{"chi", "github.com/go-chi/chi"},
		{"mux", "github.com/gorilla/mux"},
		{"beego", "github.com/beego/beego"},
	}

	for _, fw := range frameworks {
		if strings.Contains(contentStr, fw.pattern) {
			d.logger.Debug("Found Go framework in go.mod", "framework", fw.name, "module", fw.pattern)
			return fw.name
		}
	}

	return ""
}
