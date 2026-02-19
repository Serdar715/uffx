package ffuf

import (
	"errors"
	"fmt"
	"math/rand"
	"os"
	"strings"

	url "github.com/sw33tLie/neturl"
)

// CountWords counts the number of words in the given data.
// Uses strings.Fields which splits by any whitespace and ignores empty strings.
// This is the single source of truth for word counting across the codebase.
func CountWords(data []byte) int {
	return len(strings.Fields(string(data)))
}

// CountLines counts the number of lines in the given data.
// Splits by newline character and returns the count.
// This is the single source of truth for line counting across the codebase.
func CountLines(data []byte) int {
	return len(strings.Split(string(data), "\n"))
}

// used for random string generation in calibration function
var chars = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")

// RandomString returns a random string of length of parameter n
func RandomString(n int) string {
	s := make([]rune, n)
	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}
	return string(s)
}

// UniqStringSlice returns an unordered slice of unique strings. The duplicates are dropped
func UniqStringSlice(inslice []string) []string {
	found := map[string]bool{}

	for _, v := range inslice {
		found[v] = true
	}
	ret := []string{}
	for k := range found {
		ret = append(ret, k)
	}
	return ret
}

// FileExists checks if the filepath exists and is not a directory.
// Returns false in case it's not possible to describe the named file.
func FileExists(path string) bool {
	md, err := os.Stat(path)
	if err != nil {
		return false
	}

	return !md.IsDir()
}

// RequestContainsKeyword checks if a keyword is present in any field of a request
func RequestContainsKeyword(req Request, kw string) bool {
	if strings.Contains(req.Host, kw) {
		return true
	}
	if strings.Contains(req.Url, kw) {
		return true
	}
	if strings.Contains(req.Opaque, kw) {
		return true
	}
	if strings.Contains(req.Method, kw) {
		return true
	}
	if strings.Contains(string(req.Data), kw) {
		return true
	}
	for k, v := range req.Headers {
		if strings.Contains(k, kw) || strings.Contains(v, kw) {
			return true
		}
	}
	return false
}

// HostURLFromRequest gets a host + path without the filename or last part of the URL path
func HostURLFromRequest(req Request) string {
	u, _ := url.Parse(req.Url)
	u.Host = req.Host
	pathparts := strings.Split(u.Path, "/")
	trimpath := strings.TrimSpace(strings.Join(pathparts[:len(pathparts)-1], "/"))
	return u.Host + trimpath
}

// Version returns the ffuf version string
func Version() string {
	return fmt.Sprintf("%s%s", VERSION, VERSION_APPENDIX)
}

func CheckOrCreateConfigDir() error {
	var err error
	err = createConfigDir(CONFIGDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(HISTORYDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(SCRAPERDIR)
	if err != nil {
		return err
	}
	err = createConfigDir(AUTOCALIBDIR)
	if err != nil {
		return err
	}
	err = setupDefaultAutocalibrationStrategies()
	return err
}

func createConfigDir(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		var pError *os.PathError
		if errors.As(err, &pError) {
			return os.MkdirAll(path, 0750)
		}
		return err
	}
	return nil
}

func StrInSlice(key string, slice []string) bool {
	for _, v := range slice {
		if v == key {
			return true
		}
	}
	return false
}

func mergeMaps(m1 map[string][]string, m2 map[string][]string) map[string][]string {
	merged := make(map[string][]string)
	for k, v := range m1 {
		merged[k] = v
	}
	for key, value := range m2 {
		if _, ok := merged[key]; !ok {
			// Key not found, add it
			merged[key] = value
			continue
		}
		for _, entry := range value {
			if !StrInSlice(entry, merged[key]) {
				merged[key] = append(merged[key], entry)
			}
		}
	}
	return merged
}

func GenerateFuzzTargets(urls []string) []string {
	targets := make([]string, 0)
	for _, u := range urls {
		parsed, err := url.Parse(u)
		if err != nil {
			continue
		}

		if parsed.RawQuery == "" {
			continue
		}

		// Split query parameters by '&' to preserve order
		params := strings.Split(parsed.RawQuery, "&")

		for i, param := range params {
			// Split key and value
			parts := strings.SplitN(param, "=", 2)
			key := parts[0]

			// Construct the new query string
			var newQueryBuilder strings.Builder

			for j, p := range params {
				if i == j {
					// This is the parameter we are fuzzing
					newQueryBuilder.WriteString(key)
					newQueryBuilder.WriteString("=FUZZ")
				} else {
					// Keep original parameter
					newQueryBuilder.WriteString(p)
				}

				if j < len(params)-1 {
					newQueryBuilder.WriteString("&")
				}
			}

			// Update the URL with the new query string
			newURL := *parsed
			newURL.RawQuery = newQueryBuilder.String()
			targets = append(targets, newURL.String())
		}
	}
	return targets
}
