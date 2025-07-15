// Package observability provides tests for the unified observer
package observability

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"testing"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/infrastructure/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUnifiedObserver(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := DefaultObserverConfig()

	observer := NewUnifiedObserver(logger, config)

	assert.NotNil(t, observer)
	assert.Equal(t, config, observer.config)
	assert.NotNil(t, observer.errorAggregator)
	assert.False(t, observer.startTime.IsZero())
}

func TestDefaultObserverConfig(t *testing.T) {
	config := DefaultObserverConfig()

	assert.Equal(t, 1.0, config.SamplingRate)
	assert.Equal(t, 10000, config.MaxEvents)
	assert.Equal(t, 1000, config.MaxErrors)
	assert.Equal(t, time.Hour*24, config.RetentionPeriod)
	assert.True(t, config.MetricsEnabled)
	assert.True(t, config.TracingEnabled)
	assert.True(t, config.HealthCheckEnabled)
}

func TestTrackEvent(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	event := &Event{
		Name:      "test_event",
		Type:      EventTypeOperation,
		Component: "test_component",
		Operation: "test_operation",
		Success:   true,
		Properties: map[string]interface{}{
			"key": "value",
		},
		Tags: map[string]string{
			"tag": "value",
		},
	}

	ctx := context.Background()
	observer.TrackEvent(ctx, event)

	// Verify event was stored
	assert.Greater(t, observer.eventCount, int64(0))

	// Check if event exists in storage
	found := false
	observer.events.Range(func(key, value interface{}) bool {
		if storedEvent, ok := value.(*Event); ok {
			if storedEvent.Name == "test_event" {
				found = true
				assert.Equal(t, EventTypeOperation, storedEvent.Type)
				assert.Equal(t, "test_component", storedEvent.Component)
				assert.Equal(t, true, storedEvent.Success)
				return false
			}
		}
		return true
	})

	assert.True(t, found, "Event should be stored")
}

func TestTrackError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	// Test with standard error
	err := errors.New("test error")
	ctx := context.Background()
	observer.TrackError(ctx, err)

	// Verify error was tracked
	errorReport := observer.errorAggregator.GetReport()
	assert.Greater(t, errorReport.TotalErrors, int64(0))
}

func TestTrackStructuredError(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	// Create structured error
	structErr := mcperrors.NewWorkflowError("test_step", "test error", nil)
	structErr.WithWorkflowID("workflow_123").WithSessionID("session_456")

	ctx := context.Background()
	observer.TrackStructuredError(ctx, structErr)

	// Verify error was tracked
	errorReport := observer.errorAggregator.GetReport()
	assert.Greater(t, errorReport.TotalErrors, int64(0))

	// Verify error event was created
	found := false
	observer.events.Range(func(key, value interface{}) bool {
		if event, ok := value.(*Event); ok {
			if event.Type == EventTypeError {
				found = true
				assert.Equal(t, "workflow_123", event.WorkflowID)
				assert.Equal(t, "session_456", event.SessionID)
				assert.False(t, event.Success)
				return false
			}
		}
		return true
	})

	assert.True(t, found, "Error event should be created")
}

func TestStartOperation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	ctx := context.Background()
	opCtx := observer.StartOperation(ctx, "test_operation")

	assert.NotNil(t, opCtx)
	assert.Equal(t, "test_operation", opCtx.Name)
	assert.Equal(t, ctx, opCtx.Context)
	assert.Equal(t, observer, opCtx.observer)
	assert.False(t, opCtx.StartTime.IsZero())
}

func TestOperationContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	ctx := context.Background()
	opCtx := observer.StartOperation(ctx, "test_operation")

	// Add properties, metrics, and tags
	opCtx.AddProperty("prop1", "value1").
		AddMetric("metric1", 123.45).
		AddTag("tag1", "tagvalue1")

	// Finish the operation
	time.Sleep(time.Millisecond) // Ensure some duration
	opCtx.Finish(true)

	// Verify operation was tracked
	found := false
	observer.events.Range(func(key, value interface{}) bool {
		if event, ok := value.(*Event); ok {
			if event.Operation == "test_operation" && event.Type == EventTypeOperation {
				found = true
				assert.True(t, event.Success)
				assert.Greater(t, event.Duration, time.Duration(0))
				assert.Equal(t, "value1", event.Properties["prop1"])
				assert.Equal(t, 123.45, event.Metrics["metric1"])
				assert.Equal(t, "tagvalue1", event.Tags["tag1"])
				return false
			}
		}
		return true
	})

	assert.True(t, found, "Operation event should be created")
}

