package features

import (
	"fmt"
	"net/url"
	"strings"
	"sync"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

// BackupLevel constants for configuration
const (
	BackupLevelBasic      = 1
	BackupLevelCommon     = 2
	BackupLevelAggressive = 3
)

// StatusCodeSet manages which HTTP status codes trigger backup file discovery
type StatusCodeSet struct {
	codes map[int64]bool
}

// NewStatusCodeSet creates a set of status codes with validation
// Always includes 200 by default as a safety measure
func NewStatusCodeSet(codes []int64) (*StatusCodeSet, error) {
	s := &StatusCodeSet{codes: make(map[int64]bool)}

	// Validate and add provided codes
	for _, code := range codes {
		if code < 100 || code > 599 {
			return nil, fmt.Errorf("invalid HTTP status code: %d (must be 100-599)", code)
		}
		s.codes[code] = true
	}

	// Always include 200 as default (most common success case)
	s.codes[200] = true

	return s, nil
}

// Contains checks if a status code is in the set
func (s *StatusCodeSet) Contains(code int64) bool {
	if s == nil {
		// Fallback to default if not initialized
		return code == 200
	}
	return s.codes[code]
}

// Codes returns all status codes in the set as a slice
func (s *StatusCodeSet) Codes() []int64 {
	codes := make([]int64, 0, len(s.codes))
	for code := range s.codes {
		codes = append(codes, code)
	}
	return codes
}

// validStatusCodes defines default responses that trigger backup scanning
var validStatusCodes = map[int64]bool{
	200: true,
}

// BackupHook discovers backup files for found resources
type BackupHook struct {
	Extensions            []string
	Level                 int
	seen                  map[string]bool
	mu                    sync.Mutex
	requestsThisResponse  int            // Tracks backup checks per response to prevent explosion
	currentResponseTarget string         // Current response being processed
	statusCodes           *StatusCodeSet // Status codes to check for backups
}

// NewBackupHook creates a BackupHook with extensions for the given level
func NewBackupHook(level int) *BackupHook {
	return &BackupHook{
		Extensions:  getExtensionsForLevel(level),
		Level:       level,
		seen:        make(map[string]bool),
		statusCodes: &StatusCodeSet{codes: map[int64]bool{200: true}}, // Default: 200
	}
}

// NewBackupHookWithStatusCodes creates a BackupHook with custom status codes to check
func NewBackupHookWithStatusCodes(level int, statusCodes *StatusCodeSet) *BackupHook {
	return &BackupHook{
		Extensions:  getExtensionsForLevel(level),
		Level:       level,
		seen:        make(map[string]bool),
		statusCodes: statusCodes,
	}
}

// NewBackupHookWithStatusCodesList creates a BackupHook from a list of status codes
// Converts []int64 to StatusCodeSet internally
func NewBackupHookWithStatusCodesList(level int, statusCodes []int64) (*BackupHook, error) {
	codeSet, err := NewStatusCodeSet(statusCodes)
	if err != nil {
		return nil, err
	}
	return NewBackupHookWithStatusCodes(level, codeSet), nil
}

// getExtensionsForLevel returns backup extensions based on scan level
func getExtensionsForLevel(level int) []string {
	// Default hardcoded values as fallback
	exts := []string{"~", ".bak", ".bak2", ".old", ".orig", ".1", ".swp"}
	var extraCommon []string = []string{".zip", ".tar.gz", ".backup", ".save", ".copy"}
	var extraAggressive []string = []string{".git", ".tmp", ".log", ".bkp", ".inc", ".txt"}

	// Try to load from file
	configPath := GetDefaultSignaturePath("backup.json")
	if config, err := LoadBackupExtensions(configPath); err == nil {
		exts = config.Basic
		extraCommon = config.Common
		extraAggressive = config.Aggressive
	}

	if level >= BackupLevelCommon {
		exts = append(exts, extraCommon...)
	}
	if level >= BackupLevelAggressive {
		exts = append(exts, extraAggressive...)
	}
	return exts
}

// Name returns the hook identifier
func (h *BackupHook) Name() string {
	return "Backup Discovery"
}

// Execute processes a response and adds backup file targets
func (h *BackupHook) Execute(resp *ffuf.Response, req *ffuf.Request) error {
	const MAX_CHECKS_PER_RESPONSE = 5 // Prevent exponential request growth

	if !h.isValidResponse(resp) {
		return nil
	}

	// Reset counter if we moved to a different response target
	if h.currentResponseTarget != req.Url {
		h.requestsThisResponse = 0
		h.currentResponseTarget = req.Url
	}

	// Limit number of backup checks per response to prevent explosion
	if h.requestsThisResponse >= MAX_CHECKS_PER_RESPONSE {
		return nil
	}
	h.requestsThisResponse++

	basePath, err := h.extractBasePath(req.Url)
	if err != nil || basePath == "" {
		return nil
	}

	if h.isBackupFile(basePath) {
		return nil
	}

	h.addBackupTargets(resp, basePath)
	return nil
}

// isValidResponse checks if response status code warrants backup scanning
func (h *BackupHook) isValidResponse(resp *ffuf.Response) bool {
	if h.statusCodes != nil {
		return h.statusCodes.Contains(resp.StatusCode)
	}
	// Fallback to default
	return validStatusCodes[resp.StatusCode]
}

// extractBasePath parses URL and returns base path without query/fragment
func (h *BackupHook) extractBasePath(rawUrl string) (string, error) {
	parsed, err := url.Parse(rawUrl)
	if err != nil {
		return "", err
	}

	path := strings.TrimSuffix(parsed.Path, "/")
	if path == "" {
		return "", nil // Skip root path
	}

	return parsed.Scheme + "://" + parsed.Host + path, nil
}

// isBackupFile checks if path already ends with a backup extension
func (h *BackupHook) isBackupFile(path string) bool {
	for _, ext := range h.Extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}

// addBackupTargets appends backup variations to check targets
func (h *BackupHook) addBackupTargets(resp *ffuf.Response, basePath string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, ext := range h.Extensions {
		target := basePath + ext
		if !h.seen[target] {
			h.seen[target] = true
			resp.CheckTargets = append(resp.CheckTargets, target)
		}
	}
}

var _ ffuf.PostResponseHook = &BackupHook{}
