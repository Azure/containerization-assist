package analyze

import (
	"testing"

	"github.com/rs/zerolog"
)

// Test TemplateIntegration constructor
func TestNewTemplateIntegration(t *testing.T) {
	logger := zerolog.Nop()

	// Test constructor
	integration := NewTemplateIntegration(logger)
	if integration == nil {
		t.Error("NewTemplateIntegration should not return nil")
	}
}

// Test SelectDockerfileTemplate
func TestSelectDockerfileTemplate(t *testing.T) {
	logger := zerolog.Nop()
	integration := NewTemplateIntegration(logger)

	t.Run("with template name", func(t *testing.T) {
		repositoryData := map[string]interface{}{
			"language":  "go",
			"framework": "gin",
		}
		templateName := "go-gin"

		ctx, err := integration.SelectDockerfileTemplate(repositoryData, templateName)
		if err != nil {
			t.Errorf("SelectDockerfileTemplate should not return error: %v", err)
		}
		if ctx == nil {
			t.Error("SelectDockerfileTemplate should not return nil context")
		}
		if ctx.SelectedTemplate != templateName {
			t.Errorf("Expected SelectedTemplate to be '%s', got '%s'", templateName, ctx.SelectedTemplate)
		}
		if ctx.DetectedLanguage != "go" {
			t.Errorf("Expected DetectedLanguage to be 'go', got '%s'", ctx.DetectedLanguage)
		}
		if ctx.DetectedFramework != "gin" {
			t.Errorf("Expected DetectedFramework to be 'gin', got '%s'", ctx.DetectedFramework)
		}
		if ctx.SelectionMethod != "default" {
			t.Errorf("Expected SelectionMethod to be 'default', got '%s'", ctx.SelectionMethod)
		}
		if ctx.SelectionConfidence != 0.8 {
			t.Errorf("Expected SelectionConfidence to be 0.8, got %f", ctx.SelectionConfidence)
		}
	})

	t.Run("with empty template name", func(t *testing.T) {
		repositoryData := map[string]interface{}{
			"language": "python",
		}
		templateName := ""

		ctx, err := integration.SelectDockerfileTemplate(repositoryData, templateName)
		if err != nil {
			t.Errorf("SelectDockerfileTemplate should not return error: %v", err)
		}
		if ctx == nil {
			t.Error("SelectDockerfileTemplate should not return nil context")
		}
		if ctx.SelectedTemplate != "go" {
			t.Errorf("Expected SelectedTemplate to default to 'go', got '%s'", ctx.SelectedTemplate)
		}
	})

	t.Run("with nil repository data", func(t *testing.T) {
		templateName := "nodejs"

		ctx, err := integration.SelectDockerfileTemplate(nil, templateName)
		if err != nil {
			t.Errorf("SelectDockerfileTemplate should not return error with nil data: %v", err)
		}
		if ctx == nil {
			t.Error("SelectDockerfileTemplate should not return nil context with nil data")
		}
		if ctx.SelectedTemplate != templateName {
			t.Errorf("Expected SelectedTemplate to be '%s', got '%s'", templateName, ctx.SelectedTemplate)
		}
	})

	t.Run("with empty repository data", func(t *testing.T) {
		repositoryData := map[string]interface{}{}
		templateName := "python"

		ctx, err := integration.SelectDockerfileTemplate(repositoryData, templateName)
		if err != nil {
			t.Errorf("SelectDockerfileTemplate should not return error with empty data: %v", err)
		}
		if ctx.SelectedTemplate != templateName {
			t.Errorf("Expected SelectedTemplate to be '%s', got '%s'", templateName, ctx.SelectedTemplate)
		}
	})
}

