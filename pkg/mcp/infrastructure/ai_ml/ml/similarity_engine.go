// Package ml provides advanced similarity calculation for error patterns
package ml

import (
	"math"
	"strings"
	"unicode"
)

// SimilarityEngine provides advanced similarity calculation algorithms
type SimilarityEngine struct {
	// Configuration parameters
	weightExact    float64
	weightPartial  float64
	weightSemantic float64
	weightContext  float64
}

// NewSimilarityEngine creates a new similarity engine with default weights
func NewSimilarityEngine() *SimilarityEngine {
	return &SimilarityEngine{
		weightExact:    0.4,
		weightPartial:  0.3,
		weightSemantic: 0.2,
		weightContext:  0.1,
	}
}

// CalculateErrorSimilarity calculates similarity between two error messages
func (s *SimilarityEngine) CalculateErrorSimilarity(error1, error2 string) float64 {
	if error1 == error2 {
		return 1.0
	}

	// Normalize error messages
	norm1 := s.normalizeError(error1)
	norm2 := s.normalizeError(error2)

	// Calculate different types of similarity
	exactSim := s.calculateExactSimilarity(norm1, norm2)
	partialSim := s.calculatePartialSimilarity(norm1, norm2)
	semanticSim := s.calculateSemanticSimilarity(norm1, norm2)
	structuralSim := s.calculateStructuralSimilarity(norm1, norm2)

	// Weighted combination
	totalSimilarity := s.weightExact*exactSim +
		s.weightPartial*partialSim +
		s.weightSemantic*semanticSim +
		s.weightContext*structuralSim

	return math.Min(totalSimilarity, 1.0)
}

// CalculateStringSimilarity calculates similarity between two general strings
func (s *SimilarityEngine) CalculateStringSimilarity(str1, str2 string) float64 {
	if str1 == str2 {
		return 1.0
	}

	// Use Levenshtein distance for basic string similarity
	levenshtein := s.levenshteinDistance(str1, str2)
	maxLen := math.Max(float64(len(str1)), float64(len(str2)))

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - (float64(levenshtein) / maxLen)
}

// normalizeError normalizes error messages for comparison
func (s *SimilarityEngine) normalizeError(error string) string {
	// Convert to lowercase
	normalized := strings.ToLower(error)

	// Remove excessive whitespace
	normalized = strings.Join(strings.Fields(normalized), " ")

	// Remove common prefixes and suffixes
	prefixes := []string{"error:", "failed:", "exception:", "warning:"}
	suffixes := []string{".", "!", "?"}

	for _, prefix := range prefixes {
		if strings.HasPrefix(normalized, prefix) {
			normalized = strings.TrimPrefix(normalized, prefix)
			normalized = strings.TrimSpace(normalized)
			break
		}
	}

	for _, suffix := range suffixes {
		if strings.HasSuffix(normalized, suffix) {
			normalized = strings.TrimSuffix(normalized, suffix)
			normalized = strings.TrimSpace(normalized)
			break
		}
	}

	return normalized
}

// calculateExactSimilarity calculates exact string match similarity
func (s *SimilarityEngine) calculateExactSimilarity(str1, str2 string) float64 {
	if str1 == str2 {
		return 1.0
	}

	// Use Levenshtein distance
	distance := s.levenshteinDistance(str1, str2)
	maxLen := math.Max(float64(len(str1)), float64(len(str2)))

	if maxLen == 0 {
		return 1.0
	}

	return 1.0 - (float64(distance) / maxLen)
}

// calculatePartialSimilarity calculates partial match similarity
func (s *SimilarityEngine) calculatePartialSimilarity(str1, str2 string) float64 {
	words1 := strings.Fields(str1)
	words2 := strings.Fields(str2)

	if len(words1) == 0 || len(words2) == 0 {
		return 0.0
	}

	// Find common words
	commonWords := 0
	for _, word1 := range words1 {
		for _, word2 := range words2 {
			if word1 == word2 && len(word1) > 2 { // Only count words longer than 2 chars
				commonWords++
				break
			}
		}
	}

	// Calculate Jaccard similarity
	totalWords := len(words1) + len(words2) - commonWords
	if totalWords == 0 {
		return 1.0
	}

	return float64(commonWords) / float64(totalWords)
}

