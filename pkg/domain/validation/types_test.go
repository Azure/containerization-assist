package validation

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewResult(t *testing.T) {
	r := NewResult()

	assert.True(t, r.IsValid)
	assert.Empty(t, r.Findings)
	assert.Equal(t, 100, r.QualityScore)
	assert.NotNil(t, r.Stats)
	assert.NotNil(t, r.Metadata)
}

func TestAddError(t *testing.T) {
	r := NewResult()

	r.AddError("DF001", "Line 1", "Missing FROM instruction")

	assert.False(t, r.IsValid)
	assert.Len(t, r.Findings, 1)
	assert.Equal(t, "DF001", r.Findings[0].Code)
	assert.Equal(t, SeverityError, r.Findings[0].Severity)
	assert.Equal(t, "Line 1", r.Findings[0].Path)
	assert.Equal(t, "Missing FROM instruction", r.Findings[0].Message)
}

func TestAddWarning(t *testing.T) {
	r := NewResult()

	r.AddWarning("DF010", "Dockerfile", "Missing HEALTHCHECK instruction")

	assert.True(t, r.IsValid) // Warnings don't invalidate
	assert.Len(t, r.Findings, 1)
	assert.Equal(t, SeverityWarn, r.Findings[0].Severity)
}

func TestAddInfo(t *testing.T) {
	r := NewResult()

	r.AddInfo("DF020", "Line 5", "Consider using multi-stage build")

	assert.True(t, r.IsValid) // Info doesn't invalidate
	assert.Len(t, r.Findings, 1)
	assert.Equal(t, SeverityInfo, r.Findings[0].Severity)
}

func TestCountMethods(t *testing.T) {
	r := NewResult()

	r.AddError("E1", "", "Error 1")
	r.AddError("E2", "", "Error 2")
	r.AddWarning("W1", "", "Warning 1")
	r.AddInfo("I1", "", "Info 1")
	r.AddInfo("I2", "", "Info 2")
	r.AddInfo("I3", "", "Info 3")

	assert.Equal(t, 2, r.ErrorCount())
	assert.Equal(t, 1, r.WarningCount())
	assert.Equal(t, 3, r.InfoCount())
}

func TestCalculateQualityScore(t *testing.T) {
	tests := []struct {
		name          string
		setupResult   func() *Result
		expectedScore int
	}{
		{
			name: "perfect score",
			setupResult: func() *Result {
				return NewResult()
			},
			expectedScore: 100,
		},
		{
			name: "with errors",
			setupResult: func() *Result {
				r := NewResult()
				r.AddError("E1", "", "Error 1")
				r.AddError("E2", "", "Error 2")
				r.CalculateQualityScore()
				return r
			},
			expectedScore: 60, // 100 - (2 * 20)
		},
		{
			name: "with warnings",
			setupResult: func() *Result {
				r := NewResult()
				r.AddWarning("W1", "", "Warning 1")
				r.AddWarning("W2", "", "Warning 2")
				r.CalculateQualityScore()
				return r
			},
			expectedScore: 90, // 100 - (2 * 5)
		},
		{
			name: "mixed findings",
			setupResult: func() *Result {
				r := NewResult()
				r.AddError("E1", "", "Error")     // -20
				r.AddWarning("W1", "", "Warning") // -5
				r.AddInfo("I1", "", "Info")       // -1
				r.CalculateQualityScore()
				return r
			},
			expectedScore: 74, // 100 - 20 - 5 - 1
		},
		{
			name: "score floor at zero",
			setupResult: func() *Result {
				r := NewResult()
				for i := 0; i < 10; i++ {
					r.AddError("E", "", "Error")
				}
				r.CalculateQualityScore()
				return r
			},
			expectedScore: 0, // Would be -100, but floored at 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := tt.setupResult()
			assert.Equal(t, tt.expectedScore, r.QualityScore)
		})
	}
}

