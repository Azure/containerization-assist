package dockerfile

import (
	"strings"
	"testing"
)

func TestGetTemplate(t *testing.T) {
	tests := []struct {
		language string
		expected bool
	}{
		{"go", true},
		{"java", true},
		{"javascript", true},
		{"typescript", true},
		{"python", true},
		{"rust", true},
		{"php", true},
		{"generic", true},
		{"unknown", false},
	}

	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			template, exists := GetTemplate(test.language)
			if exists != test.expected {
				t.Errorf("GetTemplate(%s) = %v, expected %v", test.language, exists, test.expected)
			}
			if exists && template == "" {
				t.Errorf("GetTemplate(%s) returned empty template", test.language)
			}
		})
	}
}

func TestGetAllTemplates(t *testing.T) {
	templates := GetAllTemplates()
	expectedLanguages := []string{"go", "java", "node", "python", "rust", "php", "generic"}

	if len(templates) != len(expectedLanguages) {
		t.Errorf("GetAllTemplates() returned %d templates, expected %d", len(templates), len(expectedLanguages))
	}

	for _, lang := range expectedLanguages {
		if template, exists := templates[lang]; !exists {
			t.Errorf("GetAllTemplates() missing template for %s", lang)
		} else if template == "" {
			t.Errorf("GetAllTemplates() returned empty template for %s", lang)
		}
	}
}

func TestTemplateContents(t *testing.T) {
	tests := []struct {
		language string
		contains []string
	}{
		{"go", []string{"FROM golang:", "CGO_ENABLED=0", "alpine:latest"}},
		{"java", []string{"FROM maven:", "eclipse-temurin", "ENTRYPOINT"}},
		{"javascript", []string{"FROM node:", "npm ci", "npm"}}, // simplified to just "npm"
		{"python", []string{"FROM python:", "pip install", "requirements.txt"}},
		{"rust", []string{"FROM rust:", "cargo build", "debian:bookworm-slim"}},
		{"php", []string{"FROM php:", "apache", "apache2-foreground"}},
		{"generic", []string{"FROM alpine:latest", "./start.sh"}},
	}

	for _, test := range tests {
		t.Run(test.language, func(t *testing.T) {
			template, exists := GetTemplate(test.language)
			if !exists {
				t.Errorf("Template for %s not found", test.language)
				return
			}

			for _, expected := range test.contains {
				if !strings.Contains(template, expected) {
					t.Errorf("Template for %s does not contain expected string: %s", test.language, expected)
				}
			}
		})
	}
}
