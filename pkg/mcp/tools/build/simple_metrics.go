package build

import (
	"time"
)

// SimpleBuildMetrics provides basic build timing
type SimpleBuildMetrics struct {
	BuildStart time.Time
	BuildEnd   time.Time
}

// NewSimpleBuildMetrics creates basic metrics tracker
func NewSimpleBuildMetrics() *SimpleBuildMetrics {
	return &SimpleBuildMetrics{}
}

// StartBuild records build start time
func (m *SimpleBuildMetrics) StartBuild() {
	m.BuildStart = time.Now()
}

// EndBuild records build completion and returns duration
func (m *SimpleBuildMetrics) EndBuild() time.Duration {
	m.BuildEnd = time.Now()
	return m.BuildEnd.Sub(m.BuildStart)
}

// GetDuration returns the last build duration
func (m *SimpleBuildMetrics) GetDuration() time.Duration {
	if m.BuildEnd.IsZero() || m.BuildStart.IsZero() {
		return 0
	}
	return m.BuildEnd.Sub(m.BuildStart)
}
