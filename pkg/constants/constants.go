package constants

import "time"

const (
	// MaxDownloadSize limits the response body size to 5MB
	MaxDownloadSize = 5242880

	// DefaultUserAgent is the default user agent string
	DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/136.0.0.0 Safari/537.36"

	// DefaultTimeout is the default HTTP timeout in seconds
	DefaultTimeout = 10

	// MaxIdleConns is the maximum number of idle connections
	MaxIdleConns = 1000

	// MaxConnsPerHost is the maximum number of connections per host
	MaxConnsPerHost = 500

	// MaxIdleConnsPerHost is the maximum number of idle connections per host
	MaxIdleConnsPerHost = 500

	// WorkerCount is the default number of workers for batch processing
	DefaultWorkerCount = 10

	// DefaultDelay is 0
	DefaultDelay = 0

	// DefaultRate is 0 (unlimited)
	DefaultRate = 0

	// MaxRedirects is the maximum number of redirects to follow
	MaxRedirects = 10

	// MaxBatchConcurrency limits the number of concurrent targets in batch mode
	// This is separate from Threads (per-target fuzzing concurrency) to avoid goroutine explosion
	MaxBatchConcurrency = 5
)

var (
	// DefaultTimeoutDuration is the duration version of DefaultTimeout
	DefaultTimeoutDuration = time.Duration(DefaultTimeout) * time.Second
)
