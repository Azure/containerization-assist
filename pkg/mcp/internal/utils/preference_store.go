package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/Azure/container-copilot/pkg/mcp/internal/types"
	"github.com/rs/zerolog"
	bolt "go.etcd.io/bbolt"
)

// PreferenceStore manages user preferences across sessions
type PreferenceStore struct {
	db            *bolt.DB
	mutex         sync.RWMutex
	logger        zerolog.Logger
	encryptionKey []byte // 32-byte key for AES-256
}

// UserPreferenceStore is the bucket name for user preferences
const UserPreferencesBucket = "user_preferences"

// GlobalPreferences stores user defaults that persist across sessions
type GlobalPreferences struct {
	UserID    string    `json:"user_id"`
	UpdatedAt time.Time `json:"updated_at"`

	// General defaults
	DefaultOptimization string `json:"default_optimization"` // size, speed, security
	DefaultNamespace    string `json:"default_namespace"`
	DefaultReplicas     int    `json:"default_replicas"`
	PreferredRegistry   string `json:"preferred_registry"`
	DefaultServiceType  string `json:"default_service_type"` // ClusterIP, LoadBalancer, NodePort
	AutoRollbackEnabled bool   `json:"auto_rollback_enabled"`

	// Build preferences
	AlwaysUseHealthCheck bool   `json:"always_use_health_check"`
	PreferMultiStage     bool   `json:"prefer_multi_stage"`
	DefaultPlatform      string `json:"default_platform"` // linux/amd64, linux/arm64, etc.

	// Deployment preferences
	DefaultResourceLimits  types.ResourceLimits `json:"default_resource_limits"`
	PreferredCloudProvider string               `json:"preferred_cloud_provider"` // aws, gcp, azure, local

	// Per-language defaults
	LanguageDefaults map[string]LanguagePrefs `json:"language_defaults"`

	// Recently used values for smart defaults
	RecentRepositories []string `json:"recent_repositories"`
	RecentNamespaces   []string `json:"recent_namespaces"`
	RecentAppNames     []string `json:"recent_app_names"`
}

// LanguagePrefs stores language-specific preferences
type LanguagePrefs struct {
	PreferredBaseImage  string            `json:"preferred_base_image"`
	DefaultBuildTool    string            `json:"default_build_tool"` // npm, yarn, maven, gradle, etc.
	DefaultTestCommand  string            `json:"default_test_command"`
	CommonBuildArgs     map[string]string `json:"common_build_args"`
	DefaultPort         int               `json:"default_port"`
	HealthCheckEndpoint string            `json:"health_check_endpoint"`
}

// NewPreferenceStore creates a new preference store with optional encryption
func NewPreferenceStore(dbPath string, logger zerolog.Logger, encryptionPassphrase string) (*PreferenceStore, error) {
	// Try to open database with retries and longer timeout
	var db *bolt.DB
	var err error

	for i := 0; i < 3; i++ {
		db, err = bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: 5 * time.Second})
		if err == nil {
			break
		}

		// On timeout error and final retry, try to move the locked file
		if i == 2 && err == bolt.ErrTimeout {
			logger.Warn().
				Str("path", dbPath).
				Msg("Preference database appears to be locked, attempting recovery")

			// Try to move the locked database file
			backupPath := fmt.Sprintf("%s.locked.%d", dbPath, time.Now().Unix())
			if renameErr := os.Rename(dbPath, backupPath); renameErr == nil {
				logger.Warn().
					Str("old_path", dbPath).
					Str("new_path", backupPath).
					Msg("Moved locked preference database")

				// Try one more time with the moved file
				db, err = bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: 5 * time.Second})
				if err == nil {
					break
				}
			}
		}

		if i < 2 {
			logger.Warn().
				Err(err).
				Int("attempt", i+1).
				Msg("Failed to open preference database, retrying...")
			time.Sleep(time.Duration(i+1) * time.Second)
		}
	}

	if err != nil {
		return nil, fmt.Errorf("failed to open preference database: %w", err)
	}

	// Initialize bucket
	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(UserPreferencesBucket))
		return err
	})
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			// Log the close error but return the original error
			logger.Warn().Err(closeErr).Msg("Failed to close database after bucket creation error")
		}
		return nil, fmt.Errorf("failed to create preferences bucket: %w", err)
	}

	// Derive encryption key from passphrase
	var encryptionKey []byte
	if encryptionPassphrase != "" {
		hash := sha256.Sum256([]byte(encryptionPassphrase))
		encryptionKey = hash[:]
		logger.Info().Msg("Preference store encryption enabled")
	} else {
		logger.Warn().Msg("Preference store encryption disabled - consider using encryption for production")
	}

	return &PreferenceStore{
		db:            db,
		logger:        logger,
		encryptionKey: encryptionKey,
	}, nil
}

