package ffuf

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/sw33tLie/http"
	url "github.com/sw33tLie/neturl"

	"github.com/pelletier/go-toml"
)

type ConfigOptions struct {
	Filter  FilterOptions  `json:"filters"`
	General GeneralOptions `json:"general"`
	HTTP    HTTPOptions    `json:"http"`
	Input   InputOptions   `json:"input"`
	Matcher MatcherOptions `json:"matchers"`
	Output  OutputOptions  `json:"output"`
}

type HTTPOptions struct {
	Cookies           []string `json:"-"` // this is appended in headers
	Data              string   `json:"data"`
	FollowRedirects   bool     `json:"follow_redirects"`
	Headers           []string `json:"headers"`
	IgnoreBody        bool     `json:"ignore_body"`
	Method            string   `json:"method"`
	ProxyURL          string   `json:"proxy_url"`
	Raw               bool     `json:"raw"`
	Recursion         bool     `json:"recursion"`
	RecursionDepth    int      `json:"recursion_depth"`
	RecursionStrategy string   `json:"recursion_strategy"`
	RecursionStatus   string   `json:"recursion_status"`
	ReplayProxyURL    string   `json:"replay_proxy_url"`
	SNI               string   `json:"sni"`
	DNSDiscovery      bool     `json:"dns_discovery"`
	Timeout           int      `json:"timeout"`
	URL               string   `json:"url"`
	Opaque            string   `json:"opaque"`
	Http2             bool     `json:"http2"`
	ClientCert        string   `json:"client-cert"`
	ClientKey         string   `json:"client-key"`
	InsecureSSL       bool     `json:"insecure_ssl"`
}

type GeneralOptions struct {
	AutoCalibration           bool     `json:"autocalibration"`
	AutoCalibrationKeyword    string   `json:"autocalibration_keyword"`
	AutoCalibrationPerHost    bool     `json:"autocalibration_per_host"`
	AutoCalibrationStrategies []string `json:"autocalibration_strategies"`
	AutoCalibrationStrings    []string `json:"autocalibration_strings"`
	AutoFuzz                  bool     `json:"autofuzz"`
	AutoFuzzFile              string   `json:"autofuzz_file"`
	AutoTune                  bool     `json:"autotune"`
	Colors                    bool     `json:"colors"`
	ConfigFile                string   `toml:"-" json:"config_file"`
	Delay                     string   `json:"delay"`
	DiscoverBackup            bool     `json:"discover_backup"`
	BackupPatterns            string   `json:"backup_patterns"`
	BackupLevel               int      `json:"backup_level"`
	BackupStatusCodes         string   `json:"backup_status_codes"`
	DNS                       bool     `json:"dns"`
	DoNotSendContentLength    bool     `json:"do_not_send_content_length"`
	LFI                       bool     `json:"lfi"`
	RandomAgent               bool     `json:"random_agent"`
	MethodAsRawRequest        bool     `json:"method_as_raw_request"`
	Json                      bool     `json:"json"`
	LFIDetection              bool     `json:"lfi_detection"`
	MaxTime                   int      `json:"maxtime"`
	MaxTimeJob                int      `json:"maxtime_job"`
	Noninteractive            bool     `json:"noninteractive"`
	Quiet                     bool     `json:"quiet"`
	OutputLinksOnly           bool     `json:"output_links_only"`
	OutputLinksFile           string   `json:"output_links_file"`
	Rate                      int      `json:"rate"`
	Resume                    string   `json:"resume"`
	ScraperFile               string   `json:"scraperfile"`
	Scrapers                  string   `json:"scrapers"`
	Smart404                  bool     `json:"smart404"`
	Searchhash                string   `json:"-"`
	ShowDiff                  bool     `json:"show_diff"`
	ShowRedirect              bool     `json:"show_redirect"`
	ShowVersion               bool     `toml:"-" json:"-"`
	Spider                    bool     `json:"spider"`
	StopOn403                 bool     `json:"stop_on_403"`
	StopOnAll                 bool     `json:"stop_on_all"`
	StopOnErrors              bool     `json:"stop_on_errors"`
	Threads                   int      `json:"threads"`
	TargetFile                string   `json:"target_file"`
	Batch                     bool     `json:"batch"`
	Verbose                   bool     `json:"verbose"`
	NoBanner                  bool     `json:"no_banner"`
}

type InputOptions struct {
	DirSearchCompat        bool     `json:"dirsearch_compat"`
	AutoFuzz               bool     `json:"autofuzz"`
	Encoders               []string `json:"encoders"`
	Spider                 bool     `json:"spider"`
	Extensions             string   `json:"extensions"`
	ExtensionsKeyword      string   `json:"extensions_keyword"`
	ForceExtensions        bool     `json:"force_extensions"`
	OverwriteExtensions    bool     `json:"overwrite_extensions"`
	ExtensionPlaceholder   string   `json:"extension_placeholder"`
	Range                  string   `json:"range"`
	IgnoreWordlistComments bool     `json:"ignore_wordlist_comments"`
	InputMode              string   `json:"input_mode"`
	InputNum               int      `json:"input_num"`
	InputPrefix            string   `json:"input_prefix"`
	InputSuffix            string   `json:"input_suffix"`
	InputCapitalize        bool     `json:"input_capitalize"`
	InputRanges            []string `json:"input_ranges"`
	InputShell             string   `json:"input_shell"`
	Inputcommands          []string `json:"input_commands"`
	Request                string   `json:"request_file"`
	RequestProto           string   `json:"request_proto"`
	RequestKeepalive       bool     `json:"request_keepalive"`
	Wordlists              []string `json:"wordlists"`
}

