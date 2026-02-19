package features

import "regexp"

// LFISignatures defines the list of regex patterns for LFI detection
// These are defaults; the actual list used can be loaded from resources/signatures/lfi.json
var LFISignatures = []*regexp.Regexp{
	regexp.MustCompile(`root:x:0:0`),
	regexp.MustCompile(`\[boot loader\]`),
	regexp.MustCompile(`java\.io\.FileNotFoundException`),
	regexp.MustCompile(`Warning: include\(\)`),
	regexp.MustCompile(`Warning: require\(\)`),
	regexp.MustCompile(`Fatal error: include\(\)`),
	regexp.MustCompile(`failed to open stream`),
}

// Soft403Signatures detecting "Access Denied" in 200 OK pages
var Soft403Signatures = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Access Denied`),
	regexp.MustCompile(`(?i)You do not have permission`),
	regexp.MustCompile(`(?i)Forbidden`),
	regexp.MustCompile(`(?i)directory listing denied`),
	regexp.MustCompile(`(?i)This request has been blocked`), // WAF?
}

// Soft404Signatures detecting "Not Found" in 200 OK pages
var Soft404Signatures = []*regexp.Regexp{
	regexp.MustCompile(`(?i)Page Not Found`),
	regexp.MustCompile(`(?i)The requested URL was not found`),
	regexp.MustCompile(`(?i)Error 404`),
	regexp.MustCompile(`(?i)not be found`),
}
