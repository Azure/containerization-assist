package migration

import (
	"crypto/md5"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/infra/logging"
)

// NewPatternAnalyzer creates a new pattern analyzer
func NewPatternAnalyzer(config PatternAnalysisConfig, logger logging.Standards) *PatternAnalyzer {
	return &PatternAnalyzer{
		logger:  logger.WithComponent("pattern_analyzer"),
		config:  config,
		fileSet: token.NewFileSet(),
		statistics: PatternStatistics{
			PatternsDetected: make(map[string]int),
		},
	}
}

// AnalyzePatterns performs comprehensive pattern analysis
func (pa *PatternAnalyzer) AnalyzePatterns(rootPath string) (*PatternAnalysisResult, error) {
	pa.logger.Info().Str("path", rootPath).Msg("Starting pattern analysis")

	startTime := time.Now()
	result := &PatternAnalysisResult{
		ComplexityHotspots: []ComplexityHotspot{},
		DuplicationGroups:  []DuplicationGroup{},
		AntiPatterns:       []AntiPatternDetection{},
	}

	// Analyze complexity across all files
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.Contains(path, "vendor/") {
			return nil
		}

		pa.statistics.TotalFiles++

		file, err := parser.ParseFile(pa.fileSet, path, nil, parser.ParseComments)
		if err != nil {
			pa.logger.Warn().Err(err).Str("file", path).Msg("Failed to parse file")
			return nil
		}

		pa.statistics.FilesAnalyzed++

		// Analyze complexity
		if pa.config.EnableComplexityAnalysis {
			hotspots := pa.analyzeComplexity(file, path)
			result.ComplexityHotspots = append(result.ComplexityHotspots, hotspots...)
		}

		// Detect anti-patterns
		if pa.config.EnableAntiPatternDetection {
			antiPatterns := pa.detectAntiPatterns(file, path)
			result.AntiPatterns = append(result.AntiPatterns, antiPatterns...)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Detect duplications across all files
	if pa.config.EnableDuplicationDetection {
		result.DuplicationGroups = pa.detectDuplications(rootPath)
	}

	// Calculate metrics
	result.Metrics = pa.calculateCodeMetrics(result)

	pa.statistics.DetectionTime = time.Since(startTime)
	pa.statistics.TotalDetections = len(result.ComplexityHotspots) +
		len(result.DuplicationGroups) + len(result.AntiPatterns)

	pa.logger.Info().
		Int("hotspots", len(result.ComplexityHotspots)).
		Int("duplications", len(result.DuplicationGroups)).
		Int("anti_patterns", len(result.AntiPatterns)).
		Str("duration", pa.statistics.DetectionTime.String()).
		Msg("Pattern analysis completed")

	return result, nil
}

// analyzeComplexity finds complexity hotspots in a file
func (pa *PatternAnalyzer) analyzeComplexity(file *ast.File, filePath string) []ComplexityHotspot {
	var hotspots []ComplexityHotspot

	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			complexity := pa.calculateComplexity(fn)
			if complexity > pa.config.ComplexityThreshold {
				loc := pa.countLinesOfCode(fn)
				pos := pa.fileSet.Position(fn.Pos())

				hotspot := ComplexityHotspot{
					File:           filePath,
					Function:       fn.Name.Name,
					Complexity:     complexity,
					LinesOfCode:    loc,
					Position:       pos,
					Recommendation: pa.generateComplexityRecommendation(complexity, loc),
				}

				hotspots = append(hotspots, hotspot)
				pa.statistics.PatternsDetected["high_complexity"]++
			}
		}
		return true
	})

	return hotspots
}

