package runner

import (
	"bufio"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/sw33tLie/uff/v2/pkg/constants"
	"github.com/sw33tLie/uff/v2/pkg/ffuf"
	"github.com/sw33tLie/uff/v2/pkg/input"
)

// BatchRunner handles scanning multiple targets
type BatchRunner struct {
	Config     *ffuf.Config
	AutoFuzzer *input.AutoFuzzer
	JobRunner  func(*ffuf.Config) error
}

func NewBatchRunner(conf *ffuf.Config, jobRunner func(*ffuf.Config) error) *BatchRunner {
	return &BatchRunner{
		Config:     conf,
		AutoFuzzer: input.NewAutoFuzzer(conf),
		JobRunner:  jobRunner,
	}
}

func (batchRunner *BatchRunner) Run() error {
	// Batch concurrency is separate from per-target fuzzing threads
	workerCount := constants.MaxBatchConcurrency

	jobs := make(chan string, workerCount*2)
	var wg sync.WaitGroup

	// Start Workers
	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for target := range jobs {
				// Don't process new jobs if context is cancelled
				if batchRunner.Config.Context.Err() != nil {
					return
				}
				batchRunner.processTarget(target)
			}
		}(workerID)
	}

	// Producer: Read targets and send to jobs channel
	// We read both from file and stdin streaming to save memory
	go func() {
		defer close(jobs)

		// 1. Read from Target File (-l)
		if batchRunner.Config.TargetFile != "" {
			batchRunner.streamFile(batchRunner.Config.TargetFile, jobs)
		}
		// 2. Read from AutoFuzz File
		if batchRunner.Config.Context.Err() == nil && batchRunner.Config.AutoFuzzFile != "" {
			batchRunner.streamFile(batchRunner.Config.AutoFuzzFile, jobs)
		}
		// 3. Read from Stdin (-batch)
		if batchRunner.Config.Context.Err() == nil && batchRunner.Config.Batch {
			stat, _ := os.Stdin.Stat()
			if (stat.Mode() & os.ModeCharDevice) == 0 {
				batchRunner.streamInput(os.Stdin, jobs)
			}
		}
		// 4. Single URL (-u)
		if batchRunner.Config.Context.Err() == nil && batchRunner.Config.AutoFuzz && batchRunner.Config.Url != "" {
			select {
			case jobs <- batchRunner.Config.Url:
			case <-batchRunner.Config.Context.Done():
			}
		}
	}()

	wg.Wait()
	return nil
}

func (batchRunner *BatchRunner) streamFile(path string, jobs chan<- string) {
	file, err := os.Open(path)
	if err != nil {
		slog.Error("Error opening file", "path", path, "error", err)
		return
	}
	defer file.Close()
	batchRunner.streamInput(file, jobs)
}

func (batchRunner *BatchRunner) streamInput(r io.Reader, jobs chan<- string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		select {
		case <-batchRunner.Config.Context.Done():
			return
		default:
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				select {
				case jobs <- line:
				case <-batchRunner.Config.Context.Done():
					return
				}
			}
		}
	}
}

func (batchRunner *BatchRunner) processTarget(target string) {
	if !batchRunner.Config.Quiet {
		slog.Info("Processing target", "target", target)
	}

	if batchRunner.Config.AutoFuzz {
		batchRunner.processAutoFuzz(target)
	} else {
		batchRunner.processStandardBatch(target)
	}
}

func (batchRunner *BatchRunner) processAutoFuzz(target string) {
	fuzzTargets, err := batchRunner.AutoFuzzer.ParseURL(target)
	if err != nil {
		slog.Error("Error parsing URL for auto-fuzz", "error", err)
		return
	}

	if len(fuzzTargets) == 0 {
		slog.Warn("No parameters found to fuzz", "target", target)
		return
	}

	for _, ft := range fuzzTargets {
		if !batchRunner.Config.Quiet {
			slog.Info("Fuzzing parameter", "url", ft.URL)
		}

		// Deep Copy Config
		jobConf := batchRunner.Config.Clone()
		jobConf.Url = ft.URL
		// We must ensure that the user provided a wordlist, otherwise the job might fail or do nothing meaningful.
		// The job will use the default FUZZ keyword, which matches the payload we want to inject.
		// Since AutoFuzz replaces the parameter value with FUZZ, the existing wordlist config will work.

		if err := batchRunner.JobRunner(jobConf); err != nil {
			slog.Error("Job failed", "target", ft.URL, "error", err)
		}
	}
}

func (batchRunner *BatchRunner) processStandardBatch(target string) {
	jobConf := batchRunner.Config.Clone()
	jobConf.Url = target

	if !strings.HasPrefix(jobConf.Url, "http") {
		jobConf.Url = "https://" + jobConf.Url
	}

	if err := batchRunner.JobRunner(jobConf); err != nil {
		slog.Error("Job failed", "target", target, "error", err)
	}
}
