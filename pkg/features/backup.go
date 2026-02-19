package features

import (
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

// validStatusCodes defines which responses trigger backup scanning
var validStatusCodes = map[int64]bool{
	200: true, 201: true, 204: true,
	301: true, 302: true, 307: true, 308: true,
	403: true,
}

// BackupHook discovers backup files for found resources
type BackupHook struct {
	Extensions []string
	Level      int
	seen       map[string]bool
	mu         sync.Mutex
}

// NewBackupHook creates a BackupHook with extensions for the given level
func NewBackupHook(level int) *BackupHook {
	return &BackupHook{
		Extensions: getExtensionsForLevel(level),
		Level:      level,
		seen:       make(map[string]bool),
	}
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
	if !h.isValidResponse(resp) {
		return nil
	}

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
