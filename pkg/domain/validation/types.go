package validation

import "encoding/json"

// Severity indicates the importance of a finding
type Severity string

const (
	SeverityError Severity = "error"
	SeverityWarn  Severity = "warn"
	SeverityInfo  Severity = "info"
)

// Finding represents a single validation issue
type Finding struct {
	Code     string   `json:"code"`           // e.g., DF001, K8S001
	Severity Severity `json:"severity"`       // error|warn|info
	Path     string   `json:"path,omitempty"` // location hint (e.g., "Line 1", "spec.containers[0]")
	Message  string   `json:"message"`        // human-readable description
}

// Result is the uniform validation contract for ALL validate_* tools
type Result struct {
	IsValid      bool                   `json:"is_valid"`
	Findings     []Finding              `json:"findings,omitempty"`
	QualityScore int                    `json:"quality_score"`      // 0-100
	Stats        map[string]interface{} `json:"stats,omitempty"`    // validation-specific stats
	Metadata     map[string]interface{} `json:"metadata,omitempty"` // additional context
}

// NewResult creates a new validation result with defaults
func NewResult() *Result {
	return &Result{
		IsValid:      true,
		Findings:     []Finding{},
		QualityScore: 100,
		Stats:        make(map[string]interface{}),
		Metadata:     make(map[string]interface{}),
	}
}

// AddError adds an error finding and marks the result as invalid
func (r *Result) AddError(code, path, message string) {
	r.Findings = append(r.Findings, Finding{
		Code:     code,
		Severity: SeverityError,
		Path:     path,
		Message:  message,
	})
	r.IsValid = false
}

// AddWarning adds a warning finding
func (r *Result) AddWarning(code, path, message string) {
	r.Findings = append(r.Findings, Finding{
		Code:     code,
		Severity: SeverityWarn,
		Path:     path,
		Message:  message,
	})
}

// AddInfo adds an informational finding
func (r *Result) AddInfo(code, path, message string) {
	r.Findings = append(r.Findings, Finding{
		Code:     code,
		Severity: SeverityInfo,
		Path:     path,
		Message:  message,
	})
}

// ErrorCount returns the number of error-level findings
func (r *Result) ErrorCount() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityError {
			n++
		}
	}
	return n
}

// WarningCount returns the number of warning-level findings
func (r *Result) WarningCount() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityWarn {
			n++
		}
	}
	return n
}

// InfoCount returns the number of info-level findings
func (r *Result) InfoCount() int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == SeverityInfo {
			n++
		}
	}
	return n
}

// CalculateQualityScore computes a quality score based on findings
func (r *Result) CalculateQualityScore() {
	score := 100

	// Deduct points for errors (20 points each)
	score -= r.ErrorCount() * 20

	// Deduct points for warnings (5 points each)
	score -= r.WarningCount() * 5

	// Deduct points for info (1 point each)
	score -= r.InfoCount() * 1

	// Ensure score doesn't go below 0
	if score < 0 {
		score = 0
	}

	r.QualityScore = score
}

// ToJSON serializes the result to JSON
func (r *Result) ToJSON() (string, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// ToJSONPretty serializes the result to pretty-printed JSON
func (r *Result) ToJSONPretty() (string, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// FromJSON deserializes a validation result from JSON
func FromJSON(data []byte) (*Result, error) {
	var result Result
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// MergeResults combines multiple validation results into one
func MergeResults(results ...*Result) *Result {
	merged := NewResult()

	for _, r := range results {
		if r == nil {
			continue
		}

		// Merge findings
		merged.Findings = append(merged.Findings, r.Findings...)

		// Update validity (invalid if any result is invalid)
		if !r.IsValid {
			merged.IsValid = false
		}

		// Merge stats (last one wins for conflicts)
		for k, v := range r.Stats {
			merged.Stats[k] = v
		}

		// Merge metadata (last one wins for conflicts)
		for k, v := range r.Metadata {
			merged.Metadata[k] = v
		}
	}

	// Recalculate quality score for merged result
	merged.CalculateQualityScore()

	return merged
}
