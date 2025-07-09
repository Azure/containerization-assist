// Package migration provides tools for analyzing and detecting migration opportunities
package migration

// This file serves as the main entry point for the migration analysis functionality.
// The implementation has been split into focused modules:
//
// - analysis_types.go: Type definitions for migration analysis
// - analysis_patterns.go: Pattern detection and analysis
// - analysis_structure.go: Structural code analysis
// - analysis_detector.go: Main detection logic and report generation
// - analysis_patterns_analyzer.go: Advanced pattern analysis and metrics
//
// The migration detector helps identify opportunities for code improvement including:
// - Interface segregation violations
// - High cyclomatic complexity
// - Code duplication
// - Anti-patterns
// - Error handling issues
// - Naming convention violations
//
// Usage:
//   config := Config{
//       EnablePatternDetection: true,
//       EnableStructuralAnalysis: true,
//   }
//   detector := NewDetector(config, logger)
//   report, err := detector.DetectMigrations("./pkg")
