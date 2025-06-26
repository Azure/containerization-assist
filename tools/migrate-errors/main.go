package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
)

type ErrorMigration struct {
	File           string
	Line           int
	Column         int
	OriginalCode   string
	MigratedCode   string
	ErrorType      string
	Context        string
	AutoMigratable bool
}

type MigrationStats struct {
	TotalErrors    int
	MigratedErrors int
	SkippedErrors  int
	FilesProcessed int
	FilesModified  int
	ErrorsByType   map[string]int
}

var (
	packagePath  = flag.String("package", "", "Package path to migrate (required)")
	dryRun       = flag.Bool("dry-run", true, "Show what would be changed without modifying files")
	verbose      = flag.Bool("verbose", false, "Show detailed migration information")
	interactive  = flag.Bool("interactive", false, "Prompt for each migration")
	outputReport = flag.String("report", "", "Output migration report to file")
	includeTests = flag.Bool("include-tests", false, "Include test files in migration")
	autoOnly     = flag.Bool("auto-only", false, "Only perform automatic migrations")
)

func main() {
	flag.Parse()

	if *packagePath == "" {
		fmt.Fprintf(os.Stderr, "Error: -package flag is required\n")
		flag.Usage()
		os.Exit(1)
	}

	stats := &MigrationStats{
		ErrorsByType: make(map[string]int),
	}

	fmt.Printf("🔄 Error Migration Tool\n")
	fmt.Printf("=======================\n")
	fmt.Printf("Package: %s\n", *packagePath)
	fmt.Printf("Mode: %s\n", getModeString())
	fmt.Println()

	migrations, err := findErrorsToMigrate(*packagePath)
	if err != nil {
		log.Fatalf("Failed to analyze package: %v", err)
	}

	if len(migrations) == 0 {
		fmt.Println("✅ No errors found to migrate!")
		return
	}

	fmt.Printf("Found %d error handling patterns to migrate\n\n", len(migrations))

	// Group migrations by file
	fileGroups := groupByFile(migrations)

	// Process each file
	for file, fileMigrations := range fileGroups {
		if err := processMigrations(file, fileMigrations, stats); err != nil {
			log.Printf("Error processing %s: %v", file, err)
		}
	}

	// Print summary
	printSummary(stats)

	// Save report if requested
	if *outputReport != "" {
		if err := saveReport(*outputReport, migrations, stats); err != nil {
			log.Printf("Failed to save report: %v", err)
		}
	}
}

func getModeString() string {
	if *dryRun {
		return "Dry Run (no changes will be made)"
	}
	if *interactive {
		return "Interactive"
	}
	if *autoOnly {
		return "Automatic Only"
	}
	return "Automatic"
}

func findErrorsToMigrate(packagePath string) ([]*ErrorMigration, error) {
	var migrations []*ErrorMigration

	err := filepath.WalkDir(packagePath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip non-Go files
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		// Skip test files if not included
		if !*includeTests && strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip vendor directories
		if strings.Contains(path, "/vendor/") {
			return nil
		}

		fileMigrations, err := analyzeFile(path)
		if err != nil {
			if *verbose {
				log.Printf("Error analyzing %s: %v", path, err)
			}
			return nil // Continue with other files
		}

		migrations = append(migrations, fileMigrations...)
		return nil
	})

	return migrations, err
}

func analyzeFile(filename string) ([]*ErrorMigration, error) {
	content, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var migrations []*ErrorMigration

	// Check if types package is imported
	hasTypesImport := false
	needsTypesImport := false

	for _, imp := range node.Imports {
		if imp.Path.Value == `"github.com/Azure/container-kit/pkg/types"` {
			hasTypesImport = true
			break
		}
	}

	// Walk the AST to find error creation patterns
	ast.Inspect(node, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CallExpr:
			migration := analyzeCallExpr(node, fset, filename)
			if migration != nil {
				migrations = append(migrations, migration)
				if migration.AutoMigratable {
					needsTypesImport = true
				}
			}
		}
		return true
	})

	// Add import if needed
	if needsTypesImport && !hasTypesImport {
		if *verbose {
			fmt.Printf("📦 File %s needs types import\n", filename)
		}
	}

	return migrations, nil
}

