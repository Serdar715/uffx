package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/sw33tLie/uff/v2/pkg/config"
	"github.com/sw33tLie/uff/v2/pkg/ffuf"
	"github.com/sw33tLie/uff/v2/pkg/input"
	"github.com/sw33tLie/uff/v2/pkg/interactive"
	"github.com/sw33tLie/uff/v2/pkg/output"
	"github.com/sw33tLie/uff/v2/pkg/runner"
	"github.com/sw33tLie/uff/v2/pkg/scraper"
)

type multiStringFlag []string
type wordlistFlag []string

func (m *multiStringFlag) String() string {
	return ""
}

func (m *wordlistFlag) String() string {
	return ""
}

func (m *multiStringFlag) Set(value string) error {
	*m = append(*m, value)
	return nil
}

func (m *wordlistFlag) Set(value string) error {
	delimited := strings.Split(value, ",")

	if len(delimited) > 1 {
		*m = append(*m, delimited...)
	} else {
		*m = append(*m, value)
	}

	return nil
}

// ParseFlags parses the command line flags and (re)populates the ConfigOptions struct
func ParseFlags(opts *ffuf.ConfigOptions) *ffuf.ConfigOptions {
	// Robust argument parsing for -auto
	// HACK REMOVED: Replaced with standard -auto-file flag

	var ignored bool

	var cookies, autocalibrationstrings, autocalibrationstrategies, headers, inputcommands, inputranges multiStringFlag
	var wordlists, encoders wordlistFlag

	cookies = opts.HTTP.Cookies
	autocalibrationstrings = opts.General.AutoCalibrationStrings
	headers = opts.HTTP.Headers
	inputcommands = opts.Input.Inputcommands
	wordlists = opts.Input.Wordlists
	encoders = opts.Input.Encoders
	inputranges = opts.Input.InputRanges

	// Set AutoCalibration to true by default (before flag parsing)
	opts.General.AutoCalibration = true

	flag.BoolVar(&ignored, "compressed", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&ignored, "i", true, "Dummy flag for copy as curl functionality (ignored)")
	flag.BoolVar(&opts.HTTP.InsecureSSL, "k", opts.HTTP.InsecureSSL, "Allow insecure TLS connections (skip certificate verification)")
	flag.BoolVar(&opts.Output.OutputSkipEmptyFile, "or", opts.Output.OutputSkipEmptyFile, "Don't create the output file if we don't have results")
	flag.BoolVar(&opts.General.AutoCalibration, "ac", opts.General.AutoCalibration, "Automatically calibrate filtering options")
	flag.BoolVar(&opts.General.AutoCalibrationPerHost, "ach", opts.General.AutoCalibrationPerHost, "Per host autocalibration")
	flag.BoolVar(&opts.General.Colors, "c", opts.General.Colors, "Colorize output.")
	flag.BoolVar(&opts.General.DoNotSendContentLength, "no-content-length", opts.General.DoNotSendContentLength, "Do not send Content-Length header even when a body is provided")
	flag.BoolVar(&opts.General.MethodAsRawRequest, "method-as-raw-request", opts.General.MethodAsRawRequest, "Send a full raw request via the method (-X flag)")
	flag.BoolVar(&opts.General.ShowDiff, "diff", opts.General.ShowDiff, "Show the difference percentage between the response and the calibration response (implies -ac)")
	flag.BoolVar(&opts.General.Json, "json", opts.General.Json, "JSON output, printing newline-delimited JSON records")
	flag.BoolVar(&opts.General.Noninteractive, "noninteractive", opts.General.Noninteractive, "Disable the interactive console functionality")
	flag.BoolVar(&opts.General.Quiet, "s", opts.General.Quiet, "Do not print additional information (silent mode)")
	flag.BoolVar(&opts.General.ShowVersion, "V", opts.General.ShowVersion, "Show version information.")
	flag.BoolVar(&opts.General.StopOn403, "sf", opts.General.StopOn403, "Stop when > 95% of responses return 403 Forbidden")
	flag.BoolVar(&opts.General.StopOnAll, "sa", opts.General.StopOnAll, "Stop on all error cases. Implies -sf and -se.")
	flag.BoolVar(&opts.General.StopOnErrors, "se", opts.General.StopOnErrors, "Stop on spurious errors")
	flag.BoolVar(&opts.General.Verbose, "v", opts.General.Verbose, "Verbose output, printing full URL and redirect location (if any) with the results.")
	flag.BoolVar(&opts.HTTP.FollowRedirects, "r", opts.HTTP.FollowRedirects, "Follow redirects")
	flag.BoolVar(&opts.HTTP.IgnoreBody, "ignore-body", opts.HTTP.IgnoreBody, "Do not fetch the response content.")
	flag.BoolVar(&opts.HTTP.Raw, "raw", opts.HTTP.Raw, "Do not encode URI")
	flag.BoolVar(&opts.HTTP.Recursion, "recursion", opts.HTTP.Recursion, "Scan recursively. Only FUZZ keyword is supported, and URL (-u) has to end in it.")
	flag.BoolVar(&opts.HTTP.Http2, "http2", opts.HTTP.Http2, "Use HTTP2 protocol")
	flag.BoolVar(&opts.Input.DirSearchCompat, "D", opts.Input.DirSearchCompat, "DirSearch wordlist compatibility mode. Used in conjunction with -e flag.")
	flag.BoolVar(&opts.Input.IgnoreWordlistComments, "ic", opts.Input.IgnoreWordlistComments, "Ignore wordlist comments")
	flag.BoolVar(&opts.Input.AutoFuzz, "auto", opts.Input.AutoFuzz, "Auto-parameter fuzzing from file or stdin")
	flag.StringVar(&opts.General.AutoFuzzFile, "auto-file", opts.General.AutoFuzzFile, "File for Auto-Fuzzing")
	flag.BoolVar(&opts.General.LFI, "lfi", opts.General.LFI, "Smart LFI detection (Ultra Strict Mode)")
	flag.BoolVar(&opts.General.AutoTune, "autotune", opts.General.AutoTune, "Adaptive rate limiting for WAF evasion")
	flag.BoolVar(&opts.General.RandomAgent, "random-agent", opts.General.RandomAgent, "Choose a random User-Agent for each request")
	flag.BoolVar(&opts.Input.Spider, "spider", opts.Input.Spider, "Dynamic link extraction and crawling")
	flag.BoolVar(&opts.HTTP.DNSDiscovery, "dns", opts.HTTP.DNSDiscovery, "Virtual host discovery mode")
	flag.BoolVar(&opts.General.DiscoverBackup, "db", opts.General.DiscoverBackup, "Discover backup files (.bak, .old, .zip, .tar.gz, etc.) for found files")
	flag.BoolVar(&opts.General.DiscoverBackup, "collect-backups", opts.General.DiscoverBackup, "Discover backup files (alias of -db)")
	flag.IntVar(&opts.General.BackupLevel, "backup-level", opts.General.BackupLevel, "Backup discovery level (1: Basic, 2: Common, 3: Deep)")
	// Removed duplicate discover-backup flag definition to avoid runtime panic
	// flag.BoolVar(&opts.General.DiscoverBackup, "discover-backup", opts.General.DiscoverBackup, "Discover backup files (.bak, .old, .zip, .tar.gz, etc.) for found files")
	flag.BoolVar(&opts.General.ShowRedirect, "sr", opts.General.ShowRedirect, "Show redirect location for 301/302 responses (default: true)")
	flag.StringVar(&opts.General.OutputLinksFile, "ol", opts.General.OutputLinksFile, "Linkler dosyaya kaydet. Kullanım: -ol links.txt")
	flag.BoolVar(&opts.General.Smart404, "smart-404", opts.General.Smart404, "Smart 404 detection (heuristic filtering)")
	flag.IntVar(&opts.General.MaxTime, "maxtime", opts.General.MaxTime, "Maximum running time in seconds for entire process.")
	flag.IntVar(&opts.General.MaxTimeJob, "maxtime-job", opts.General.MaxTimeJob, "Maximum running time in seconds per job.")
	flag.IntVar(&opts.General.Rate, "rate", opts.General.Rate, "Rate of requests per second")
	flag.IntVar(&opts.General.Threads, "t", opts.General.Threads, "Number of concurrent threads.")
	flag.IntVar(&opts.HTTP.RecursionDepth, "recursion-depth", opts.HTTP.RecursionDepth, "Maximum recursion depth.")
	flag.IntVar(&opts.HTTP.Timeout, "timeout", opts.HTTP.Timeout, "HTTP request timeout in seconds.")
	flag.IntVar(&opts.Input.InputNum, "input-num", opts.Input.InputNum, "Number of inputs to test. Used in conjunction with --input-cmd.")
	flag.StringVar(&opts.General.AutoCalibrationKeyword, "ack", opts.General.AutoCalibrationKeyword, "Autocalibration keyword")
	flag.StringVar(&opts.HTTP.ClientCert, "cc", "", "Client cert for authentication. Client key needs to be defined as well for this to work")
	flag.StringVar(&opts.HTTP.ClientKey, "ck", "", "Client key for authentication. Client certificate needs to be defined as well for this to work")
	flag.StringVar(&opts.General.ConfigFile, "config", "", "Load configuration from a file")
	flag.StringVar(&opts.General.ScraperFile, "scraperfile", "", "Custom scraper file path")
	flag.StringVar(&opts.General.Scrapers, "scrapers", opts.General.Scrapers, "Active scraper groups")
	flag.StringVar(&opts.Filter.Mode, "fmode", opts.Filter.Mode, "Filter set operator. Either of: and, or")
	flag.StringVar(&opts.Filter.Lines, "fl", opts.Filter.Lines, "Filter by amount of lines in response. Comma separated list of line counts and ranges")
	flag.StringVar(&opts.Filter.Regexp, "fr", opts.Filter.Regexp, "Filter regexp")
	flag.StringVar(&opts.Filter.Size, "fs", opts.Filter.Size, "Filter HTTP response size. Comma separated list of sizes and ranges")
	flag.StringVar(&opts.Filter.Status, "fc", opts.Filter.Status, "Filter HTTP status codes from response. Comma separated list of codes and ranges")
	flag.StringVar(&opts.Filter.Time, "ft", opts.Filter.Time, "Filter by number of milliseconds to the first response byte, either greater or less than. EG: >100 or <100")
	flag.StringVar(&opts.Filter.Words, "fw", opts.Filter.Words, "Filter by amount of words in response. Comma separated list of word counts and ranges")
	flag.StringVar(&opts.General.Delay, "p", opts.General.Delay, "Seconds of `delay` between requests, or a range of random delay. For example \"0.1\" or \"0.1-2.0\"")

	// Add Range Filters
	flag.StringVar(&opts.Filter.MinLines, "min-lines", "", "Minimum number of lines in response")
	flag.StringVar(&opts.Filter.MaxLines, "max-lines", "", "Maximum number of lines in response")
	flag.StringVar(&opts.Filter.MinSize, "min-length", "", "Minimum response size")
	flag.StringVar(&opts.Filter.MaxSize, "max-length", "", "Maximum response size")
	flag.StringVar(&opts.Filter.MinWords, "min-words", "", "Minimum number of words")
	flag.StringVar(&opts.Filter.MaxWords, "max-words", "", "Maximum number of words")

	flag.StringVar(&opts.General.Resume, "resume", "", "Resume scan from state file")
	flag.StringVar(&opts.General.TargetFile, "l", opts.General.TargetFile, "List of targets for batch scanning")
	flag.BoolVar(&opts.General.Batch, "batch", opts.General.Batch, "Enable batch mode (read targets from stdin or -l)")

	flag.StringVar(&opts.General.Searchhash, "search", opts.General.Searchhash, "Search for a FFUFHASH payload from ffuf history")
	flag.StringVar(&opts.HTTP.Data, "d", opts.HTTP.Data, "POST data")
	flag.StringVar(&opts.HTTP.Data, "data", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Data, "data-ascii", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Data, "data-binary", opts.HTTP.Data, "POST data (alias of -d)")
	flag.StringVar(&opts.HTTP.Method, "X", opts.HTTP.Method, "HTTP method to use")
	flag.StringVar(&opts.HTTP.ProxyURL, "x", opts.HTTP.ProxyURL, "Proxy URL (SOCKS5 or HTTP). For example: http://127.0.0.1:8080 or socks5://127.0.0.1:8080")
	flag.StringVar(&opts.HTTP.ReplayProxyURL, "replay-proxy", opts.HTTP.ReplayProxyURL, "Replay matched requests using this proxy.")
	flag.StringVar(&opts.HTTP.RecursionStrategy, "recursion-strategy", opts.HTTP.RecursionStrategy, "Recursion strategy: \"default\" for a redirect based, and \"greedy\" to recurse on all matches")
	flag.StringVar(&opts.HTTP.RecursionStatus, "recursion-status", opts.HTTP.RecursionStatus, "Status codes to recurse on. Comma separated list of status codes.")
	flag.StringVar(&opts.HTTP.URL, "u", opts.HTTP.URL, "Target URL")
	flag.StringVar(&opts.HTTP.Opaque, "opaque", opts.HTTP.Opaque, "Opaque absolute URI override. Can fuzz like GET http://FUZZ/ HTTP/1.1")
	flag.StringVar(&opts.HTTP.SNI, "sni", opts.HTTP.SNI, "Target TLS SNI, does not support FUZZ keyword")
	flag.StringVar(&opts.Input.Extensions, "e", opts.Input.Extensions, "Comma separated list of extensions. Extends FUZZ keyword.")
	flag.BoolVar(&opts.Input.ForceExtensions, "force-extensions", opts.Input.ForceExtensions, "Only use extensions, do not use the raw word")
	flag.BoolVar(&opts.Input.OverwriteExtensions, "overwrite-extensions", opts.Input.OverwriteExtensions, "Overwrite existing extension (from wordlist) with extensions from -e")
	flag.StringVar(&opts.Input.ExtensionPlaceholder, "ext-placeholder", opts.Input.ExtensionPlaceholder, "Placeholder for extensions (default: %EXT%)")
	flag.StringVar(&opts.Input.InputPrefix, "prefix", opts.Input.InputPrefix, "Prefix to add to each wordlist entry")
	flag.StringVar(&opts.Input.InputSuffix, "suffix", opts.Input.InputSuffix, "Suffix to add to each wordlist entry")
	flag.BoolVar(&opts.Input.InputCapitalize, "capitalize", opts.Input.InputCapitalize, "Capitalize the first letter of each wordlist entry")
	flag.StringVar(&opts.Input.InputMode, "mode", opts.Input.InputMode, "Multi-wordlist operation mode. Available modes: clusterbomb, pitchfork, sniper")
	flag.StringVar(&opts.Input.InputShell, "input-shell", opts.Input.InputShell, "Shell to be used for running command")
	flag.StringVar(&opts.Input.Request, "request", opts.Input.Request, "File containing the raw http request (or -req)")
	flag.StringVar(&opts.Input.Request, "req", opts.Input.Request, "File containing the raw http request (alias of -request)")
	flag.StringVar(&opts.Input.RequestProto, "request-proto", opts.Input.RequestProto, "Protocol to use along with raw request")
	flag.BoolVar(&opts.Input.RequestKeepalive, "request-keepalive", opts.Input.RequestKeepalive, "Overwrite Connection: header and set it to keep-alive in request file")
	flag.StringVar(&opts.Matcher.Mode, "mmode", opts.Matcher.Mode, "Matcher set operator. Either of: and, or")
	flag.StringVar(&opts.Matcher.Lines, "ml", opts.Matcher.Lines, "Match amount of lines in response")
	flag.StringVar(&opts.Matcher.Regexp, "mr", opts.Matcher.Regexp, "Match regexp")
	flag.StringVar(&opts.Matcher.Size, "ms", opts.Matcher.Size, "Match HTTP response size")
	flag.StringVar(&opts.Matcher.Status, "mc", opts.Matcher.Status, "Match HTTP status codes, or \"all\" for everything.")
	flag.StringVar(&opts.Matcher.Time, "mt", opts.Matcher.Time, "Match how many milliseconds to the first response byte, either greater or less than. EG: >100 or <100")
	flag.StringVar(&opts.Matcher.Words, "mw", opts.Matcher.Words, "Match amount of words in response")
	flag.StringVar(&opts.Output.AuditLog, "audit-log", opts.Output.AuditLog, "Write audit log containing all requests, responses and config")
	flag.StringVar(&opts.Output.DebugLog, "debug-log", opts.Output.DebugLog, "Write all of the internal logging to the specified file.")
	flag.StringVar(&opts.Output.OutputDirectory, "od", opts.Output.OutputDirectory, "Directory path to store matched results to.")
	flag.StringVar(&opts.Output.OutputFile, "o", opts.Output.OutputFile, "Write output to file")
	flag.StringVar(&opts.Output.OutputFormat, "of", opts.Output.OutputFormat, "Output file format. Available formats: json, ejson, html, md, csv, ecsv (or, 'all' for all formats)")
	flag.Var(&autocalibrationstrings, "acc", "Custom auto-calibration string. Can be used multiple times. Implies -ac")
	flag.Var(&autocalibrationstrategies, "acs", "Custom auto-calibration strategies. Can be used multiple times. Implies -ac")
	flag.Var(&cookies, "b", "Cookie data `\"NAME1=VALUE1; NAME2=VALUE2\"` for copy as curl functionality.")
	flag.Var(&cookies, "cookie", "Cookie data (alias of -b)")
	flag.Var(&headers, "H", "Header `\"Name: Value\"`, separated by colon. Multiple -H flags are accepted.")
	flag.Var(&inputcommands, "input-cmd", "Command producing the input. --input-num is required when using this input method. Overrides -w.")

	flag.Var(&wordlists, "w", "Wordlist file path and (optional) keyword separated by colon. eg. '/path/to/wordlist:KEYWORD'")
	flag.Var(&inputranges, "range", "Range input, eg. '1-100', 'a-z', '0x1-0xff'. Supports step and zero-padding. Can be used multiple times.")
	flag.Var(&encoders, "enc", "Encoders for keywords, eg. 'FUZZ:url,base64'. Supported: md5, sha1, sha256, sha512, base64, hex, url, doubleurl, html, uppercase, lowercase")
	flag.Usage = Usage
	flag.Parse()

	// Handle -ac false / -ac true pattern (since Go's flag doesn't support -flag false format)
	// Check if -ac flag was explicitly passed with a value
	for i, arg := range os.Args {
		if arg == "-ac" && i+1 < len(os.Args) {
			nextArg := os.Args[i+1]
			// Only interpret as value if it's explicitly true or false
			if nextArg == "true" {
				opts.General.AutoCalibration = true
			} else if nextArg == "false" {
				opts.General.AutoCalibration = false
			}
		}
	}
	opts.General.AutoCalibrationStrings = autocalibrationstrings
	if len(autocalibrationstrategies) > 0 {
		opts.General.AutoCalibrationStrategies = []string{}
		for _, strategy := range autocalibrationstrategies {
			opts.General.AutoCalibrationStrategies = append(opts.General.AutoCalibrationStrategies, strings.Split(strategy, ",")...)
		}
	}
	opts.HTTP.Cookies = cookies
	opts.HTTP.Headers = headers
	opts.Input.Inputcommands = inputcommands
	opts.Input.Wordlists = wordlists
	opts.Input.InputRanges = inputranges
	opts.Input.Encoders = encoders
	// Set the first range value for backwards compatibility with single range usage
	if len(inputranges) > 0 {
		opts.Input.Range = inputranges[0]
	}
	return opts
}

