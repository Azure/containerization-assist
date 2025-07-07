package build

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"
	"time"

	"log/slog"

	errors "github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// PerformanceDashboard provides visualization and reporting for build performance
type PerformanceDashboard struct {
	logger    *slog.Logger
	monitor   *PerformanceMonitor
	analyzer  *PerformanceAnalyzer
	dataStore *PerformanceDataStore
}

// PerformanceDataStore stores historical performance data
type PerformanceDataStore struct {
	builds     []BuildRecord
	maxRecords int
}

// BuildRecord represents a single build's performance data
type BuildRecord struct {
	ID            string        `json:"id"`
	Timestamp     time.Time     `json:"timestamp"`
	Tool          string        `json:"tool"`
	ImageName     string        `json:"image_name"`
	Duration      time.Duration `json:"duration"`
	Success       bool          `json:"success"`
	CacheHitRate  float64       `json:"cache_hit_rate"`
	ImageSize     int64         `json:"image_size"`
	LayerCount    int           `json:"layer_count"`
	Stages        []StageRecord `json:"stages"`
	Optimizations []string      `json:"optimizations"`
}

// StageRecord represents performance data for a build stage
type StageRecord struct {
	Name     string        `json:"name"`
	Duration time.Duration `json:"duration"`
	Success  bool          `json:"success"`
}

// DashboardData contains data for dashboard rendering
type DashboardData struct {
	GeneratedAt     time.Time             `json:"generated_at"`
	TotalBuilds     int                   `json:"total_builds"`
	SuccessRate     float64               `json:"success_rate"`
	AvgBuildTime    time.Duration         `json:"avg_build_time"`
	AvgCacheHitRate float64               `json:"avg_cache_hit_rate"`
	RecentBuilds    []BuildRecord         `json:"recent_builds"`
	TopSlowBuilds   []BuildRecord         `json:"top_slow_builds"`
	TrendData       []TrendPoint          `json:"trend_data"`
	Recommendations []string              `json:"recommendations"`
	ByTool          map[string]ToolStats  `json:"by_tool"`
	ByImage         map[string]ImageStats `json:"by_image"`
}

// TrendPoint represents a point in time series data
type TrendPoint struct {
	Time        time.Time     `json:"time"`
	AvgDuration time.Duration `json:"avg_duration"`
	SuccessRate float64       `json:"success_rate"`
	BuildCount  int           `json:"build_count"`
}

// ToolStats contains statistics for a specific tool
type ToolStats struct {
	BuildCount  int           `json:"build_count"`
	SuccessRate float64       `json:"success_rate"`
	AvgDuration time.Duration `json:"avg_duration"`
	TotalTime   time.Duration `json:"total_time"`
}

// ImageStats contains statistics for a specific image
type ImageStats struct {
	BuildCount    int           `json:"build_count"`
	SuccessRate   float64       `json:"success_rate"`
	AvgDuration   time.Duration `json:"avg_duration"`
	TotalTime     time.Duration `json:"total_time"`
	AvgSize       int64         `json:"avg_size"`
	LastBuildTime time.Time     `json:"last_build_time"`
}

// NewPerformanceDashboard creates a new performance dashboard
func NewPerformanceDashboard(logger *slog.Logger, monitor *PerformanceMonitor) *PerformanceDashboard {
	return &PerformanceDashboard{
		logger:   logger.With("component", "performance_dashboard"),
		monitor:  monitor,
		analyzer: NewPerformanceAnalyzer(logger),
		dataStore: &PerformanceDataStore{
			builds:     make([]BuildRecord, 0, 1000),
			maxRecords: 1000,
		},
	}
}

// RecordBuild records a build's performance data
func (d *PerformanceDashboard) RecordBuild(record BuildRecord) {
	d.dataStore.addRecord(record)
	d.logger.Debug("Recorded build performance", "build_id", record.ID, "duration", record.Duration, "success", record.Success)
}