type OutputOptions struct {
	AuditLog            string `json:"audit_log"`
	DebugLog            string `json:"debug_log"`
	OutputDirectory     string `json:"output_directory"`
	OutputFile          string `json:"output_file"`
	OutputFormat        string `json:"output_format"`
	OutputSkipEmptyFile bool   `json:"output_skip_empty"`
}

type FilterOptions struct {
	Mode     string `json:"mode"`
	Lines    string `json:"lines"`
	Regexp   string `json:"regexp"`
	Size     string `json:"size"`
	Status   string `json:"status"`
	Time     string `json:"time"`
	Words    string `json:"words"`
	MinLines string `json:"min_lines"`
	MaxLines string `json:"max_lines"`
	MinSize  string `json:"min_size"`
	MaxSize  string `json:"max_size"`
	MinWords string `json:"min_words"`
	MaxWords string `json:"max_words"`
}

type MatcherOptions struct {
	Mode   string `json:"mode"`
	Lines  string `json:"lines"`
	Regexp string `json:"regexp"`
	Size   string `json:"size"`
	Status string `json:"status"`
	Time   string `json:"time"`
	Words  string `json:"words"`
}

// NewConfigOptions returns a newly created ConfigOptions struct with default values
func NewConfigOptions() *ConfigOptions {
	c := &ConfigOptions{}
	c.Filter.Mode = "or"
	c.Filter.Lines = ""
	c.Filter.Regexp = ""
	c.Filter.Size = ""
	c.Filter.Status = ""
	c.Filter.Time = ""
	c.Filter.Words = ""
	c.General.AutoCalibration = false
	c.General.AutoCalibrationKeyword = "FUZZ"
	c.General.AutoCalibrationStrategies = []string{"basic"}
	c.General.AutoTune = false
	c.General.Colors = true
	c.General.Delay = ""
	c.General.DiscoverBackup = false
	c.General.BackupLevel = 2
	c.General.BackupStatusCodes = "200"
	c.General.BackupPatterns = ""
	c.General.DoNotSendContentLength = false
	c.General.MethodAsRawRequest = false
	c.General.Json = false
	c.General.LFIDetection = false
	c.General.MaxTime = 0
	c.General.MaxTimeJob = 0
	c.General.Noninteractive = false
	c.General.Quiet = false
	c.General.OutputLinksOnly = false
	c.General.OutputLinksFile = ""
	c.General.Rate = 0
	c.General.Searchhash = ""
	c.General.ScraperFile = ""
	c.General.Scrapers = "all"
	c.General.Smart404 = false
	c.General.ShowDiff = false
	c.General.ShowRedirect = true
	c.General.ShowVersion = false
	c.General.StopOn403 = false
	c.General.StopOnAll = false
	c.General.StopOnErrors = false
	c.General.Threads = 50
	c.General.Verbose = false
	c.HTTP.Data = ""
	c.HTTP.FollowRedirects = false
	c.HTTP.IgnoreBody = false
	c.HTTP.Method = ""
	c.HTTP.ProxyURL = ""
	c.HTTP.Raw = false
	c.HTTP.Recursion = false
	c.HTTP.RecursionDepth = 0
	c.HTTP.RecursionStrategy = "default"
	c.HTTP.RecursionStatus = ""
	c.HTTP.ReplayProxyURL = ""
	c.HTTP.DNSDiscovery = false
	c.HTTP.Timeout = 10
	c.HTTP.SNI = ""
	c.HTTP.URL = ""
	c.HTTP.Opaque = ""
	c.HTTP.Http2 = false
	c.HTTP.InsecureSSL = true
	c.Input.DirSearchCompat = false
	c.Input.AutoFuzz = false
	c.Input.Encoders = []string{}
	c.Input.Spider = false
	c.Input.Extensions = ""
	c.Input.ExtensionsKeyword = "FUZZ"
	c.Input.ForceExtensions = false
	c.Input.OverwriteExtensions = false
	c.Input.ExtensionPlaceholder = "%EXT%"
	c.Input.Range = ""
	c.Input.IgnoreWordlistComments = true
	c.Input.InputMode = "clusterbomb"
	c.Input.InputNum = 100
	c.Input.InputPrefix = ""
	c.Input.InputSuffix = ""
	c.Input.InputCapitalize = false
	c.Input.Request = ""
	c.Input.RequestProto = "https"
	c.Input.RequestKeepalive = false
	c.Matcher.Mode = "or"
	c.Matcher.Lines = ""
	c.Matcher.Regexp = ""
	c.Matcher.Size = ""
	c.Matcher.Status = "200-299,301,302,307,401,403,405,500"
	c.Matcher.Time = ""
	c.Matcher.Words = ""
	c.Output.AuditLog = ""
	c.Output.DebugLog = ""
	c.Output.OutputDirectory = ""
	c.Output.OutputFile = ""
	c.Output.OutputFormat = "json"
	c.Output.OutputSkipEmptyFile = false
	return c
}

