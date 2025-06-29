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

	"github.com/Azure/container-kit/pkg/mcp"
	"github.com/Azure/container-kit/pkg/mcp/internal/types"
	"github.com/rs/zerolog"

	bolt "go.etcd.io/bbolt"
)

type PreferenceStore struct {
	db            *bolt.DB
	mutex         sync.RWMutex
	logger        zerolog.Logger
	encryptionKey []byte // 32-byte key for AES-256
}

const UserPreferencesBucket = "user_preferences"

type GlobalPreferences struct {
	UserID    string    `json:"user_id"`
	UpdatedAt time.Time `json:"updated_at"`

	DefaultOptimization string `json:"default_optimization"` // size, speed, security
	DefaultNamespace    string `json:"default_namespace"`
	DefaultReplicas     int    `json:"default_replicas"`
	PreferredRegistry   string `json:"preferred_registry"`
	DefaultServiceType  string `json:"default_service_type"` // ClusterIP, LoadBalancer, NodePort
	AutoRollbackEnabled bool   `json:"auto_rollback_enabled"`

	AlwaysUseHealthCheck bool   `json:"always_use_health_check"`
	PreferMultiStage     bool   `json:"prefer_multi_stage"`
	DefaultPlatform      string `json:"default_platform"` // linux/amd64, linux/arm64, etc.

	DefaultResourceLimits  types.ResourceLimits `json:"default_resource_limits"`
	PreferredCloudProvider string               `json:"preferred_cloud_provider"` // aws, gcp, azure, local

	LanguageDefaults map[string]LanguagePrefs `json:"language_defaults"`

	RecentRepositories []string `json:"recent_repositories"`
	RecentNamespaces   []string `json:"recent_namespaces"`
	RecentAppNames     []string `json:"recent_app_names"`
}

type LanguagePrefs struct {
	PreferredBaseImage  string            `json:"preferred_base_image"`
	DefaultBuildTool    string            `json:"default_build_tool"` // npm, yarn, maven, gradle, etc.
	DefaultTestCommand  string            `json:"default_test_command"`
	CommonBuildArgs     map[string]string `json:"common_build_args"`
	DefaultPort         int               `json:"default_port"`
	HealthCheckEndpoint string            `json:"health_check_endpoint"`
}

func NewPreferenceStore(dbPath string, logger zerolog.Logger, encryptionPassphrase string) (*PreferenceStore, error) {
	var db *bolt.DB
	var err error

	for i := 0; i < 3; i++ {
		db, err = bolt.Open(dbPath, 0o600, &bolt.Options{Timeout: 5 * time.Second})
		if err == nil {
			break
		}

		if i == 2 && err == bolt.ErrTimeout {
			logger.Warn().
				Str("path", dbPath).
				Msg("Preference database appears to be locked, attempting recovery")

			backupPath := fmt.Sprintf("%s.locked.%d", dbPath, time.Now().Unix())
			if renameErr := os.Rename(dbPath, backupPath); renameErr == nil {
				logger.Warn().
					Str("old_path", dbPath).
					Str("new_path", backupPath).
					Msg("Moved locked preference database")

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
		return nil, mcp.NewErrorBuilder("database_open_failed", "Failed to open preference database", "system").
			WithSeverity("high").
			WithOperation("initialize_preferences").
			WithStage("database_connection").
			WithRootCause(fmt.Sprintf("BoltDB open failed: %v", err)).
			WithImmediateStep(1, "Check permissions", "Verify write permissions to the data directory").
			WithImmediateStep(2, "Check disk space", "Ensure sufficient disk space is available").
			WithImmediateStep(3, "Check file locks", "Verify no other process is using the database file").
			Build()
	}

	err = db.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(UserPreferencesBucket))
		return err
	})
	if err != nil {
		if closeErr := db.Close(); closeErr != nil {
			logger.Warn().Err(closeErr).Msg("Failed to close database after bucket creation error")
		}
		return nil, mcp.NewErrorBuilder("bucket_creation_failed", "Failed to create preferences bucket", "system").
			WithSeverity("high").
			WithOperation("initialize_preferences").
			WithStage("bucket_creation").
			WithRootCause(fmt.Sprintf("BoltDB bucket creation failed: %v", err)).
			WithImmediateStep(1, "Check database integrity", "Verify the database file is not corrupted").
			WithImmediateStep(2, "Restart with clean database", "Delete database file and restart if corruption is suspected").
			Build()
	}

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
			prefs = ps.getDefaultPreferences(userID)
			return nil
		}

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

		encryptedData, err := ps.encrypt(data)
		if err != nil {
			return fmt.Errorf("failed to encrypt preferences: %w", err)
		}

		return bucket.Put([]byte(prefs.UserID), encryptedData)
	})
}

