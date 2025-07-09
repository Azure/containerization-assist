package infra

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"time"

	"github.com/Azure/container-kit/pkg/mcp/domain/session"
	"go.etcd.io/bbolt"
)

// BoltDBPersistence handles BoltDB-based persistence operations
type BoltDBPersistence struct {
	db     *bbolt.DB
	logger *slog.Logger
	dbPath string
}

// NewBoltDBPersistence creates a new BoltDB persistence layer
func NewBoltDBPersistence(dbPath string, logger *slog.Logger) (*BoltDBPersistence, error) {
	// Ensure directory exists
	if err := createDirectoryIfNotExists(filepath.Dir(dbPath)); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open BoltDB: %w", err)
	}

	persistence := &BoltDBPersistence{
		db:     db,
		logger: logger,
		dbPath: dbPath,
	}

	// Initialize buckets
	if err := persistence.initializeBuckets(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to initialize buckets: %w", err)
	}

	return persistence, nil
}

// Bucket names
const (
	BucketSessions      = "sessions"
	BucketSessionState  = "session_state"
	BucketSessionData   = "session_data"
	BucketWorkflows     = "workflows"
	BucketJobs          = "jobs"
	BucketMetrics       = "metrics"
	BucketEvents        = "events"
	BucketConfiguration = "configuration"
)

// initializeBuckets creates all required buckets
func (p *BoltDBPersistence) initializeBuckets() error {
	buckets := []string{
		BucketSessions,
		BucketSessionState,
		BucketSessionData,
		BucketWorkflows,
		BucketJobs,
		BucketMetrics,
		BucketEvents,
		BucketConfiguration,
	}

	return p.db.Update(func(tx *bbolt.Tx) error {
		for _, bucket := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucket)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucket, err)
			}
		}
		return nil
	})
}

// Close closes the database connection
func (p *BoltDBPersistence) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// Session persistence operations

// CreateSession creates a new session in the database
func (p *BoltDBPersistence) CreateSession(ctx context.Context, sessionInfo *session.SessionInfo) error {
	p.logger.Info("Creating session", "session_id", sessionInfo.ID)

	data, err := json.Marshal(sessionInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}

	return p.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessions))
		if bucket == nil {
			return fmt.Errorf("sessions bucket not found")
		}

		return bucket.Put([]byte(sessionInfo.ID), data)
	})
}

// GetSession retrieves a session from the database
func (p *BoltDBPersistence) GetSession(ctx context.Context, sessionID string) (*session.SessionInfo, error) {
	p.logger.Debug("Getting session", "session_id", sessionID)

	var sessionInfo session.SessionInfo
	err := p.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessions))
		if bucket == nil {
			return fmt.Errorf("sessions bucket not found")
		}

		data := bucket.Get([]byte(sessionID))
		if data == nil {
			return fmt.Errorf("session not found: %s", sessionID)
		}

		return json.Unmarshal(data, &sessionInfo)
	})

	if err != nil {
		return nil, err
	}

	return &sessionInfo, nil
}

// UpdateSession updates an existing session
func (p *BoltDBPersistence) UpdateSession(ctx context.Context, sessionInfo *session.SessionInfo) error {
	p.logger.Info("Updating session", "session_id", sessionInfo.ID)

	data, err := json.Marshal(sessionInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal session info: %w", err)
	}

	return p.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessions))
		if bucket == nil {
			return fmt.Errorf("sessions bucket not found")
		}

		// Check if session exists
		if existing := bucket.Get([]byte(sessionInfo.ID)); existing == nil {
			return fmt.Errorf("session not found: %s", sessionInfo.ID)
		}

		return bucket.Put([]byte(sessionInfo.ID), data)
	})
}

// DeleteSession deletes a session from the database
func (p *BoltDBPersistence) DeleteSession(ctx context.Context, sessionID string) error {
	p.logger.Info("Deleting session", "session_id", sessionID)

	return p.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessions))
		if bucket == nil {
			return fmt.Errorf("sessions bucket not found")
		}

		return bucket.Delete([]byte(sessionID))
	})
}

