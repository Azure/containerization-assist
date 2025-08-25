package sampling

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManifestFix_Validation(t *testing.T) {
	tests := []struct {
		name    string
		fix     ManifestFix
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid manifest fix",
			fix: ManifestFix{
				FixedManifest: `apiVersion: v1
kind: Pod
metadata:
  name: test`,
			},
			wantErr: false,
		},
		{
			name: "empty manifest",
			fix: ManifestFix{
				FixedManifest: "",
			},
			wantErr: true,
			errMsg:  "fixed manifest cannot be empty",
		},
		{
			name: "missing apiVersion",
			fix: ManifestFix{
				FixedManifest: `kind: Pod
metadata:
  name: test`,
			},
			wantErr: true,
			errMsg:  "fixed manifest must contain apiVersion",
		},
		{
			name: "missing kind",
			fix: ManifestFix{
				FixedManifest: `apiVersion: v1
metadata:
  name: test`,
			},
			wantErr: true,
			errMsg:  "fixed manifest must contain kind",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fix.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDockerfileFix_Validation(t *testing.T) {
	tests := []struct {
		name    string
		fix     DockerfileFix
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid dockerfile fix",
			fix: DockerfileFix{
				FixedDockerfile: `FROM node:16
WORKDIR /app
COPY . .
RUN npm install`,
			},
			wantErr: false,
		},
		{
			name: "empty dockerfile",
			fix: DockerfileFix{
				FixedDockerfile: "",
			},
			wantErr: true,
			errMsg:  "fixed dockerfile cannot be empty",
		},
		{
			name: "missing FROM instruction",
			fix: DockerfileFix{
				FixedDockerfile: `WORKDIR /app
COPY . .
RUN npm install`,
			},
			wantErr: true,
			errMsg:  "dockerfile must contain FROM instruction",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fix.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestParseManifestFix(t *testing.T) {

	t.Run("parse manifest from code block", func(t *testing.T) {
		content := `Here's the fixed manifest:

` + "```yaml" + `
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
  - name: app
    image: nginx:latest
` + "```" + `

This fixes the issue with the missing image tag.`

		result, err := ParseManifestFix(content)
		require.NoError(t, err)

		assert.Contains(t, result.FixedManifest, "apiVersion: v1")
		assert.Contains(t, result.FixedManifest, "kind: Pod")
		assert.Contains(t, result.FixedManifest, "nginx:latest")
		assert.True(t, result.ValidationStatus.SyntaxValid)
	})

	t.Run("parse manifest without code block", func(t *testing.T) {
		content := `apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  selector:
    app: test
  ports:
  - port: 80`

		result, err := ParseManifestFix(content)
		require.NoError(t, err)

		assert.Contains(t, result.FixedManifest, "apiVersion: v1")
		assert.Contains(t, result.FixedManifest, "kind: Service")
		assert.True(t, result.ValidationStatus.SyntaxValid)
	})

	t.Run("invalid manifest content", func(t *testing.T) {
		content := "This is not a valid Kubernetes manifest"

		result, err := ParseManifestFix(content)
		require.NoError(t, err)

		assert.False(t, result.ValidationStatus.SyntaxValid)
		assert.Len(t, result.ValidationStatus.Errors, 1)
	})
}

func TestParseDockerfileFix(t *testing.T) {

	t.Run("parse dockerfile with instructions", func(t *testing.T) {
		content := `Here's the fixed Dockerfile:

FROM node:16-alpine
WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY . .
EXPOSE 3000
CMD ["npm", "start"]

This fixes the security issues and optimizes the build.`

		result, err := ParseDockerfileFix(content)
		require.NoError(t, err)

		assert.Contains(t, result.FixedDockerfile, "FROM node:16-alpine")
		assert.Contains(t, result.FixedDockerfile, "WORKDIR /app")
		assert.Contains(t, result.FixedDockerfile, "EXPOSE 3000")
		assert.True(t, result.ValidationStatus.SyntaxValid)
	})

	t.Run("invalid dockerfile content", func(t *testing.T) {
		content := "This is not a valid Dockerfile"

		result, err := ParseDockerfileFix(content)
		require.NoError(t, err)

		assert.False(t, result.ValidationStatus.SyntaxValid)
		assert.Len(t, result.ValidationStatus.Errors, 1)
	})
}

func TestParseSecurityAnalysis(t *testing.T) {

	t.Run("parse security analysis with sections", func(t *testing.T) {
		content := `Security Analysis Results:

Critical Vulnerabilities:
- CVE-2023-1234: High severity SQL injection in library X
- CVE-2023-5678: Critical buffer overflow in component Y

Remediations:
- Update library X to version 2.1.0 or later
- Replace component Y with secure alternative
- Add input validation for user data

Alternative Base Images:
- alpine:3.18
- distroless/java:11
- ubuntu:22.04

The risk level is CRITICAL due to the SQL injection vulnerability.`

		result, err := ParseSecurityAnalysis(content)
		require.NoError(t, err)

		assert.Equal(t, RiskLevelCritical, result.RiskLevel)
		assert.Greater(t, len(result.Remediations), 0)
		assert.Greater(t, len(result.AlternativeImages), 0)
		assert.Contains(t, result.AlternativeImages, "alpine:3.18")
	})

	t.Run("parse medium risk analysis", func(t *testing.T) {
		content := `Security scan shows some medium severity issues that should be addressed.`

		result, err := ParseSecurityAnalysis(content)
		require.NoError(t, err)

		assert.Equal(t, RiskLevelMedium, result.RiskLevel)
	})
}

func TestParseRepositoryAnalysis(t *testing.T) {

	t.Run("parse java spring boot analysis", func(t *testing.T) {
		content := `Repository Analysis:

Language: Java
Framework: Spring Boot
Build Tool: Maven
Port: 8080

This appears to be a standard Spring Boot application with REST endpoints.
It includes dependencies for web, data-jpa, and security modules.
The application should run on port 8080 by default.`

		result, err := ParseRepositoryAnalysis(content)
		require.NoError(t, err)

		assert.Equal(t, "java", result.Language)
		assert.Equal(t, "spring-boot", result.Framework)
		assert.Contains(t, result.SuggestedPorts, 8080)
		assert.Greater(t, result.Confidence, 0.0)
	})

	t.Run("parse node.js express analysis", func(t *testing.T) {
		content := `This is a Node.js application using Express framework.
Default port is typically 3000 for Express apps.`

		result, err := ParseRepositoryAnalysis(content)
		require.NoError(t, err)

		assert.Equal(t, "javascript", result.Language)
		assert.Equal(t, "express", result.Framework)
		assert.Contains(t, result.SuggestedPorts, 3000)
	})
}

func TestSecurityAnalysis_Validation(t *testing.T) {
	tests := []struct {
		name     string
		analysis SecurityAnalysis
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid analysis with remediations",
			analysis: SecurityAnalysis{
				Remediations: []Remediation{
					{Action: "Update package", Priority: PriorityHigh},
				},
			},
			wantErr: false,
		},
		{
			name: "valid analysis with recommendations",
			analysis: SecurityAnalysis{
				Recommendations: []string{"Use secure base image"},
			},
			wantErr: false,
		},
		{
			name: "empty analysis",
			analysis: SecurityAnalysis{
				Remediations:    []Remediation{},
				Recommendations: []string{},
			},
			wantErr: true,
			errMsg:  "security analysis must contain remediations or recommendations",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.analysis.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestRepositoryAnalysis_Validation(t *testing.T) {
	tests := []struct {
		name     string
		analysis RepositoryAnalysis
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid analysis",
			analysis: RepositoryAnalysis{
				Language:   "java",
				Confidence: 0.9,
			},
			wantErr: false,
		},
		{
			name: "missing language",
			analysis: RepositoryAnalysis{
				Confidence: 0.9,
			},
			wantErr: true,
			errMsg:  "repository analysis must identify language",
		},
		{
			name: "invalid confidence too low",
			analysis: RepositoryAnalysis{
				Language:   "java",
				Confidence: -0.1,
			},
			wantErr: true,
			errMsg:  "confidence must be between 0.0 and 1.0",
		},
		{
			name: "invalid confidence too high",
			analysis: RepositoryAnalysis{
				Language:   "java",
				Confidence: 1.1,
			},
			wantErr: true,
			errMsg:  "confidence must be between 0.0 and 1.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.analysis.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestJSON_Marshaling(t *testing.T) {
	t.Run("manifest fix json serialization", func(t *testing.T) {
		fix := ManifestFix{
			OriginalIssues: []string{"Missing image tag"},
			FixedManifest:  "apiVersion: v1\nkind: Pod",
			ChangesApplied: []Change{
				{
					Type:        ChangeTypeModified,
					Section:     "spec.containers[0].image",
					Description: "Added latest tag",
					OldValue:    "nginx",
					NewValue:    "nginx:latest",
				},
			},
			ValidationStatus: ValidationResult{
				IsValid:       true,
				SyntaxValid:   true,
				BestPractices: true,
			},
			Metadata: ResponseMetadata{
				TemplateID:  "kubernetes-manifest-fix",
				GeneratedAt: time.Now(),
				TokensUsed:  150,
				Confidence:  0.95,
			},
		}

		data, err := json.Marshal(fix)
		require.NoError(t, err)

		var unmarshaled ManifestFix
		err = json.Unmarshal(data, &unmarshaled)
		require.NoError(t, err)

		assert.Equal(t, fix.OriginalIssues, unmarshaled.OriginalIssues)
		assert.Equal(t, fix.FixedManifest, unmarshaled.FixedManifest)
		assert.Equal(t, fix.ValidationStatus.IsValid, unmarshaled.ValidationStatus.IsValid)
	})

	t.Run("security analysis json serialization", func(t *testing.T) {
		analysis := SecurityAnalysis{
			CriticalIssues: []SecurityIssue{
				{
					CVE:         "CVE-2023-1234",
					Severity:    SeverityCritical,
					Component:   "openssl",
					Description: "Buffer overflow vulnerability",
					FixVersion:  "1.1.1q",
				},
			},
			Remediations: []Remediation{
				{
					IssueType: "outdated-package",
					Action:    "Update openssl to version 1.1.1q",
					Commands:  []string{"apt-get update", "apt-get install openssl=1.1.1q"},
					Priority:  PriorityImmediate,
					Effort:    EffortMinimal,
				},
			},
			RiskLevel: RiskLevelCritical,
		}

		data, err := json.Marshal(analysis)
		require.NoError(t, err)

		// Verify JSON structure
		assert.Contains(t, string(data), "CVE-2023-1234")
		assert.Contains(t, string(data), "critical")
		assert.Contains(t, string(data), "immediate")
	})
}

func TestString_Methods(t *testing.T) {
	fix := ManifestFix{
		OriginalIssues: []string{"Test issue"},
		FixedManifest:  "test manifest",
	}

	str := fix.String()
	assert.Contains(t, str, "original_issues")
	assert.Contains(t, str, "Test issue")
	assert.True(t, json.Valid([]byte(str)))
}

func TestEnums(t *testing.T) {
	t.Run("change types", func(t *testing.T) {
		assert.Equal(t, "added", string(ChangeTypeAdded))
		assert.Equal(t, "removed", string(ChangeTypeRemoved))
		assert.Equal(t, "modified", string(ChangeTypeModified))
	})

	t.Run("severity levels", func(t *testing.T) {
		assert.Equal(t, "critical", string(SeverityCritical))
		assert.Equal(t, "high", string(SeverityHigh))
		assert.Equal(t, "medium", string(SeverityMedium))
		assert.Equal(t, "low", string(SeverityLow))
	})

	t.Run("risk levels", func(t *testing.T) {
		assert.Equal(t, "critical", string(RiskLevelCritical))
		assert.Equal(t, "minimal", string(RiskLevelMinimal))
	})

	t.Run("priorities", func(t *testing.T) {
		assert.Equal(t, "immediate", string(PriorityImmediate))
		assert.Equal(t, "low", string(PriorityLow))
	})

	t.Run("effort levels", func(t *testing.T) {
		assert.Equal(t, "minimal", string(EffortMinimal))
		assert.Equal(t, "extensive", string(EffortExtensive))
	})
}

func TestComplexParsing(t *testing.T) {
	t.Run("complex manifest fix with multiple sections", func(t *testing.T) {
		content := `Analysis of the Kubernetes manifest revealed several issues:

Issues found:
- Missing resource limits
- No health checks configured
- Incorrect image tag

Here's the corrected manifest:

` + "```yaml" + `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web-app
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: web-app
        image: nginx:1.21-alpine
        resources:
          requests:
            memory: "64Mi"
` + "```" + `

Key improvements:
- Added resource requests and limits
- Used specific image tag instead of 'latest'`

		result, err := ParseManifestFix(content)
		require.NoError(t, err)

		// Verify the parsed manifest contains key elements
		manifest := result.FixedManifest
		assert.Contains(t, manifest, "apiVersion: apps/v1")
		assert.Contains(t, manifest, "kind: Deployment")
		assert.Contains(t, manifest, "nginx:1.21-alpine")

		// Verify validation passed
		assert.True(t, result.ValidationStatus.SyntaxValid)
		assert.True(t, result.ValidationStatus.IsValid)

		// Verify metadata
		assert.NotZero(t, result.Metadata.GeneratedAt)
	})
}