// GetUserPreferences retrieves preferences for a user
func (ps *PreferenceStore) GetUserPreferences(userID string) (*GlobalPreferences, error) {
	ps.mutex.RLock()
	defer ps.mutex.RUnlock()

	var prefs GlobalPreferences

	err := ps.db.View(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(UserPreferencesBucket))
		if bucket == nil {
			return fmt.Errorf("preferences bucket not found")
		}

		data := bucket.Get([]byte(userID))
		if data == nil {
			// Return default preferences for new user
			prefs = ps.getDefaultPreferences(userID)
			return nil
		}

		// Decrypt data if encryption is enabled
		decryptedData, err := ps.decrypt(data)
		if err != nil {
			return fmt.Errorf("failed to decrypt preferences: %w", err)
		}

		return json.Unmarshal(decryptedData, &prefs)
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get user preferences: %w", err)
	}

	return &prefs, nil
}

// SaveUserPreferences saves user preferences
func (ps *PreferenceStore) SaveUserPreferences(prefs *GlobalPreferences) error {
	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	prefs.UpdatedAt = time.Now()

	return ps.db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(UserPreferencesBucket))
		if bucket == nil {
			return fmt.Errorf("preferences bucket not found")
		}

		data, err := json.Marshal(prefs)
		if err != nil {
			return fmt.Errorf("failed to marshal preferences: %w", err)
		}

		// Encrypt data if encryption is enabled
		encryptedData, err := ps.encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt preferences: %w", err)
		}

		return bucket.Put([]byte(prefs.UserID), encryptedData)
	})
}

// UpdatePreferencesFromSession updates preferences based on session choices
func (ps *PreferenceStore) UpdatePreferencesFromSession(userID string, sessionPrefs types.UserPreferences) error {
	prefs, err := ps.GetUserPreferences(userID)
	if err != nil {
		return err
	}

	// Update with non-default values from session
	if sessionPrefs.Optimization != "" && sessionPrefs.Optimization != prefs.DefaultOptimization {
		prefs.DefaultOptimization = sessionPrefs.Optimization
	}

	if sessionPrefs.Namespace != "" && sessionPrefs.Namespace != "default" {
		prefs.DefaultNamespace = sessionPrefs.Namespace
		ps.addToRecentList(&prefs.RecentNamespaces, sessionPrefs.Namespace, 5)
	}

	if sessionPrefs.Replicas > 0 && sessionPrefs.Replicas != prefs.DefaultReplicas {
		prefs.DefaultReplicas = sessionPrefs.Replicas
	}

	if sessionPrefs.ServiceType != "" && sessionPrefs.ServiceType != prefs.DefaultServiceType {
		prefs.DefaultServiceType = sessionPrefs.ServiceType
	}

	// Update security preferences

	// Save updated preferences
	return ps.SaveUserPreferences(prefs)
}

// GetLanguageDefaults retrieves language-specific defaults
func (ps *PreferenceStore) GetLanguageDefaults(userID, language string) (LanguagePrefs, error) {
	prefs, err := ps.GetUserPreferences(userID)
	if err != nil {
		return LanguagePrefs{}, err
	}

	if langPrefs, ok := prefs.LanguageDefaults[language]; ok {
		return langPrefs, nil
	}

	// Return system defaults for language
	return ps.getSystemLanguageDefaults(language), nil
}

// UpdateLanguageDefaults updates language-specific preferences
func (ps *PreferenceStore) UpdateLanguageDefaults(userID, language string, langPrefs LanguagePrefs) error {
	prefs, err := ps.GetUserPreferences(userID)
	if err != nil {
		return err
	}

	if prefs.LanguageDefaults == nil {
		prefs.LanguageDefaults = make(map[string]LanguagePrefs)
	}

	prefs.LanguageDefaults[language] = langPrefs

	return ps.SaveUserPreferences(prefs)
}

// ApplyPreferencesToSession applies saved preferences to a new session
func (ps *PreferenceStore) ApplyPreferencesToSession(userID string, sessionPrefs *types.UserPreferences) error {
	prefs, err := ps.GetUserPreferences(userID)
	if err != nil {
		return err
	}

	// Apply saved defaults only if session doesn't already have values
	if sessionPrefs.Optimization == "" {
		sessionPrefs.Optimization = prefs.DefaultOptimization
	}

	if sessionPrefs.Namespace == "" {
		sessionPrefs.Namespace = prefs.DefaultNamespace
	}

	if sessionPrefs.Replicas == 0 {
		sessionPrefs.Replicas = prefs.DefaultReplicas
	}

	if sessionPrefs.ServiceType == "" {
		sessionPrefs.ServiceType = prefs.DefaultServiceType
	}

	// Apply resource limits if not set
	if sessionPrefs.ResourceLimits.CPULimit == "" && prefs.DefaultResourceLimits.CPULimit != "" {
		sessionPrefs.ResourceLimits = prefs.DefaultResourceLimits
	}

	// Apply security settings
	sessionPrefs.IncludeHealthCheck = sessionPrefs.IncludeHealthCheck || prefs.AlwaysUseHealthCheck
	sessionPrefs.AutoRollback = sessionPrefs.AutoRollback || prefs.AutoRollbackEnabled

	return nil
}