// calculateSemanticSimilarity calculates semantic similarity based on error types
func (s *SimilarityEngine) calculateSemanticSimilarity(str1, str2 string) float64 {
	// Define semantic groups of related terms
	semanticGroups := map[string][]string{
		"network":  {"connection", "timeout", "unreachable", "dns", "proxy", "tls", "ssl"},
		"docker":   {"build", "image", "container", "dockerfile", "registry", "push", "pull"},
		"k8s":      {"kubernetes", "kubectl", "pod", "deployment", "service", "namespace", "cluster"},
		"build":    {"compilation", "compile", "build", "dependency", "package", "module", "import"},
		"auth":     {"authentication", "authorization", "permission", "denied", "unauthorized", "credentials"},
		"resource": {"memory", "cpu", "disk", "quota", "limit", "resource", "capacity"},
	}

	// Find semantic categories for each string
	categories1 := s.findSemanticCategories(str1, semanticGroups)
	categories2 := s.findSemanticCategories(str2, semanticGroups)

	// Calculate category overlap
	if len(categories1) == 0 || len(categories2) == 0 {
		return 0.0
	}

	overlap := 0
	for cat1 := range categories1 {
		if _, exists := categories2[cat1]; exists {
			overlap++
		}
	}

	totalCategories := len(categories1) + len(categories2) - overlap
	if totalCategories == 0 {
		return 1.0
	}

	return float64(overlap) / float64(totalCategories)
}

// calculateStructuralSimilarity calculates structural similarity
func (s *SimilarityEngine) calculateStructuralSimilarity(str1, str2 string) float64 {
	// Extract structural features
	features1 := s.extractStructuralFeatures(str1)
	features2 := s.extractStructuralFeatures(str2)

	// Calculate feature similarity
	similarity := 0.0
	featureCount := 0

	// Length similarity
	lenSim := 1.0 - math.Abs(float64(features1.Length-features2.Length))/math.Max(float64(features1.Length), float64(features2.Length))
	similarity += lenSim
	featureCount++

	// Word count similarity
	wordSim := 1.0 - math.Abs(float64(features1.WordCount-features2.WordCount))/math.Max(float64(features1.WordCount), float64(features2.WordCount))
	similarity += wordSim
	featureCount++

	// Punctuation similarity
	punctSim := 1.0 - math.Abs(float64(features1.PunctuationCount-features2.PunctuationCount))/math.Max(float64(features1.PunctuationCount), float64(features2.PunctuationCount))
	similarity += punctSim
	featureCount++

	// Number similarity
	numSim := 1.0 - math.Abs(float64(features1.NumberCount-features2.NumberCount))/math.Max(float64(features1.NumberCount), float64(features2.NumberCount))
	similarity += numSim
	featureCount++

	if featureCount == 0 {
		return 0.0
	}

	return similarity / float64(featureCount)
}

// findSemanticCategories finds semantic categories for a string
func (s *SimilarityEngine) findSemanticCategories(str string, groups map[string][]string) map[string]bool {
	categories := make(map[string]bool)
	lowerStr := strings.ToLower(str)

	for category, terms := range groups {
		for _, term := range terms {
			if strings.Contains(lowerStr, term) {
				categories[category] = true
				break
			}
		}
	}

	return categories
}

