package security

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCVEDatabase_GetCVE(t *testing.T) {
	logger := zerolog.Nop()

	// Mock NVD response
	mockResponse := `{
		"resultsPerPage": 1,
		"startIndex": 0,
		"totalResults": 1,
		"vulnerabilities": [{
			"cve": {
				"id": "CVE-2021-44228",
				"sourceIdentifier": "security@apache.org",
				"published": "2021-12-10T10:15:09.127",
				"lastModified": "2021-12-14T12:15:18.140",
				"vulnStatus": "Analyzed",
				"descriptions": [{
					"lang": "en",
					"value": "Apache Log4j2 <=2.14.1 JNDI features used in configuration, log messages, and parameters do not protect against attacker controlled LDAP and other JNDI related endpoints."
				}],
				"metrics": {
					"cvssMetricV31": [{
						"source": "nvd@nist.gov",
						"type": "Primary",
						"cvssData": {
							"version": "3.1",
							"vectorString": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
							"baseScore": 10.0,
							"baseSeverity": "CRITICAL",
							"exploitabilityScore": 3.9,
							"impactScore": 6.0,
							"attackVector": "NETWORK",
							"attackComplexity": "LOW",
							"privilegesRequired": "NONE",
							"userInteraction": "NONE",
							"scope": "CHANGED",
							"confidentialityImpact": "HIGH",
							"integrityImpact": "HIGH",
							"availabilityImpact": "HIGH"
						}
					}]
				},
				"weaknesses": [{
					"source": "nvd@nist.gov",
					"type": "Primary",
					"description": [{
						"lang": "en",
						"value": "CWE-502"
					}]
				}],
				"references": [{
					"url": "https://logging.apache.org/log4j/2.x/security.html",
					"source": "apache.org",
					"tags": ["Vendor Advisory"]
				}]
			}
		}]
	}`

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/rest/json/cves/2.0", r.URL.Path)
		assert.Equal(t, "CVE-2021-44228", r.URL.Query().Get("cveId"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	// Create CVE database with test server
	db := NewCVEDatabase(logger, "")
	db.baseURL = server.URL + "/rest/json/cves/2.0"

	// Test GetCVE
	ctx := context.Background()
	cve, err := db.GetCVE(ctx, "CVE-2021-44228")
	require.NoError(t, err)
	assert.NotNil(t, cve)

	// Verify CVE data
	assert.Equal(t, "CVE-2021-44228", cve.ID)
	assert.Contains(t, cve.Description, "Apache Log4j2")
	assert.Equal(t, "CRITICAL", cve.Severity)
	assert.Equal(t, 10.0, cve.Score)
	assert.Equal(t, "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H", cve.Vector)
	assert.Len(t, cve.CWE, 1)
	assert.Equal(t, "CWE-502", cve.CWE[0])
	assert.Len(t, cve.References, 1)
	assert.Equal(t, "https://logging.apache.org/log4j/2.x/security.html", cve.References[0].URL)

	// Verify CVSS v3 data
	assert.NotNil(t, cve.CVSSV3)
	assert.Equal(t, "3.1", cve.CVSSV3.Version)
	assert.Equal(t, 10.0, cve.CVSSV3.BaseScore)
	assert.Equal(t, "NETWORK", cve.CVSSV3.AttackVector)
	assert.Equal(t, "LOW", cve.CVSSV3.AttackComplexity)
}

func TestCVEDatabase_GetCVE_NotFound(t *testing.T) {
	logger := zerolog.Nop()

	// Mock empty response
	mockResponse := `{
		"resultsPerPage": 0,
		"startIndex": 0,
		"totalResults": 0,
		"vulnerabilities": []
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r // unused in this test
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	db := NewCVEDatabase(logger, "")
	db.baseURL = server.URL + "/rest/json/cves/2.0"

	ctx := context.Background()
	_, err := db.GetCVE(ctx, "CVE-9999-9999")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestCVEDatabase_GetCVE_Cache(t *testing.T) {
	logger := zerolog.Nop()

	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r // unused in this test
		callCount++
		mockResponse := `{
			"resultsPerPage": 1,
			"startIndex": 0,
			"totalResults": 1,
			"vulnerabilities": [{
				"cve": {
					"id": "CVE-2021-44228",
					"sourceIdentifier": "security@apache.org",
					"published": "2021-12-10T10:15:09.127",
					"lastModified": "2021-12-14T12:15:18.140",
					"vulnStatus": "Analyzed",
					"descriptions": [{
						"lang": "en",
						"value": "Test description"
					}],
					"metrics": {},
					"references": []
				}
			}]
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	db := NewCVEDatabase(logger, "")
	db.baseURL = server.URL + "/rest/json/cves/2.0"

	ctx := context.Background()

	// First call should hit the server
	cve1, err := db.GetCVE(ctx, "CVE-2021-44228")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// Second call should use cache
	cve2, err := db.GetCVE(ctx, "CVE-2021-44228")
	require.NoError(t, err)
	assert.Equal(t, 1, callCount) // Should not increment

	assert.Equal(t, cve1.ID, cve2.ID)
	assert.Equal(t, cve1.Description, cve2.Description)
}

func TestCVEDatabase_EnrichVulnerability(t *testing.T) {
	logger := zerolog.Nop()

	mockResponse := `{
		"resultsPerPage": 1,
		"startIndex": 0,
		"totalResults": 1,
		"vulnerabilities": [{
			"cve": {
				"id": "CVE-2021-44228",
				"sourceIdentifier": "security@apache.org",
				"published": "2021-12-10T10:15:09.127",
				"lastModified": "2021-12-14T12:15:18.140",
				"vulnStatus": "Analyzed",
				"descriptions": [{
					"lang": "en",
					"value": "Detailed description from NVD"
				}],
				"metrics": {
					"cvssMetricV31": [{
						"source": "nvd@nist.gov",
						"type": "Primary",
						"cvssData": {
							"version": "3.1",
							"vectorString": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:C/C:H/I:H/A:H",
							"baseScore": 10.0,
							"baseSeverity": "CRITICAL",
							"exploitabilityScore": 3.9,
							"impactScore": 6.0
						}
					}]
				},
				"weaknesses": [{
					"source": "nvd@nist.gov",
					"type": "Primary",
					"description": [{
						"lang": "en",
						"value": "CWE-502"
					}]
				}],
				"references": [{
					"url": "https://example.com/advisory",
					"source": "vendor"
				}]
			}
		}]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r // unused in this test
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	db := NewCVEDatabase(logger, "")
	db.baseURL = server.URL + "/rest/json/cves/2.0"

	// Create a basic vulnerability
	vuln := &Vulnerability{
		VulnerabilityID:  "CVE-2021-44228",
		PkgName:          "log4j",
		InstalledVersion: "2.14.1",
		Severity:         "HIGH",
		Description:      "Short description",
		References:       []string{"https://existing.com"},
	}

	ctx := context.Background()
	err := db.EnrichVulnerability(ctx, vuln)
	require.NoError(t, err)

	// Verify enrichment
	assert.Equal(t, "Detailed description from NVD", vuln.Description)
	assert.Len(t, vuln.CWE, 1)
	assert.Equal(t, "CWE-502", vuln.CWE[0])
	assert.Equal(t, 10.0, vuln.CVSSV3.Score)
	// Note: AttackVector may not be populated in this mock test
	assert.Contains(t, vuln.References, "https://example.com/advisory")
	assert.Contains(t, vuln.References, "https://existing.com") // Original reference preserved
	assert.Equal(t, "NVD", vuln.DataSource.ID)
}

func TestCVEDatabase_EnrichVulnerability_NonCVE(t *testing.T) {
	logger := zerolog.Nop()
	db := NewCVEDatabase(logger, "")

	// Create a non-CVE vulnerability
	vuln := &Vulnerability{
		VulnerabilityID:  "GHSA-1234-5678",
		PkgName:          "example",
		InstalledVersion: "1.0.0",
		Severity:         "HIGH",
		Description:      "GitHub security advisory",
	}

	ctx := context.Background()
	err := db.EnrichVulnerability(ctx, vuln)

	// Should not error for non-CVE vulnerabilities
	assert.NoError(t, err)

	// Should not modify the vulnerability
	assert.Equal(t, "GitHub security advisory", vuln.Description)
	assert.Len(t, vuln.CWE, 0)
}

func TestCVEDatabase_APIKey(t *testing.T) {
	logger := zerolog.Nop()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify API key header
		assert.Equal(t, "test-api-key", r.Header.Get("apiKey"))

		mockResponse := `{
			"resultsPerPage": 0,
			"startIndex": 0,
			"totalResults": 0,
			"vulnerabilities": []
		}`
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	db := NewCVEDatabase(logger, "test-api-key")
	db.baseURL = server.URL + "/rest/json/cves/2.0"

	ctx := context.Background()
	_, _ = db.GetCVE(ctx, "CVE-9999-9999")
}

func TestCVEDatabase_GetCacheStats(t *testing.T) {
	logger := zerolog.Nop()
	db := NewCVEDatabase(logger, "")

	stats := db.GetCacheStats()
	assert.Equal(t, 0, stats["total_entries"])
	assert.Equal(t, 24.0, stats["ttl_hours"])
}

func TestCVEDatabase_ClearCache(t *testing.T) {
	logger := zerolog.Nop()
	db := NewCVEDatabase(logger, "")

	// Add something to cache
	cve := &CVEInfo{ID: "CVE-2021-44228", Description: "Test"}
	db.cache.set("CVE-2021-44228", cve)

	stats := db.GetCacheStats()
	assert.Equal(t, 1, stats["total_entries"])

	// Clear cache
	db.ClearCache()

	stats = db.GetCacheStats()
	assert.Equal(t, 0, stats["total_entries"])
}

func TestCVECache_Expiration(t *testing.T) {
	// Use short TTL for testing
	cache := newCVECache(10 * time.Millisecond)

	cve := &CVEInfo{ID: "CVE-2021-44228", Description: "Test"}
	cache.set("CVE-2021-44228", cve)

	// Should be available immediately
	retrieved := cache.get("CVE-2021-44228")
	assert.NotNil(t, retrieved)
	assert.Equal(t, "CVE-2021-44228", retrieved.ID)

	// Wait for expiration
	time.Sleep(20 * time.Millisecond)

	// Should be expired
	retrieved = cache.get("CVE-2021-44228")
	assert.Nil(t, retrieved)
}

func TestCVEDatabase_SearchCVEs(t *testing.T) {
	logger := zerolog.Nop()

	mockResponse := `{
		"resultsPerPage": 20,
		"startIndex": 0,
		"totalResults": 1,
		"format": "NVD_CVE",
		"version": "2.0",
		"timestamp": "2023-01-01T00:00:00.000",
		"vulnerabilities": []
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify search parameters
		assert.Equal(t, "log4j", r.URL.Query().Get("keywordSearch"))
		assert.Equal(t, "CRITICAL", r.URL.Query().Get("cvssV3Severity"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	db := NewCVEDatabase(logger, "")
	db.baseURL = server.URL + "/rest/json/cves/2.0"

	options := CVESearchOptions{
		KeywordSearch:  "log4j",
		CVSSV3Severity: "CRITICAL",
		ResultsPerPage: 20,
	}

	ctx := context.Background()
	result, err := db.SearchCVEs(ctx, options)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 20, result.ResultsPerPage)
	assert.Equal(t, 1, result.TotalResults)
	assert.Equal(t, "NVD_CVE", result.Format)
}