// Test DockerfileTemplateContext structure
func TestDockerfileTemplateContext(t *testing.T) {
	ctx := &DockerfileTemplateContext{
		SelectedTemplate:     "go",
		DetectedLanguage:     "go",
		DetectedFramework:    "gin",
		SelectionMethod:      "auto",
		SelectionConfidence:  0.9,
		AvailableTemplates:   []TemplateOptionInternal{},
		AlternativeOptions:   []AlternativeTemplateOption{},
		SelectionReasoning:   []string{"High confidence", "Framework detected"},
		CustomizationOptions: map[string]interface{}{"port": 8080},
	}

	if ctx.SelectedTemplate != "go" {
		t.Errorf("Expected SelectedTemplate to be 'go', got '%s'", ctx.SelectedTemplate)
	}
	if ctx.DetectedLanguage != "go" {
		t.Errorf("Expected DetectedLanguage to be 'go', got '%s'", ctx.DetectedLanguage)
	}
	if ctx.DetectedFramework != "gin" {
		t.Errorf("Expected DetectedFramework to be 'gin', got '%s'", ctx.DetectedFramework)
	}
	if ctx.SelectionMethod != "auto" {
		t.Errorf("Expected SelectionMethod to be 'auto', got '%s'", ctx.SelectionMethod)
	}
	if ctx.SelectionConfidence != 0.9 {
		t.Errorf("Expected SelectionConfidence to be 0.9, got %f", ctx.SelectionConfidence)
	}
	if len(ctx.SelectionReasoning) != 2 {
		t.Errorf("Expected 2 selection reasons, got %d", len(ctx.SelectionReasoning))
	}
	if ctx.CustomizationOptions["port"] != 8080 {
		t.Errorf("Expected customization port to be 8080, got %v", ctx.CustomizationOptions["port"])
	}
}

// Test TemplateOptionInternal structure
func TestTemplateOptionInternal(t *testing.T) {
	option := TemplateOptionInternal{
		Name:        "go-gin",
		Description: "Go application with Gin framework",
		BestFor:     []string{"web services", "APIs"},
		Limitations: []string{"not suitable for large monoliths"},
		MatchScore:  0.85,
	}

	if option.Name != "go-gin" {
		t.Errorf("Expected Name to be 'go-gin', got '%s'", option.Name)
	}
	if option.Description != "Go application with Gin framework" {
		t.Errorf("Expected correct description, got '%s'", option.Description)
	}
	if len(option.BestFor) != 2 {
		t.Errorf("Expected 2 BestFor items, got %d", len(option.BestFor))
	}
	if option.BestFor[0] != "web services" {
		t.Errorf("Expected first BestFor to be 'web services', got '%s'", option.BestFor[0])
	}
	if len(option.Limitations) != 1 {
		t.Errorf("Expected 1 limitation, got %d", len(option.Limitations))
	}
	if option.MatchScore != 0.85 {
		t.Errorf("Expected MatchScore to be 0.85, got %f", option.MatchScore)
	}
}

// Test AlternativeTemplateOption structure
func TestAlternativeTemplateOption(t *testing.T) {
	alternative := AlternativeTemplateOption{
		Template:  "go-standard",
		Reason:    "More generic, wider compatibility",
		TradeOffs: []string{"less optimized", "larger size"},
		UseCases:  []string{"general applications", "batch jobs"},
	}

	if alternative.Template != "go-standard" {
		t.Errorf("Expected Template to be 'go-standard', got '%s'", alternative.Template)
	}
	if alternative.Reason != "More generic, wider compatibility" {
		t.Errorf("Expected correct reason, got '%s'", alternative.Reason)
	}
	if len(alternative.TradeOffs) != 2 {
		t.Errorf("Expected 2 trade-offs, got %d", len(alternative.TradeOffs))
	}
	if alternative.TradeOffs[0] != "less optimized" {
		t.Errorf("Expected first trade-off to be 'less optimized', got '%s'", alternative.TradeOffs[0])
	}
	if len(alternative.UseCases) != 2 {
		t.Errorf("Expected 2 use cases, got %d", len(alternative.UseCases))
	}
	if alternative.UseCases[0] != "general applications" {
		t.Errorf("Expected first use case to be 'general applications', got '%s'", alternative.UseCases[0])
	}
}