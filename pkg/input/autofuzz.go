package input

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

// AutoFuzzer handles automatic definition of fuzzing points
type AutoFuzzer struct {
	Config *ffuf.Config
}

// FuzzTarget represents a generated target URL with a FUZZ keyword
type FuzzTarget struct {
	URL     string
	Keyword string
}

func NewAutoFuzzer(conf *ffuf.Config) *AutoFuzzer {
	return &AutoFuzzer{Config: conf}
}

// ParseURL takes a raw URL and returns a list of FuzzTargets for every parameter found
func (a *AutoFuzzer) ParseURL(rawUrl string) ([]FuzzTarget, error) {
	if !strings.HasPrefix(rawUrl, "http") {
		rawUrl = "https://" + rawUrl
	}

	u, err := url.Parse(rawUrl)
	if err != nil {
		return nil, fmt.Errorf("invalid url: %s", err)
	}

	targets := make([]FuzzTarget, 0)
	query := u.Query()

	if len(query) == 0 {
		return nil, nil // No parameters to fuzz
	}

	// Iterate over each parameter and create a target where that parameter is Fuzzable
	for param, values := range query {
		// Create a copy of query params
		newQuery := u.Query() // Parse fresh copy
		// Set the parameter to FUZZ
		newQuery.Set(param, "FUZZ")

		// Rebuild URL
		// Note: u.RawQuery = newQuery.Encode() handles encoding.
		// "FUZZ" usually doesn't need encoding, but if it does, ffuf runner usually expects the keyword literal.
		// Encode() might turn FUZZ into FUZZ which is fine.

		// Optimization: We manually construct the query string to ensure FUZZ is not double-encoded if that's an issue,
		// but standard library Encode is robust.

		// To be safe and explicit:
		// We want http://host/path?param=FUZZ&other=val

		// Let's rely on standard logic but check the output string
		uCopy := *u
		uCopy.RawQuery = newQuery.Encode()

		// Add to targets
		targets = append(targets, FuzzTarget{
			URL:     uCopy.String(),
			Keyword: "FUZZ",
		})

		// If user wants to try fuzzing the *names* of parameters? Default behavior is usually values.
		// Maintaining just values for now as per standard tools.

		// Avoid unused variable warning if values not used
		_ = values
	}

	return targets, nil
}