// detectAntiPatterns detects common anti-patterns
func (pa *PatternAnalyzer) detectAntiPatterns(file *ast.File, filePath string) []AntiPatternDetection {
	var detections []AntiPatternDetection

	// God struct detection
	ast.Inspect(file, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if structType, ok := typeSpec.Type.(*ast.StructType); ok {
				fieldCount := len(structType.Fields.List)
				if fieldCount > 20 {
					pos := pa.fileSet.Position(typeSpec.Pos())
					detections = append(detections, AntiPatternDetection{
						Type:        "god_struct",
						File:        filePath,
						Position:    pos,
						Description: fmt.Sprintf("Struct '%s' has %d fields", typeSpec.Name.Name, fieldCount),
						Severity:    "HIGH",
						Suggestion:  "Consider breaking down this struct into smaller, focused components",
						Context: map[string]interface{}{
							"struct_name": typeSpec.Name.Name,
							"field_count": fieldCount,
						},
					})
					pa.statistics.PatternsDetected["god_struct"]++
				}
			}
		}
		return true
	})

	// Long parameter list detection
	ast.Inspect(file, func(n ast.Node) bool {
		if fn, ok := n.(*ast.FuncDecl); ok {
			paramCount := 0
			if fn.Type.Params != nil {
				paramCount = len(fn.Type.Params.List)
			}
			if paramCount > 5 {
				pos := pa.fileSet.Position(fn.Pos())
				detections = append(detections, AntiPatternDetection{
					Type:        "long_parameter_list",
					File:        filePath,
					Position:    pos,
					Description: fmt.Sprintf("Function '%s' has %d parameters", fn.Name.Name, paramCount),
					Severity:    "MEDIUM",
					Suggestion:  "Consider using a struct or functional options pattern",
					Context: map[string]interface{}{
						"function_name": fn.Name.Name,
						"param_count":   paramCount,
					},
				})
				pa.statistics.PatternsDetected["long_parameter_list"]++
			}
		}
		return true
	})

	// Empty catch blocks
	ast.Inspect(file, func(n ast.Node) bool {
		if ifStmt, ok := n.(*ast.IfStmt); ok {
			// Check for error handling that does nothing
			if pa.isEmptyErrorHandling(ifStmt) {
				pos := pa.fileSet.Position(ifStmt.Pos())
				detections = append(detections, AntiPatternDetection{
					Type:        "empty_error_handling",
					File:        filePath,
					Position:    pos,
					Description: "Empty error handling block detected",
					Severity:    "HIGH",
					Suggestion:  "Handle errors appropriately or document why they're ignored",
					Context:     map[string]interface{}{},
				})
				pa.statistics.PatternsDetected["empty_error_handling"]++
			}
		}
		return true
	})

	return detections
}

// detectDuplications finds duplicated code blocks
func (pa *PatternAnalyzer) detectDuplications(rootPath string) []DuplicationGroup {
	// Simple duplication detection based on function signatures
	// In a real implementation, this would use more sophisticated algorithms

	functionHashes := make(map[string][]DuplicationInstance)

	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}

		file, err := parser.ParseFile(pa.fileSet, path, nil, 0)
		if err != nil {
			return nil
		}

		ast.Inspect(file, func(n ast.Node) bool {
			if fn, ok := n.(*ast.FuncDecl); ok {
				hash := pa.hashFunction(fn)
				pos := pa.fileSet.Position(fn.Pos())
				endPos := pa.fileSet.Position(fn.End())

				instance := DuplicationInstance{
					File:      path,
					StartLine: pos.Line,
					EndLine:   endPos.Line,
					CodeHash:  hash,
				}

				functionHashes[hash] = append(functionHashes[hash], instance)
			}
			return true
		})

		return nil
	})

	// Group duplications
	var groups []DuplicationGroup
	groupID := 1

	for _, instances := range functionHashes {
		if len(instances) > 1 {
			group := DuplicationGroup{
				ID:          fmt.Sprintf("DUP-%03d", groupID),
				Instances:   instances,
				LineCount:   instances[0].EndLine - instances[0].StartLine + 1,
				Similarity:  1.0, // Exact match in this simple implementation
				ImpactScore: float64(len(instances)) * float64(instances[0].EndLine-instances[0].StartLine),
			}
			groups = append(groups, group)
			groupID++

			pa.statistics.PatternsDetected["code_duplication"]++
		}
	}

	return groups
}

// calculateCodeMetrics calculates overall code metrics
func (pa *PatternAnalyzer) calculateCodeMetrics(result *PatternAnalysisResult) CodeMetrics {
	metrics := CodeMetrics{}

	// Calculate from complexity hotspots
	if len(result.ComplexityHotspots) > 0 {
		metrics.AverageComplexity = pa.calculateAverageComplexity(result.ComplexityHotspots)
		metrics.MaxComplexity = pa.findMaxComplexity(result.ComplexityHotspots)
	}

	// Calculate duplication ratio
	if pa.statistics.TotalFiles > 0 {
		filesWithDuplication := make(map[string]bool)
		for _, group := range result.DuplicationGroups {
			for _, instance := range group.Instances {
				filesWithDuplication[instance.File] = true
			}
		}
		metrics.DuplicationRatio = float64(len(filesWithDuplication)) / float64(pa.statistics.TotalFiles)
	}

	// Calculate technical debt score (simplified)
	metrics.TechnicalDebtScore = pa.calculateTechnicalDebtScore(result)

	metrics.TotalLines = pa.statistics.TotalFiles * 200 // Rough estimate
	metrics.TotalFunctions = len(result.ComplexityHotspots)

	return metrics
}