func TestStartSpan(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	ctx := context.Background()
	spanCtx := observer.StartSpan(ctx, "test_span")

	assert.NotNil(t, spanCtx)
	assert.Equal(t, "test_span", spanCtx.Name)
	assert.NotEmpty(t, spanCtx.TraceID)
	assert.NotEmpty(t, spanCtx.SpanID)
	assert.Equal(t, ctx, spanCtx.Context)
	assert.Equal(t, observer, spanCtx.observer)
}

func TestSpanContext(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	ctx := context.Background()
	spanCtx := observer.StartSpan(ctx, "test_span")

	// Add tags
	spanCtx.AddTag("span_tag", "span_value")

	// Finish the span
	time.Sleep(time.Millisecond) // Ensure some duration
	spanCtx.Finish(true)

	// Verify span was tracked
	found := false
	observer.events.Range(func(key, value interface{}) bool {
		if event, ok := value.(*Event); ok {
			if event.Name == "test_span" {
				found = true
				assert.True(t, event.Success)
				assert.Greater(t, event.Duration, time.Duration(0))
				assert.Equal(t, spanCtx.TraceID, event.Properties["trace_id"])
				assert.Equal(t, spanCtx.SpanID, event.Properties["span_id"])
				assert.Equal(t, "span_value", event.Tags["span_tag"])
				return false
			}
		}
		return true
	})

	assert.True(t, found, "Span event should be created")
}

func TestRecordHealthCheck(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	observer.RecordHealthCheck("test_component", HealthStatusHealthy, time.Millisecond*100)

	// Verify health check was recorded
	if health, ok := observer.healthChecks.Load("test_component"); ok {
		if componentHealth, ok := health.(*ComponentHealth); ok {
			assert.Equal(t, HealthStatusHealthy, componentHealth.Status)
			assert.Equal(t, time.Millisecond*100, componentHealth.ResponseTime)
			assert.False(t, componentHealth.LastCheck.IsZero())
		} else {
			t.Fatal("Health check not stored as ComponentHealth")
		}
	} else {
		t.Fatal("Health check not stored")
	}

	// Verify health event was created
	found := false
	observer.events.Range(func(key, value interface{}) bool {
		if event, ok := value.(*Event); ok {
			if event.Type == EventTypeHealth && event.Component == "test_component" {
				found = true
				assert.True(t, event.Success)
				assert.Equal(t, "health_check", event.Operation)
				return false
			}
		}
		return true
	})

	assert.True(t, found, "Health event should be created")
}

func TestMetrics(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	tags := map[string]string{"tag1": "value1"}

	// Test counter
	observer.IncrementCounter("test_counter", tags)
	observer.IncrementCounter("test_counter", tags) // Increment again

	// Test gauge
	observer.SetGauge("test_gauge", 42.5, tags)

	// Test histogram
	observer.RecordHistogram("test_histogram", 10.0, tags)
	observer.RecordHistogram("test_histogram", 20.0, tags)

	// Verify metrics were stored
	key := observer.buildMetricKey("test_counter", tags)
	if counter, ok := observer.counters.Load(key); ok {
		if counterMetric, ok := counter.(*CounterMetric); ok {
			assert.Equal(t, int64(2), counterMetric.Value)
		}
	}

	key = observer.buildMetricKey("test_gauge", tags)
	if gauge, ok := observer.gauges.Load(key); ok {
		if gaugeMetric, ok := gauge.(*GaugeMetric); ok {
			assert.Equal(t, 42.5, gaugeMetric.Value)
		}
	}

	key = observer.buildMetricKey("test_histogram", tags)
	if histogram, ok := observer.histograms.Load(key); ok {
		if histogramMetric, ok := histogram.(*HistogramMetric); ok {
			assert.Len(t, histogramMetric.Values, 2)
			assert.Contains(t, histogramMetric.Values, 10.0)
			assert.Contains(t, histogramMetric.Values, 20.0)
		}
	}
}

