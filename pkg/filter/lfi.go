package filter

import (
	"encoding/base64"
	"regexp"
	"strings"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

type LFIFilter struct {
	// We check for common LFI signatures here
}

func NewLFIFilter(value string) (ffuf.FilterProvider, error) {
	return &LFIFilter{}, nil
}

func (f *LFIFilter) Filter(response *ffuf.Response) (bool, error) {
	body := string(response.Data)

	// Minimum response size - real file contents should be substantial
	if len(body) < 100 {
		return false, nil
	}

	// Skip HTML pages - LFI returns raw file content, not HTML
	if f.looksLikeHTML(body) {
		return false, nil
	}

	// 1. STRICT Linux /etc/passwd Detection
	if f.detectEtcPasswd(body) {
		return true, nil
	}

	// 2. STRICT /etc/shadow Detection
	if f.detectEtcShadow(body) {
		return true, nil
	}

	// 3. STRICT Windows win.ini/boot.ini Detection
	if f.detectWindowsIni(body) {
		return true, nil
	}

	// 4. PHP Source Code via php://filter (Base64 Detection)
	if f.detectPHPFilter(body) {
		return true, nil
	}

	// 5. /proc filesystem detection - very strict
	if f.detectProcFilesystem(body) {
		return true, nil
	}

	return false, nil
}

// looksLikeHTML checks if the response looks like an HTML page
func (f *LFIFilter) looksLikeHTML(body string) bool {
	bodyLower := strings.ToLower(body[:min(len(body), 1000)])
	htmlIndicators := []string{"<!doctype", "<html", "<head", "<body", "<div", "<script", "<meta"}

	for _, indicator := range htmlIndicators {
		if strings.Contains(bodyLower, indicator) {
			return true
		}
	}
	return false
}

// detectEtcPasswd strictly validates /etc/passwd format
func (f *LFIFilter) detectEtcPasswd(body string) bool {
	// Strict pattern: username:x:uid:gid:gecos:homedir:shell
	// Each field must be valid
	passwdLineRegex := regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,30}:[x*!]?:\d{1,5}:\d{1,5}:[^:]*:/[^:]+:/[^\n]*$`)

	lines := strings.Split(body, "\n")
	matchCount := 0
	hasRoot := false
	hasNobody := false
	hasBin := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Must have exactly 7 colon-separated fields
		fields := strings.Split(line, ":")
		if len(fields) != 7 {
			continue
		}

		// Check for root:x:0:0: specifically
		if fields[0] == "root" && fields[2] == "0" && fields[3] == "0" {
			hasRoot = true
			matchCount++
			continue
		}

		// Check for other well-known system users
		if fields[0] == "nobody" {
			hasNobody = true
			matchCount++
			continue
		}
		if fields[0] == "bin" && fields[2] == "1" {
			hasBin = true
			matchCount++
			continue
		}

		// Validate line format
		if passwdLineRegex.MatchString(line) {
			matchCount++
		}
	}

	// STRICT: Require root AND at least one other known user AND 5+ total entries
	return hasRoot && (hasNobody || hasBin) && matchCount >= 5
}

// detectEtcShadow detects /etc/shadow format
func (f *LFIFilter) detectEtcShadow(body string) bool {
	// Shadow format: username:$hashtype$salt$hash:lastchange:min:max:warn:inactive:expire:
	// Hash types: $1$ (MD5), $5$ (SHA-256), $6$ (SHA-512), $y$ (yescrypt)
	shadowRegex := regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,30}:\$[156y]\$[A-Za-z0-9./]{8,}\$[A-Za-z0-9./]+:\d*:\d*:\d*:`)

	lines := strings.Split(body, "\n")
	matchCount := 0
	hasRoot := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for root entry
		if strings.HasPrefix(line, "root:") {
			hasRoot = true
		}

		if shadowRegex.MatchString(line) {
			matchCount++
		}
	}

	// Require root entry AND at least 3 valid shadow entries
	return hasRoot && matchCount >= 3
}

