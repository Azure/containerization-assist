package version

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// detectJavaVersion detects Java version from various sources
func (d *Detector) detectJavaVersion(repoPath string) string {
	// Check pom.xml for maven.compiler.source or java.version
	pomPath := filepath.Join(repoPath, "pom.xml")
	if content, err := os.ReadFile(pomPath); err == nil {
		// Look for maven.compiler.source property
		sourceRegex := regexp.MustCompile(`<maven\.compiler\.source>([^<]+)</maven\.compiler\.source>`)
		if matches := sourceRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Java version in pom.xml (compiler.source)", "version", version)
			return version
		}

		// Look for java.version property
		versionRegex := regexp.MustCompile(`<java\.version>([^<]+)</java\.version>`)
		if matches := versionRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Java version in pom.xml (java.version)", "version", version)
			return version
		}
	}

	// Check build.gradle for Java version
	gradlePath := filepath.Join(repoPath, "build.gradle")
	if content, err := os.ReadFile(gradlePath); err == nil {
		javaRegex := regexp.MustCompile(`sourceCompatibility\s*=\s*['"]*([^'"\s]+)['"]*`)
		if matches := javaRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Java version in build.gradle", "version", version)
			return version
		}
	}

	return ""
}

// detectJavaFrameworkVersion detects version of Java build tools
func (d *Detector) detectJavaFrameworkVersion(repoPath, framework string) string {
	if strings.Contains(framework, "maven") {
		return d.detectMavenVersion(repoPath)
	}
	if strings.Contains(framework, "gradle") {
		return d.detectGradleVersion(repoPath)
	}
	return ""
}

// detectMavenVersion detects Maven version from various sources
func (d *Detector) detectMavenVersion(repoPath string) string {
	// Check maven-wrapper.properties
	wrapperPath := filepath.Join(repoPath, ".mvn", "wrapper", "maven-wrapper.properties")
	if content, err := os.ReadFile(wrapperPath); err == nil {
		versionRegex := regexp.MustCompile(`distributionUrl=.*apache-maven-([^-]+)-`)
		if matches := versionRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Maven version in wrapper", "version", version)
			return version
		}
	}

	// Check pom.xml for minimum Maven version
	pomPath := filepath.Join(repoPath, "pom.xml")
	if content, err := os.ReadFile(pomPath); err == nil {
		versionRegex := regexp.MustCompile(`<maven\.version>([^<]+)</maven\.version>`)
		if matches := versionRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Maven version in pom.xml", "version", version)
			return version
		}
	}

	return ""
}

// detectGradleVersion detects Gradle version from wrapper
func (d *Detector) detectGradleVersion(repoPath string) string {
	wrapperPath := filepath.Join(repoPath, "gradle", "wrapper", "gradle-wrapper.properties")
	content, err := os.ReadFile(wrapperPath)
	if err != nil {
		return ""
	}

	versionRegex := regexp.MustCompile(`distributionUrl=.*gradle-([^-]+)-`)
	if matches := versionRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
		version := matches[1]
		d.logger.Debug("Found Gradle version in wrapper", "version", version)
		return version
	}

	return ""
}

// detectSpringVersion detects Spring/Spring Boot version
func (d *Detector) detectSpringVersion(repoPath string) string {
	// Check pom.xml for Spring Boot parent
	pomPath := filepath.Join(repoPath, "pom.xml")
	if content, err := os.ReadFile(pomPath); err == nil {
		springBootRegex := regexp.MustCompile(`<groupId>org\.springframework\.boot</groupId>\s*<artifactId>spring-boot-starter-parent</artifactId>\s*<version>([^<]+)</version>`)
		if matches := springBootRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Spring Boot version in pom.xml", "version", version)
			return version
		}
	}

	// Check build.gradle for Spring Boot plugin
	gradlePath := filepath.Join(repoPath, "build.gradle")
	if content, err := os.ReadFile(gradlePath); err == nil {
		springBootRegex := regexp.MustCompile(`org\.springframework\.boot['"]\s*version\s*['"]*([^'"\s]+)['"]*`)
		if matches := springBootRegex.FindStringSubmatch(string(content)); len(matches) > 1 {
			version := matches[1]
			d.logger.Debug("Found Spring Boot version in build.gradle", "version", version)
			return version
		}
	}

	return ""
}