func (ps *PreferenceStore) UpdatePreferencesFromSession(userID string, sessionPrefs types.UserPreferences) error {
	prefs, err := ps.GetUserPreferences(userID)
	if err != nil {
		return err
	}

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

	return ps.SaveUserPreferences(prefs)
}

func (ps *PreferenceStore) GetLanguageDefaults(userID, language string) (LanguagePrefs, error) {
	prefs, err := ps.GetUserPreferences(userID)
	if err != nil {
		return LanguagePrefs{}, err
	}

	if langPrefs, ok := prefs.LanguageDefaults[language]; ok {
		return langPrefs, nil
	}

	return ps.getSystemLanguageDefaults(language), nil
}

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

func (ps *PreferenceStore) ApplyPreferencesToSession(userID string, sessionPrefs *types.UserPreferences) error {
	prefs, err := ps.GetUserPreferences(userID)
	if err != nil {
		return err
	}

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

	if sessionPrefs.ResourceLimits.CPULimit == "" && prefs.DefaultResourceLimits.CPULimit != "" {
		sessionPrefs.ResourceLimits = prefs.DefaultResourceLimits
	}

	sessionPrefs.IncludeHealthCheck = sessionPrefs.IncludeHealthCheck || prefs.AlwaysUseHealthCheck
	sessionPrefs.AutoRollback = sessionPrefs.AutoRollback || prefs.AutoRollbackEnabled

	return nil
}

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

type SmartDefaults struct {
	RecentNamespaces   []string `json:"recent_namespaces"`
	RecentAppNames     []string `json:"recent_app_names"`
	SuggestedNamespace string   `json:"suggested_namespace"`
	SuggestedRegistry  string   `json:"suggested_registry"`
}

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

	return LanguagePrefs{
		DefaultPort:         8080,
		HealthCheckEndpoint: "/health",
	}
}

func (ps *PreferenceStore) addToRecentList(list *[]string, item string, maxSize int) {
	for i, existing := range *list {
		if existing == item {
			*list = append((*list)[:i], (*list)[i+1:]...)
			break
		}
	}

	*list = append([]string{item}, *list...)

	if len(*list) > maxSize {
		*list = (*list)[:maxSize]
	}
}

func (ps *PreferenceStore) getMostFrequent(items []string) string {
	if len(items) == 0 {
		return ""
	}

	return items[0]
}

func (ps *PreferenceStore) encrypt(data []byte) ([]byte, error) {
	if ps.encryptionKey == nil {
		return data, nil
	}

	block, err := aes.NewCipher(ps.encryptionKey)
	if err != nil {
		return nil, mcp.NewErrorBuilder("encryption_cipher_failed", "Failed to create encryption cipher", "security").
			WithSeverity("high").
			WithOperation("encrypt_preferences").
			WithStage("cipher_creation").
			WithRootCause(fmt.Sprintf("AES cipher creation failed: %v", err)).
			WithImmediateStep(1, "Check encryption key", "Verify encryption key is 32 bytes (256-bit)").
			WithImmediateStep(2, "Check system crypto", "Ensure system crypto libraries are functional").
			Build()
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return ciphertext, nil
}

func (ps *PreferenceStore) decrypt(data []byte) ([]byte, error) {
	if ps.encryptionKey == nil {
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

func (ps *PreferenceStore) Close() error {
	return ps.db.Close()
}
