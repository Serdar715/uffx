package features

import (
	"regexp"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

type LFIHook struct {
	Signatures []*regexp.Regexp
}

func NewLFIHook() *LFIHook {
	sigs := LFISignatures // Start with defaults

	// Try to load from file
	configPath := GetDefaultSignaturePath("lfi.json")
	if loadedData, err := LoadLFISignatures(configPath); err == nil {
		if compiled, err := CompileSignatures(loadedData); err == nil {
			sigs = compiled
		}
	}

	return &LFIHook{
		Signatures: sigs,
	}
}

func (h *LFIHook) Name() string {
	return "Smart LFI Detection"
}

func (h *LFIHook) Execute(resp *ffuf.Response, req *ffuf.Request) error {
	// Only check if we have data
	if len(resp.Data) == 0 {
		return nil
	}
	for _, sig := range h.Signatures {
		if sig.Match(resp.Data) {
			// Found LFI — annotate via ScraperData for structured output
			if resp.ScraperData == nil {
				resp.ScraperData = make(map[string][]string)
			}
			resp.ScraperData["LFI"] = []string{sig.String()}
			return nil
		}
	}
	return nil
}

// Ensure compliance
var _ ffuf.PostResponseHook = &LFIHook{}
