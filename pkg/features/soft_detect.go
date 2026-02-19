package features

import (
	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

// SoftErrorHook detects soft 403/404 errors in 200 OK responses
type SoftErrorHook struct{}

// NewSoftErrorHook creates a new instance of SoftErrorHook
func NewSoftErrorHook() *SoftErrorHook {
	return &SoftErrorHook{}
}

// Name returns the name of the hook
func (hook *SoftErrorHook) Name() string {
	return "Soft Error Detector"
}

// Execute inspects response for soft errors
func (hook *SoftErrorHook) Execute(resp *ffuf.Response, req *ffuf.Request) error {
	// Only check if status is 200 OK
	if resp.StatusCode != 200 {
		return nil
	}

	// Check for Soft 403 — use Match([]byte) to avoid string copy
	for _, sig := range Soft403Signatures {
		if sig.Match(resp.Data) {
			if resp.ScraperData == nil {
				resp.ScraperData = make(map[string][]string)
			}
			resp.ScraperData["SoftError"] = []string{"Soft 403 Forbidden detected"}
			resp.ScraperData["SoftStatusCode"] = []string{"403"}
			return nil
		}
	}

	// Check for Soft 404
	for _, sig := range Soft404Signatures {
		if sig.Match(resp.Data) {
			if resp.ScraperData == nil {
				resp.ScraperData = make(map[string][]string)
			}
			resp.ScraperData["SoftError"] = []string{"Soft 404 Not Found detected"}
			resp.ScraperData["SoftStatusCode"] = []string{"404"}
			return nil
		}
	}

	return nil
}

var _ ffuf.PostResponseHook = &SoftErrorHook{}
