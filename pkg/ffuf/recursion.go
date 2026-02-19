package ffuf

import (
	"strconv"
	"strings"
	"sync"
)

// RecursionCoordinator manages global recursion state across all jobs.
// It is shared between parent and child jobs to prevent duplicate scans.
type RecursionCoordinator struct {
	visited  sync.Map
	maxDepth int
	enabled  bool
}

// NewRecursionCoordinator creates a coordinator with the given configuration.
func NewRecursionCoordinator(enabled bool, maxDepth int) *RecursionCoordinator {
	return &RecursionCoordinator{
		enabled:  enabled,
		maxDepth: maxDepth,
	}
}

// TryEnqueue checks whether the given URL should be enqueued for recursive scanning.
// Returns the recursion URL and true if it should be enqueued.
// Returns the recursion URL and false if depth is exceeded (for warning).
// Returns empty string and false if disabled or already visited.
func (rc *RecursionCoordinator) TryEnqueue(baseURL string, currentDepth int) (string, bool) {
	if !rc.enabled {
		return "", false
	}

	// Normalize: ensure exactly one trailing slash before appending FUZZ
	baseURL = strings.TrimRight(baseURL, "/") + "/"
	recURL := baseURL + "FUZZ"

	// Atomically check-and-set to prevent duplicate enqueue across all goroutines
	if _, alreadyVisited := rc.visited.LoadOrStore(recURL, true); alreadyVisited {
		return "", false
	}

	// Check depth limit (0 means unlimited)
	if rc.maxDepth > 0 && currentDepth >= rc.maxDepth {
		return recURL, false // caller should log "depth exceeded" warning
	}

	return recURL, true
}

// IsDepthExceeded returns true if the current depth has reached or exceeded the max.
// Useful for generating warning messages without re-checking in the caller.
func (rc *RecursionCoordinator) IsDepthExceeded(currentDepth int) bool {
	return rc.maxDepth > 0 && currentDepth >= rc.maxDepth
}

type RecursionStrategy interface {
	ShouldRecurse(resp *Response) (bool, string)
}

// DefaultRecursionStrategy follows redirects to detect directories
type DefaultRecursionStrategy struct {
	Config *Config
}

func (s *DefaultRecursionStrategy) ShouldRecurse(resp *Response) (bool, string) {
	// If recursion status codes are provided, we trust them.
	// This enables users to recurse on 200 OK or other non-redirect codes.
	if len(s.Config.RecursionStatus) > 0 {
		if !shouldRecurseOnStatus(resp, s.Config.RecursionStatus) {
			return false, ""
		}
		// If it matches, we recurse.
		// Try to use redirect location if it looks correct (ending in /)
		loc := resp.GetRedirectLocation(true)
		if loc != "" && strings.HasSuffix(loc, "/") {
			return true, loc
		}
		// Otherwise, construct the URL based on the request URL
		// The caller (handleRecursion) handles ensuring trailing slash
		return true, resp.Request.Url
	}

	// Legacy behavior: Check if it's a directory redirect
	// The original logic checked if the redirect location ends in /
	loc := resp.GetRedirectLocation(true)
	if (resp.Request.Url + "/") != loc {
		return false, ""
	}

	return true, loc
}

// GreedyRecursionStrategy recurses on everything that matches the filter (already determined by caller)
// recursing on status codes if specified, otherwise everything.
type GreedyRecursionStrategy struct {
	Config *Config
}

func (s *GreedyRecursionStrategy) ShouldRecurse(resp *Response) (bool, string) {
	if len(s.Config.RecursionStatus) > 0 {
		if !shouldRecurseOnStatus(resp, s.Config.RecursionStatus) {
			return false, ""
		}
	}
	// Greedy recurses on the original URL + /
	// We need to handle the slash carefully
	url := resp.Request.Url
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	return true, url
}

// Helper to check status codes
func shouldRecurseOnStatus(resp *Response, statusList []string) bool {
	statusCode := strconv.FormatInt(resp.StatusCode, 10)
	for _, s := range statusList {
		if s == statusCode {
			return true
		}
		// ranges
		if strings.Contains(s, "-") {
			parts := strings.Split(s, "-")
			if len(parts) == 2 {
				min, err1 := strconv.Atoi(parts[0])
				max, err2 := strconv.Atoi(parts[1])
				current, err3 := strconv.Atoi(statusCode)
				if err1 == nil && err2 == nil && err3 == nil {
					if current >= min && current <= max {
						return true
					}
				}
			}
		}
	}
	return false
}