func TestRecordResourceUsage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	resource := &ResourceUsage{
		Component: "test_component",
		CPU: &ResourceMetric{
			Used:      50.0,
			Available: 100.0,
			Percent:   50.0,
			Unit:      "percent",
		},
		Memory: &ResourceMetric{
			Used:      2048.0,
			Available: 4096.0,
			Percent:   50.0,
			Unit:      "MB",
		},
	}

	ctx := context.Background()
	observer.RecordResourceUsage(ctx, resource)

	// Verify resource usage was stored
	if stored, ok := observer.resourceUsage.Load("test_component"); ok {
		if storedResource, ok := stored.(*ResourceUsage); ok {
			assert.Equal(t, "test_component", storedResource.Component)
			assert.NotNil(t, storedResource.CPU)
			assert.Equal(t, 50.0, storedResource.CPU.Percent)
		}
	}

	// Verify resource event was created
	found := false
	observer.events.Range(func(key, value interface{}) bool {
		if event, ok := value.(*Event); ok {
			if event.Type == EventTypeResource && event.Component == "test_component" {
				found = true
				assert.True(t, event.Success)
				assert.NotNil(t, event.Metrics)
				assert.Equal(t, 50.0, event.Metrics["cpu_percent"])
				assert.Equal(t, 50.0, event.Metrics["memory_percent"])
				return false
			}
		}
		return true
	})

	assert.True(t, found, "Resource event should be created")
}

func TestGetObservabilityReport(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	// Add some test data
	ctx := context.Background()

	// Add events
	observer.TrackEvent(ctx, &Event{
		Name:      "test_event",
		Type:      EventTypeOperation,
		Component: "test_component",
		Success:   true,
		Duration:  time.Millisecond * 100,
	})

	// Add error
	observer.TrackStructuredError(ctx, mcperrors.NewValidationError("field", "invalid"))

	// Add health check
	observer.RecordHealthCheck("test_component", HealthStatusHealthy, time.Millisecond*50)

	// Generate report
	report := observer.GetObservabilityReport()

	require.NotNil(t, report)
	assert.False(t, report.GeneratedAt.IsZero())
	assert.Equal(t, observer.startTime, report.Period.Start)
	assert.Greater(t, report.EventSummary.TotalEvents, int64(0))
	assert.Greater(t, report.ErrorAnalysis.TotalErrors, int64(0))
	assert.NotEmpty(t, report.HealthStatus)
}

func TestSamplingRate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := DefaultObserverConfig()
	config.SamplingRate = 0.0 // No sampling
	observer := NewUnifiedObserver(logger, config)

	// This event should not be tracked due to sampling
	ctx := context.Background()
	observer.TrackEvent(ctx, &Event{
		Name:      "sampled_event",
		Type:      EventTypeOperation,
		Component: "test_component",
		Success:   true,
	})

	// Verify event was not stored (due to 0% sampling)
	assert.Equal(t, int64(0), observer.eventCount)

	// Set sampling to 100%
	observer.SetSamplingRate(1.0)

	// Now event should be tracked
	observer.TrackEvent(ctx, &Event{
		Name:      "tracked_event",
		Type:      EventTypeOperation,
		Component: "test_component",
		Success:   true,
	})

	assert.Greater(t, observer.eventCount, int64(0))
}

func TestLogger(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	retrievedLogger := observer.Logger()
	assert.NotNil(t, retrievedLogger)
	// Note: Can't directly compare loggers, but we can verify it's not nil
}

func TestSetLogLevel(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	observer := NewUnifiedObserver(logger, DefaultObserverConfig())

	observer.SetLogLevel(slog.LevelDebug)

	level := observer.logLevel.Load().(slog.Level)
	assert.Equal(t, slog.LevelDebug, level)
}
