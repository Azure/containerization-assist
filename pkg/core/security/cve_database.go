// Package security provides comprehensive vulnerability database integration
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	mcperrors "github.com/Azure/container-kit/pkg/mcp/errors"
	"github.com/rs/zerolog"
)

// NVDTime represents time format used by NVD API
type NVDTime struct {
	time.Time
}

// UnmarshalJSON implements json.Unmarshaler for NVD time format
func (nt *NVDTime) UnmarshalJSON(data []byte) error {
	str := strings.Trim(string(data), `"`)
	if str == "null" || str == "" {
		return nil
	}

	// NVD uses format like "2021-12-10T10:15:09.127"
	t, err := time.Parse("2006-01-02T15:04:05.000", str)
	if err != nil {
		// Try alternative format without milliseconds
		t, err = time.Parse("2006-01-02T15:04:05", str)
		if err != nil {
			return err
		}
	}
	nt.Time = t
	return nil
}

// CVEDatabase provides access to vulnerability database information
type CVEDatabase struct {
	logger     zerolog.Logger
	httpClient *http.Client
	cache      *cveCache
	baseURL    string
	apiKey     string
}

// NewCVEDatabase creates a new CVE database client
func NewCVEDatabase(logger zerolog.Logger, apiKey string) *CVEDatabase {
	return &CVEDatabase{
		logger: logger.With().Str("component", "cve_database").Logger(),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		cache:   newCVECache(24 * time.Hour), // Cache for 24 hours
		baseURL: "https://services.nvd.nist.gov/rest/json/cves/2.0",
		apiKey:  apiKey,
	}
}

// CVEInfo represents detailed CVE information from NIST NVD
type CVEInfo struct {
	ID                     string          `json:"id"`
	Description            string          `json:"description"`
	PublishedDate          time.Time       `json:"published_date"`
	LastModifiedDate       time.Time       `json:"last_modified_date"`
	CVSSV3                 *CVSSV3Metrics  `json:"cvss_v3,omitempty"`
	CVSSV2                 *CVSSV2Metrics  `json:"cvss_v2,omitempty"`
	Severity               string          `json:"severity"`
	Score                  float64         `json:"score"`
	Vector                 string          `json:"vector,omitempty"`
	References             []CVEReference  `json:"references"`
	CPEMatches             []CPEMatch      `json:"cpe_matches,omitempty"`
	CWE                    []string        `json:"cwe,omitempty"`
	VendorComments         []VendorComment `json:"vendor_comments,omitempty"`
	Configurations         []Configuration `json:"configurations,omitempty"`
	Impact                 ImpactMetrics   `json:"impact"`
	ExploitabilitySubScore float64         `json:"exploitability_subscore,omitempty"`
	ImpactSubScore         float64         `json:"impact_subscore,omitempty"`
	Source                 string          `json:"source"`
	Type                   string          `json:"type"`
	AssignerOrgID          string          `json:"assigner_org_id,omitempty"`
	AssignerShortName      string          `json:"assigner_short_name,omitempty"`
}

// CVSSV3Metrics represents CVSS v3 metrics
type CVSSV3Metrics struct {
	Version               string  `json:"version"`
	VectorString          string  `json:"vector_string"`
	BaseScore             float64 `json:"base_score"`
	BaseSeverity          string  `json:"base_severity"`
	ExploitabilityScore   float64 `json:"exploitability_score,omitempty"`
	ImpactScore           float64 `json:"impact_score,omitempty"`
	AttackVector          string  `json:"attack_vector,omitempty"`
	AttackComplexity      string  `json:"attack_complexity,omitempty"`
	PrivilegesRequired    string  `json:"privileges_required,omitempty"`
	UserInteraction       string  `json:"user_interaction,omitempty"`
	Scope                 string  `json:"scope,omitempty"`
	ConfidentialityImpact string  `json:"confidentiality_impact,omitempty"`
	IntegrityImpact       string  `json:"integrity_impact,omitempty"`
	AvailabilityImpact    string  `json:"availability_impact,omitempty"`
}