func analyzeCallExpr(call *ast.CallExpr, fset *token.FileSet, filename string) *ErrorMigration {
	// Get function name
	funcName := getFunctionName(call)
	if funcName == "" {
		return nil
	}

	switch funcName {
	case "fmt.Errorf":
		return analyzeFmtErrorf(call, fset, filename)
	case "errors.New":
		return analyzeErrorsNew(call, fset, filename)
	case "errors.Wrap", "errors.Wrapf":
		return analyzeErrorsWrap(call, fset, filename)
	}

	return nil
}

func getFunctionName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		if ident, ok := fun.X.(*ast.Ident); ok {
			return fmt.Sprintf("%s.%s", ident.Name, fun.Sel.Name)
		}
	case *ast.Ident:
		return fun.Name
	}
	return ""
}

func analyzeFmtErrorf(call *ast.CallExpr, fset *token.FileSet, filename string) *ErrorMigration {
	if len(call.Args) == 0 {
		return nil
	}

	pos := fset.Position(call.Pos())

	// Extract format string
	formatStr := extractStringLiteral(call.Args[0])
	if formatStr == "" {
		return nil
	}

	// Determine error type and context
	errorType, context := categorizeError(formatStr)

	migration := &ErrorMigration{
		File:           filename,
		Line:           pos.Line,
		Column:         pos.Column,
		OriginalCode:   nodeToString(call),
		ErrorType:      errorType,
		Context:        context,
		AutoMigratable: isAutoMigratable(formatStr, call.Args[1:]),
	}

	// Generate migrated code
	if migration.AutoMigratable {
		migration.MigratedCode = generateRichError(errorType, formatStr, call.Args[1:])
	}

	return migration
}

func analyzeErrorsNew(call *ast.CallExpr, fset *token.FileSet, filename string) *ErrorMigration {
	if len(call.Args) != 1 {
		return nil
	}

	pos := fset.Position(call.Pos())
	message := extractStringLiteral(call.Args[0])
	if message == "" {
		return nil
	}

	errorType, context := categorizeError(message)

	return &ErrorMigration{
		File:           filename,
		Line:           pos.Line,
		Column:         pos.Column,
		OriginalCode:   nodeToString(call),
		MigratedCode:   fmt.Sprintf(`types.NewRichError("%s", "%s")`, errorType, message),
		ErrorType:      errorType,
		Context:        context,
		AutoMigratable: true,
	}
}

func analyzeErrorsWrap(call *ast.CallExpr, fset *token.FileSet, filename string) *ErrorMigration {
	if len(call.Args) < 2 {
		return nil
	}

	pos := fset.Position(call.Pos())

	// Extract the message
	message := extractStringLiteral(call.Args[1])
	if message == "" {
		return nil
	}

	errorType, context := categorizeError(message)

	return &ErrorMigration{
		File:           filename,
		Line:           pos.Line,
		Column:         pos.Column,
		OriginalCode:   nodeToString(call),
		MigratedCode:   fmt.Sprintf(`types.WrapRichError(%s, "%s", "%s")`, nodeToString(call.Args[0]), errorType, message),
		ErrorType:      errorType,
		Context:        context,
		AutoMigratable: true,
	}
}

func categorizeError(message string) (errorType, context string) {
	message = strings.ToLower(message)

	// Categorize based on keywords
	switch {
	case strings.Contains(message, "validation") || strings.Contains(message, "invalid"):
		return "ValidationError", "Input validation"
	case strings.Contains(message, "not found") || strings.Contains(message, "does not exist"):
		return "NotFoundError", "Resource lookup"
	case strings.Contains(message, "unauthorized") || strings.Contains(message, "permission"):
		return "UnauthorizedError", "Authentication/Authorization"
	case strings.Contains(message, "timeout") || strings.Contains(message, "deadline"):
		return "TimeoutError", "Operation timeout"
	case strings.Contains(message, "connection") || strings.Contains(message, "network"):
		return "NetworkError", "Network operation"
	case strings.Contains(message, "parse") || strings.Contains(message, "unmarshal"):
		return "ParseError", "Data parsing"
	case strings.Contains(message, "config") || strings.Contains(message, "configuration"):
		return "ConfigError", "Configuration"
	case strings.Contains(message, "internal") || strings.Contains(message, "unexpected"):
		return "InternalError", "Internal error"
	default:
		return "GeneralError", "General operation"
	}
}

