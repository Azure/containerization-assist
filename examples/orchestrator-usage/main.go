package main

import (
	"context"
	"fmt"
	"log"

	"github.com/Azure/container-copilot/pkg/mcp"
	"github.com/Azure/container-copilot/pkg/mcp/internal/orchestration"
	"github.com/rs/zerolog"
)

// Example tools for orchestration

// DataFetcherTool fetches data from a source
type DataFetcherTool struct{}

func (t *DataFetcherTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	fetchArgs := args.(*DataFetcherArgs)
	
	// Simulate data fetching
	data := map[string]interface{}{
		"source":     fetchArgs.Source,
		"records":    100,
		"timestamp":  "2024-01-15T10:30:00Z",
		"raw_data":   []int{1, 2, 3, 4, 5},
	}
	
	fmt.Printf("✓ Fetched data from %s\n", fetchArgs.Source)
	return data, nil
}

func (t *DataFetcherTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
		Name:        "data_fetcher",
		Description: "Fetches data from various sources",
		Version:     "1.0.0",
		Category:    "data",
	}
}

func (t *DataFetcherTool) Validate(ctx context.Context, args interface{}) error {
	fetchArgs, ok := args.(*DataFetcherArgs)
	if !ok || fetchArgs.Source == "" {
		return fmt.Errorf("source is required")
	}
	return nil
}

type DataFetcherArgs struct {
	Source string `json:"source"`
}

// DataProcessorTool processes fetched data
type DataProcessorTool struct{}

func (t *DataProcessorTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	procArgs := args.(*DataProcessorArgs)
	
	// Simulate data processing
	result := map[string]interface{}{
		"processed_records": 100,
		"average":          3.0,
		"sum":             15,
		"transformation":   procArgs.Transform,
	}
	
	fmt.Printf("✓ Processed data with %s transformation\n", procArgs.Transform)
	return result, nil
}

func (t *DataProcessorTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
		Name:        "data_processor",
		Description: "Processes and transforms data",
		Version:     "1.0.0",
		Category:    "data",
	}
}

func (t *DataProcessorTool) Validate(ctx context.Context, args interface{}) error {
	procArgs, ok := args.(*DataProcessorArgs)
	if !ok || procArgs.Data == nil {
		return fmt.Errorf("data is required")
	}
	return nil
}

type DataProcessorArgs struct {
	Data      interface{} `json:"data"`
	Transform string      `json:"transform"`
}

// DataStorageTool stores processed data
type DataStorageTool struct{}

func (t *DataStorageTool) Execute(ctx context.Context, args interface{}) (interface{}, error) {
	storeArgs := args.(*DataStorageArgs)
	
	// Simulate data storage
	result := map[string]interface{}{
		"stored":      true,
		"destination": storeArgs.Destination,
		"size_bytes":  1024,
		"checksum":    "abc123def456",
	}
	
	fmt.Printf("✓ Stored data to %s\n", storeArgs.Destination)
	return result, nil
}

func (t *DataStorageTool) GetMetadata() mcp.ToolMetadata {
	return mcp.ToolMetadata{
		Name:        "data_storage",
		Description: "Stores data to various destinations",
		Version:     "1.0.0",
		Category:    "data",
	}
}

func (t *DataStorageTool) Validate(ctx context.Context, args interface{}) error {
	storeArgs, ok := args.(*DataStorageArgs)
	if !ok || storeArgs.Destination == "" {
		return fmt.Errorf("destination is required")
	}
	return nil
}

type DataStorageArgs struct {
	Data        interface{} `json:"data"`
	Destination string      `json:"destination"`
}

// Ensure compliance
var (
	_ mcp.Tool = (*DataFetcherTool)(nil)
	_ mcp.Tool = (*DataProcessorTool)(nil)
	_ mcp.Tool = (*DataStorageTool)(nil)
)

// OrchestratorExample demonstrates using the tool orchestrator
type OrchestratorExample struct {
	orchestrator *orchestration.ToolOrchestrator
	logger       zerolog.Logger
}

func NewOrchestratorExample() *OrchestratorExample {
	logger := zerolog.New(zerolog.ConsoleWriter{}).With().Timestamp().Logger()
	
	// Create orchestrator with registry
	registry := orchestration.NewToolRegistry()
	orchestrator := orchestration.NewToolOrchestrator(registry, logger)
	
	return &OrchestratorExample{
		orchestrator: orchestrator,
		logger:       logger,
	}
}

func (o *OrchestratorExample) RegisterTools() error {
	// Register all tools
	tools := []mcp.Tool{
		&DataFetcherTool{},
		&DataProcessorTool{},
		&DataStorageTool{},
	}
	
	for _, tool := range tools {
		metadata := tool.GetMetadata()
		if err := o.orchestrator.RegisterTool(metadata.Name, tool); err != nil {
			return fmt.Errorf("failed to register tool %s: %w", metadata.Name, err)
		}
		o.logger.Info().Str("tool", metadata.Name).Msg("Registered tool")
	}
	
	return nil
}