// CVSSV2Metrics represents CVSS v2 metrics
type CVSSV2Metrics struct {
	Version                 string  `json:"version"`
	VectorString            string  `json:"vector_string"`
	BaseScore               float64 `json:"base_score"`
	BaseSeverity            string  `json:"base_severity,omitempty"`
	ExploitabilityScore     float64 `json:"exploitability_score,omitempty"`
	ImpactScore             float64 `json:"impact_score,omitempty"`
	AcInsufInfo             bool    `json:"ac_insuf_info,omitempty"`
	ObtainAllPrivilege      bool    `json:"obtain_all_privilege,omitempty"`
	ObtainUserPrivilege     bool    `json:"obtain_user_privilege,omitempty"`
	ObtainOtherPrivilege    bool    `json:"obtain_other_privilege,omitempty"`
	UserInteractionRequired bool    `json:"user_interaction_required,omitempty"`
	AccessVector            string  `json:"access_vector,omitempty"`
	AccessComplexity        string  `json:"access_complexity,omitempty"`
	Authentication          string  `json:"authentication,omitempty"`
	ConfidentialityImpact   string  `json:"confidentiality_impact,omitempty"`
	IntegrityImpact         string  `json:"integrity_impact,omitempty"`
	AvailabilityImpact      string  `json:"availability_impact,omitempty"`
}

// CVEReference represents a reference to external information
type CVEReference struct {
	URL    string   `json:"url"`
	Source string   `json:"source"`
	Tags   []string `json:"tags,omitempty"`
}

// CPEMatch represents CPE (Common Platform Enumeration) matching criteria
type CPEMatch struct {
	Vulnerable            bool   `json:"vulnerable"`
	Criteria              string `json:"criteria"`
	MatchCriteriaID       string `json:"match_criteria_id,omitempty"`
	VersionStartIncluding string `json:"version_start_including,omitempty"`
	VersionStartExcluding string `json:"version_start_excluding,omitempty"`
	VersionEndIncluding   string `json:"version_end_including,omitempty"`
	VersionEndExcluding   string `json:"version_end_excluding,omitempty"`
}

// VendorComment represents vendor-specific comments
type VendorComment struct {
	Organization string    `json:"organization"`
	Comment      string    `json:"comment"`
	LastModified time.Time `json:"last_modified"`
}

// Configuration represents vulnerability configuration
type Configuration struct {
	Nodes []ConfigNode `json:"nodes"`
}

// ConfigNode represents a configuration node
type ConfigNode struct {
	Operator string     `json:"operator"`
	Negate   bool       `json:"negate,omitempty"`
	CPEMatch []CPEMatch `json:"cpe_match"`
}

// ImpactMetrics represents impact scoring
type ImpactMetrics struct {
	BaseMetricV3 *BaseMetricV3 `json:"base_metric_v3,omitempty"`
	BaseMetricV2 *BaseMetricV2 `json:"base_metric_v2,omitempty"`
}

// BaseMetricV3 represents base metrics for CVSS v3
type BaseMetricV3 struct {
	CVSSV3              CVSSV3Metrics `json:"cvss_v3"`
	ExploitabilityScore float64       `json:"exploitability_score"`
	ImpactScore         float64       `json:"impact_score"`
}

// BaseMetricV2 represents base metrics for CVSS v2
type BaseMetricV2 struct {
	CVSSV2                  CVSSV2Metrics `json:"cvss_v2"`
	Severity                string        `json:"severity"`
	ExploitabilityScore     float64       `json:"exploitability_score"`
	ImpactScore             float64       `json:"impact_score"`
	AcInsufInfo             bool          `json:"ac_insuf_info"`
	ObtainAllPrivilege      bool          `json:"obtain_all_privilege"`
	ObtainUserPrivilege     bool          `json:"obtain_user_privilege"`
	ObtainOtherPrivilege    bool          `json:"obtain_other_privilege"`
	UserInteractionRequired bool          `json:"user_interaction_required"`
}