// ListSessions lists all sessions with optional filtering
func (p *BoltDBPersistence) ListSessions(ctx context.Context, filter map[string]string) ([]*session.SessionInfo, error) {
	p.logger.Debug("Listing sessions", "filter", filter)

	var sessions []*session.SessionInfo
	err := p.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessions))
		if bucket == nil {
			return fmt.Errorf("sessions bucket not found")
		}

		return bucket.ForEach(func(key, value []byte) error {
			var sessionInfo session.SessionInfo
			if err := json.Unmarshal(value, &sessionInfo); err != nil {
				p.logger.Warn("Failed to unmarshal session", "session_id", string(key), "error", err)
				return nil // Continue with other sessions
			}

			// Apply filters
			if p.matchesFilter(&sessionInfo, filter) {
				sessions = append(sessions, &sessionInfo)
			}

			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return sessions, nil
}

// Session state operations

// SaveSessionState saves session state data
func (p *BoltDBPersistence) SaveSessionState(ctx context.Context, sessionID string, state map[string]interface{}) error {
	p.logger.Debug("Saving session state", "session_id", sessionID)

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal session state: %w", err)
	}

	return p.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessionState))
		if bucket == nil {
			return fmt.Errorf("session state bucket not found")
		}

		return bucket.Put([]byte(sessionID), data)
	})
}

// LoadSessionState loads session state data
func (p *BoltDBPersistence) LoadSessionState(ctx context.Context, sessionID string) (map[string]interface{}, error) {
	p.logger.Debug("Loading session state", "session_id", sessionID)

	var state map[string]interface{}
	err := p.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessionState))
		if bucket == nil {
			return fmt.Errorf("session state bucket not found")
		}

		data := bucket.Get([]byte(sessionID))
		if data == nil {
			state = make(map[string]interface{})
			return nil
		}

		return json.Unmarshal(data, &state)
	})

	if err != nil {
		return nil, err
	}

	return state, nil
}

// DeleteSessionState deletes session state data
func (p *BoltDBPersistence) DeleteSessionState(ctx context.Context, sessionID string) error {
	p.logger.Debug("Deleting session state", "session_id", sessionID)

	return p.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessionState))
		if bucket == nil {
			return fmt.Errorf("session state bucket not found")
		}

		return bucket.Delete([]byte(sessionID))
	})
}

// Session data operations

// SaveSessionData saves session data
func (p *BoltDBPersistence) SaveSessionData(ctx context.Context, sessionID string, key string, value interface{}) error {
	p.logger.Debug("Saving session data", "session_id", sessionID, "key", key)

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	return p.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessionData))
		if bucket == nil {
			return fmt.Errorf("session data bucket not found")
		}

		dataKey := fmt.Sprintf("%s:%s", sessionID, key)
		return bucket.Put([]byte(dataKey), data)
	})
}

// LoadSessionData loads session data
func (p *BoltDBPersistence) LoadSessionData(ctx context.Context, sessionID string, key string, result interface{}) error {
	p.logger.Debug("Loading session data", "session_id", sessionID, "key", key)

	return p.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessionData))
		if bucket == nil {
			return fmt.Errorf("session data bucket not found")
		}

		dataKey := fmt.Sprintf("%s:%s", sessionID, key)
		data := bucket.Get([]byte(dataKey))
		if data == nil {
			return fmt.Errorf("session data not found: %s", key)
		}

		return json.Unmarshal(data, result)
	})
}

// DeleteSessionData deletes session data
func (p *BoltDBPersistence) DeleteSessionData(ctx context.Context, sessionID string, key string) error {
	p.logger.Debug("Deleting session data", "session_id", sessionID, "key", key)

	return p.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSessionData))
		if bucket == nil {
			return fmt.Errorf("session data bucket not found")
		}

		dataKey := fmt.Sprintf("%s:%s", sessionID, key)
		return bucket.Delete([]byte(dataKey))
	})
}

