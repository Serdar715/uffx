package filter

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

type RegexpFilter struct {
	Value    *regexp.Regexp
	valueRaw string
}

func NewRegexpFilter(value string) (ffuf.FilterProvider, error) {
	re, err := regexp.Compile(value)
	if err != nil {
		return &RegexpFilter{}, fmt.Errorf("Regexp filter or matcher (-fr / -mr): invalid value: %s", value)
	}
	return &RegexpFilter{Value: re, valueRaw: value}, nil
}

func (f *RegexpFilter) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		Value string `json:"value"`
	}{
		Value: f.valueRaw,
	})
}

func (f *RegexpFilter) Filter(response *ffuf.Response) (bool, error) {
	// Build match data: headers + body
	var matchBuilder strings.Builder
	for k, v := range response.Headers {
		for _, iv := range v {
			matchBuilder.WriteString(k)
			matchBuilder.WriteString(": ")
			matchBuilder.WriteString(iv)
			matchBuilder.WriteString("\r\n")
		}
	}
	matchBuilder.Write(response.Data)
	matchdata := []byte(matchBuilder.String())

	// Check if pattern contains FUZZ keywords that need replacement
	pattern := f.valueRaw
	needsRecompile := false
	for keyword, inputitem := range response.Request.Input {
		if strings.Contains(pattern, keyword) {
			pattern = strings.ReplaceAll(pattern, keyword, regexp.QuoteMeta(string(inputitem)))
			needsRecompile = true
		}
	}

	// Use pre-compiled regex if no keyword replacement needed
	if !needsRecompile {
		return f.Value.Match(matchdata), nil
	}

	// Compile and match for dynamic patterns
	re, err := regexp.Compile(pattern)
	if err != nil {
		// Invalid regex after replacement - don't filter (safe default)
		return false, nil
	}
	return re.Match(matchdata), nil
}

func (f *RegexpFilter) Repr() string {
	return f.valueRaw
}

func (f *RegexpFilter) ReprVerbose() string {
	return fmt.Sprintf("Regexp: %s", f.valueRaw)
}