// Helper methods

func (pa *PatternAnalyzer) calculateComplexity(fn *ast.FuncDecl) int {
	complexity := 1
	ast.Inspect(fn, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.IfStmt:
			complexity++
		case *ast.ForStmt, *ast.RangeStmt:
			complexity++
		case *ast.SwitchStmt, *ast.TypeSwitchStmt:
			complexity++
		case *ast.CaseClause:
			complexity++
		}
		return true
	})
	return complexity
}

func (pa *PatternAnalyzer) countLinesOfCode(fn *ast.FuncDecl) int {
	start := pa.fileSet.Position(fn.Pos()).Line
	end := pa.fileSet.Position(fn.End()).Line
	return end - start + 1
}

func (pa *PatternAnalyzer) generateComplexityRecommendation(complexity, loc int) string {
	if complexity > 20 {
		return "Critical complexity - urgent refactoring needed"
	} else if complexity > 15 {
		return "High complexity - consider breaking down this function"
	} else if loc > 50 {
		return "Long function - consider extracting helper functions"
	}
	return "Moderate complexity - review for possible simplification"
}

func (pa *PatternAnalyzer) isEmptyErrorHandling(ifStmt *ast.IfStmt) bool {
	// Check if this is an error check
	if !pa.isErrorCheck(ifStmt.Cond) {
		return false
	}

	// Check if the body is empty or just returns
	if ifStmt.Body == nil || len(ifStmt.Body.List) == 0 {
		return true
	}

	// Check for minimal handling (just return)
	if len(ifStmt.Body.List) == 1 {
		if _, ok := ifStmt.Body.List[0].(*ast.ReturnStmt); ok {
			return true
		}
	}

	return false
}

func (pa *PatternAnalyzer) isErrorCheck(expr ast.Expr) bool {
	if binExpr, ok := expr.(*ast.BinaryExpr); ok {
		if binExpr.Op == token.NEQ {
			if ident, ok := binExpr.X.(*ast.Ident); ok && ident.Name == "err" {
				if ident, ok := binExpr.Y.(*ast.Ident); ok && ident.Name == "nil" {
					return true
				}
			}
		}
	}
	return false
}

func (pa *PatternAnalyzer) hashFunction(fn *ast.FuncDecl) string {
	// Simple hash based on function signature
	// In production, would use AST comparison
	sig := fn.Name.Name
	if fn.Type.Params != nil {
		sig += fmt.Sprintf("_%d_params", len(fn.Type.Params.List))
	}
	if fn.Type.Results != nil {
		sig += fmt.Sprintf("_%d_results", len(fn.Type.Results.List))
	}

	h := md5.Sum([]byte(sig))
	return fmt.Sprintf("%x", h)[:8]
}

func (pa *PatternAnalyzer) calculateAverageComplexity(hotspots []ComplexityHotspot) float64 {
	if len(hotspots) == 0 {
		return 0
	}

	total := 0
	for _, hotspot := range hotspots {
		total += hotspot.Complexity
	}

	return float64(total) / float64(len(hotspots))
}

func (pa *PatternAnalyzer) findMaxComplexity(hotspots []ComplexityHotspot) int {
	maxComplexity := 0
	for _, hotspot := range hotspots {
		if hotspot.Complexity > maxComplexity {
			maxComplexity = hotspot.Complexity
		}
	}
	return maxComplexity
}

func (pa *PatternAnalyzer) calculateTechnicalDebtScore(result *PatternAnalysisResult) float64 {
	score := 0.0

	// Weight different factors
	score += float64(len(result.ComplexityHotspots)) * 2.0
	score += float64(len(result.DuplicationGroups)) * 3.0
	score += float64(len(result.AntiPatterns)) * 2.5

	// Normalize to 0-100 scale
	if score > 100 {
		score = 100
	}

	return score
}