func main() {

	var err, optserr error
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	setupSignalHandler(cancel)

	// prepare the default config options from default config file
	var opts *ffuf.ConfigOptions
	opts, optserr = ffuf.ReadDefaultConfig()

	opts = ParseFlags(opts)

	// Initialize structured logger
	// We use standard JSON flag to determine log format as well, to play nice with parsers.
	output.SetupLogger(opts.General.Verbose, opts.General.Json)

	// Sanitize URL: remove backslashes introduced by shell auto-escaping
	opts.HTTP.URL = strings.ReplaceAll(opts.HTTP.URL, "\\", "")

	// Handle DNS/Subdomain fuzzing
	if opts.General.DNS {
		if opts.HTTP.URL == "" {
			slog.Error("URL is required for -dns mode")
			os.Exit(1)
		}
		u, err := url.Parse(opts.HTTP.URL)
		if err != nil {
			slog.Error("Error parsing URL", "error", err)
			os.Exit(1)
		}
		host := u.Hostname()
		// Add Host header
		headerVal := fmt.Sprintf("Host:FUZZ.%s", host)
		opts.HTTP.Headers = append(opts.HTTP.Headers, headerVal)
	}

	// Handle searchhash functionality and exit
	if opts.General.Searchhash != "" {
		coptions, pos, err := ffuf.SearchHash(opts.General.Searchhash)
		if err != nil {
			slog.Error("SearchHash error", "error", err)
			os.Exit(1)
		}
		if len(coptions) > 0 {
			slog.Info("Request candidate(s) found", "hash", opts.General.Searchhash)
		}
		for _, copt := range coptions {
			conf, err := ffuf.ConfigFromOptions(&copt.ConfigOptions, ctx, cancel)
			if err != nil {
				continue
			}
			ok, reason := ffuf.HistoryReplayable(conf)
			if ok {
				printSearchResults(conf, pos, copt.Time, opts.General.Searchhash)
			} else {
				slog.Error("Hash cannot be mapped back", "reason", reason)
			}

		}
		// err is already nil here because we checked it above
		os.Exit(0)
	}

	if opts.General.ShowVersion {
		fmt.Printf("ffuf version: %s\n", ffuf.Version())
		os.Exit(0)
	}
	if len(opts.Output.DebugLog) != 0 {
		f, err := os.OpenFile(opts.Output.DebugLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			slog.Error("Disabling logging, encountered error(s)", "error", err)
			log.SetOutput(io.Discard)
		} else {
			log.SetOutput(f)
			defer f.Close()
		}
	} else {
		log.SetOutput(io.Discard)
	}
	if optserr != nil {
		// Only log if it's NOT a "file not found" error, or if verbose is on
		if !os.IsNotExist(optserr) {
			slog.Warn("Error while opening default config file", "error", optserr)
		}
	}

	if opts.General.ConfigFile != "" {
		opts, err = ffuf.ReadConfig(opts.General.ConfigFile)
		if err != nil {
			slog.Error("Encountered error(s)", "error", err)
			Usage()
			// slog.Error again? No need to duplicate if it's arguably the same error.
			os.Exit(1)
		}
		// Reset the flag package state
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		// Re-parse the cli options
		opts = ParseFlags(opts)
	}

	// AutoFuzz preparation is now handled by the BatchRunner in pkg/runner/batch.go
	// to avoid stdin race conditions and simplify logic.
	// Check -u
	// This block was part of the prepareAutoFuzz function, but is now removed.
	// The instruction implies this comment and the following `if opts.HTTP.URL != ""` block
	// should be inserted here, but `urls` is not defined in this scope.
	// Therefore, only the comment is inserted as per the instruction's intent to remove the function and its call.

	if opts.HTTP.DNSDiscovery {
		if err := setupDNS(opts); err != nil {
			fmt.Printf("[ERR] DNS Discovery: %s\n", err)
			os.Exit(1)
		}
	}

	// Set up Config struct
	conf, err := ffuf.ConfigFromOptions(opts, ctx, cancel)
	if err != nil {
		slog.Error("Encountered error(s)", "error", err)
	}
	if err := config.SetupFilters(opts, conf); err != nil {
		slog.Error("Encountered error(s)", "error", err)
		Usage()
		os.Exit(1)
	}

	// Handle Batch & AutoFuzz Modes
	if conf.Batch || conf.TargetFile != "" || conf.AutoFuzz {
		// Define the job runner function
		jobRunner := func(jobConf *ffuf.Config) error {
			return runFuzzingJob(jobConf)
		}

		batchRunner := runner.NewBatchRunner(conf, jobRunner)
		if err := batchRunner.Run(); err != nil {
			slog.Error("Batch runner failed", "error", err)
			os.Exit(1)
		}
		return
	}

	// Single Target Mode
	if err := runFuzzingJob(conf); err != nil {
		slog.Error("Encountered error(s)", "error", err)
		Usage()
		os.Exit(1)
	}
}

