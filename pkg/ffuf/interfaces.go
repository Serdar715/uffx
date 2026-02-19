package ffuf

// MatcherManager provides functions for managing matchers and filters
type MatcherManager interface {
	SetCalibrated(calibrated bool)
	SetCalibratedForHost(host string, calibrated bool)
	AddFilter(name string, option string, replace bool) error
	AddPerDomainFilter(domain string, name string, option string) error
	RemoveFilter(name string)
	AddMatcher(name string, option string) error
	GetFilters() map[string]FilterProvider
	GetMatchers() map[string]FilterProvider
	FiltersForDomain(domain string) map[string]FilterProvider
	CalibratedForDomain(domain string) bool
	Calibrated() bool
	Clone() MatcherManager
}

// FilterProvider is a generic interface for both Matchers and Filters
type FilterProvider interface {
	Filter(response *Response) (bool, error)
	Repr() string
	ReprVerbose() string
}

// RunnerProvider is an interface for request executors
type RunnerProvider interface {
	Prepare(input map[string][]byte, basereq *Request) (Request, error)
	Execute(req *Request) (Response, error)
	Dump(req *Request) ([]byte, error)
}

// InputProvider interface handles the input data for RunnerProvider
type InputProvider interface {
	ActivateKeywords([]string)
	AddProvider(InputProviderConfig) error
	Keywords() []string
	Next() bool
	Position() int
	SetPosition(int)
	Reset()
	Value() map[string][]byte
	Total() int
}

// InternalInputProvider interface handles providing input data to InputProvider
type InternalInputProvider interface {
	Keyword() string
	Next() bool
	Position() int
	SetPosition(int)
	ResetPosition()
	IncrementPosition()
	Value() []byte
	Total() int
	Active() bool
	Enable()
	Disable()
}

// OutputProvider is responsible of providing output from the RunnerProvider
type OutputProvider interface {
	Banner()
	Finalize() error
	Progress(status Progress)
	Info(infostring string)
	Error(errstring string)
	Raw(output string)
	Warning(warnstring string)
	Result(resp Response)
	PrintResult(res Result)
	SaveFile(filename, format string) error
	GetCurrentResults() []Result
	SetCurrentResults(results []Result)
	Reset()
	Cycle()
}

// AuditLogger is responsible for providing auditing output of every request/response
// sent and recieved by FFUF
type AuditLogger interface {
	Close()
	Write(data interface{}) error
}

type Scraper interface {
	Execute(resp *Response, matched bool) []ScraperResult
	AppendFromFile(path string) error
}

type ScraperResult struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Action  []string `json:"action"`
	Results []string `json:"results"`
}

// Hook is the base interface that all hooks must match
type Hook interface {
	Name() string
}

// PreRequestHook is executed before a request is made.
type PreRequestHook interface {
	Hook
	Execute(req *Request) error
}

// PostResponseHook is executed after a response is received.
type PostResponseHook interface {
	Hook
	Execute(resp *Response, req *Request) error
}

// TaskExecutor defines the contract for executing a single unit of work (request)
type TaskExecutor interface {
	ExecuteTask(input map[string][]byte, position int, retry bool)
}

// ResultProcessor defines the contract for processing the result of a task
type ResultProcessor interface {
	ProcessResult(resp Response)
}

// JobManager defines the contract for managing the lifecycle of a job
type JobManager interface {
	Start()
	Stop()
	Pause()
	Resume()
}

// ErrorHandler defines the contract for handling errors during job execution
type ErrorHandler interface {
	HandleError(err error)
}

// InputProviderFactory creates new InputProvider instances
type InputProviderFactory interface {
	NewInputProvider(conf *Config) (InputProvider, error)
}
