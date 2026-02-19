package ffuf

import (
	"context"
	"sync"
)

type Config struct {
	AuditLog                  string                `json:"auditlog"`
	AutoCalibration           bool                  `json:"autocalibration"`
	AutoCalibrationKeyword    string                `json:"autocalibration_keyword"`
	AutoCalibrationPerHost    bool                  `json:"autocalibration_perhost"`
	AutoCalibrationStrategies []string              `json:"autocalibration_strategies"`
	AutoCalibrationStrings    []string              `json:"autocalibration_strings"`
	AutoFuzz                  bool                  `json:"autofuzz"`
	AutoFuzzFile              string                `json:"autofuzz_file"`
	AutoTune                  bool                  `json:"autotune"`
	Cancel                    context.CancelFunc    `json:"-"`
	Colors                    bool                  `json:"colors"`
	CommandKeywords           []string              `json:"-"`
	CommandLine               string                `json:"cmdline"`
	ConfigFile                string                `json:"configfile"`
	Context                   context.Context       `json:"-"`
	Data                      string                `json:"postdata"`
	Debuglog                  string                `json:"debuglog"`
	Delay                     optRange              `json:"delay"`
	DirSearchCompat           bool                  `json:"dirsearch_compatibility"`
	DiscoverBackup            bool                  `json:"discover_backup"`
	BackupLevel               int                   `json:"backup_level"`
	DNSDiscovery              bool                  `json:"dns_discovery"`
	Batch                     bool                  `json:"batch"`
	TargetFile                string                `json:"target_file"`
	DoNotSendContentLength    bool                  `json:"do_not_send_content_length"`
	AdvancedRange             string                `json:"advanced_range"`
	MethodAsRawRequest        bool                  `json:"method_as_raw_request"`
	Encoders                  []string              `json:"encoders"`
	Extensions                []string              `json:"extensions"`
	ForceExtensions           bool                  `json:"force_extensions"`
	OverwriteExtensions       bool                  `json:"overwrite_extensions"`
	ExtensionPlaceholder      string                `json:"extension_placeholder"`
	FilterMode                string                `json:"fmode"`
	FollowRedirects           bool                  `json:"follow_redirects"`
	Headers                   map[string]string     `json:"headers"`
	IgnoreBody                bool                  `json:"ignorebody"`
	IgnoreWordlistComments    bool                  `json:"ignore_wordlist_comments"`
	InputMode                 string                `json:"inputmode"`
	InputNum                  int                   `json:"cmd_inputnum"`
	InputPrefix               string                `json:"input_prefix"`
	InputSuffix               string                `json:"input_suffix"`
	InputCapitalize           bool                  `json:"input_capitalize"`
	InputProviders            []InputProviderConfig `json:"inputproviders"`
	InputShell                string                `json:"inputshell"`
	Json                      bool                  `json:"json"`
	LFI                       bool                  `json:"lfi"`
	RandomAgent               bool                  `json:"random_agent"`
	LFIDetection              bool                  `json:"lfi_detection"`
	MatcherManager            MatcherManager        `json:"matchers"`
	MatcherMode               string                `json:"mmode"`
	LinesRange                string                `json:"lines_range"`
	SizeRange                 string                `json:"size_range"`
	WordsRange                string                `json:"words_range"`
	MaxTime                   int                   `json:"maxtime"`
	MaxTimeJob                int                   `json:"maxtime_job"`
	Method                    string                `json:"method"`
	Noninteractive            bool                  `json:"noninteractive"`
	OutputDirectory           string                `json:"outputdirectory"`
	OutputFile                string                `json:"outputfile"`
	OutputFormat              string                `json:"outputformat"`
	OutputSkipEmptyFile       bool                  `json:"OutputSkipEmptyFile"`
	ProgressFrequency         int                   `json:"-"`
	ProxyURL                  string                `json:"proxyurl"`
	Quiet                     bool                  `json:"quiet"`
	OutputLinks               bool                  `json:"output_links"`
	Rate                      int64                 `json:"rate"`
	Raw                       bool                  `json:"raw"`
	Recursion                 bool                  `json:"recursion"`
	RecursionDepth            int                   `json:"recursion_depth"`
	RecursionStrategy         string                `json:"recursion_strategy"`
	RecursionStatus           []string              `json:"recursion_status"`
	RecursionWait             *sync.WaitGroup       `json:"-"` // Global WaitGroup for recursion
	RecursionSemaphore        chan struct{}         `json:"-"` // Semaphore to limit concurrent recursive jobs
	RecursionCoordinator      *RecursionCoordinator `json:"-"` // Global coordinator for recursion dedup
	ReplayProxyURL            string                `json:"replayproxyurl"`
	RequestFile               string                `json:"requestfile"`
	Resume                    string                `json:"resume"`
	ResumePosition            int                   `json:"resume_position"`
	RequestProto              string                `json:"requestproto"`
	RequestKeepalive          bool                  `json:"requestkeepalive"`
	ScraperFile               string                `json:"scraperfile"`
	Scrapers                  string                `json:"scrapers"`
	Smart404                  bool                  `json:"smart404"`
	ShowDiff                  bool                  `json:"show_diff"`
	ShowRedirect              bool                  `json:"show_redirect"`
	SNI                       string                `json:"sni"`
	Spider                    bool                  `json:"spider"`
	StopOn403                 bool                  `json:"stop_403"`
	StopOnAll                 bool                  `json:"stop_all"`
	StopOnErrors              bool                  `json:"stop_errors"`
	Threads                   int                   `json:"threads"`
	Timeout                   int                   `json:"timeout"`
	Url                       string                `json:"url"`
	Opaque                    string                `json:"opaque"`
	Verbose                   bool                  `json:"verbose"`
	Wordlists                 []string              `json:"wordlists"`
	Http2                     bool                  `json:"http2"`
	ClientCert                string                `json:"client-cert"`
	ClientKey                 string                `json:"client-key"`
	InsecureSSL               bool                  `json:"insecure_ssl"`
}

