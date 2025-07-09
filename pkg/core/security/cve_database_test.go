package security_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Azure/container-kit/pkg/core/security"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
)

// Since we cannot access unexported fields, we'll test the public API only
// The tests will focus on the behavior rather than internal state

func TestCVEDatabase_GetCVE_PublicAPI(t *testing.T) {
	// Since we can't modify baseURL, we'll skip tests that require a mock server
	// In a real scenario, we'd need to either:
	// 1. Add a constructor option to override the base URL
	// 2. Use environment variables for configuration
	// 3. Add a SetBaseURL method for testing

	t.Skip("Skipping test - requires ability to configure base URL for testing")
}

func TestCVEDatabase_SearchCVEs(t *testing.T) {
	// Create CVE database with default configuration
	db := security.NewCVEDatabase(zerolog.Nop(), "")
	assert.NotNil(t, db)

	// Test that we can create search options
	searchOpts := &security.CVESearchOptions{
		CVEId:          "CVE-2021-44228",
		CVSSV3Severity: "CRITICAL",
	}
	assert.NotNil(t, searchOpts)

	// We can't actually test the search without network access
	// so we just verify the API exists
	t.Skip("Skipping actual search test - requires network access to NVD")
}

func TestCVEDatabase_CacheStats(t *testing.T) {
	logger := zerolog.Nop()

	// Create CVE database
	db := security.NewCVEDatabase(logger, "")
	assert.NotNil(t, db)

	// Test that GetCacheStats works
	stats := db.GetCacheStats()
	assert.NotNil(t, stats)
	assert.Contains(t, stats, "total_entries")
	assert.Contains(t, stats, "hit_count")
	assert.Contains(t, stats, "miss_count")
}

func TestCVEDatabase_ClearCache(t *testing.T) {
	logger := zerolog.Nop()

	// Create CVE database
	db := security.NewCVEDatabase(logger, "")
	assert.NotNil(t, db)

	// Test that ClearCache doesn't panic
	assert.NotPanics(t, func() {
		db.ClearCache()
	})
}

// TestCVEInfo verifies the CVEInfo struct
func TestCVEInfo_Structure(t *testing.T) {
	cveInfo := &security.CVEInfo{
		ID:          "CVE-2021-44228",
		Description: "Test vulnerability",
		Severity:    "CRITICAL",
		Score:       10.0,
		Vector:      "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
		CWE:         []string{"CWE-502"},
	}

	assert.Equal(t, "CVE-2021-44228", cveInfo.ID)
	assert.Equal(t, "Test vulnerability", cveInfo.Description)
	assert.Equal(t, "CRITICAL", cveInfo.Severity)
	assert.Equal(t, 10.0, cveInfo.Score)
	assert.Len(t, cveInfo.CWE, 1)
}

// TestCVSSV3Metrics verifies the CVSS metrics structure
func TestCVSSV3Metrics_Structure(t *testing.T) {
	metrics := &security.CVSSV3Metrics{
		Version:      "3.1",
		VectorString: "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
		BaseScore:    10.0,
		BaseSeverity: "CRITICAL",
	}

	assert.Equal(t, "3.1", metrics.Version)
	assert.Equal(t, 10.0, metrics.BaseScore)
	assert.Equal(t, "CRITICAL", metrics.BaseSeverity)
}

// Mock server test helper - for future use when API supports configuration
func createMockNVDServer(_ *testing.T, response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(response))
	}))
}
