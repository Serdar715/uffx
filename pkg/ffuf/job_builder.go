package ffuf

// JobBuilder constructs a Job with dependencies
type JobBuilder struct {
	config       *Config
	input        InputProvider
	runner       RunnerProvider
	replayRunner RunnerProvider
	output       OutputProvider
	scraper      Scraper
	auditLogger  AuditLogger
	inputFactory InputProviderFactory
}

// NewJobBuilder creates a new JobBuilder
func NewJobBuilder(conf *Config) *JobBuilder {
	return &JobBuilder{config: conf}
}

// WithInput sets the input provider
func (b *JobBuilder) WithInput(input InputProvider) *JobBuilder {
	b.input = input
	return b
}

// WithRunner sets the runner provider
func (b *JobBuilder) WithRunner(runner RunnerProvider) *JobBuilder {
	b.runner = runner
	return b
}

// WithReplayRunner sets the replay runner provider
func (b *JobBuilder) WithReplayRunner(runner RunnerProvider) *JobBuilder {
	b.replayRunner = runner
	return b
}

// WithOutput sets the output provider
func (b *JobBuilder) WithOutput(output OutputProvider) *JobBuilder {
	b.output = output
	return b
}

// WithScraper sets the scraper
func (b *JobBuilder) WithScraper(scraper Scraper) *JobBuilder {
	b.scraper = scraper
	return b
}

// WithAuditLogger sets the audit logger
func (b *JobBuilder) WithAuditLogger(logger AuditLogger) *JobBuilder {
	b.auditLogger = logger
	return b
}

// WithInputFactory sets the input provider factory
func (b *JobBuilder) WithInputFactory(factory InputProviderFactory) *JobBuilder {
	b.inputFactory = factory
	return b
}

// Build constructs the Job
func (b *JobBuilder) Build() *Job {
	j := NewJob(b.config)
	if b.input != nil {
		j.Input = b.input
	}
	if b.runner != nil {
		j.Runner = b.runner
	}
	if b.replayRunner != nil {
		j.ReplayRunner = b.replayRunner
	}
	if b.output != nil {
		j.Output = b.output
	}
	if b.scraper != nil {
		j.Scraper = b.scraper
	}
	if b.auditLogger != nil {
		j.AuditLogger = b.auditLogger
	}
	if b.inputFactory != nil {
		j.InputFactory = b.inputFactory
	}
	return j
}