// GenerateDashboard generates dashboard data
func (d *PerformanceDashboard) GenerateDashboard(ctx context.Context) (*DashboardData, error) {
	d.logger.Info("Generating performance dashboard")

	data := &DashboardData{
		GeneratedAt: time.Now(),
		ByTool:      make(map[string]ToolStats),
		ByImage:     make(map[string]ImageStats),
	}

	// Get all builds
	builds := d.dataStore.getAllBuilds()
	data.TotalBuilds = len(builds)

	if data.TotalBuilds == 0 {
		return data, nil
	}

	// Calculate basic metrics
	successCount := 0
	totalDuration := time.Duration(0)
	totalCacheHitRate := float64(0)

	for _, build := range builds {
		if build.Success {
			successCount++
		}
		totalDuration += build.Duration
		totalCacheHitRate += build.CacheHitRate

		// Update tool stats
		toolStats := data.ByTool[build.Tool]
		toolStats.BuildCount++
		toolStats.TotalTime += build.Duration
		if build.Success {
			toolStats.SuccessRate = float64(toolStats.BuildCount) / float64(toolStats.BuildCount) * 100
		}
		data.ByTool[build.Tool] = toolStats

		// Update image stats
		imageStats := data.ByImage[build.ImageName]
		imageStats.BuildCount++
		if build.Success {
			imageStats.SuccessRate = float64(imageStats.BuildCount) / float64(imageStats.BuildCount) * 100
		}
		imageStats.AvgSize = (imageStats.AvgSize*int64(imageStats.BuildCount-1) + build.ImageSize) / int64(imageStats.BuildCount)
		imageStats.LastBuildTime = build.Timestamp
		data.ByImage[build.ImageName] = imageStats
	}

	// Calculate averages
	data.SuccessRate = float64(successCount) / float64(data.TotalBuilds) * 100
	data.AvgBuildTime = totalDuration / time.Duration(data.TotalBuilds)
	data.AvgCacheHitRate = totalCacheHitRate / float64(data.TotalBuilds)

	// Get recent builds
	data.RecentBuilds = d.getRecentBuilds(builds, 10)

	// Get slowest builds
	data.TopSlowBuilds = d.getSlowestBuilds(builds, 5)

	// Generate trend data
	data.TrendData = d.generateTrendData(builds)

	// Generate recommendations
	data.Recommendations = d.generateRecommendations(data)

	// Calculate tool averages
	for tool, stats := range data.ByTool {
		// Average duration would be calculated from build history in real implementation
		data.ByTool[tool] = stats
	}

	// Calculate image averages
	for image, stats := range data.ByImage {
		// Average duration would be calculated from build history in real implementation
		data.ByImage[image] = stats
	}

	return data, nil
}

// RenderHTML renders the dashboard as HTML
func (d *PerformanceDashboard) RenderHTML(data *DashboardData) (string, error) {
	tmpl := `
<!DOCTYPE html>
<html>
<head>
    <title>Build Performance Dashboard</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background-color: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; }
        .header { background-color: #2c3e50; color: white; padding: 20px; border-radius: 5px; }
        .metric-card { background-color: white; padding: 20px; margin: 10px; border-radius: 5px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .metric-value { font-size: 2em; font-weight: bold; color: #3498db; }
        .metric-label { color: #7f8c8d; margin-top: 5px; }
        .grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(250px, 1fr)); gap: 20px; }
        table { width: 100%; border-collapse: collapse; margin-top: 20px; }
        th, td { padding: 10px; text-align: left; border-bottom: 1px solid #ecf0f1; }
        th { background-color: #34495e; color: white; }
        .success { color: #27ae60; }
        .failure { color: #e74c3c; }
        .recommendation { background-color: #f39c12; color: white; padding: 10px; margin: 5px 0; border-radius: 3px; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Build Performance Dashboard</h1>
            <p>Generated at: {{.GeneratedAt.Format "2006-01-02 15:04:05"}}</p>
        </div>

        <div class="grid">
            <div class="metric-card">
                <div class="metric-value">{{.TotalBuilds}}</div>
                <div class="metric-label">Total Builds</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{printf "%.1f%%" .SuccessRate}}</div>
                <div class="metric-label">Success Rate</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{.AvgBuildTime}}</div>
                <div class="metric-label">Average Build Time</div>
            </div>
            <div class="metric-card">
                <div class="metric-value">{{printf "%.1f%%" .AvgCacheHitRate}}</div>
                <div class="metric-label">Cache Hit Rate</div>
            </div>
        </div>

        {{if .Recommendations}}
        <div class="metric-card">
            <h2>Recommendations</h2>
            {{range .Recommendations}}
            <div class="recommendation">{{.}}</div>
            {{end}}
        </div>
        {{end}}

        <div class="metric-card">
            <h2>Recent Builds</h2>
            <table>
                <tr>
                    <th>Time</th>
                    <th>Image</th>
                    <th>Duration</th>
                    <th>Status</th>
                    <th>Cache Hit</th>
                </tr>
                {{range .RecentBuilds}}
                <tr>
                    <td>{{.Timestamp.Format "15:04:05"}}</td>
                    <td>{{.ImageName}}</td>
                    <td>{{.Duration}}</td>
                    <td class="{{if .Success}}success{{else}}failure{{end}}">
                        {{if .Success}}Success{{else}}Failed{{end}}
                    </td>
                    <td>{{printf "%.1f%%" .CacheHitRate}}</td>
                </tr>
                {{end}}
            </table>
        </div>

        <div class="metric-card">
            <h2>Slowest Builds</h2>
            <table>
                <tr>
                    <th>Image</th>
                    <th>Duration</th>
                    <th>Time</th>
                </tr>
                {{range .TopSlowBuilds}}
                <tr>
                    <td>{{.ImageName}}</td>
                    <td>{{.Duration}}</td>
                    <td>{{.Timestamp.Format "2006-01-02 15:04"}}</td>
                </tr>
                {{end}}
            </table>
        </div>
    </div>
</body>
</html>
`

	t, err := template.New("dashboard").Parse(tmpl)
	if err != nil {
		return "", errors.NewError().Message("failed to parse template").Cause(err).Build()
	}

	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return "", errors.NewError().Message("failed to execute template").Cause(err).Build()
	}

	return buf.String(), nil
}

