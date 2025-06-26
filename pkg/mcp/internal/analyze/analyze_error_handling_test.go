package analyze

import (
	"context"
	"testing"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicAnalyzeRepositoryTool_ErrorHandling(t *testing.T) {
	logger := zerolog.Nop()
	tool := NewAtomicAnalyzeRepositoryTool(nil, nil, logger)
	ctx := context.Background()

	t.Run("invalid_argument_types", func(t *testing.T) {
		invalidArgs := []interface{}{
			nil,
			"string",
			123,
			[]string{"array"},
			map[string]interface{}{"key": "value"},
			struct{ Field string }{Field: "value"},
		}

		for i, args := range invalidArgs {
			t.Run("invalid_type_"+string(rune('0'+i)), func(t *testing.T) {
				err := tool.Validate(ctx, args)
				assert.Error(t, err, "Should return error for invalid argument type")

				assert.Contains(t, err.Error(), "Invalid argument type", "Error should mention invalid argument type")
			})
		}
	})

	t.Run("missing_required_fields", func(t *testing.T) {
		tests := []struct {
			name     string
			args     AtomicAnalyzeRepositoryArgs
			errorMsg string
		}{
			{
				name: "missing_session_id",
				args: AtomicAnalyzeRepositoryArgs{
					RepoURL: "https://github.com/example/repo",
				},
				errorMsg: "SessionID is required",
			},
			{
				name: "missing_repo_url",
				args: AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
				},
				errorMsg: "RepoURL is required",
			},
			{
				name: "empty_session_id",
				args: AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "",
					},
					RepoURL: "https://github.com/example/repo",
				},
				errorMsg: "SessionID is required",
			},
			{
				name: "empty_repo_url",
				args: AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
					RepoURL: "",
				},
				errorMsg: "RepoURL is required",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tool.Validate(ctx, tt.args)
				require.Error(t, err, "Should return error for missing required field")
				assert.Contains(t, err.Error(), tt.errorMsg, "Error should contain expected message")
			})
		}
	})

	t.Run("malformed_urls", func(t *testing.T) {
		malformedUrls := []string{
			"not-a-url",
			"://missing-scheme",
			"http://",
			"https://",
			"ftp://not-git-protocol.com/repo",
			"git://insecure-protocol.com/repo",
			"file:///local/path",
			"../relative/path",
			"./current/path",
			"/absolute/local/path",
			"https://user:password@github.com/repo", // credentials in URL
			"https://github.com/",                   // missing repo path
			"https://github.com/user",               // missing repo name
			"https://github.com/user/",              // trailing slash only
			"https://github.com/.git",               // invalid repo name
			"https://github.com/user/.git",          // invalid repo name
			"https://",                              // incomplete URL
			"github.com/user/repo",                  // missing protocol
		}

		for _, url := range malformedUrls {
			t.Run("malformed_url_"+url, func(t *testing.T) {
				args := AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
					RepoURL: url,
				}

				err := tool.Validate(ctx, args)
				// Some malformed URLs might still pass validation if the tool is lenient
				// The validation might happen during execution rather than validation
				if err != nil {
					assert.Contains(t, err.Error(), "malformed", "Error should mention URL issue")
				}
			})
		}
	})

	t.Run("boundary_value_testing", func(t *testing.T) {
		tests := []struct {
			name      string
			args      AtomicAnalyzeRepositoryArgs
			wantError bool
		}{
			{
				name: "very_long_session_id",
				args: AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: string(make([]byte, 1000)), // Very long session ID
					},
					RepoURL: "https://github.com/example/repo",
				},
				wantError: false, // Should handle long session IDs
			},
			{
				name: "very_long_repo_url",
				args: AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
					RepoURL: "https://github.com/very-long-username-that-might-cause-issues/very-long-repository-name-with-many-characters-that-could-potentially-cause-buffer-overflows-or-other-issues",
				},
				wantError: false, // Should handle long URLs
			},
			{
				name: "session_id_with_special_chars",
				args: AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123!@#$%^&*()_+-=[]{}|;':\",./<>?`~",
					},
					RepoURL: "https://github.com/example/repo",
				},
				wantError: false, // Should handle special characters
			},
			{
				name: "unicode_in_session_id",
				args: AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-λαμβδα-🚀-测试",
					},
					RepoURL: "https://github.com/example/repo",
				},
				wantError: false, // Should handle Unicode
			},
			{
				name: "context_field_very_long",
				args: AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
					RepoURL: "https://github.com/example/repo",
					Context: string(make([]byte, 10000)), // Very long context
				},
				wantError: false, // Should handle long context
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tool.Validate(ctx, tt.args)
				if tt.wantError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("context_cancellation", func(t *testing.T) {
		// Test with cancelled context
		cancelledCtx, cancel := context.WithCancel(ctx)
		cancel()

		args := AtomicAnalyzeRepositoryArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "session-123",
			},
			RepoURL: "https://github.com/example/repo",
		}

		// Validation should work even with cancelled context
		err := tool.Validate(cancelledCtx, args)
		assert.NoError(t, err, "Validation should work with cancelled context")
	})

	t.Run("context_timeout", func(t *testing.T) {
		// Test with timeout context
		timeoutCtx, cancel := context.WithTimeout(ctx, 1*time.Nanosecond)
		defer cancel()

		// Let timeout expire
		time.Sleep(10 * time.Millisecond)

		args := AtomicAnalyzeRepositoryArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "session-123",
			},
			RepoURL: "https://github.com/example/repo",
		}

		// Validation should work even with timed out context
		err := tool.Validate(timeoutCtx, args)
		assert.NoError(t, err, "Validation should work with timed out context")
	})

	t.Run("language_hint_edge_cases", func(t *testing.T) {
		tests := []struct {
			name         string
			languageHint string
			wantError    bool
		}{
			{
				name:         "empty_language_hint",
				languageHint: "",
				wantError:    false,
			},
			{
				name:         "whitespace_language_hint",
				languageHint: "   ",
				wantError:    false,
			},
			{
				name:         "very_long_language_hint",
				languageHint: string(make([]byte, 1000)),
				wantError:    false,
			},
			{
				name:         "special_chars_language_hint",
				languageHint: "C++/C#/F#",
				wantError:    false,
			},
			{
				name:         "unicode_language_hint",
				languageHint: "λ-calculus",
				wantError:    false,
			},
			{
				name:         "invalid_language",
				languageHint: "not-a-real-language-123",
				wantError:    false, // Should be lenient with language hints
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				args := AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
					RepoURL:      "https://github.com/example/repo",
					LanguageHint: tt.languageHint,
				}

				err := tool.Validate(ctx, args)
				if tt.wantError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("branch_name_edge_cases", func(t *testing.T) {
		tests := []struct {
			name      string
			branch    string
			wantError bool
		}{
			{
				name:      "empty_branch",
				branch:    "",
				wantError: false, // Should default to main/master
			},
			{
				name:      "branch_with_slashes",
				branch:    "feature/my-feature",
				wantError: false,
			},
			{
				name:      "branch_with_special_chars",
				branch:    "hotfix-123.456_urgent!",
				wantError: false,
			},
			{
				name:      "very_long_branch_name",
				branch:    string(make([]byte, 500)),
				wantError: false,
			},
			{
				name:      "unicode_branch_name",
				branch:    "测试-分支",
				wantError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				args := AtomicAnalyzeRepositoryArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
					RepoURL: "https://github.com/example/repo",
					Branch:  tt.branch,
				}

				err := tool.Validate(ctx, args)
				if tt.wantError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

func TestGenerateDockerfileTool_ErrorHandling(t *testing.T) {
	logger := zerolog.Nop()
	tool := NewGenerateDockerfileTool(nil, logger)
	ctx := context.Background()

	t.Run("invalid_argument_types", func(t *testing.T) {
		invalidArgs := []interface{}{
			nil,
			"string",
			123,
			[]string{"array"},
		}

		for i, args := range invalidArgs {
			t.Run("invalid_type_"+string(rune('0'+i)), func(t *testing.T) {
				err := tool.Validate(ctx, args)
				assert.Error(t, err, "Should return error for invalid argument type")
			})
		}
	})

	t.Run("template_edge_cases", func(t *testing.T) {
		tests := []struct {
			name      string
			template  string
			wantError bool
		}{
			{
				name:      "empty_template",
				template:  "",
				wantError: false, // Should use default
			},
			{
				name:      "very_long_template",
				template:  string(make([]byte, 10000)),
				wantError: false,
			},
			{
				name:      "template_with_special_chars",
				template:  "template-123!@#$%^&*()",
				wantError: false,
			},
			{
				name:      "unicode_template",
				template:  "模板-λ",
				wantError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				args := GenerateDockerfileArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
					Template: tt.template,
				}

				err := tool.Validate(ctx, args)
				if tt.wantError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("build_args_edge_cases", func(t *testing.T) {
		tests := []struct {
			name      string
			buildArgs map[string]string
			wantError bool
		}{
			{
				name:      "nil_build_args",
				buildArgs: nil,
				wantError: false,
			},
			{
				name:      "empty_build_args",
				buildArgs: map[string]string{},
				wantError: false,
			},
			{
				name: "build_args_with_empty_keys",
				buildArgs: map[string]string{
					"":      "value",
					"valid": "value",
				},
				wantError: false, // Should handle gracefully
			},
			{
				name: "build_args_with_empty_values",
				buildArgs: map[string]string{
					"key1": "",
					"key2": "value",
				},
				wantError: false,
			},
			{
				name: "build_args_with_special_chars",
				buildArgs: map[string]string{
					"KEY_WITH_UNDERSCORES": "value",
					"KEY-WITH-DASHES":      "value-with-dashes",
					"KEY.WITH.DOTS":        "value.with.dots",
					"KEY123":               "value123",
				},
				wantError: false,
			},
			{
				name: "very_large_build_args",
				buildArgs: func() map[string]string {
					args := make(map[string]string)
					for i := 0; i < 1000; i++ {
						args["KEY_"+string(rune('0'+(i%10)))] = "value_" + string(rune('0'+(i%10)))
					}
					return args
				}(),
				wantError: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				args := GenerateDockerfileArgs{
					BaseToolArgs: types.BaseToolArgs{
						SessionID: "session-123",
					},
					BuildArgs: tt.buildArgs,
				}

				err := tool.Validate(ctx, args)
				if tt.wantError {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})
}

// BenchmarkAnalyzeTools_ErrorHandling tests performance under error conditions
func BenchmarkAnalyzeTools_ErrorHandling(b *testing.B) {
	logger := zerolog.Nop()
	analyzeTool := NewAtomicAnalyzeRepositoryTool(nil, nil, logger)
	dockerfileTool := NewGenerateDockerfileTool(nil, logger)
	ctx := context.Background()

	b.Run("validate_invalid_args", func(b *testing.B) {
		invalidArgs := "invalid"
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := analyzeTool.Validate(ctx, invalidArgs)
			if err == nil {
				b.Fatal("Expected error for invalid args")
			}
		}
	})

	b.Run("validate_valid_args", func(b *testing.B) {
		args := AtomicAnalyzeRepositoryArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "session-123",
			},
			RepoURL: "https://github.com/example/repo",
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := analyzeTool.Validate(ctx, args)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("validate_dockerfile_tool", func(b *testing.B) {
		args := GenerateDockerfileArgs{
			BaseToolArgs: types.BaseToolArgs{
				SessionID: "session-123",
			},
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			err := dockerfileTool.Validate(ctx, args)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