// parseBackupStatusCodes parses comma-separated status codes from a string
// Returns a slice of int64 status codes, always includes 200 as default
func parseBackupStatusCodes(input string) ([]int64, error) {
	codes := make([]int64, 0)

	if input == "" || input == "200" {
		// Default: only 200
		return []int64{200}, nil
	}

	// Parse comma-separated codes
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		code, err := strconv.ParseInt(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid status code '%s': %w", part, err)
		}
		if code < 100 || code > 599 {
			return nil, fmt.Errorf("invalid HTTP status code: %d (must be 100-599)", code)
		}
		codes = append(codes, code)
	}

	// Always include 200 as default
	hasDefault := false
	for _, code := range codes {
		if code == 200 {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		codes = append(codes, 200)
	}

	return codes, nil
}

// ConfigFromOptions parses the values in ConfigOptions struct, ensures that the values are sane,
// and creates a Config struct out of them.
func ConfigFromOptions(parseOpts *ConfigOptions, ctx context.Context, cancel context.CancelFunc) (*Config, error) {
	//TODO: refactor in a proper flag library that can handle things like required flags
	errs := NewMultierror()
	conf := NewConfig(ctx, cancel)

	var err error
	var err2 error
	if len(parseOpts.HTTP.URL) == 0 && !parseOpts.General.Batch && !parseOpts.Input.AutoFuzz && parseOpts.General.TargetFile == "" && parseOpts.Input.Request == "" {
		errs.Add(fmt.Errorf("-u flag is required"))
	}

	// prepare extensions
	if parseOpts.Input.Extensions != "" {
		extensions := strings.Split(parseOpts.Input.Extensions, ",")
		conf.Extensions = extensions
	}

	// Convert cookies to a header
	if len(parseOpts.HTTP.Cookies) > 0 {
		parseOpts.HTTP.Headers = append(parseOpts.HTTP.Headers, "Cookie: "+strings.Join(parseOpts.HTTP.Cookies, "; "))
	}

	//Prepare inputproviders
	conf.InputMode = parseOpts.Input.InputMode

	validmode := false
	for _, mode := range []string{"clusterbomb", "pitchfork", "sniper"} {
		if conf.InputMode == mode {
			validmode = true
		}
	}
	if !validmode {
		errs.Add(fmt.Errorf("Input mode (-mode) %s not recognized", conf.InputMode))
	}

	template := ""
	// sniper mode needs some additional checking
	if conf.InputMode == "sniper" {
		template = "§"

		if len(parseOpts.Input.Wordlists) > 1 {
			errs.Add(fmt.Errorf("sniper mode only supports one wordlist"))
		}

		if len(parseOpts.Input.Inputcommands) > 1 {
			errs.Add(fmt.Errorf("sniper mode only supports one input command"))
		}
	}
	tmpEncoders := make(map[string]string)
	for _, e := range parseOpts.Input.Encoders {
		if strings.Contains(e, ":") {
			key := strings.Split(e, ":")[0]
			val := strings.Split(e, ":")[1]
			tmpEncoders[key] = val
		} else {
			// Implicit FUZZ keyword
			tmpEncoders["FUZZ"] = e
		}
	}

	// Default to common.txt if no other input provider is specified
	if len(parseOpts.Input.Wordlists) == 0 && parseOpts.Input.Range == "" && len(parseOpts.Input.Inputcommands) == 0 && !parseOpts.Input.AutoFuzz {
		// No default wordlist assignment. Fail-fast if no input provided.
	}

	tmpWordlists := make([]string, 0)
	for _, v := range parseOpts.Input.Wordlists {
		var wl []string
		if runtime.GOOS == "windows" {
			// Try to ensure that Windows file paths like C:\path\to\wordlist.txt:KEYWORD are treated properly
			if FileExists(v) {
				// The wordlist was supplied without a keyword parameter
				wl = []string{v}
			} else {
				filepart := v
				if strings.Contains(filepart, ":") {
					filepart = v[:strings.LastIndex(filepart, ":")]
				}

				if FileExists(filepart) {
					wl = []string{filepart, v[strings.LastIndex(v, ":")+1:]}
				} else {
					// The file was not found. Use full wordlist parameter value for more concise error message down the line
					wl = []string{v}
				}
			}
		} else {
			wl = strings.SplitN(v, ":", 2)
		}
		// Try to use absolute paths for wordlists
		fullpath := ""
		if wl[0] != "-" {
			fullpath, err = filepath.Abs(wl[0])
		} else {
			fullpath = wl[0]
		}

		if err == nil {
			wl[0] = fullpath
		}
		if len(wl) == 2 {
			if conf.InputMode == "sniper" {
				errs.Add(fmt.Errorf("sniper mode does not support wordlist keywords"))
			} else {
				newp := InputProviderConfig{
					Name:    "wordlist",
					Value:   wl[0],
					Keyword: wl[1],
				}
				// Add encoders if set
				enc, ok := tmpEncoders[wl[1]]
				if ok {
					newp.Encoders = enc
				}
				conf.InputProviders = append(conf.InputProviders, newp)
			}
		} else {
			newp := InputProviderConfig{
				Name:     "wordlist",
				Value:    wl[0],
				Keyword:  "FUZZ",
				Template: template,
			}
			// Add encoders if set
			enc, ok := tmpEncoders["FUZZ"]
			if ok {
				newp.Encoders = enc
			}
			conf.InputProviders = append(conf.InputProviders, newp)
		}
		tmpWordlists = append(tmpWordlists, strings.Join(wl, ":"))
	}
	conf.Wordlists = tmpWordlists

	for _, v := range parseOpts.Input.Inputcommands {
		ic := strings.SplitN(v, ":", 2)
		if len(ic) == 2 {
			if conf.InputMode == "sniper" {
				errs.Add(fmt.Errorf("sniper mode does not support command keywords"))
			} else {
				newp := InputProviderConfig{
					Name:    "command",
					Value:   ic[0],
					Keyword: ic[1],
				}
				enc, ok := tmpEncoders[ic[1]]
				if ok {
					newp.Encoders = enc
				}
				conf.InputProviders = append(conf.InputProviders, newp)
				conf.CommandKeywords = append(conf.CommandKeywords, ic[0])
			}
		} else {
			newp := InputProviderConfig{
				Name:     "command",
				Value:    ic[0],
				Keyword:  "FUZZ",
				Template: template,
			}
			enc, ok := tmpEncoders["FUZZ"]
			if ok {
				newp.Encoders = enc
			}
			conf.InputProviders = append(conf.InputProviders, newp)
			conf.CommandKeywords = append(conf.CommandKeywords, "FUZZ")
		}
	}

	if len(conf.InputProviders) == 0 && len(parseOpts.Input.InputRanges) == 0 && parseOpts.Input.Range == "" && !parseOpts.Input.AutoFuzz {
		if !parseOpts.General.DiscoverBackup {
			errs.Add(fmt.Errorf("Either -w, --input-cmd or -range flag is required"))
		} else {
			// If only -db is specified, use static input
			conf.InputProviders = append(conf.InputProviders, InputProviderConfig{
				Name:    "static",
				Value:   "",
				Keyword: "FUZZ",
			})
		}
	}

	// Handle Range Input — support multiple -range flags
	rangeSpecs := parseOpts.Input.InputRanges
	// Backwards compat: if InputRanges is empty but Range (string) is set (e.g. from config file)
	if len(rangeSpecs) == 0 && parseOpts.Input.Range != "" {
		rangeSpecs = []string{parseOpts.Input.Range}
	}
	for idx, rangeVal := range rangeSpecs {
		keyword := "FUZZ"
		// Support keyword syntax: "1-100:KEYWORD"
		parts := strings.SplitN(rangeVal, ":", 2)
		if len(parts) == 2 {
			rangeVal = parts[0]
			keyword = parts[1]
		} else if idx > 0 {
			keyword = fmt.Sprintf("RANGE%d", idx+1)
		}
		newp := InputProviderConfig{
			Name:     "range",
			Value:    rangeVal,
			Keyword:  keyword,
			Template: template,
		}
		enc, ok := tmpEncoders[keyword]
		if ok {
			newp.Encoders = enc
		}
		conf.InputProviders = append(conf.InputProviders, newp)
	}

	// Prepare the request using body
	if parseOpts.Input.Request != "" {
		err := parseRawRequest(parseOpts, &conf)
		if err != nil {
			errmsg := fmt.Sprintf("Could not parse raw request: %s", err)
			errs.Add(fmt.Errorf("%s", errmsg))
		}
	}

	//Prepare URL
	if parseOpts.HTTP.URL != "" {
		conf.Url = parseOpts.HTTP.URL
	}

	//Prepare Opaque
	if parseOpts.HTTP.Opaque != "" {
		conf.Opaque = parseOpts.HTTP.Opaque
	}

	// Prepare SNI
	if parseOpts.HTTP.SNI != "" {
		conf.SNI = parseOpts.HTTP.SNI
	}

	// prepare cert
	if parseOpts.HTTP.ClientCert != "" {
		conf.ClientCert = parseOpts.HTTP.ClientCert
	}
	if parseOpts.HTTP.ClientKey != "" {
		conf.ClientKey = parseOpts.HTTP.ClientKey
	}

	//Prepare headers and make canonical
	for _, v := range parseOpts.HTTP.Headers {
		hs := strings.SplitN(v, ":", 2)
		if len(hs) == 2 {
			// uff change: removed all trimming and canonization
			conf.Headers[hs[0]] = hs[1]
		} else {
			conf.Headers[v] = "NOCOLON" // hardcoded label in github.com/sw33tLie/http
		}
	}

	//Prepare delay
	d := strings.Split(parseOpts.General.Delay, "-")
	if len(d) > 2 {
		errs.Add(fmt.Errorf("Delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\""))
	} else if len(d) == 2 {
		conf.Delay.IsRange = true
		conf.Delay.HasDelay = true
		conf.Delay.Min, err = strconv.ParseFloat(d[0], 64)
		conf.Delay.Max, err2 = strconv.ParseFloat(d[1], 64)
		if err != nil || err2 != nil {
			errs.Add(fmt.Errorf("Delay range min and max values need to be valid floats. For example: 0.1-0.5"))
		}
	} else if len(parseOpts.General.Delay) > 0 {
		conf.Delay.IsRange = false
		conf.Delay.HasDelay = true
		conf.Delay.Min, err = strconv.ParseFloat(parseOpts.General.Delay, 64)
		if err != nil {
			errs.Add(fmt.Errorf("Delay needs to be either a single float: \"0.1\" or a range of floats, delimited by dash: \"0.1-0.8\""))
		}
	}

	// Verify proxy url format
	if len(parseOpts.HTTP.ProxyURL) > 0 {
		u, err := url.Parse(parseOpts.HTTP.ProxyURL)
		if err != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5") {
			errs.Add(fmt.Errorf("Bad proxy url (-x) format. Expected http, https or socks5 url"))
		} else {
			conf.ProxyURL = parseOpts.HTTP.ProxyURL
		}
	}

	// Verify replayproxy url format
	if len(parseOpts.HTTP.ReplayProxyURL) > 0 {
		u, err := url.Parse(parseOpts.HTTP.ReplayProxyURL)
		if err != nil || u.Opaque != "" || (u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" && u.Scheme != "socks5h") {
			errs.Add(fmt.Errorf("Bad replay-proxy url (-replay-proxy) format. Expected http, https or socks5 url"))
		} else {
			conf.ReplayProxyURL = parseOpts.HTTP.ReplayProxyURL
		}
	}

	//Check the output file format option
	if parseOpts.Output.OutputFile != "" {
		//No need to check / error out if output file isn't defined
		outputFormats := []string{"all", "json", "ejson", "html", "md", "csv", "ecsv"}
		found := false
		for _, f := range outputFormats {
			if f == parseOpts.Output.OutputFormat {
				conf.OutputFormat = f
				found = true
			}
		}
		if !found {
			errs.Add(fmt.Errorf("Unknown output file format (-of): %s", parseOpts.Output.OutputFormat))
		}
	}

	// Auto-calibration strings
	if len(parseOpts.General.AutoCalibrationStrings) > 0 {
		conf.AutoCalibrationStrings = parseOpts.General.AutoCalibrationStrings
	}
	// Auto-calibration strategies
	if len(parseOpts.General.AutoCalibrationStrategies) > 0 {
		conf.AutoCalibrationStrategies = parseOpts.General.AutoCalibrationStrategies
	}
	// Using -acc implies -ac
	if len(parseOpts.General.AutoCalibrationStrings) > 0 {
		conf.AutoCalibration = true
	}
	// Using -acs implies -ac
	if len(parseOpts.General.AutoCalibrationStrategies) > 0 {
		conf.AutoCalibration = true
	}

	if parseOpts.General.Rate < 0 {
		conf.Rate = 0
	} else {
		conf.Rate = int64(parseOpts.General.Rate)
	}

	if conf.Method == "" {
		if parseOpts.HTTP.Method == "" {
			// Only set if defined on command line, because we might be reparsing the CLI after
			// populating it through raw request in the first iteration
			conf.Method = "GET"
		} else {
			conf.Method = parseOpts.HTTP.Method
		}
	} else {
		if parseOpts.HTTP.Method != "" {
			// Method overridden in CLI
			conf.Method = parseOpts.HTTP.Method
		}
	}

	if parseOpts.HTTP.Data != "" {
		// Only set if defined on command line, because we might be reparsing the CLI after
		// populating it through raw request in the first iteration
		conf.Data = parseOpts.HTTP.Data
	}

	// Common stuff
	conf.IgnoreWordlistComments = parseOpts.Input.IgnoreWordlistComments
	conf.DirSearchCompat = parseOpts.Input.DirSearchCompat
	conf.DoNotSendContentLength = parseOpts.General.DoNotSendContentLength
	conf.MethodAsRawRequest = parseOpts.General.MethodAsRawRequest
	conf.Resume = parseOpts.General.Resume

	conf.AutoTune = parseOpts.General.AutoTune
	conf.RandomAgent = parseOpts.General.RandomAgent
	conf.LFIDetection = parseOpts.General.LFIDetection
	conf.AutoFuzzFile = parseOpts.General.AutoFuzzFile
	conf.Colors = parseOpts.General.Colors
	conf.InputNum = parseOpts.Input.InputNum

	conf.InputPrefix = parseOpts.Input.InputPrefix
	conf.InputSuffix = parseOpts.Input.InputSuffix
	conf.InputCapitalize = parseOpts.Input.InputCapitalize

	conf.AutoFuzz = parseOpts.Input.AutoFuzz
	conf.Spider = parseOpts.Input.Spider
	conf.AdvancedRange = parseOpts.Input.Range
	conf.ForceExtensions = parseOpts.Input.ForceExtensions
	conf.OverwriteExtensions = parseOpts.Input.OverwriteExtensions
	// ExtensionsKeyword removed from CLI, default is "FUZZ" (set in NewConfig)

	if parseOpts.Input.ExtensionPlaceholder != "" {
		conf.ExtensionPlaceholder = parseOpts.Input.ExtensionPlaceholder
	}

	conf.DNSDiscovery = parseOpts.HTTP.DNSDiscovery

	conf.InputShell = parseOpts.Input.InputShell
	conf.AuditLog = parseOpts.Output.AuditLog
	conf.OutputFile = parseOpts.Output.OutputFile
	conf.OutputDirectory = parseOpts.Output.OutputDirectory
	conf.OutputSkipEmptyFile = parseOpts.Output.OutputSkipEmptyFile
	conf.IgnoreBody = parseOpts.HTTP.IgnoreBody
	conf.Quiet = parseOpts.General.Quiet
	conf.OutputLinksOnly = parseOpts.General.OutputLinksOnly
	conf.OutputLinksFile = parseOpts.General.OutputLinksFile
	conf.ScraperFile = parseOpts.General.ScraperFile
	conf.Scrapers = parseOpts.General.Scrapers
	conf.StopOn403 = parseOpts.General.StopOn403
	conf.StopOnAll = parseOpts.General.StopOnAll
	conf.StopOnErrors = parseOpts.General.StopOnErrors
	conf.FollowRedirects = parseOpts.HTTP.FollowRedirects
	conf.Raw = parseOpts.HTTP.Raw
	conf.Recursion = parseOpts.HTTP.Recursion
	conf.RecursionDepth = parseOpts.HTTP.RecursionDepth
	conf.RecursionStrategy = parseOpts.HTTP.RecursionStrategy
	if parseOpts.HTTP.RecursionStatus != "" {
		conf.RecursionStatus = strings.Split(parseOpts.HTTP.RecursionStatus, ",")
	}
	conf.AutoCalibration = parseOpts.General.AutoCalibration
	// uff modification: AutoCalibration is disabled by default for raw request files
	// to prevent inconsistent results due to random calibration strings.
	// Only disable if user didn't explicitly enable it via flag.
	if parseOpts.Input.Request != "" && !parseOpts.General.AutoCalibration {
		conf.AutoCalibration = false
	}
	conf.AutoCalibrationPerHost = parseOpts.General.AutoCalibrationPerHost
	conf.AutoCalibrationStrategies = parseOpts.General.AutoCalibrationStrategies
	conf.Threads = parseOpts.General.Threads
	conf.Timeout = parseOpts.HTTP.Timeout
	conf.MaxTime = parseOpts.General.MaxTime
	conf.MaxTimeJob = parseOpts.General.MaxTimeJob
	conf.Noninteractive = parseOpts.General.Noninteractive
	conf.Verbose = parseOpts.General.Verbose
	conf.NoBanner = parseOpts.General.NoBanner
	conf.Json = parseOpts.General.Json
	conf.Http2 = parseOpts.HTTP.Http2
	conf.InsecureSSL = parseOpts.HTTP.InsecureSSL
	conf.DiscoverBackup = parseOpts.General.DiscoverBackup
	conf.BackupLevel = parseOpts.General.BackupLevel

	// Parse backup status codes
	statusCodes, err := parseBackupStatusCodes(parseOpts.General.BackupStatusCodes)
	if err != nil {
		return nil, fmt.Errorf("backup status codes: %w", err)
	}
	conf.BackupStatusCodes = statusCodes

	conf.Smart404 = parseOpts.General.Smart404
	conf.LFI = parseOpts.General.LFI
	// AutoCalibration implies Smart404 (User request)
	if conf.AutoCalibration {
		conf.Smart404 = true
	}

	conf.Batch = parseOpts.General.Batch
	conf.TargetFile = parseOpts.General.TargetFile

	// Parse Ranges
	if parseOpts.Filter.MinLines != "" || parseOpts.Filter.MaxLines != "" {
		rangeStr := ""
		if parseOpts.Filter.MinLines != "" && parseOpts.Filter.MaxLines != "" {
			rangeStr = fmt.Sprintf("%s-%s", parseOpts.Filter.MinLines, parseOpts.Filter.MaxLines)
		} else if parseOpts.Filter.MinLines != "" {
			rangeStr = fmt.Sprintf(">%s", parseOpts.Filter.MinLines)
		} else {
			rangeStr = fmt.Sprintf("<%s", parseOpts.Filter.MaxLines)
		}
		if conf.FilterMode == "or" && len(parseOpts.Filter.Lines) > 0 {
			parseOpts.Filter.Lines = parseOpts.Filter.Lines + "," + rangeStr
		} else {
			parseOpts.Filter.Lines = rangeStr
		}
	}

	if parseOpts.Filter.MinSize != "" || parseOpts.Filter.MaxSize != "" {
		rangeStr := ""
		if parseOpts.Filter.MinSize != "" && parseOpts.Filter.MaxSize != "" {
			rangeStr = fmt.Sprintf("%s-%s", parseOpts.Filter.MinSize, parseOpts.Filter.MaxSize)
		} else if parseOpts.Filter.MinSize != "" {
			rangeStr = fmt.Sprintf(">%s", parseOpts.Filter.MinSize)
		} else {
			rangeStr = fmt.Sprintf("<%s", parseOpts.Filter.MaxSize)
		}
		if conf.FilterMode == "or" && len(parseOpts.Filter.Size) > 0 {
			parseOpts.Filter.Size = parseOpts.Filter.Size + "," + rangeStr
		} else {
			parseOpts.Filter.Size = rangeStr
		}
	}

	if parseOpts.Filter.MinWords != "" || parseOpts.Filter.MaxWords != "" {
		rangeStr := ""
		if parseOpts.Filter.MinWords != "" && parseOpts.Filter.MaxWords != "" {
			rangeStr = fmt.Sprintf("%s-%s", parseOpts.Filter.MinWords, parseOpts.Filter.MaxWords)
		} else if parseOpts.Filter.MinWords != "" {
			rangeStr = fmt.Sprintf(">%s", parseOpts.Filter.MinWords)
		} else {
			rangeStr = fmt.Sprintf("<%s", parseOpts.Filter.MaxWords)
		}
		if conf.FilterMode == "or" && len(parseOpts.Filter.Words) > 0 {
			parseOpts.Filter.Words = parseOpts.Filter.Words + "," + rangeStr
		} else {
			parseOpts.Filter.Words = rangeStr
		}
	}

	// Check that fmode and mmode have sane values
	valid_opmodes := []string{"and", "or"}
	fmode_found := false
	mmode_found := false
	for _, v := range valid_opmodes {
		if v == parseOpts.Filter.Mode {
			fmode_found = true
		}
		if v == parseOpts.Matcher.Mode {
			mmode_found = true
		}
	}
	if !fmode_found {
		errmsg := fmt.Sprintf("Unrecognized value for parameter fmode: %s, valid values are: and, or", parseOpts.Filter.Mode)
		errs.Add(fmt.Errorf("%s", errmsg))
	}
	if !mmode_found {
		errmsg := fmt.Sprintf("Unrecognized value for parameter mmode: %s, valid values are: and, or", parseOpts.Matcher.Mode)
		errs.Add(fmt.Errorf("%s", errmsg))
	}
	conf.FilterMode = parseOpts.Filter.Mode
	conf.MatcherMode = parseOpts.Matcher.Mode

	if conf.AutoCalibrationPerHost {
		// AutoCalibrationPerHost implies AutoCalibration
		conf.AutoCalibration = true
	}

	// OLD: Handle copy as curl situation where POST method is implied by --data flag. If method is set to anything but GET, NOOP
	if len(conf.Data) > 0 &&
		conf.Method == "GET" &&
		//don't modify the method automatically if a request file is being used as input
		len(parseOpts.Input.Request) == 0 {

		slog.Warn("Sending body with GET request. This may or may not be what you want. Specify -X POST if not")

	}

	conf.CommandLine = strings.Join(os.Args, " ")

	newInputProviders := []InputProviderConfig{}
	for _, provider := range conf.InputProviders {
		if provider.Template != "" {
			if !templatePresent(provider.Template, &conf) {
				errmsg := fmt.Sprintf("Template %s defined, but not found in pairs in headers, method, URL, Opaque or POST data.", provider.Template)
				errs.Add(fmt.Errorf("%s", errmsg))
			} else {
				newInputProviders = append(newInputProviders, provider)
			}
		} else {
			if !keywordPresent(provider.Keyword, &conf) {
				if conf.Batch || conf.AutoFuzz || conf.TargetFile != "" {
					// In batch/autofuzz mode, the keyword might land in the final URL via the batch processor,
					// so we shouldn't discard the input provider even if not present in the base config.
					newInputProviders = append(newInputProviders, provider)
				} else {
					errmsg := fmt.Sprintf("Keyword %s defined, but not found in headers, method, URL, Opaque or POST data.", provider.Keyword)
					slog.Error(errmsg)
				}
			} else {
				newInputProviders = append(newInputProviders, provider)
			}
		}
	}
	conf.InputProviders = newInputProviders

	// If sniper mode, ensure there is no FUZZ keyword
	if conf.InputMode == "sniper" {
		if keywordPresent("FUZZ", &conf) {
			errs.Add(fmt.Errorf("FUZZ keyword defined, but we are using sniper mode."))
		}
	}

	// Do checks for recursion mode
	if parseOpts.HTTP.Recursion {

		if !strings.HasSuffix(conf.Url, "FUZZ") && !strings.HasSuffix(conf.Url, "FUZZ/") {
			errmsg := "When using -recursion the URL (-u) must end with FUZZ keyword."
			errs.Add(fmt.Errorf("%s", errmsg))
		}
		if conf.FollowRedirects && conf.RecursionStrategy != "greedy" {
			slog.Warn("Warning: -r (follow redirects) and -recursion are incompatible with default strategy. " +
				"DefaultRecursionStrategy depends on 301 redirects to detect directories. " +
				"Use -recursion-strategy greedy instead.")
		}
	}

	// Make verbose mutually exclusive with json
	if parseOpts.General.Verbose && parseOpts.General.Json {
		errs.Add(fmt.Errorf("Cannot have -json and -v"))
	}
	return &conf, errs.ErrorOrNil()
}