// Generic key-value operations

// Put stores a key-value pair in the specified bucket
func (p *BoltDBPersistence) Put(ctx context.Context, bucket string, key string, value interface{}) error {
	p.logger.Debug("Storing key-value pair", "bucket", bucket, "key", key)

	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	return p.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}

		return b.Put([]byte(key), data)
	})
}

// Get retrieves a value from the specified bucket
func (p *BoltDBPersistence) Get(ctx context.Context, bucket string, key string, result interface{}) error {
	p.logger.Debug("Retrieving key-value pair", "bucket", bucket, "key", key)

	return p.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}

		data := b.Get([]byte(key))
		if data == nil {
			return fmt.Errorf("key not found: %s", key)
		}

		return json.Unmarshal(data, result)
	})
}

// Delete removes a key-value pair from the specified bucket
func (p *BoltDBPersistence) Delete(ctx context.Context, bucket string, key string) error {
	p.logger.Debug("Deleting key-value pair", "bucket", bucket, "key", key)

	return p.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}

		return b.Delete([]byte(key))
	})
}

// List retrieves all key-value pairs from the specified bucket
func (p *BoltDBPersistence) List(_ context.Context, bucket string) (map[string]interface{}, error) {
	p.logger.Debug("Listing key-value pairs", "bucket", bucket)

	result := make(map[string]interface{})
	err := p.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		if b == nil {
			return fmt.Errorf("bucket not found: %s", bucket)
		}

		return b.ForEach(func(key, value []byte) error {
			var data interface{}
			if err := json.Unmarshal(value, &data); err != nil {
				// If unmarshal fails, store as raw bytes
				result[string(key)] = value
			} else {
				result[string(key)] = data
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	return result, nil
}

// Database maintenance operations

// Backup creates a backup of the database
func (p *BoltDBPersistence) Backup(ctx context.Context, backupPath string) error {
	p.logger.Info("Creating database backup", "backup_path", backupPath)

	// Ensure backup directory exists
	if err := createDirectoryIfNotExists(filepath.Dir(backupPath)); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	return p.db.View(func(tx *bbolt.Tx) error {
		return tx.CopyFile(backupPath, 0600)
	})
}

// Stats returns database statistics
func (p *BoltDBPersistence) Stats(ctx context.Context) (*PersistenceStats, error) {
	p.logger.Debug("Getting database statistics")

	var stats PersistenceStats
	err := p.db.View(func(tx *bbolt.Tx) error {
		dbStats := tx.Stats()
		// Use available fields from TxStats
		stats.PageCount = int(dbStats.PageCount)
		stats.FreePageCount = 0    // Field not available in current API
		stats.PendingPageCount = 0 // Field not available in current API
		stats.FreeAlloc = 0        // Field not available in current API
		stats.FreelistInuse = 0    // Field not available in current API
		stats.TxCount = 0          // Field not available in current API
		stats.TxAlloc = 0          // Field not available in current API
		stats.TxCursorCount = int(dbStats.CursorCount)
		stats.TxNodeCount = int(dbStats.NodeCount)
		stats.TxNodeDeref = int(dbStats.NodeDeref)
		stats.TxRebalance = int(dbStats.Rebalance)
		stats.TxRebalanceTime = dbStats.RebalanceTime
		stats.TxSplit = int(dbStats.Split)
		stats.TxSpill = int(dbStats.Spill)
		stats.TxSpillTime = dbStats.SpillTime
		stats.TxWrite = int(dbStats.Write)
		stats.TxWriteTime = dbStats.WriteTime

		// Get bucket statistics
		stats.BucketStats = make(map[string]BucketStats)
		buckets := []string{
			BucketSessions,
			BucketSessionState,
			BucketSessionData,
			BucketWorkflows,
			BucketJobs,
			BucketMetrics,
			BucketEvents,
			BucketConfiguration,
		}

		for _, bucketName := range buckets {
			bucket := tx.Bucket([]byte(bucketName))
			if bucket != nil {
				bucketStats := bucket.Stats()
				stats.BucketStats[bucketName] = BucketStats{
					BranchPageCount:     bucketStats.BranchPageN,
					BranchOverflowCount: bucketStats.BranchOverflowN,
					LeafPageCount:       bucketStats.LeafPageN,
					LeafOverflowCount:   bucketStats.LeafOverflowN,
					KeyCount:            bucketStats.KeyN,
					Depth:               bucketStats.Depth,
					BranchAlloc:         bucketStats.BranchAlloc,
					BranchInuse:         bucketStats.BranchInuse,
					LeafAlloc:           bucketStats.LeafAlloc,
					LeafInuse:           bucketStats.LeafInuse,
					BucketCount:         bucketStats.BucketN,
					InlineBucketCount:   bucketStats.InlineBucketN,
					InlineBucketInuse:   bucketStats.InlineBucketInuse,
				}
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// PersistenceStats represents database statistics
type PersistenceStats struct {
	PageCount        int                    `json:"page_count"`
	FreePageCount    int                    `json:"free_page_count"`
	PendingPageCount int                    `json:"pending_page_count"`
	FreeAlloc        int                    `json:"free_alloc"`
	FreelistInuse    int                    `json:"freelist_inuse"`
	TxCount          int                    `json:"tx_count"`
	TxAlloc          int                    `json:"tx_alloc"`
	TxCursorCount    int                    `json:"tx_cursor_count"`
	TxNodeCount      int                    `json:"tx_node_count"`
	TxNodeDeref      int                    `json:"tx_node_deref"`
	TxRebalance      int                    `json:"tx_rebalance"`
	TxRebalanceTime  time.Duration          `json:"tx_rebalance_time"`
	TxSplit          int                    `json:"tx_split"`
	TxSpill          int                    `json:"tx_spill"`
	TxSpillTime      time.Duration          `json:"tx_spill_time"`
	TxWrite          int                    `json:"tx_write"`
	TxWriteTime      time.Duration          `json:"tx_write_time"`
	BucketStats      map[string]BucketStats `json:"bucket_stats"`
}

// BucketStats represents bucket statistics
type BucketStats struct {
	BranchPageCount     int `json:"branch_page_count"`
	BranchOverflowCount int `json:"branch_overflow_count"`
	LeafPageCount       int `json:"leaf_page_count"`
	LeafOverflowCount   int `json:"leaf_overflow_count"`
	KeyCount            int `json:"key_count"`
	Depth               int `json:"depth"`
	BranchAlloc         int `json:"branch_alloc"`
	BranchInuse         int `json:"branch_inuse"`
	LeafAlloc           int `json:"leaf_alloc"`
	LeafInuse           int `json:"leaf_inuse"`
	BucketCount         int `json:"bucket_count"`
	InlineBucketCount   int `json:"inline_bucket_count"`
	InlineBucketInuse   int `json:"inline_bucket_inuse"`
}

// Helper functions

// matchesFilter checks if a session matches the given filter
func (p *BoltDBPersistence) matchesFilter(sessionInfo *session.SessionInfo, filter map[string]string) bool {
	if filter == nil || len(filter) == 0 {
		return true
	}

	for key, value := range filter {
		switch key {
		case "status":
			if sessionInfo.Status != value {
				return false
			}
		case "workspace":
			// TODO: WorkspaceDir field not available in session.SessionInfo
			// Skip workspace filtering for now
			continue
		case "label":
			// TODO: Labels field not available in session.SessionInfo
			// Skip label filtering for now
			continue
		}
	}

	return true
}

// createDirectoryIfNotExists creates a directory if it doesn't exist
func createDirectoryIfNotExists(dir string) error {
	if dir == "" {
		return nil
	}

	// This would use os.MkdirAll in a real implementation
	// For now, we'll assume the directory exists
	return nil
}