type InputProviderConfig struct {
	Name     string `json:"name"`
	Keyword  string `json:"keyword"`
	Value    string `json:"value"`
	Encoders string `json:"encoders"`
	Template string `json:"template"` // the templating string used for sniper mode (usually "§")
}

func NewConfig(ctx context.Context, cancel context.CancelFunc) Config {
	var conf Config
	conf.AutoCalibrationKeyword = "FUZZ"
	conf.AutoCalibrationStrategies = []string{"basic"}
	conf.AutoCalibrationStrings = make([]string, 0)
	conf.AutoFuzz = false
	conf.AutoFuzzFile = ""
	conf.AutoTune = false
	conf.Batch = false
	conf.TargetFile = ""
	conf.CommandKeywords = make([]string, 0)
	conf.Context = ctx
	conf.Cancel = cancel
	conf.Data = ""
	conf.Debuglog = ""
	conf.Delay = optRange{0, 0, false, false}
	conf.DirSearchCompat = false
	conf.DiscoverBackup = false
	conf.BackupLevel = 2
	conf.DNSDiscovery = false
	conf.DoNotSendContentLength = false
	conf.AdvancedRange = ""
	conf.MethodAsRawRequest = false
	conf.Encoders = make([]string, 0)
	conf.Extensions = make([]string, 0)
	conf.ForceExtensions = false
	conf.OverwriteExtensions = false
	conf.ExtensionPlaceholder = "%EXT%"
	conf.FilterMode = "or"
	conf.FollowRedirects = false
	conf.Headers = make(map[string]string)
	conf.IgnoreWordlistComments = false
	conf.InputMode = "clusterbomb"
	conf.InputNum = 0
	conf.InputPrefix = ""
	conf.InputSuffix = ""
	conf.InputCapitalize = false
	conf.InputShell = ""
	conf.InputProviders = make([]InputProviderConfig, 0)
	conf.Json = false
	conf.LFIDetection = false
	conf.MatcherMode = "or"
	conf.LinesRange = ""
	conf.SizeRange = ""
	conf.WordsRange = ""
	conf.MaxTime = 0
	conf.MaxTimeJob = 0
	conf.Method = "GET"
	conf.Noninteractive = false
	conf.ProgressFrequency = 125
	conf.ProxyURL = ""
	conf.Quiet = false
	conf.Rate = 0
	conf.Raw = false
	conf.Recursion = false
	conf.RecursionDepth = 0
	conf.RecursionStrategy = "default"
	conf.RecursionStatus = []string{}
	conf.RequestFile = ""
	conf.Resume = ""
	conf.RequestProto = "https"
	conf.RequestKeepalive = false
	conf.ShowDiff = false
	conf.ShowRedirect = true // Default to showing redirects
	conf.SNI = ""
	conf.Spider = false
	conf.ScraperFile = ""
	conf.Scrapers = "all"
	conf.Smart404 = false
	conf.StopOn403 = false
	conf.StopOnAll = false
	conf.StopOnErrors = false
	conf.Timeout = 10
	conf.Url = ""
	conf.Opaque = ""
	conf.Verbose = false
	conf.Wordlists = []string{}
	conf.Http2 = false
	conf.InsecureSSL = true
	return conf
}

func (c *Config) SetContext(ctx context.Context, cancel context.CancelFunc) {
	c.Context = ctx
	c.Cancel = cancel
}