// GetSmartDefaults returns intelligent defaults based on recent usage
func (ps *PreferenceStore) GetSmartDefaults(userID string) (SmartDefaults, error) {
	prefs, err := ps.GetUserPreferences(userID)
	if err != nil {
		return SmartDefaults{}, err
	}

	return SmartDefaults{
		RecentNamespaces:   prefs.RecentNamespaces,
		RecentAppNames:     prefs.RecentAppNames,
		SuggestedNamespace: ps.getMostFrequent(prefs.RecentNamespaces),
		SuggestedRegistry:  prefs.PreferredRegistry,
	}, nil
}

// SmartDefaults provides intelligent suggestions based on usage patterns
type SmartDefaults struct {
	RecentNamespaces   []string `json:"recent_namespaces"`
	RecentAppNames     []string `json:"recent_app_names"`
	SuggestedNamespace string   `json:"suggested_namespace"`
	SuggestedRegistry  string   `json:"suggested_registry"`
}

// Helper methods

func (ps *PreferenceStore) getDefaultPreferences(userID string) GlobalPreferences {
	return GlobalPreferences{
		UserID:               userID,
		UpdatedAt:            time.Now(),
		DefaultOptimization:  "balanced",
		DefaultNamespace:     "default",
		DefaultReplicas:      1,
		DefaultServiceType:   "ClusterIP",
		AutoRollbackEnabled:  true,
		AlwaysUseHealthCheck: true,
		PreferMultiStage:     true,
		DefaultPlatform:      "linux/amd64",
		DefaultResourceLimits: types.ResourceLimits{
			CPURequest:    "100m",
			CPULimit:      "500m",
			MemoryRequest: "128Mi",
			MemoryLimit:   "512Mi",
		},
		LanguageDefaults:   make(map[string]LanguagePrefs),
		RecentRepositories: make([]string, 0),
		RecentNamespaces:   make([]string, 0),
		RecentAppNames:     make([]string, 0),
	}
}

func (ps *PreferenceStore) getSystemLanguageDefaults(language string) LanguagePrefs {
	defaults := map[string]LanguagePrefs{
		"Go": {
			PreferredBaseImage:  "golang:1.21-alpine",
			DefaultBuildTool:    "go",
			DefaultTestCommand:  "go test ./...",
			DefaultPort:         8080,
			HealthCheckEndpoint: "/health",
		},
		"Node.js": {
			PreferredBaseImage:  "node:20-alpine",
			DefaultBuildTool:    "npm",
			DefaultTestCommand:  "npm test",
			DefaultPort:         3000,
			HealthCheckEndpoint: "/health",
			CommonBuildArgs: map[string]string{
				"NODE_ENV": "production",
			},
		},
		"Python": {
			PreferredBaseImage:  "python:3.11-slim",
			DefaultBuildTool:    "pip",
			DefaultTestCommand:  "pytest",
			DefaultPort:         8000,
			HealthCheckEndpoint: "/health",
		},
		"Java": {
			PreferredBaseImage:  "openjdk:17-alpine",
			DefaultBuildTool:    "maven",
			DefaultTestCommand:  "mvn test",
			DefaultPort:         8080,
			HealthCheckEndpoint: "/actuator/health",
		},
	}

	if prefs, ok := defaults[language]; ok {
		return prefs
	}

	// Generic defaults
	return LanguagePrefs{
		DefaultPort:         8080,
		HealthCheckEndpoint: "/health",
	}
}

func (ps *PreferenceStore) addToRecentList(list *[]string, item string, maxSize int) {
	// Remove if already exists
	for i, existing := range *list {
		if existing == item {
			*list = append((*list)[:i], (*list)[i+1:]...)
			break
		}
	}

	// Add to front
	*list = append([]string{item}, *list...)

	// Trim to max size
	if len(*list) > maxSize {
		*list = (*list)[:maxSize]
	}
}

func (ps *PreferenceStore) getMostFrequent(items []string) string {
	if len(items) == 0 {
		return ""
	}

	// Simple heuristic: return most recent (first item)
	// Could be enhanced with frequency counting
	return items[0]
}

// encrypt encrypts data using AES-GCM if encryption is enabled
func (ps *PreferenceStore) encrypt(data []byte) ([]byte, error) {
	if ps.encryptionKey == nil {
		// No encryption - return data as-is
		return data, nil
	}

	block, err := aes.NewCipher(ps.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

// decrypt decrypts data using AES-GCM if encryption is enabled
func (ps *PreferenceStore) decrypt(data []byte) ([]byte, error) {
	if ps.encryptionKey == nil {
		// No encryption - return data as-is
		return data, nil
	}

	if len(data) < aes.BlockSize {
		return nil, fmt.Errorf("encrypted data too short")
	}

	block, err := aes.NewCipher(ps.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// Close closes the preference store
func (ps *PreferenceStore) Close() error {
	return ps.db.Close()
}