// CVESearchOptions configures CVE search parameters
type CVESearchOptions struct {
	CVEId             string
	CPEName           string
	KeywordSearch     string
	KeywordExactMatch bool
	LastModStartDate  *time.Time
	LastModEndDate    *time.Time
	PubStartDate      *time.Time
	PubEndDate        *time.Time
	CVSSV2Severity    string
	CVSSV3Severity    string
	CVSSScoreMin      *float64
	CVSSScoreMax      *float64
	CWE               string
	HasCertAlerts     bool
	HasCertNotes      bool
	HasKev            bool
	HasOval           bool
	IsVulnerable      bool
	NoRejected        bool
	ResultsPerPage    int
	StartIndex        int
}

// CVESearchResult represents the response from CVE search
type CVESearchResult struct {
	ResultsPerPage  int       `json:"resultsPerPage"`
	StartIndex      int       `json:"startIndex"`
	TotalResults    int       `json:"totalResults"`
	Format          string    `json:"format"`
	Version         string    `json:"version"`
	Timestamp       NVDTime   `json:"timestamp"`
	Vulnerabilities []CVEInfo `json:"vulnerabilities"`
}

// GetCVE retrieves detailed information for a specific CVE ID
func (db *CVEDatabase) GetCVE(ctx context.Context, cveID string) (*CVEInfo, error) {
	// Check cache first
	if cached := db.cache.get(cveID); cached != nil {
		db.logger.Debug().Str("cve_id", cveID).Msg("Returning cached CVE data")
		return cached, nil
	}

	db.logger.Info().Str("cve_id", cveID).Msg("Fetching CVE data from NVD")

	// Build URL for specific CVE
	reqURL := fmt.Sprintf("%s?cveId=%s", db.baseURL, url.QueryEscape(cveID))

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "security", "failed to create request", err)
	}

	if db.apiKey != "" {
		req.Header.Set("apiKey", db.apiKey)
	}
	req.Header.Set("User-Agent", "container-kit-security/1.0")

	resp, err := db.httpClient.Do(req)
	if err != nil {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "core", "failed to fetch CVE data", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, mcperrors.New(mcperrors.CodeInternalError, "core", "NVD API returned status %d", nil)
	}

	var nvdResponse struct {
		ResultsPerPage  int `json:"resultsPerPage"`
		StartIndex      int `json:"startIndex"`
		TotalResults    int `json:"totalResults"`
		Vulnerabilities []struct {
			CVE struct {
				ID               string  `json:"id"`
				SourceIdentifier string  `json:"sourceIdentifier"`
				Published        NVDTime `json:"published"`
				LastModified     NVDTime `json:"lastModified"`
				VulnStatus       string  `json:"vulnStatus"`
				Descriptions     []struct {
					Lang  string `json:"lang"`
					Value string `json:"value"`
				} `json:"descriptions"`
				Metrics struct {
					CvssMetricV31 []struct {
						Source   string `json:"source"`
						Type     string `json:"type"`
						CvssData struct {
							Version               string  `json:"version"`
							VectorString          string  `json:"vectorString"`
							BaseScore             float64 `json:"baseScore"`
							BaseSeverity          string  `json:"baseSeverity"`
							ExploitabilityScore   float64 `json:"exploitabilityScore"`
							ImpactScore           float64 `json:"impactScore"`
							AttackVector          string  `json:"attackVector"`
							AttackComplexity      string  `json:"attackComplexity"`
							PrivilegesRequired    string  `json:"privilegesRequired"`
							UserInteraction       string  `json:"userInteraction"`
							Scope                 string  `json:"scope"`
							ConfidentialityImpact string  `json:"confidentialityImpact"`
							IntegrityImpact       string  `json:"integrityImpact"`
							AvailabilityImpact    string  `json:"availabilityImpact"`
						} `json:"cvssData"`
					} `json:"cvssMetricV31,omitempty"`
					CvssMetricV30 []struct {
						Source   string `json:"source"`
						Type     string `json:"type"`
						CvssData struct {
							Version               string  `json:"version"`
							VectorString          string  `json:"vectorString"`
							BaseScore             float64 `json:"baseScore"`
							BaseSeverity          string  `json:"baseSeverity"`
							ExploitabilityScore   float64 `json:"exploitabilityScore"`
							ImpactScore           float64 `json:"impactScore"`
							AttackVector          string  `json:"attackVector"`
							AttackComplexity      string  `json:"attackComplexity"`
							PrivilegesRequired    string  `json:"privilegesRequired"`
							UserInteraction       string  `json:"userInteraction"`
							Scope                 string  `json:"scope"`
							ConfidentialityImpact string  `json:"confidentialityImpact"`
							IntegrityImpact       string  `json:"integrityImpact"`
							AvailabilityImpact    string  `json:"availabilityImpact"`
						} `json:"cvssData"`
					} `json:"cvssMetricV30,omitempty"`
					CvssMetricV2 []struct {
						Source   string `json:"source"`
						Type     string `json:"type"`
						CvssData struct {
							Version                 string  `json:"version"`
							VectorString            string  `json:"vectorString"`
							BaseScore               float64 `json:"baseScore"`
							AccessVector            string  `json:"accessVector"`
							AccessComplexity        string  `json:"accessComplexity"`
							Authentication          string  `json:"authentication"`
							ConfidentialityImpact   string  `json:"confidentialityImpact"`
							IntegrityImpact         string  `json:"integrityImpact"`
							AvailabilityImpact      string  `json:"availabilityImpact"`
							ExploitabilityScore     float64 `json:"exploitabilityScore"`
							ImpactScore             float64 `json:"impactScore"`
							AcInsufInfo             bool    `json:"acInsufInfo"`
							ObtainAllPrivilege      bool    `json:"obtainAllPrivilege"`
							ObtainUserPrivilege     bool    `json:"obtainUserPrivilege"`
							ObtainOtherPrivilege    bool    `json:"obtainOtherPrivilege"`
							UserInteractionRequired bool    `json:"userInteractionRequired"`
						} `json:"cvssData"`
						BaseSeverity string `json:"baseSeverity"`
					} `json:"cvssMetricV2,omitempty"`
				} `json:"metrics"`
				Weaknesses []struct {
					Source      string `json:"source"`
					Type        string `json:"type"`
					Description []struct {
						Lang  string `json:"lang"`
						Value string `json:"value"`
					} `json:"description"`
				} `json:"weaknesses,omitempty"`
				References []struct {
					URL    string   `json:"url"`
					Source string   `json:"source,omitempty"`
					Tags   []string `json:"tags,omitempty"`
				} `json:"references"`
			} `json:"cve"`
		} `json:"vulnerabilities"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&nvdResponse); err != nil {
		return nil, mcperrors.New(mcperrors.CodeOperationFailed, "core", "failed to decode NVD response", err)
	}

	if len(nvdResponse.Vulnerabilities) == 0 {
		return nil, mcperrors.New(mcperrors.CodeNotFound, "security", fmt.Sprintf("CVE %s not found", cveID), nil)
	}

	nvdCVE := nvdResponse.Vulnerabilities[0].CVE
	cveInfo := &CVEInfo{
		ID:               nvdCVE.ID,
		PublishedDate:    nvdCVE.Published.Time,
		LastModifiedDate: nvdCVE.LastModified.Time,
		Source:           nvdCVE.SourceIdentifier,
		Type:             nvdCVE.VulnStatus,
	}

	// Extract description (prefer English)
	for _, desc := range nvdCVE.Descriptions {
		if desc.Lang == "en" {
			cveInfo.Description = desc.Value
			break
		}
	}
	if cveInfo.Description == "" && len(nvdCVE.Descriptions) > 0 {
		cveInfo.Description = nvdCVE.Descriptions[0].Value
	}

	// Extract CVSS v3.1 metrics (preferred)
	if len(nvdCVE.Metrics.CvssMetricV31) > 0 {
		metric := nvdCVE.Metrics.CvssMetricV31[0]
		cveInfo.CVSSV3 = &CVSSV3Metrics{
			Version:               metric.CvssData.Version,
			VectorString:          metric.CvssData.VectorString,
			BaseScore:             metric.CvssData.BaseScore,
			BaseSeverity:          metric.CvssData.BaseSeverity,
			ExploitabilityScore:   metric.CvssData.ExploitabilityScore,
			ImpactScore:           metric.CvssData.ImpactScore,
			AttackVector:          metric.CvssData.AttackVector,
			AttackComplexity:      metric.CvssData.AttackComplexity,
			PrivilegesRequired:    metric.CvssData.PrivilegesRequired,
			UserInteraction:       metric.CvssData.UserInteraction,
			Scope:                 metric.CvssData.Scope,
			ConfidentialityImpact: metric.CvssData.ConfidentialityImpact,
			IntegrityImpact:       metric.CvssData.IntegrityImpact,
			AvailabilityImpact:    metric.CvssData.AvailabilityImpact,
		}
		cveInfo.Score = metric.CvssData.BaseScore
		cveInfo.Severity = metric.CvssData.BaseSeverity
		cveInfo.Vector = metric.CvssData.VectorString
		cveInfo.ExploitabilitySubScore = metric.CvssData.ExploitabilityScore
		cveInfo.ImpactSubScore = metric.CvssData.ImpactScore
	} else if len(nvdCVE.Metrics.CvssMetricV30) > 0 {
		// Fall back to CVSS v3.0
		metric := nvdCVE.Metrics.CvssMetricV30[0]
		cveInfo.CVSSV3 = &CVSSV3Metrics{
			Version:               metric.CvssData.Version,
			VectorString:          metric.CvssData.VectorString,
			BaseScore:             metric.CvssData.BaseScore,
			BaseSeverity:          metric.CvssData.BaseSeverity,
			ExploitabilityScore:   metric.CvssData.ExploitabilityScore,
			ImpactScore:           metric.CvssData.ImpactScore,
			AttackVector:          metric.CvssData.AttackVector,
			AttackComplexity:      metric.CvssData.AttackComplexity,
			PrivilegesRequired:    metric.CvssData.PrivilegesRequired,
			UserInteraction:       metric.CvssData.UserInteraction,
			Scope:                 metric.CvssData.Scope,
			ConfidentialityImpact: metric.CvssData.ConfidentialityImpact,
			IntegrityImpact:       metric.CvssData.IntegrityImpact,
			AvailabilityImpact:    metric.CvssData.AvailabilityImpact,
		}
		cveInfo.Score = metric.CvssData.BaseScore
		cveInfo.Severity = metric.CvssData.BaseSeverity
		cveInfo.Vector = metric.CvssData.VectorString
		cveInfo.ExploitabilitySubScore = metric.CvssData.ExploitabilityScore
		cveInfo.ImpactSubScore = metric.CvssData.ImpactScore
	}

	// Extract CVSS v2 metrics if available
	if len(nvdCVE.Metrics.CvssMetricV2) > 0 {
		metric := nvdCVE.Metrics.CvssMetricV2[0]
		cveInfo.CVSSV2 = &CVSSV2Metrics{
			Version:                 metric.CvssData.Version,
			VectorString:            metric.CvssData.VectorString,
			BaseScore:               metric.CvssData.BaseScore,
			BaseSeverity:            metric.BaseSeverity,
			ExploitabilityScore:     metric.CvssData.ExploitabilityScore,
			ImpactScore:             metric.CvssData.ImpactScore,
			AccessVector:            metric.CvssData.AccessVector,
			AccessComplexity:        metric.CvssData.AccessComplexity,
			Authentication:          metric.CvssData.Authentication,
			ConfidentialityImpact:   metric.CvssData.ConfidentialityImpact,
			IntegrityImpact:         metric.CvssData.IntegrityImpact,
			AvailabilityImpact:      metric.CvssData.AvailabilityImpact,
			AcInsufInfo:             metric.CvssData.AcInsufInfo,
			ObtainAllPrivilege:      metric.CvssData.ObtainAllPrivilege,
			ObtainUserPrivilege:     metric.CvssData.ObtainUserPrivilege,
			ObtainOtherPrivilege:    metric.CvssData.ObtainOtherPrivilege,
			UserInteractionRequired: metric.CvssData.UserInteractionRequired,
		}

		// Use CVSS v2 data if no v3 available
		if cveInfo.Score == 0 {
			cveInfo.Score = metric.CvssData.BaseScore
			cveInfo.Severity = metric.BaseSeverity
			cveInfo.Vector = metric.CvssData.VectorString
			cveInfo.ExploitabilitySubScore = metric.CvssData.ExploitabilityScore
			cveInfo.ImpactSubScore = metric.CvssData.ImpactScore
		}
	}

	// Extract CWE information
	cveInfo.CWE = make([]string, 0)
	for _, weakness := range nvdCVE.Weaknesses {
		for _, desc := range weakness.Description {
			if desc.Lang == "en" {
				cveInfo.CWE = append(cveInfo.CWE, desc.Value)
			}
		}
	}

	// Extract references
	cveInfo.References = make([]CVEReference, len(nvdCVE.References))
	for i, ref := range nvdCVE.References {
		cveInfo.References[i] = CVEReference{
			URL:    ref.URL,
			Source: ref.Source,
			Tags:   ref.Tags,
		}
	}

	// Cache the result
	db.cache.set(cveID, cveInfo)

	db.logger.Info().
		Str("cve_id", cveID).
		Str("severity", cveInfo.Severity).
		Float64("score", cveInfo.Score).
		Msg("Successfully fetched CVE data")

	return cveInfo, nil
}

// SearchCVEs searches for CVEs based on the provided criteria
func (db *CVEDatabase) SearchCVEs(ctx context.Context, options CVESearchOptions) (*CVESearchResult, error) {
	params := db.buildSearchParams(options)
	return db.executeSearch(ctx, params)
}

// buildSearchParams builds URL parameters from search options
func (db *CVEDatabase) buildSearchParams(options CVESearchOptions) url.Values {
	params := url.Values{}

	db.addStringParam(params, "cveId", options.CVEId)
	db.addStringParam(params, "cpeName", options.CPEName)
	db.addKeywordParams(params, options)
	db.addDateParams(params, options)
	db.addCVSSParams(params, options)
	db.addStringParam(params, "cweId", options.CWE)
	db.addBooleanParams(params, options)
	db.addPaginationParams(params, options)

	return params
}

// addStringParam adds a string parameter if not empty
func (db *CVEDatabase) addStringParam(params url.Values, key, value string) {
	if value != "" {
		params.Set(key, value)
	}
}

// addKeywordParams adds keyword search parameters
func (db *CVEDatabase) addKeywordParams(params url.Values, options CVESearchOptions) {
	if options.KeywordSearch != "" {
		params.Set("keywordSearch", options.KeywordSearch)
		if options.KeywordExactMatch {
			params.Set("keywordExactMatch", "true")
		}
	}
}

// addDateParams adds date-related parameters
func (db *CVEDatabase) addDateParams(params url.Values, options CVESearchOptions) {
	if options.LastModStartDate != nil {
		params.Set("lastModStartDate", options.LastModStartDate.Format("2006-01-02T15:04:05.000"))
	}
	if options.LastModEndDate != nil {
		params.Set("lastModEndDate", options.LastModEndDate.Format("2006-01-02T15:04:05.000"))
	}
	if options.PubStartDate != nil {
		params.Set("pubStartDate", options.PubStartDate.Format("2006-01-02T15:04:05.000"))
	}
	if options.PubEndDate != nil {
		params.Set("pubEndDate", options.PubEndDate.Format("2006-01-02T15:04:05.000"))
	}
}

// addCVSSParams adds CVSS-related parameters
func (db *CVEDatabase) addCVSSParams(params url.Values, options CVESearchOptions) {
	db.addStringParam(params, "cvssV2Severity", options.CVSSV2Severity)
	db.addStringParam(params, "cvssV3Severity", options.CVSSV3Severity)

	if options.CVSSScoreMin != nil {
		params.Set("cvssV3Metrics", strconv.FormatFloat(*options.CVSSScoreMin, 'f', 1, 64))
	}
	if options.CVSSScoreMax != nil {
		params.Set("cvssV3Metrics", strconv.FormatFloat(*options.CVSSScoreMax, 'f', 1, 64))
	}
}

// addBooleanParams adds boolean flag parameters
func (db *CVEDatabase) addBooleanParams(params url.Values, options CVESearchOptions) {
	boolFlags := map[string]bool{
		"hasCertAlerts": options.HasCertAlerts,
		"hasCertNotes":  options.HasCertNotes,
		"hasKev":        options.HasKev,
		"hasOval":       options.HasOval,
		"isVulnerable":  options.IsVulnerable,
		"noRejected":    options.NoRejected,
	}

	for key, value := range boolFlags {
		if value {
			params.Set(key, "true")
		}
	}
}

// addPaginationParams adds pagination parameters
func (db *CVEDatabase) addPaginationParams(params url.Values, options CVESearchOptions) {
	if options.ResultsPerPage > 0 {
		params.Set("resultsPerPage", strconv.Itoa(options.ResultsPerPage))
	}
	if options.StartIndex > 0 {
		params.Set("startIndex", strconv.Itoa(options.StartIndex))
	}
}

// executeSearch performs the actual HTTP request and response processing
func (db *CVEDatabase) executeSearch(ctx context.Context, params url.Values) (*CVESearchResult, error) {
	reqURL := fmt.Sprintf("%s?%s", db.baseURL, params.Encode())

	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	db.setRequestHeaders(req)

	resp, err := db.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search CVEs: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NVD API returned status %d", resp.StatusCode)
	}

	var result CVESearchResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode search response: %w", err)
	}

	return &result, nil
}

// setRequestHeaders sets common request headers
func (db *CVEDatabase) setRequestHeaders(req *http.Request) {
	if db.apiKey != "" {
		req.Header.Set("apiKey", db.apiKey)
	}
	req.Header.Set("User-Agent", "container-kit-security/1.0")
}

// EnrichVulnerability enriches a vulnerability with additional CVE data
func (db *CVEDatabase) EnrichVulnerability(ctx context.Context, vuln *Vulnerability) error {
	if vuln.VulnerabilityID == "" || !strings.HasPrefix(vuln.VulnerabilityID, "CVE-") {
		// Not a CVE, nothing to enrich
		return nil
	}

	cveInfo, err := db.GetCVE(ctx, vuln.VulnerabilityID)
	if err != nil {
		db.logger.Warn().
			Err(err).
			Str("cve_id", vuln.VulnerabilityID).
			Msg("Failed to enrich vulnerability with CVE data")
		return err
	}

	// Enrich vulnerability with CVE data
	if vuln.Description == "" || len(vuln.Description) < len(cveInfo.Description) {
		vuln.Description = cveInfo.Description
	}

	if vuln.PublishedDate == "" {
		vuln.PublishedDate = cveInfo.PublishedDate.Format(time.RFC3339)
	}

	if vuln.LastModifiedDate == "" {
		vuln.LastModifiedDate = cveInfo.LastModifiedDate.Format(time.RFC3339)
	}

	// Update CVSS information with more detailed data
	if cveInfo.CVSSV3 != nil && (vuln.CVSSV3.Score == 0 || cveInfo.CVSSV3.BaseScore > vuln.CVSSV3.Score) {
		vuln.CVSSV3 = CVSSV3Info{
			Vector:                cveInfo.CVSSV3.VectorString,
			Score:                 cveInfo.CVSSV3.BaseScore,
			ExploitabilityScore:   cveInfo.CVSSV3.ExploitabilityScore,
			ImpactScore:           cveInfo.CVSSV3.ImpactScore,
			AttackVector:          cveInfo.CVSSV3.AttackVector,
			AttackComplexity:      cveInfo.CVSSV3.AttackComplexity,
			PrivilegesRequired:    cveInfo.CVSSV3.PrivilegesRequired,
			UserInteraction:       cveInfo.CVSSV3.UserInteraction,
			Scope:                 cveInfo.CVSSV3.Scope,
			ConfidentialityImpact: cveInfo.CVSSV3.ConfidentialityImpact,
			IntegrityImpact:       cveInfo.CVSSV3.IntegrityImpact,
			AvailabilityImpact:    cveInfo.CVSSV3.AvailabilityImpact,
		}
	}

	// Update general CVSS info
	if vuln.CVSS.Score == 0 || cveInfo.Score > vuln.CVSS.Score {
		vuln.CVSS = CVSSInfo{
			Version: "3.1",
			Vector:  cveInfo.Vector,
			Score:   cveInfo.Score,
		}
		if cveInfo.CVSSV2 != nil && cveInfo.CVSSV3 == nil {
			vuln.CVSS.Version = "2.0"
		}
	}

	// Update CWE information
	if len(vuln.CWE) == 0 && len(cveInfo.CWE) > 0 {
		vuln.CWE = cveInfo.CWE
	}

	// Add additional references
	referenceSet := make(map[string]bool)
	for _, ref := range vuln.References {
		referenceSet[ref] = true
	}

	for _, cveRef := range cveInfo.References {
		if !referenceSet[cveRef.URL] {
			vuln.References = append(vuln.References, cveRef.URL)
		}
	}

	// Update data source
	if vuln.DataSource.ID == "" {
		vuln.DataSource = VulnDataSource{
			ID:   "NVD",
			Name: "National Vulnerability Database",
			URL:  fmt.Sprintf("https://nvd.nist.gov/vuln/detail/%s", vuln.VulnerabilityID),
		}
	}

	return nil
}

// cveCache provides caching for CVE data
type cveCache struct {
	mu    sync.RWMutex
	cache map[string]*cacheEntryData
	ttl   time.Duration
}

type cacheEntryData struct {
	cve       *CVEInfo
	expiresAt time.Time
}

func newCVECache(ttl time.Duration) *cveCache {
	cc := &cveCache{
		cache: make(map[string]*cacheEntryData),
		ttl:   ttl,
	}

	// Start cleanup goroutine
	go cc.cleanup()

	return cc
}

func (cc *cveCache) get(cveID string) *CVEInfo {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	entry, ok := cc.cache[cveID]
	if !ok || time.Now().After(entry.expiresAt) {
		return nil
	}

	return entry.cve
}

func (cc *cveCache) set(cveID string, cve *CVEInfo) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	cc.cache[cveID] = &cacheEntryData{
		cve:       cve,
		expiresAt: time.Now().Add(cc.ttl),
	}
}

func (cc *cveCache) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		cc.mu.Lock()
		now := time.Now()

		for cveID, entry := range cc.cache {
			if now.After(entry.expiresAt) {
				delete(cc.cache, cveID)
			}
		}

		cc.mu.Unlock()
	}
}

// GetCacheStats returns cache statistics
func (db *CVEDatabase) GetCacheStats() map[string]interface{} {
	db.cache.mu.RLock()
	defer db.cache.mu.RUnlock()

	return map[string]interface{}{
		"total_entries": len(db.cache.cache),
		"ttl_hours":     db.cache.ttl.Hours(),
		"hit_count":     0, // TODO: Add hit counter to cache
		"miss_count":    0, // TODO: Add miss counter to cache
	}
}

// ClearCache clears the CVE cache
func (db *CVEDatabase) ClearCache() {
	db.cache.mu.Lock()
	defer db.cache.mu.Unlock()

	db.cache.cache = make(map[string]*cacheEntryData)
	db.logger.Info().Msg("CVE cache cleared")
}