func runFuzzingJob(conf *ffuf.Config) error {
	// Initialize recursion synchronization
	conf.RecursionWait = &sync.WaitGroup{}
	// Limit concurrent recursive jobs to avoid file descriptor exhaustion.
	// DefaultRecursionLimit is defined in pkg/ffuf/constants.go (currently 128)
	conf.RecursionSemaphore = make(chan struct{}, ffuf.DefaultRecursionLimit)
	conf.RecursionCoordinator = ffuf.NewRecursionCoordinator(conf.Recursion, conf.RecursionDepth)

	job, err := prepareJob(conf)
	if err != nil {
		return err
	}

	// Handle Resume
	if conf.Resume != "" {
		slog.Info("Resuming from state file", "file", conf.Resume)
		state, err := ffuf.LoadState(conf.Resume)
		if err != nil {
			return fmt.Errorf("Could not load resume state: %s", err)
		}
		job.ImportState(state)
		slog.Info("State loaded", "queue_size", len(state.Queue), "visited_urls", len(state.VisitedURLs))
	}

	if conf.DNSDiscovery { // Changed from opts.General.DNS to conf.DNSDiscovery check if possible, or pass opts?
		// conf has DNSDiscovery bool from opts.HTTP.DNSDiscovery?
		// config.go: DNSDiscovery bool `json:"dns_discovery"`
		slog.Info("DNS Fuzzing Mode initialized", "words", job.Input.Total())
	}

	if job.AuditLogger != nil {
		defer job.AuditLogger.Close()
	}

	if !conf.Noninteractive {
		go func() {
			err := interactive.Handle(job)
			if err != nil {
				slog.Error("Error initializing interactive session", "error", err)
			}
		}()
	}

	// Add root job to waitgroup
	conf.RecursionWait.Add(1)
	job.Start()

	// Wait for all recursive jobs to finish
	conf.RecursionWait.Wait()

	return nil
}