// extractStructuralFeatures extracts structural features from a string
func (s *SimilarityEngine) extractStructuralFeatures(str string) StructuralFeatures {
	features := StructuralFeatures{}

	features.Length = len(str)
	features.WordCount = len(strings.Fields(str))

	for _, char := range str {
		if unicode.IsPunct(char) {
			features.PunctuationCount++
		} else if unicode.IsDigit(char) {
			features.NumberCount++
		}
	}

	return features
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func (s *SimilarityEngine) levenshteinDistance(str1, str2 string) int {
	if len(str1) == 0 {
		return len(str2)
	}
	if len(str2) == 0 {
		return len(str1)
	}

	// Create a matrix
	matrix := make([][]int, len(str1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(str2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(str1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(str2); j++ {
		matrix[0][j] = j
	}

	// Fill the matrix
	for i := 1; i <= len(str1); i++ {
		for j := 1; j <= len(str2); j++ {
			cost := 0
			if str1[i-1] != str2[j-1] {
				cost = 1
			}

			matrix[i][j] = min3(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(str1)][len(str2)]
}

// CalculateContextSimilarity calculates similarity between workflow contexts
func (s *SimilarityEngine) CalculateContextSimilarity(ctx1, ctx2 WorkflowContext) float64 {
	similarity := 0.0
	factors := 0

	// Step name similarity
	if ctx1.StepName == ctx2.StepName {
		similarity += 1.0
	}
	factors++

	// Language similarity
	if ctx1.Language != "" && ctx2.Language != "" {
		if ctx1.Language == ctx2.Language {
			similarity += 1.0
		}
		factors++
	}

	// Framework similarity
	if ctx1.Framework != "" && ctx2.Framework != "" {
		if ctx1.Framework == ctx2.Framework {
			similarity += 1.0
		}
		factors++
	}

	// Repository similarity (domain-based)
	if ctx1.RepoURL != "" && ctx2.RepoURL != "" {
		domain1 := s.extractDomain(ctx1.RepoURL)
		domain2 := s.extractDomain(ctx2.RepoURL)
		if domain1 == domain2 {
			similarity += 0.5
		}
		factors++
	}

	// Dependencies similarity
	if len(ctx1.Dependencies) > 0 && len(ctx2.Dependencies) > 0 {
		depSim := s.calculateDependencySimilarity(ctx1.Dependencies, ctx2.Dependencies)
		similarity += depSim
		factors++
	}

	if factors == 0 {
		return 0.0
	}

	return similarity / float64(factors)
}

// calculateDependencySimilarity calculates similarity between dependency lists
func (s *SimilarityEngine) calculateDependencySimilarity(deps1, deps2 []string) float64 {
	if len(deps1) == 0 || len(deps2) == 0 {
		return 0.0
	}

	// Convert to sets for intersection calculation
	set1 := make(map[string]bool)
	set2 := make(map[string]bool)

	for _, dep := range deps1 {
		set1[dep] = true
	}
	for _, dep := range deps2 {
		set2[dep] = true
	}

	// Calculate intersection
	intersection := 0
	for dep := range set1 {
		if set2[dep] {
			intersection++
		}
	}

	// Calculate union
	union := len(set1) + len(set2) - intersection

	if union == 0 {
		return 1.0
	}

	return float64(intersection) / float64(union)
}

// extractDomain extracts domain from a URL
func (s *SimilarityEngine) extractDomain(url string) string {
	// Simple domain extraction
	if strings.Contains(url, "://") {
		parts := strings.Split(url, "://")
		if len(parts) > 1 {
			domainPart := parts[1]
			if strings.Contains(domainPart, "/") {
				domainPart = strings.Split(domainPart, "/")[0]
			}
			return domainPart
		}
	}
	return url
}

// GetSimilarityMetrics returns metrics about similarity calculations
func (s *SimilarityEngine) GetSimilarityMetrics() SimilarityMetrics {
	return SimilarityMetrics{
		ExactWeight:    s.weightExact,
		PartialWeight:  s.weightPartial,
		SemanticWeight: s.weightSemantic,
		ContextWeight:  s.weightContext,
	}
}

// UpdateWeights updates similarity calculation weights
func (s *SimilarityEngine) UpdateWeights(exact, partial, semantic, context float64) {
	total := exact + partial + semantic + context
	if total > 0 {
		s.weightExact = exact / total
		s.weightPartial = partial / total
		s.weightSemantic = semantic / total
		s.weightContext = context / total
	}
}

// Supporting types

// StructuralFeatures represents structural features of a string
type StructuralFeatures struct {
	Length           int `json:"length"`
	WordCount        int `json:"word_count"`
	PunctuationCount int `json:"punctuation_count"`
	NumberCount      int `json:"number_count"`
}

// SimilarityMetrics provides metrics about similarity calculations
type SimilarityMetrics struct {
	ExactWeight    float64 `json:"exact_weight"`
	PartialWeight  float64 `json:"partial_weight"`
	SemanticWeight float64 `json:"semantic_weight"`
	ContextWeight  float64 `json:"context_weight"`
}

// Helper function for min of three values
func min3(a, b, c int) int {
	if a <= b && a <= c {
		return a
	} else if b <= c {
		return b
	}
	return c
}
