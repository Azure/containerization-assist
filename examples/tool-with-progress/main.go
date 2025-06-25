package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp"
	mcptypes "github.com/Azure/container-copilot/pkg/mcp/types"
)

// LongRunningTool demonstrates a tool with progress reporting
type LongRunningTool struct {
	name string
}

// NewLongRunningTool creates a new instance
func NewLongRunningTool() *LongRunningTool {
	return &LongRunningTool{
		name: "long_running_tool",
	}
}

// Execute performs the main function with progress reporting
func (t *LongRunningTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	// For tools that support progress, check if a reporter is available
	reporter, hasReporter := ctx.Value("progress_reporter").(mcptypes.ProgressReporter)

	toolArgs, ok := args.(*LongRunningArgs)
	if !ok {
		return nil, fmt.Errorf("invalid arguments type")
	}

	startTime := time.Now()
	results := &LongRunningResult{
		Status: "in_progress",
		Steps:  []StepResult{},
	}

	// Simulate long-running operation with multiple steps
	steps := []string{
		"Initializing",
		"Analyzing input",
		"Processing data",
		"Optimizing results",
		"Finalizing output",
	}

	totalSteps := len(steps)
	for i, step := range steps {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Report progress if reporter available
		if hasReporter {
			progress := float64(i) / float64(totalSteps)
			reporter.ReportProgress(progress, fmt.Sprintf("Step %d/%d: %s", i+1, totalSteps, step))
		}

		// Simulate work
		duration := time.Duration(toolArgs.StepDuration) * time.Millisecond
		time.Sleep(duration)

		// Add step result
		results.Steps = append(results.Steps, StepResult{
			Name:     step,
			Status:   "completed",
			Duration: duration.String(),
		})

		// Log progress
		fmt.Printf("[%d/%d] Completed: %s\n", i+1, totalSteps, step)
	}

	// Final progress report
	if hasReporter {
		reporter.ReportProgress(1.0, "Operation complete")
	}

	// Update final result
	results.Status = "completed"
	results.TotalDuration = time.Since(startTime).String()
	results.Message = fmt.Sprintf("Successfully processed %d items in %d steps",
		toolArgs.ItemCount, totalSteps)

	return results, nil
}

// ExecuteWithProgress is an alternative method that receives the progress reporter directly
func (t *LongRunningTool) ExecuteWithProgress(
	ctx context.Context,
	args interface{},
	reporter mcptypes.ProgressReporter,
) (interface{}, error) {
	// Add reporter to context and delegate to Execute
	ctxWithReporter := context.WithValue(ctx, "progress_reporter", reporter)
	return t.Execute(ctxWithReporter, args)
}

// GetMetadata provides tool information
func (t *LongRunningTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
		Name:        "long_running_tool",
		Description: "Demonstrates progress reporting for long-running operations",
		Version:     "1.0.0",
		Category:    "example",
		Capabilities: []string{
			"progress-reporting",
			"cancellable",
			"multi-step",
		},
		Parameters: map[string]string{
			"item_count":    "required - Number of items to process",
			"step_duration": "optional - Duration of each step in milliseconds (default: 500)",
		},
		Examples: []mcp.ToolExample{
			{
				Description: "Process 100 items with default timing",
				Args: map[string]interface{}{
					"item_count": 100,
				},
			},
			{
				Description: "Fast processing with 100ms steps",
				Args: map[string]interface{}{
					"item_count":    50,
					"step_duration": 100,
				},
			},
		},
	}
}

// Validate checks input arguments
func (t *LongRunningTool) Validate(ctx context.Context, args interface{}) error {
	toolArgs, ok := args.(*LongRunningArgs)
	if !ok {
		return fmt.Errorf("invalid arguments type")
	}

	if toolArgs.ItemCount <= 0 {
		return fmt.Errorf("item_count must be greater than 0")
	}

	if toolArgs.ItemCount > 10000 {
		return fmt.Errorf("item_count must be 10000 or less")
	}

	// Set default step duration if not provided
	if toolArgs.StepDuration == 0 {
		toolArgs.StepDuration = 500
	}

	if toolArgs.StepDuration < 10 || toolArgs.StepDuration > 5000 {
		return fmt.Errorf("step_duration must be between 10 and 5000 milliseconds")
	}

	return nil
}

// LongRunningArgs defines input arguments
type LongRunningArgs struct {
	ItemCount    int `json:"item_count" description:"Number of items to process"`
	StepDuration int `json:"step_duration,omitempty" description:"Duration of each step in milliseconds"`
}

// LongRunningResult defines the output
type LongRunningResult struct {
	Status        string       `json:"status"`
	Message       string       `json:"message"`
	Steps         []StepResult `json:"steps"`
	TotalDuration string       `json:"total_duration"`
}

// StepResult represents a single step's result
type StepResult struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Duration string `json:"duration"`
}

// MockProgressReporter implements a simple progress reporter for demonstration
type MockProgressReporter struct {
	updates []ProgressUpdate
}

type ProgressUpdate struct {
	Progress float64
	Message  string
	Time     time.Time
}

func (r *MockProgressReporter) ReportProgress(progress float64, message string) {
	update := ProgressUpdate{
		Progress: progress,
		Message:  message,
		Time:     time.Now(),
	}
	r.updates = append(r.updates, update)

	// Visual progress bar
	barWidth := 30
	filled := int(progress * float64(barWidth))
	bar := "["
	for i := 0; i < barWidth; i++ {
		if i < filled {
			bar += "="
		} else {
			bar += " "
		}
	}
	bar += "]"

	fmt.Printf("\r%s %.0f%% - %s", bar, progress*100, message)
	if progress >= 1.0 {
		fmt.Println() // New line at completion
	}
}

// Ensure compliance
var _ mcp.Tool = (*LongRunningTool)(nil)

func main() {
	tool := NewLongRunningTool()

	fmt.Println("=== Long Running Tool Example ===")
	fmt.Println()

	// Example 1: With progress reporting
	fmt.Println("Example 1: Execution with progress reporting")

	args := &LongRunningArgs{
		ItemCount:    100,
		StepDuration: 300, // 300ms per step
	}

	// Validate
	if err := tool.Validate(context.Background(), args); err != nil {
		fmt.Printf("Validation failed: %v\n", err)
		return
	}

	// Create progress reporter
	reporter := &MockProgressReporter{}

	// Execute with progress
	ctx := context.Background()
	result, err := tool.ExecuteWithProgress(ctx, args, reporter)
	if err != nil {
		fmt.Printf("Execution failed: %v\n", err)
		return
	}

	// Display results
	if res, ok := result.(*LongRunningResult); ok {
		fmt.Printf("\n✓ Execution completed\n")
		fmt.Printf("  Status: %s\n", res.Status)
		fmt.Printf("  Message: %s\n", res.Message)
		fmt.Printf("  Total duration: %s\n", res.TotalDuration)
		fmt.Printf("  Steps completed: %d\n", len(res.Steps))
	}

	fmt.Println()

	// Example 2: With cancellation
	fmt.Println("Example 2: Execution with cancellation")

	args2 := &LongRunningArgs{
		ItemCount:    100,
		StepDuration: 1000, // 1 second per step
	}

	// Create cancellable context
	ctx2, cancel := context.WithCancel(context.Background())

	// Cancel after 2.5 seconds
	go func() {
		time.Sleep(2500 * time.Millisecond)
		fmt.Println("\n! Cancelling operation...")
		cancel()
	}()

	// Execute
	_, err = tool.Execute(ctx2, args2)
	if err != nil {
		fmt.Printf("\n✓ Operation cancelled as expected: %v\n", err)
	}
}