func prepareJob(conf *ffuf.Config) (*ffuf.Job, error) {
	var err error
	var errs ffuf.Multierror

	builder := ffuf.NewJobBuilder(conf)

	inputProvider, inputErrs := input.NewInputProvider(conf)
	if err := inputErrs.ErrorOrNil(); err != nil {
		errs.Add(err)
	} else {
		builder.WithInput(inputProvider)
		// Inject the factory for recursive jobs
		builder.WithInputFactory(&input.StandardInputProviderFactory{})
	}

	// TODO: implement error handling for runnerprovider and outputprovider
	// We only have http runner right now
	builder.WithRunner(runner.NewRunnerByName("http", conf, false))

	if len(conf.ReplayProxyURL) > 0 {
		builder.WithReplayRunner(runner.NewRunnerByName("http", conf, true))
	}

	// We only have stdout outputprovider right now
	builder.WithOutput(output.NewOutputProviderByName("stdout", conf))

	// Initialize the audit logger if specified
	if len(conf.AuditLog) > 0 {
		auditLogger, err := output.NewAuditLogger(conf.AuditLog)
		if err != nil {
			errs.Add(err)
		} else {
			err = auditLogger.Write(conf)
			if err != nil {
				errs.Add(err)
			}
			builder.WithAuditLogger(auditLogger)
		}
	}

	// Initialize scraper
	newscraper, scraper_err := scraper.FromDir(ffuf.SCRAPERDIR, conf.Scrapers)
	if scraper_err.ErrorOrNil() != nil {
		slog.Warn("Scraper initialization failed", "error", scraper_err.ErrorOrNil())
	}

	if conf.ScraperFile != "" {
		err = newscraper.AppendFromFile(conf.ScraperFile)
		if err != nil {
			errs.Add(err)
		}
	}
	builder.WithScraper(newscraper)

	return builder.Build(), errs.ErrorOrNil()
}

