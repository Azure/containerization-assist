// Package ml provides build history tracking for resource prediction optimization.
package ml

import (
	"sync"
	"time"
)

// BuildRecord represents a historical build record
type BuildRecord struct {
	ID           string        `json:"id"`
	Profile      BuildProfile  `json:"profile"`
	Resources    ResourceUsage `json:"resources"`
	Duration     time.Duration `json:"duration"`
	Success      bool          `json:"success"`
	Timestamp    time.Time     `json:"timestamp"`
	ErrorType    string        `json:"error_type,omitempty"`
	CacheHitRate float64       `json:"cache_hit_rate"`
}

// ResourceUsage represents actual resource usage during a build
type ResourceUsage struct {
	PeakCPU      float64 `json:"peak_cpu_percent"`
	PeakMemoryMB int     `json:"peak_memory_mb"`
	DiskIOMB     int     `json:"disk_io_mb"`
	NetworkMB    int     `json:"network_mb"`
}

// BuildHistoryStore stores and retrieves build history for learning
type BuildHistoryStore struct {
	mu      sync.RWMutex
	records map[string]*BuildRecord
	index   map[string][]string // indexed by language+framework
}

// NewBuildHistoryStore creates a new build history store
func NewBuildHistoryStore() *BuildHistoryStore {
	return &BuildHistoryStore{
		records: make(map[string]*BuildRecord),
		index:   make(map[string][]string),
	}
}

// RecordBuild stores a build record
func (s *BuildHistoryStore) RecordBuild(record *BuildRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store record
	s.records[record.ID] = record

	// Update index
	key := s.buildIndexKey(record.Profile)
	s.index[key] = append(s.index[key], record.ID)

	// Keep only recent records (last 1000)
	if len(s.records) > 1000 {
		s.pruneOldRecords()
	}
}

// FindSimilarBuilds finds builds with similar profiles
func (s *BuildHistoryStore) FindSimilarBuilds(profile BuildProfile) []BuildRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := s.buildIndexKey(profile)
	recordIDs, exists := s.index[key]
	if !exists {
		return []BuildRecord{}
	}

	// Filter and return similar builds
	similar := make([]BuildRecord, 0)
	for _, id := range recordIDs {
		if record, exists := s.records[id]; exists {
			if s.isSimilarProfile(record.Profile, profile) {
				similar = append(similar, *record)
			}
		}
	}

	// Return most recent 10
	if len(similar) > 10 {
		return similar[len(similar)-10:]
	}
	return similar
}

// GetAverageResources calculates average resource usage for a profile
func (s *BuildHistoryStore) GetAverageResources(profile BuildProfile) *ResourceUsage {
	similar := s.FindSimilarBuilds(profile)
	if len(similar) == 0 {
		return nil
	}

	avg := &ResourceUsage{}
	for _, record := range similar {
		avg.PeakCPU += record.Resources.PeakCPU
		avg.PeakMemoryMB += record.Resources.PeakMemoryMB
		avg.DiskIOMB += record.Resources.DiskIOMB
		avg.NetworkMB += record.Resources.NetworkMB
	}

	count := float64(len(similar))
	avg.PeakCPU /= count
	avg.PeakMemoryMB = int(float64(avg.PeakMemoryMB) / count)
	avg.DiskIOMB = int(float64(avg.DiskIOMB) / count)
	avg.NetworkMB = int(float64(avg.NetworkMB) / count)

	return avg
}

// buildIndexKey creates an index key from profile
func (s *BuildHistoryStore) buildIndexKey(profile BuildProfile) string {
	return profile.Language + ":" + profile.Framework
}

// isSimilarProfile checks if two profiles are similar enough
func (s *BuildHistoryStore) isSimilarProfile(a, b BuildProfile) bool {
	// Same language and framework
	if a.Language != b.Language || a.Framework != b.Framework {
		return false
	}

	// Similar dependency count (within 20%)
	depDiff := float64(abs(a.Dependencies - b.Dependencies))
	avgDeps := float64(a.Dependencies+b.Dependencies) / 2
	if avgDeps > 0 && depDiff/avgDeps > 0.2 {
		return false
	}

	// Similar size (within 50%)
	sizeDiffCode := abs(int(a.CodeSizeMB - b.CodeSizeMB))
	avgSizeCode := int((a.CodeSizeMB + b.CodeSizeMB) / 2)
	if avgSizeCode > 0 && float64(sizeDiffCode)/float64(avgSizeCode) > 0.5 {
		return false
	}

	return true
}

// pruneOldRecords removes oldest records to maintain size limit
func (s *BuildHistoryStore) pruneOldRecords() {
	// Find oldest 100 records
	type recordAge struct {
		id        string
		timestamp time.Time
	}

	ages := make([]recordAge, 0, len(s.records))
	for id, record := range s.records {
		ages = append(ages, recordAge{id: id, timestamp: record.Timestamp})
	}

	// Sort by timestamp (simple bubble sort for small dataset)
	for i := 0; i < len(ages)-1; i++ {
		for j := 0; j < len(ages)-i-1; j++ {
			if ages[j].timestamp.After(ages[j+1].timestamp) {
				ages[j], ages[j+1] = ages[j+1], ages[j]
			}
		}
	}

	// Remove oldest 100
	toRemove := 100
	if toRemove > len(ages) {
		toRemove = len(ages) / 10
	}

	for i := 0; i < toRemove && i < len(ages); i++ {
		delete(s.records, ages[i].id)
	}

	// Rebuild index
	s.rebuildIndex()
}

// rebuildIndex rebuilds the index from current records
func (s *BuildHistoryStore) rebuildIndex() {
	s.index = make(map[string][]string)
	for id, record := range s.records {
		key := s.buildIndexKey(record.Profile)
		s.index[key] = append(s.index[key], id)
	}
}

// abs returns absolute value of an integer
func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