// sw33tLie patch
// sw33tLie patch
func parseRawRequest(parseOpts *ConfigOptions, conf *Config) error {
	conf.RequestFile = parseOpts.Input.Request
	conf.RequestProto = parseOpts.Input.RequestProto
	conf.RequestKeepalive = parseOpts.Input.RequestKeepalive

	// Read the whole file
	fileContent, err := os.ReadFile(parseOpts.Input.Request)
	if err != nil {
		return fmt.Errorf("could not open request file: %w", err)
	}

	// Ensure the file ends with a double newline to avoid unexpected EOF in ReadRequest
	// We trim existing trailing whitespace/lines and append exactly what we need.
	if !bytes.HasSuffix(fileContent, []byte("\n\n")) && !bytes.HasSuffix(fileContent, []byte("\r\n\r\n")) {
		fileContent = append(fileContent, []byte("\r\n\r\n")...)
	}

	r := bufio.NewReader(bytes.NewReader(fileContent))
	s, err := http.ReadRequest(r)
	if err != nil {
		return fmt.Errorf("could not read request: %w", err)
	}

	conf.Method = s.Method
	conf.Headers = make(map[string]string)
	for name, values := range s.Header {
		conf.Headers[name] = values[0]
	}

	if conf.RequestKeepalive {
		conf.Headers["Connection"] = "keep-alive"
	}

	// Body
	bodyBytes, err := io.ReadAll(s.Body)
	if err != nil {
		return fmt.Errorf("could not read request body: %w", err)
	}
	conf.Data = string(bodyBytes)

	// URL parsing if not set
	if conf.Url == "" {
		host := s.Host
		if host == "" {
			host = conf.Headers["Host"]
		}
		if host == "" {
			// Try to find Host header case-insensitively if needed, but s.Header should have canonicalized it?
			// net/http/httputil ReadRequest does canonicalize headers.
			slog.Warn("Host header not found in request file and no URL provided. This might cause issues.")
		}
		scheme := conf.RequestProto
		// Handle specific edge case where s.URL.String() might be just path or full URL
		reqUrl := s.URL.String()
		if strings.HasPrefix(reqUrl, "http") {
			conf.Url = reqUrl
		} else {
			conf.Url = fmt.Sprintf("%s://%s%s", scheme, host, reqUrl)
		}
	}

	return nil
}