func printSearchResults(conf *ffuf.Config, pos int, exectime time.Time, hash string) {
	inp, err := input.NewInputProvider(conf)
	if err.ErrorOrNil() != nil {
		fmt.Printf("-------------------------------------------\n")
		fmt.Println("Encountered error that prevents reproduction of the request:")
		fmt.Println(err.ErrorOrNil())
		return
	}
	inp.SetPosition(pos)
	inputdata := inp.Value()
	inputdata["FFUFHASH"] = []byte(hash)
	basereq := ffuf.BaseRequest(conf)
	dummyrunner := runner.NewRunnerByName("simple", conf, false)
	ffufreq, _ := dummyrunner.Prepare(inputdata, &basereq)
	rawreq, _ := dummyrunner.Dump(&ffufreq)
	fmt.Printf("-------------------------------------------\n")
	fmt.Printf("ffuf job started at: %s\n\n", exectime.Format(time.RFC3339))
	fmt.Printf("%s\n", string(rawreq))
}

func setupDNS(opts *ffuf.ConfigOptions) error {
	if opts.HTTP.URL == "" {
		return fmt.Errorf("URL is required for DNS discovery")
	}
	u, err := url.Parse(opts.HTTP.URL)
	if err != nil {
		return err
	}

	// Set Host header to FUZZ.hostname
	host := u.Hostname()
	// Check if port is included in Host() usually it is if non-standard.
	// u.Host gives hostname:port if present.

	// If user specifically provided -H "Host: ...", don't override?
	// But -dns implies we want to fuzz host.
	opts.HTTP.Headers = append(opts.HTTP.Headers, fmt.Sprintf("Host:FUZZ.%s", host))

	// Add simple filter for 404? No, VHosts usually return default page (200) or 404.
	// We rely on -ac (AutoCalibration) which is usually good for VHost.
	// Enable AutoCalibration if not set?
	if !opts.General.AutoCalibration {
		opts.General.AutoCalibration = true
		slog.Info("DNS Discovery: Auto-calibration enabled")
	}

	return nil
}

func setupSignalHandler(cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		slog.Warn("Ctrl+C detected, stopping...")
		cancel()
	}()
}