// detectWindowsIni strictly validates Windows configuration files
func (f *LFIFilter) detectWindowsIni(body string) bool {
	bodyLower := strings.ToLower(body)

	// win.ini detection - need multiple specific sections
	winIniCount := 0
	winIniSections := []string{"[fonts]", "[extensions]", "[mci extensions]", "[files]", "[mail]"}
	for _, section := range winIniSections {
		if strings.Contains(bodyLower, section) {
			winIniCount++
		}
	}
	// Require at least 3 win.ini sections
	if winIniCount >= 3 {
		return true
	}

	// boot.ini detection - very specific format
	if strings.Contains(body, "[boot loader]") &&
		strings.Contains(body, "[operating systems]") &&
		strings.Contains(body, "timeout=") {
		return true
	}

	return false
}

// detectPHPFilter detects successful php://filter base64 output
func (f *LFIFilter) detectPHPFilter(body string) bool {
	cleanBody := strings.TrimSpace(body)

	// Body must be substantial and look like pure base64
	if len(cleanBody) < 200 {
		return false
	}

	// Must be almost entirely base64 characters (allow some whitespace)
	base64Content := strings.ReplaceAll(strings.ReplaceAll(cleanBody, "\n", ""), "\r", "")
	base64Regex := regexp.MustCompile(`^[A-Za-z0-9+/]+=*$`)

	if !base64Regex.MatchString(base64Content) {
		return false
	}

	// Length must be valid base64 (divisible by 4 after padding)
	if len(base64Content)%4 != 0 {
		return false
	}

	// Try to decode
	decoded, err := base64.StdEncoding.DecodeString(base64Content)
	if err != nil {
		return false
	}

	decodedStr := string(decoded)

	// Must contain PHP opening tag
	hasOpenTag := strings.Contains(decodedStr, "<?php") ||
		strings.Contains(decodedStr, "<?PHP") ||
		strings.Contains(decodedStr, "<?=")

	if !hasOpenTag {
		return false
	}

	// Must also contain closing tag or multiple PHP constructs
	phpConstructs := 0
	phpPatterns := []string{"function ", "class ", "$_GET", "$_POST", "$_REQUEST",
		"include(", "require(", "echo ", "return ", "if (", "foreach("}

	for _, pattern := range phpPatterns {
		if strings.Contains(decodedStr, pattern) {
			phpConstructs++
		}
	}

	// Require PHP tag AND at least 2 PHP constructs
	return phpConstructs >= 2
}

// detectProcFilesystem detects Linux /proc filesystem content - very strict
func (f *LFIFilter) detectProcFilesystem(body string) bool {
	// Skip if looks like HTML
	if f.looksLikeHTML(body) {
		return false
	}

	// /proc/self/environ - environment variables without separators (null-byte separated in real file)
	// Real environ is a continuous string without newlines between variables
	if !strings.Contains(body, "\n") || strings.Count(body, "\n") < 3 {
		// Check for multiple consecutive env vars pattern
		envVarRegex := regexp.MustCompile(`([A-Z_]+=[^\x00]+){5,}`)
		if envVarRegex.MatchString(body) {
			// Additional check: must have typical Linux env vars
			requiredEnvs := []string{"PATH=", "HOME=", "USER=", "SHELL=", "PWD="}
			matchCount := 0
			for _, env := range requiredEnvs {
				if strings.Contains(body, env) {
					matchCount++
				}
			}
			if matchCount >= 4 {
				return true
			}
		}
	}

	// /proc/version - very specific format
	versionRegex := regexp.MustCompile(`Linux version \d+\.\d+\.\d+[^\n]+ \(.*@.*\) \(gcc`)
	if versionRegex.MatchString(body) {
		return true
	}

	// /proc/cpuinfo - strict format check
	if strings.Contains(body, "processor\t:") &&
		strings.Contains(body, "vendor_id\t:") &&
		strings.Contains(body, "model name\t:") &&
		strings.Contains(body, "cpu MHz\t\t:") {
		return true
	}

	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (f *LFIFilter) Repr() string {
	return "lfi"
}

func (f *LFIFilter) ReprVerbose() string {
	return "LFI Detection (Ultra Strict Mode)"
}