func (o *OrchestratorExample) RunDataPipeline(ctx context.Context) error {
	fmt.Println("\n=== Running Data Pipeline ===")
	
	// Step 1: Fetch data
	fetchResult, err := o.orchestrator.ExecuteTool(ctx, "data_fetcher", &DataFetcherArgs{
		Source: "api.example.com",
	})
	if err != nil {
		return fmt.Errorf("fetch failed: %w", err)
	}
	
	// Step 2: Process data
	processResult, err := o.orchestrator.ExecuteTool(ctx, "data_processor", &DataProcessorArgs{
		Data:      fetchResult,
		Transform: "normalize",
	})
	if err != nil {
		return fmt.Errorf("process failed: %w", err)
	}
	
	// Step 3: Store data
	storeResult, err := o.orchestrator.ExecuteTool(ctx, "data_storage", &DataStorageArgs{
		Data:        processResult,
		Destination: "database.example.com",
	})
	if err != nil {
		return fmt.Errorf("storage failed: %w", err)
	}
	
	fmt.Printf("\n✓ Pipeline completed successfully!\n")
	fmt.Printf("  Storage result: %v\n", storeResult)
	
	return nil
}

func (o *OrchestratorExample) RunParallelExecution(ctx context.Context) error {
	fmt.Println("\n=== Running Parallel Execution ===")
	
	// Execute multiple tools in parallel
	type toolExecution struct {
		name string
		args interface{}
	}
	
	executions := []toolExecution{
		{
			name: "data_fetcher",
			args: &DataFetcherArgs{Source: "source1.example.com"},
		},
		{
			name: "data_fetcher",
			args: &DataFetcherArgs{Source: "source2.example.com"},
		},
		{
			name: "data_fetcher",
			args: &DataFetcherArgs{Source: "source3.example.com"},
		},
	}
	
	// Channel for results
	type result struct {
		source string
		data   interface{}
		err    error
	}
	
	results := make(chan result, len(executions))
	
	// Launch parallel executions
	for _, exec := range executions {
		go func(e toolExecution) {
			args := e.args.(*DataFetcherArgs)
			data, err := o.orchestrator.ExecuteTool(ctx, e.name, e.args)
			results <- result{
				source: args.Source,
				data:   data,
				err:    err,
			}
		}(exec)
	}
	
	// Collect results
	var allData []interface{}
	for i := 0; i < len(executions); i++ {
		res := <-results
		if res.err != nil {
			o.logger.Error().Err(res.err).Str("source", res.source).Msg("Fetch failed")
			continue
		}
		allData = append(allData, res.data)
		o.logger.Info().Str("source", res.source).Msg("Fetch completed")
	}
	
	fmt.Printf("\n✓ Parallel execution completed!\n")
	fmt.Printf("  Successfully fetched from %d sources\n", len(allData))
	
	return nil
}

func (o *OrchestratorExample) ListAvailableTools() {
	fmt.Println("\n=== Available Tools ===")
	
	tools := o.orchestrator.ListTools()
	for _, toolName := range tools {
		tool, err := o.orchestrator.GetTool(toolName)
		if err != nil {
			continue
		}
		
		metadata := tool.GetMetadata()
		fmt.Printf("\n%s (v%s)\n", metadata.Name, metadata.Version)
		fmt.Printf("  Description: %s\n", metadata.Description)
		fmt.Printf("  Category: %s\n", metadata.Category)
		
		if len(metadata.Capabilities) > 0 {
			fmt.Printf("  Capabilities: %v\n", metadata.Capabilities)
		}
	}
}

func main() {
	// Create orchestrator example
	example := NewOrchestratorExample()
	
	// Register tools
	if err := example.RegisterTools(); err != nil {
		log.Fatalf("Failed to register tools: %v", err)
	}
	
	// List available tools
	example.ListAvailableTools()
	
	ctx := context.Background()
	
	// Run sequential pipeline
	if err := example.RunDataPipeline(ctx); err != nil {
		log.Printf("Pipeline failed: %v", err)
	}
	
	// Run parallel execution
	if err := example.RunParallelExecution(ctx); err != nil {
		log.Printf("Parallel execution failed: %v", err)
	}
	
	// Demonstrate tool validation
	fmt.Println("\n=== Tool Validation Example ===")
	
	// Try with invalid args
	_, err := example.orchestrator.ExecuteTool(ctx, "data_fetcher", &DataFetcherArgs{
		// Missing required source field
	})
	if err != nil {
		fmt.Printf("✓ Validation correctly failed: %v\n", err)
	}
	
	// Demonstrate error handling
	fmt.Println("\n=== Error Handling Example ===")
	
	// Try to execute non-existent tool
	_, err = example.orchestrator.ExecuteTool(ctx, "non_existent_tool", nil)
	if err != nil {
		fmt.Printf("✓ Correctly handled missing tool: %v\n", err)
	}
}