func TestJSONSerialization(t *testing.T) {
	r := NewResult()
	r.AddError("DF001", "Line 1", "Missing FROM instruction")
	r.AddWarning("DF010", "Dockerfile", "Missing HEALTHCHECK")
	r.Stats["lines"] = 50
	r.Metadata["validated_by"] = "dockerfile-validator"
	r.CalculateQualityScore()

	// Test ToJSON
	jsonStr, err := r.ToJSON()
	require.NoError(t, err)

	// Verify JSON structure
	var data map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &data)
	require.NoError(t, err)

	assert.False(t, data["is_valid"].(bool))
	assert.Equal(t, float64(75), data["quality_score"].(float64)) // 100 - 20 - 5

	findings := data["findings"].([]interface{})
	assert.Len(t, findings, 2)

	// Test FromJSON
	r2, err := FromJSON([]byte(jsonStr))
	require.NoError(t, err)

	assert.Equal(t, r.IsValid, r2.IsValid)
	assert.Equal(t, r.QualityScore, r2.QualityScore)
	assert.Len(t, r2.Findings, 2)
	assert.Equal(t, float64(50), r2.Stats["lines"]) // JSON unmarshals numbers as float64
}

func TestToJSONPretty(t *testing.T) {
	r := NewResult()
	r.AddError("DF001", "Line 1", "Missing FROM instruction")

	jsonStr, err := r.ToJSONPretty()
	require.NoError(t, err)

	// Check that it's pretty-printed (contains newlines and spaces)
	assert.Contains(t, jsonStr, "\n")
	assert.Contains(t, jsonStr, "  ")
}

func TestMergeResults(t *testing.T) {
	r1 := NewResult()
	r1.AddError("E1", "", "Error 1")
	r1.Stats["check1"] = "passed"
	r1.Metadata["source"] = "validator1"

	r2 := NewResult()
	r2.AddWarning("W1", "", "Warning 1")
	r2.Stats["check2"] = "passed"
	r2.Metadata["source"] = "validator2" // Overwrites r1's source

	r3 := NewResult()
	r3.AddInfo("I1", "", "Info 1")
	r3.Stats["check3"] = "passed"

	merged := MergeResults(r1, r2, r3)

	assert.False(t, merged.IsValid) // r1 had an error
	assert.Len(t, merged.Findings, 3)
	assert.Equal(t, 1, merged.ErrorCount())
	assert.Equal(t, 1, merged.WarningCount())
	assert.Equal(t, 1, merged.InfoCount())

	// Check merged stats
	assert.Equal(t, "passed", merged.Stats["check1"])
	assert.Equal(t, "passed", merged.Stats["check2"])
	assert.Equal(t, "passed", merged.Stats["check3"])

	// Check metadata (last one wins)
	assert.Equal(t, "validator2", merged.Metadata["source"])

	// Check quality score
	assert.Equal(t, 74, merged.QualityScore) // 100 - 20 - 5 - 1
}

func TestMergeResultsWithNil(t *testing.T) {
	r1 := NewResult()
	r1.AddError("E1", "", "Error 1")

	merged := MergeResults(r1, nil, nil)

	assert.False(t, merged.IsValid)
	assert.Len(t, merged.Findings, 1)
}

func TestUniformContract(t *testing.T) {
	// This test verifies the uniform contract structure
	r := NewResult()
	r.AddError("DF001", "Line 1", "Missing FROM instruction")
	r.AddWarning("DF010", "Dockerfile", "Missing HEALTHCHECK instruction")
	r.Stats["syntax_valid"] = false
	r.Stats["best_practices"] = true
	r.CalculateQualityScore()

	jsonStr, err := r.ToJSON()
	require.NoError(t, err)

	// Parse and verify exact structure
	var contract map[string]interface{}
	err = json.Unmarshal([]byte(jsonStr), &contract)
	require.NoError(t, err)

	// Verify required fields exist
	_, hasIsValid := contract["is_valid"]
	_, hasFindings := contract["findings"]
	_, hasQualityScore := contract["quality_score"]

	assert.True(t, hasIsValid, "Contract must have 'is_valid' field")
	assert.True(t, hasFindings, "Contract must have 'findings' field")
	assert.True(t, hasQualityScore, "Contract must have 'quality_score' field")

	// Verify findings structure
	findings := contract["findings"].([]interface{})
	if len(findings) > 0 {
		finding := findings[0].(map[string]interface{})

		_, hasCode := finding["code"]
		_, hasSeverity := finding["severity"]
		_, hasMessage := finding["message"]

		assert.True(t, hasCode, "Finding must have 'code' field")
		assert.True(t, hasSeverity, "Finding must have 'severity' field")
		assert.True(t, hasMessage, "Finding must have 'message' field")
	}
}
