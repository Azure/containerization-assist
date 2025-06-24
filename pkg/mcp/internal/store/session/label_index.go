package session

import (
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// LabelIndex provides fast label-based lookups for sessions
type LabelIndex struct {
	// Label to session ID mapping
	labelToSessions map[string][]string

	// K8s label to session ID mapping
	k8sLabelToSessions map[string]map[string][]string

	// Reverse index for fast lookups
	sessionToLabels map[string][]string

	// Session to K8s labels mapping
	sessionToK8sLabels map[string]map[string]string

	// Cached queries for performance
	queryCache map[string]*CachedQuery

	// Mutex for thread safety
	mutex sync.RWMutex

	// Logger
	logger zerolog.Logger

	// Index metadata
	lastUpdated time.Time
	indexSize   int
}

// CachedQuery represents a cached query result
type CachedQuery struct {
	Query      string
	Result     []string
	Timestamp  time.Time
	ExpiresAt  time.Time
	HitCount   int
}

// NewLabelIndex creates a new label index
func NewLabelIndex(logger zerolog.Logger) *LabelIndex {
	return &LabelIndex{
		labelToSessions:    make(map[string][]string),
		k8sLabelToSessions: make(map[string]map[string][]string),
		sessionToLabels:    make(map[string][]string),
		sessionToK8sLabels: make(map[string]map[string]string),
		queryCache:         make(map[string]*CachedQuery),
		logger:             logger.With().Str("component", "label_index").Logger(),
		lastUpdated:        time.Now(),
	}
}

// AddSessionLabels adds labels for a session to the index
func (li *LabelIndex) AddSessionLabels(sessionID string, labels []string) {
	li.mutex.Lock()
	defer li.mutex.Unlock()

	li.logger.Debug().
		Str("session_id", sessionID).
		Strs("labels", labels).
		Msg("Adding session labels to index")

	// Remove existing labels for this session
	li.removeSessionLabelsInternal(sessionID)

	// Add new labels
	li.sessionToLabels[sessionID] = make([]string, len(labels))
	copy(li.sessionToLabels[sessionID], labels)

	// Update label to sessions mapping
	for _, label := range labels {
		if _, exists := li.labelToSessions[label]; !exists {
			li.labelToSessions[label] = make([]string, 0)
		}
		li.labelToSessions[label] = li.addUniqueSessionID(li.labelToSessions[label], sessionID)
	}

	li.updateIndexMetadata()
	li.invalidateQueryCache()
}

// AddSessionK8sLabels adds K8s labels for a session to the index
func (li *LabelIndex) AddSessionK8sLabels(sessionID string, k8sLabels map[string]string) {
	li.mutex.Lock()
	defer li.mutex.Unlock()

	li.logger.Debug().
		Str("session_id", sessionID).
		Interface("k8s_labels", k8sLabels).
		Msg("Adding session K8s labels to index")

	// Remove existing K8s labels for this session
	li.removeSessionK8sLabelsInternal(sessionID)

	// Add new K8s labels
	if len(k8sLabels) > 0 {
		li.sessionToK8sLabels[sessionID] = make(map[string]string)
		for key, value := range k8sLabels {
			li.sessionToK8sLabels[sessionID][key] = value

			// Update K8s label to sessions mapping
			if _, exists := li.k8sLabelToSessions[key]; !exists {
				li.k8sLabelToSessions[key] = make(map[string][]string)
			}
			if _, exists := li.k8sLabelToSessions[key][value]; !exists {
				li.k8sLabelToSessions[key][value] = make([]string, 0)
			}
			li.k8sLabelToSessions[key][value] = li.addUniqueSessionID(li.k8sLabelToSessions[key][value], sessionID)
		}
	}

	li.updateIndexMetadata()
	li.invalidateQueryCache()
}

// RemoveSession removes all labels for a session from the index
func (li *LabelIndex) RemoveSession(sessionID string) {
	li.mutex.Lock()
	defer li.mutex.Unlock()

	li.logger.Debug().
		Str("session_id", sessionID).
		Msg("Removing session from index")

	li.removeSessionLabelsInternal(sessionID)
	li.removeSessionK8sLabelsInternal(sessionID)

	li.updateIndexMetadata()
	li.invalidateQueryCache()
}

// GetSessionsWithLabel returns session IDs that have the specified label
func (li *LabelIndex) GetSessionsWithLabel(label string) []string {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	if sessionIDs, exists := li.labelToSessions[label]; exists {
		result := make([]string, len(sessionIDs))
		copy(result, sessionIDs)
		return result
	}

	return []string{}
}

// GetSessionsWithK8sLabel returns session IDs that have the specified K8s label
func (li *LabelIndex) GetSessionsWithK8sLabel(key, value string) []string {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	if keyMap, exists := li.k8sLabelToSessions[key]; exists {
		if sessionIDs, exists := keyMap[value]; exists {
			result := make([]string, len(sessionIDs))
			copy(result, sessionIDs)
			return result
		}
	}

	return []string{}
}

// GetLabelsForSession returns labels for a specific session
func (li *LabelIndex) GetLabelsForSession(sessionID string) []string {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	if labels, exists := li.sessionToLabels[sessionID]; exists {
		result := make([]string, len(labels))
		copy(result, labels)
		return result
	}

	return []string{}
}

// GetK8sLabelsForSession returns K8s labels for a specific session
func (li *LabelIndex) GetK8sLabelsForSession(sessionID string) map[string]string {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	if k8sLabels, exists := li.sessionToK8sLabels[sessionID]; exists {
		result := make(map[string]string)
		for key, value := range k8sLabels {
			result[key] = value
		}
		return result
	}

	return make(map[string]string)
}

// GetAllLabels returns all unique labels in the index
func (li *LabelIndex) GetAllLabels() []string {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	labels := make([]string, 0, len(li.labelToSessions))
	for label := range li.labelToSessions {
		labels = append(labels, label)
	}

	return labels
}

// GetIndexStats returns statistics about the index
func (li *LabelIndex) GetIndexStats() IndexStats {
	li.mutex.RLock()
	defer li.mutex.RUnlock()

	return IndexStats{
		TotalSessions:     len(li.sessionToLabels),
		TotalLabels:       len(li.labelToSessions),
		TotalK8sLabels:    li.countK8sLabels(),
		CachedQueries:     len(li.queryCache),
		LastUpdated:       li.lastUpdated,
		IndexSize:         li.indexSize,
	}
}

// IndexStats represents statistics about the label index
type IndexStats struct {
	TotalSessions  int       `json:"total_sessions"`
	TotalLabels    int       `json:"total_labels"`
	TotalK8sLabels int       `json:"total_k8s_labels"`
	CachedQueries  int       `json:"cached_queries"`
	LastUpdated    time.Time `json:"last_updated"`
	IndexSize      int       `json:"index_size_bytes"`
}

// removeSessionLabelsInternal removes labels for a session (internal, assumes lock held)
func (li *LabelIndex) removeSessionLabelsInternal(sessionID string) {
	// Get existing labels for this session
	if existingLabels, exists := li.sessionToLabels[sessionID]; exists {
		// Remove session from each label's session list
		for _, label := range existingLabels {
			if sessionIDs, exists := li.labelToSessions[label]; exists {
				li.labelToSessions[label] = li.removeSessionID(sessionIDs, sessionID)
				// Remove empty label entries
				if len(li.labelToSessions[label]) == 0 {
					delete(li.labelToSessions, label)
				}
			}
		}
		// Remove session from labels mapping
		delete(li.sessionToLabels, sessionID)
	}
}

// removeSessionK8sLabelsInternal removes K8s labels for a session (internal, assumes lock held)
func (li *LabelIndex) removeSessionK8sLabelsInternal(sessionID string) {
	// Get existing K8s labels for this session
	if existingK8sLabels, exists := li.sessionToK8sLabels[sessionID]; exists {
		// Remove session from each K8s label's session list
		for key, value := range existingK8sLabels {
			if keyMap, exists := li.k8sLabelToSessions[key]; exists {
				if sessionIDs, exists := keyMap[value]; exists {
					li.k8sLabelToSessions[key][value] = li.removeSessionID(sessionIDs, sessionID)
					// Remove empty entries
					if len(li.k8sLabelToSessions[key][value]) == 0 {
						delete(li.k8sLabelToSessions[key], value)
						if len(li.k8sLabelToSessions[key]) == 0 {
							delete(li.k8sLabelToSessions, key)
						}
					}
				}
			}
		}
		// Remove session from K8s labels mapping
		delete(li.sessionToK8sLabels, sessionID)
	}
}

// addUniqueSessionID adds a session ID to a slice if it's not already present
func (li *LabelIndex) addUniqueSessionID(sessionIDs []string, sessionID string) []string {
	for _, id := range sessionIDs {
		if id == sessionID {
			return sessionIDs // Already exists
		}
	}
	return append(sessionIDs, sessionID)
}

// removeSessionID removes a session ID from a slice
func (li *LabelIndex) removeSessionID(sessionIDs []string, sessionID string) []string {
	result := make([]string, 0, len(sessionIDs))
	for _, id := range sessionIDs {
		if id != sessionID {
			result = append(result, id)
		}
	}
	return result
}

// updateIndexMetadata updates index metadata
func (li *LabelIndex) updateIndexMetadata() {
	li.lastUpdated = time.Now()
	// Simple size estimation (can be made more accurate)
	li.indexSize = len(li.sessionToLabels)*50 + len(li.labelToSessions)*20 + len(li.k8sLabelToSessions)*30
}

// invalidateQueryCache clears the query cache
func (li *LabelIndex) invalidateQueryCache() {
	li.queryCache = make(map[string]*CachedQuery)
}

// countK8sLabels counts total K8s label pairs
func (li *LabelIndex) countK8sLabels() int {
	count := 0
	for _, keyMap := range li.k8sLabelToSessions {
		count += len(keyMap)
	}
	return count
}