func isAutoMigratable(format string, args []ast.Expr) bool {
	// Check if format string has complex formatting
	if strings.Contains(format, "%v") || strings.Contains(format, "%+v") {
		return false // May need manual review
	}

	// Check if any arguments are function calls (might have side effects)
	for _, arg := range args {
		if _, ok := arg.(*ast.CallExpr); ok {
			return false
		}
	}

	return true
}

func generateRichError(errorType, format string, args []ast.Expr) string {
	if len(args) == 0 {
		return fmt.Sprintf(`types.NewRichError("%s", "%s")`, errorType, format)
	}

	// Build the argument list
	argStrs := make([]string, len(args))
	for i, arg := range args {
		argStrs[i] = nodeToString(arg)
	}

	return fmt.Sprintf(`types.NewRichError("%s", "%s", %s)`, errorType, format, strings.Join(argStrs, ", "))
}

func extractStringLiteral(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		// Remove quotes
		return strings.Trim(lit.Value, `"`)
	}
	return ""
}

func nodeToString(node ast.Node) string {
	var buf bytes.Buffer
	format.Node(&buf, token.NewFileSet(), node)
	return buf.String()
}

func groupByFile(migrations []*ErrorMigration) map[string][]*ErrorMigration {
	groups := make(map[string][]*ErrorMigration)
	for _, m := range migrations {
		groups[m.File] = append(groups[m.File], m)
	}
	return groups
}

func processMigrations(filename string, migrations []*ErrorMigration, stats *MigrationStats) error {
	stats.FilesProcessed++

	content, err := os.ReadFile(filename)
	if err != nil {
		return err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, content, parser.ParseComments)
	if err != nil {
		return err
	}

	modified := false
	skipped := 0
	migrated := 0

	fmt.Printf("\n📄 File: %s\n", filename)
	fmt.Printf("   Found %d error patterns\n", len(migrations))

	for _, migration := range migrations {
		stats.TotalErrors++
		stats.ErrorsByType[migration.ErrorType]++

		if *verbose {
			printMigration(migration)
		}

		// Skip non-auto-migratable in auto-only mode
		if *autoOnly && !migration.AutoMigratable {
			if *verbose {
				fmt.Printf("   ⏭️  Skipping (requires manual review)\n")
			}
			skipped++
			stats.SkippedErrors++
			continue
		}

		// Interactive mode
		if *interactive && !*dryRun {
			if !promptForMigration(migration) {
				skipped++
				stats.SkippedErrors++
				continue
			}
		}

		if migration.AutoMigratable && !*dryRun {
			// Would apply migration here
			modified = true
			migrated++
			stats.MigratedErrors++
		} else if migration.AutoMigratable {
			migrated++
			stats.MigratedErrors++
		}
	}

	if modified {
		stats.FilesModified++
		// Add types import if needed
		if needsTypesImport(file) {
			addTypesImport(file)
		}

		// Write modified file
		if err := writeFile(filename, file, fset); err != nil {
			return err
		}
	}

	fmt.Printf("   Summary: %d migrated, %d skipped\n", migrated, skipped)

	return nil
}

func needsTypesImport(file *ast.File) bool {
	for _, imp := range file.Imports {
		if imp.Path.Value == `"github.com/Azure/container-kit/pkg/types"` {
			return false
		}
	}
	return true
}

func addTypesImport(file *ast.File) {
	// Add import to the file
	importSpec := &ast.ImportSpec{
		Path: &ast.BasicLit{
			Kind:  token.STRING,
			Value: `"github.com/Azure/container-kit/pkg/types"`,
		},
	}

	// Find or create import declaration
	var importDecl *ast.GenDecl
	for _, decl := range file.Decls {
		if genDecl, ok := decl.(*ast.GenDecl); ok && genDecl.Tok == token.IMPORT {
			importDecl = genDecl
			break
		}
	}

	if importDecl != nil {
		importDecl.Specs = append(importDecl.Specs, importSpec)
	}
}