func keywordPresent(keyword string, conf *Config) bool {
	//Search for keyword from HTTP method, URL and POST data too
	if strings.Contains(conf.Method, keyword) {
		return true
	}
	if strings.Contains(conf.Url, keyword) {
		return true
	}
	if strings.Contains(conf.Opaque, keyword) {
		return true
	}
	if strings.Contains(conf.Data, keyword) {
		return true
	}
	for k, v := range conf.Headers {
		if strings.Contains(k, keyword) {
			return true
		}
		if strings.Contains(v, keyword) {
			return true
		}
	}
	return false
}

func templatePresent(template string, conf *Config) bool {
	// Search for input location identifiers, these must exist in pairs
	sane := false

	if c := strings.Count(conf.Method, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	if c := strings.Count(conf.Url, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	if c := strings.Count(conf.Opaque, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	if c := strings.Count(conf.Data, template); c > 0 {
		if c%2 != 0 {
			return false
		}
		sane = true
	}
	for k, v := range conf.Headers {
		if c := strings.Count(k, template); c > 0 {
			if c%2 != 0 {
				return false
			}
			sane = true
		}
		if c := strings.Count(v, template); c > 0 {
			if c%2 != 0 {
				return false
			}
			sane = true
		}
	}

	return sane
}

func ReadConfig(configFile string) (*ConfigOptions, error) {
	conf := NewConfigOptions()
	configData, err := os.ReadFile(configFile)
	if err == nil {
		err = toml.Unmarshal(configData, conf)
	}
	return conf, err
}

func ReadDefaultConfig() (*ConfigOptions, error) {
	// Try to create configuration directory, ignore the potential error
	_ = CheckOrCreateConfigDir()
	conffile := filepath.Join(CONFIGDIR, "ffufrc")
	if !FileExists(conffile) {
		userhome, err := os.UserHomeDir()
		if err == nil {
			conffile = filepath.Join(userhome, ".ffufrc")
		}
	}
	return ReadConfig(conffile)
}
