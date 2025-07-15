// Package metrics provides wire bindings for metrics collection
package metrics

import (
	"github.com/Azure/container-kit/pkg/mcp/domain/workflow"
	"github.com/google/wire"
)

// MetricsBindings provides wire bindings for metrics interfaces
var MetricsBindings = wire.NewSet(
	// Bind WorkflowMetricsCollector to all metrics interfaces
	wire.Bind(new(workflow.MetricsCollector), new(*WorkflowMetricsCollector)),
	wire.Bind(new(workflow.ExtendedMetricsCollector), new(*WorkflowMetricsCollector)),
	wire.Bind(new(workflow.WorkflowMetricsCollector), new(*WorkflowMetricsCollector)),
)