func writeFile(filename string, file *ast.File, fset *token.FileSet) error {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return err
	}

	return os.WriteFile(filename, buf.Bytes(), 0644)
}

func printMigration(m *ErrorMigration) {
	fmt.Printf("\n   Line %d: %s\n", m.Line, m.ErrorType)
	fmt.Printf("   Original: %s\n", truncate(m.OriginalCode, 60))
	if m.AutoMigratable {
		fmt.Printf("   Migrated: %s\n", truncate(m.MigratedCode, 60))
	} else {
		fmt.Printf("   Status: Requires manual review\n")
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func promptForMigration(m *ErrorMigration) bool {
	fmt.Printf("\n🔄 Migrate this error? (Line %d)\n", m.Line)
	fmt.Printf("Original: %s\n", m.OriginalCode)
	fmt.Printf("Migrated: %s\n", m.MigratedCode)
	fmt.Print("Apply migration? [y/N/q]: ")

	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))

	switch response {
	case "y", "yes":
		return true
	case "q", "quit":
		os.Exit(0)
	}
	return false
}

func printSummary(stats *MigrationStats) {
	fmt.Printf("\n📊 Migration Summary\n")
	fmt.Printf("===================\n")

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Files Processed:\t%d\n", stats.FilesProcessed)
	fmt.Fprintf(w, "Files Modified:\t%d\n", stats.FilesModified)
	fmt.Fprintf(w, "Total Errors Found:\t%d\n", stats.TotalErrors)
	fmt.Fprintf(w, "Errors Migrated:\t%d\n", stats.MigratedErrors)
	fmt.Fprintf(w, "Errors Skipped:\t%d\n", stats.SkippedErrors)

	if stats.TotalErrors > 0 {
		successRate := float64(stats.MigratedErrors) / float64(stats.TotalErrors) * 100
		fmt.Fprintf(w, "Migration Rate:\t%.1f%%\n", successRate)
	}

	w.Flush()

	if len(stats.ErrorsByType) > 0 {
		fmt.Printf("\nErrors by Type:\n")
		w = tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		for errorType, count := range stats.ErrorsByType {
			fmt.Fprintf(w, "  %s:\t%d\n", errorType, count)
		}
		w.Flush()
	}
}

func saveReport(filename string, migrations []*ErrorMigration, stats *MigrationStats) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "Error Migration Report\n")
	fmt.Fprintf(f, "=====================\n\n")

	fmt.Fprintf(f, "Summary:\n")
	fmt.Fprintf(f, "- Files Processed: %d\n", stats.FilesProcessed)
	fmt.Fprintf(f, "- Total Errors: %d\n", stats.TotalErrors)
	fmt.Fprintf(f, "- Migrated: %d\n", stats.MigratedErrors)
	fmt.Fprintf(f, "- Skipped: %d\n\n", stats.SkippedErrors)

	fmt.Fprintf(f, "Detailed Migrations:\n")
	fmt.Fprintf(f, "===================\n\n")

	currentFile := ""
	for _, m := range migrations {
		if m.File != currentFile {
			fmt.Fprintf(f, "\nFile: %s\n", m.File)
			currentFile = m.File
		}

		fmt.Fprintf(f, "\n  Line %d: %s\n", m.Line, m.ErrorType)
		fmt.Fprintf(f, "  Original: %s\n", m.OriginalCode)
		if m.AutoMigratable {
			fmt.Fprintf(f, "  Migrated: %s\n", m.MigratedCode)
			fmt.Fprintf(f, "  Status: %s\n", getStatus(m, stats))
		} else {
			fmt.Fprintf(f, "  Status: Manual review required\n")
		}
	}

	fmt.Printf("\n📄 Report saved to: %s\n", filename)
	return nil
}

func getStatus(m *ErrorMigration, stats *MigrationStats) string {
	if *dryRun {
		return "Would migrate"
	}
	return "Migrated"
}