// RenderJSON renders the dashboard as JSON
func (d *PerformanceDashboard) RenderJSON(data *DashboardData) (string, error) {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", errors.NewError().Message("failed to marshal dashboard data").Cause(err).Build()
	}
	return string(jsonData), nil
}

// Helper methods

func (d *PerformanceDataStore) addRecord(record BuildRecord) {
	d.builds = append(d.builds, record)

	// Maintain max records limit
	if len(d.builds) > d.maxRecords {
		d.builds = d.builds[len(d.builds)-d.maxRecords:]
	}
}

func (d *PerformanceDataStore) getAllBuilds() []BuildRecord {
	return d.builds
}

func (d *PerformanceDashboard) getRecentBuilds(builds []BuildRecord, count int) []BuildRecord {
	if len(builds) <= count {
		return builds
	}

	// Sort by timestamp descending
	sorted := make([]BuildRecord, len(builds))
	copy(sorted, builds)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.After(sorted[j].Timestamp)
	})

	return sorted[:count]
}

func (d *PerformanceDashboard) getSlowestBuilds(builds []BuildRecord, count int) []BuildRecord {
	// Filter successful builds only
	successful := make([]BuildRecord, 0)
	for _, b := range builds {
		if b.Success {
			successful = append(successful, b)
		}
	}

	if len(successful) <= count {
		return successful
	}

	// Sort by duration descending
	sort.Slice(successful, func(i, j int) bool {
		return successful[i].Duration > successful[j].Duration
	})

	return successful[:count]
}

func (d *PerformanceDashboard) generateTrendData(builds []BuildRecord) []TrendPoint {
	// Group builds by hour
	hourlyData := make(map[time.Time]*TrendPoint)

	for _, build := range builds {
		hour := build.Timestamp.Truncate(time.Hour)

		point, exists := hourlyData[hour]
		if !exists {
			point = &TrendPoint{
				Time: hour,
			}
			hourlyData[hour] = point
		}

		point.BuildCount++
		point.AvgDuration = (point.AvgDuration*time.Duration(point.BuildCount-1) + build.Duration) / time.Duration(point.BuildCount)

		if build.Success {
			point.SuccessRate = float64(point.BuildCount) / float64(point.BuildCount) * 100
		}
	}

	// Convert to slice and sort
	trends := make([]TrendPoint, 0, len(hourlyData))
	for _, point := range hourlyData {
		trends = append(trends, *point)
	}

	sort.Slice(trends, func(i, j int) bool {
		return trends[i].Time.Before(trends[j].Time)
	})

	return trends
}

func (d *PerformanceDashboard) generateRecommendations(data *DashboardData) []string {
	recommendations := []string{}

	// Check success rate
	if data.SuccessRate < 90 {
		recommendations = append(recommendations,
			fmt.Sprintf("Build success rate is %.1f%%. Consider reviewing failed builds for common patterns.", data.SuccessRate))
	}

	// Check average build time
	if data.AvgBuildTime > 5*time.Minute {
		recommendations = append(recommendations,
			"Average build time exceeds 5 minutes. Consider implementing build optimization strategies.")
	}

	// Check cache hit rate
	if data.AvgCacheHitRate < 70 {
		recommendations = append(recommendations,
			fmt.Sprintf("Cache hit rate is %.1f%%. Optimize Dockerfile layer ordering to improve caching.", data.AvgCacheHitRate))
	}

	// Check for slow builds
	if len(data.TopSlowBuilds) > 0 && data.TopSlowBuilds[0].Duration > 10*time.Minute {
		recommendations = append(recommendations,
			"Some builds exceed 10 minutes. Consider using multi-stage builds or reducing build context size.")
	}

	// Tool-specific recommendations
	for tool, stats := range data.ByTool {
		if stats.SuccessRate < 80 {
			recommendations = append(recommendations,
				fmt.Sprintf("%s tool has low success rate (%.1f%%). Review error patterns.", tool, stats.SuccessRate))
		}
	}

	return recommendations
}
