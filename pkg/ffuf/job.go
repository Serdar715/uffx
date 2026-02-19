package ffuf

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

var ()

type Job struct {
	AuditLogger       AuditLogger
	Config            *Config
	ErrorMutex        sync.Mutex
	Input             InputProvider
	Runner            RunnerProvider
	ReplayRunner      RunnerProvider
	Scraper           Scraper
	Output            OutputProvider
	Jobhash           string
	running           atomic.Bool
	runningJob        atomic.Bool
	Paused            bool
	Error             string
	Rate              *RateThrottle
	Stats             *JobStats
	startTime         time.Time
	startTimeJob      time.Time
	queuejobs         []QueueJob
	queuepos          int
	skipQueue         bool
	currentDepth      int
	calibMutex        sync.Mutex
	pauseWg           sync.WaitGroup
	VisitedURLs       map[string]bool
	VisitedMutex      sync.Mutex
	QueueMutex        sync.Mutex
	RecursionStrategy RecursionStrategy
	Detector          FalsePositiveDetector
	InputFactory      InputProviderFactory
	SingleShot        bool
}

func NewJob(conf *Config) *Job {
	var j Job
	j.Config = conf
	j.Stats = &JobStats{}
	j.running.Store(false)
	j.runningJob.Store(false)
	j.Paused = false
	j.queuepos = 0
	j.queuejobs = make([]QueueJob, 0)
	j.currentDepth = 0
	j.Rate = NewRateThrottle(conf)
	j.skipQueue = false
	j.VisitedURLs = make(map[string]bool)

	if conf.RecursionStrategy == "greedy" {
		j.RecursionStrategy = &GreedyRecursionStrategy{Config: conf}
	} else {
		j.RecursionStrategy = &DefaultRecursionStrategy{Config: conf}
	}

	if conf.Smart404 {
		j.Detector = NewSmart404Detector(conf)
	} else {
		j.Detector = &NoopDetector{}
	}
	j.SingleShot = false
	return &j
}

// incError increments the error counter
func (j *Job) incError() {
	j.Stats.IncError()
}

// inc403 increments the 403 response counter
func (j *Job) inc403() {
	j.Stats.Inc403()
}

// inc429 increments the 429 response counter
func (j *Job) inc429() {
	j.Stats.Inc429()
}

// resetSpuriousErrors resets the spurious error counter
func (j *Job) resetSpuriousErrors() {
	j.Stats.ResetSpuriousErrors()
}

// DeleteQueueItem deletes a recursion job from the queue by its index in the slice
func (j *Job) DeleteQueueItem(index int) {
	j.QueueMutex.Lock()
	defer j.QueueMutex.Unlock()
	index = j.queuepos + index - 1
	if index < 0 || index >= len(j.queuejobs) {
		return
	}
	j.queuejobs = append(j.queuejobs[:index], j.queuejobs[index+1:]...)
}

// QueuedJobs returns the slice of queued recursive jobs
func (j *Job) QueuedJobs() []QueueJob {
	j.QueueMutex.Lock()
	defer j.QueueMutex.Unlock()
	// Return a copy to avoid race conditions with the caller
	if j.queuepos > len(j.queuejobs) {
		return []QueueJob{}
	}
	// Copying might be expensive if queue is large, but safe.
	// However, usually this is just for display.
	// For now, let's just slice safely if possible or return as is if we assume caller is read-only.
	// But append reallocation invalidates pointers.
	// Safe way:
	start := j.queuepos - 1
	if start < 0 {
		start = 0
	}
	if start >= len(j.queuejobs) {
		return []QueueJob{}
	}

	// Create a copy
	output := make([]QueueJob, len(j.queuejobs)-start)
	copy(output, j.queuejobs[start:])
	return output
}

// IsRunning returns the running state of the job
func (j *Job) IsRunning() bool {
	return j.running.Load()
}

// IsRunningJob returns the running state of the current sub-job
func (j *Job) IsRunningJob() bool {
	return j.runningJob.Load()
}

// Start the execution of the Job
func (j *Job) Start() {
	// Safety: WaitGroup is managed by parent/main
	defer j.Config.RecursionWait.Done()

	// Rate Limiting: Acquire Semaphore
	j.Config.RecursionSemaphore <- struct{}{}
	defer func() { <-j.Config.RecursionSemaphore }()

	j.initializeJob()

	rand.Seed(time.Now().UnixNano())

	// Don't defer j.Stop() because it cancels the global context!
	defer func() {
		j.running.Store(false)
		j.runningJob.Store(false)
	}()

	j.running.Store(true)
	j.runningJob.Store(true)

	if !j.Config.Quiet && !j.Config.NoBanner && j.currentDepth == 0 && !j.SingleShot {
		j.Output.Banner()
	}

	j.interruptMonitor()

	j.processJobQueue()

	// Check if we need to save state (interrupted or unfinished)
	if j.Config.Context.Err() == context.Canceled {
		err := SaveState(j.Config.Url, j)
		if err != nil {
			j.Output.Error(fmt.Sprintf("Could not save resume state: %s", err))
		} else {
			j.Output.Info(fmt.Sprintf("Saved scan state to %s ...", GetStateFilename(j.Config.Url)))
		}
	}

	err := j.Output.Finalize()
	if err != nil {
		j.Output.Error(err.Error())
	}
}

func (j *Job) initializeJob() {
	if j.startTime.IsZero() {
		j.startTime = time.Now()
	}

	basereq := BaseRequest(j.Config)
	j.startTimeJob = time.Now()

	// Calibrate detector if needed - Disable for SingleShot jobs
	if j.Config.Smart404 && !j.SingleShot {
		err := j.Detector.Calibrate(j.Runner, &basereq)
		if err != nil {
			j.Output.Error(fmt.Sprintf("Calibration failed: %s", err))
		}
	}

	if j.Config.InputMode == "sniper" {
		// process multiple payload locations and create a queue job for each location
		reqs := SniperRequests(&basereq, j.Config.InputProviders[0].Template)
		for _, r := range reqs {
			j.queuejobs = append(j.queuejobs, QueueJob{Url: j.Config.Url, depth: 0, req: r, SingleShot: false})
		}
		j.Stats.Total = j.Input.Total() * len(reqs)
	} else {
		// Add the default job to job queue
		j.queuejobs = append(j.queuejobs, QueueJob{Url: j.Config.Url, depth: 0, req: BaseRequest(j.Config), SingleShot: j.SingleShot})
		j.Stats.Total = j.Input.Total()
	}
}

func (j *Job) processJobQueue() {
	i := 0
	for {
		select {
		case <-j.Config.Context.Done():
			return
		default:
		}

		j.QueueMutex.Lock()
		queueLen := len(j.queuejobs)
		var jobToRun *QueueJob
		if i < queueLen {
			val := j.queuejobs[i]
			jobToRun = &val
			i++
		}
		j.QueueMutex.Unlock()

		if jobToRun == nil {
			break
		}

		if i-1 == 0 {
			j.runRootJob()
		} else {
			j.spawnChildJob(jobToRun)
		}
	}
}

func (j *Job) runRootJob() {
	// Root / First job runs on this instance (synchronously in this goroutine)
	j.prepareQueueJob()
	j.Reset(true)
	j.runningJob.Store(true)
	j.startExecution()
}

func (j *Job) spawnChildJob(jobToRun *QueueJob) {
	child, err := NewJobFromQueue(*jobToRun, j)
	if err != nil {
		// Provide context-aware error messages
		if jobToRun.SingleShot {
			j.Output.Error(fmt.Sprintf("Backup check failed for %s: %s", jobToRun.Url, err))
		} else {
			j.Output.Error(fmt.Sprintf("Failed to spawn child job for %s: %s", jobToRun.Url, err))
		}
		return
	}

	// Add to Global WaitGroup before spawning
	j.Config.RecursionWait.Add(1)
	go child.Start()
}

// NewJobFromQueue creates a new independent Job from a queue execution request
func NewJobFromQueue(q QueueJob, parent *Job) (*Job, error) {
	// Clone config
	newConf := *parent.Config
	newConf.Url = q.Url

	// If it's a SingleShot job, we force a static single input provider
	if q.SingleShot {
		newConf.InputProviders = []InputProviderConfig{{Name: "static", Keyword: "FUZZ", Value: ""}}
	}

	// Clone MatcherManager
	newConf.MatcherManager = parent.Config.MatcherManager.Clone()

	// Create new job
	newJob := NewJob(&newConf)

	// Inject Factory
	newJob.InputFactory = parent.InputFactory

	// Setup Input using Factory
	if parent.InputFactory != nil {
		inputProvider, err := parent.InputFactory.NewInputProvider(&newConf)
		if err != nil {
			return nil, err
		}
		newJob.Input = inputProvider
	} else {
		return nil, fmt.Errorf("InputFactory not initialized")
	}

	// Copy providers (assumed thread-safe)
	newJob.Runner = parent.Runner
	newJob.Output = parent.Output
	newJob.Scraper = parent.Scraper
	newJob.ReplayRunner = parent.ReplayRunner
	newJob.AuditLogger = parent.AuditLogger

	// Set depth and singleshot
	newJob.currentDepth = q.depth
	newJob.SingleShot = q.SingleShot

	// Setup stats?
	// NewJob creates fresh stats.

	return newJob, nil
}

// Reset resets the counters and wordlist position for a job
func (j *Job) Reset(cycle bool) {
	j.Input.Reset()
	j.Stats.Counter = 0
	j.skipQueue = false
	j.startTimeJob = time.Now()
	if cycle {
		j.Output.Cycle()
	} else {
		j.Output.Reset()
	}
}

func (j *Job) jobsInQueue() bool {
	return j.queuepos < len(j.queuejobs)
}

func (j *Job) prepareQueueJob() {
	j.Config.Url = j.queuejobs[j.queuepos].Url
	j.Config.Opaque = j.queuejobs[j.queuepos].req.Opaque
	j.currentDepth = j.queuejobs[j.queuepos].depth

	//Find all keywords present in new queued job
	kws := j.Input.Keywords()
	found_kws := make([]string, 0)
	for _, k := range kws {
		if RequestContainsKeyword(j.queuejobs[j.queuepos].req, k) {
			found_kws = append(found_kws, k)
		}
	}
	//And activate / disable inputproviders as needed
	j.Input.ActivateKeywords(found_kws)
	j.queuepos += 1
	j.Jobhash, _ = WriteHistoryEntry(j.Config)
}

// SkipQueue allows to skip the current job and advance to the next queued recursion job
func (j *Job) SkipQueue() {
	j.skipQueue = true
}

func (j *Job) sleepIfNeeded() {
	var sleepDuration time.Duration
	if j.Config.Delay.HasDelay {
		if j.Config.Delay.IsRange {
			sTime := j.Config.Delay.Min + rand.Float64()*(j.Config.Delay.Max-j.Config.Delay.Min)
			sleepDuration = time.Duration(sTime * 1000)
		} else {
			sleepDuration = time.Duration(j.Config.Delay.Min * 1000)
		}
		sleepDuration = sleepDuration * time.Millisecond
	}
	// makes the sleep cancellable by context
	select {
	case <-j.Config.Context.Done(): // cancelled
	case <-time.After(sleepDuration): // sleep
	}
}

// Pause pauses the job process
func (j *Job) Pause() {
	if !j.Paused {
		j.Paused = true
		j.pauseWg.Add(1)
		j.Output.Info("------ PAUSING ------")
	}
}

// Resume resumes the job process
func (j *Job) Resume() {
	if j.Paused {
		j.Paused = false
		j.Output.Info("------ RESUMING -----")
		j.pauseWg.Done()
	}
}

func (j *Job) startExecution() {
	var wg sync.WaitGroup
	wg.Add(1)

	if !j.SingleShot {
		go j.runBackgroundTasks(&wg)
	} else {
		// Just for wg consistency if we don't spawn background tasks
		wg.Done()
	}

	isSingleShot := false
	if j.queuepos > 0 && j.queuepos <= len(j.queuejobs) {
		isSingleShot = j.queuejobs[j.queuepos-1].SingleShot
	}

	if (j.queuepos > 1 || j.currentDepth > 0) && !isSingleShot && !j.SingleShot {
		if j.Config.InputMode == "sniper" {
			j.Output.Info(fmt.Sprintf("Starting queued sniper job (%d of %d) on target: %s", j.queuepos, len(j.queuejobs), j.Config.Url))
		} else {
			j.Output.Info(fmt.Sprintf("Starting queued job on target: %s", j.Config.Url))
		}
	}

	threadlimiter := make(chan bool, j.Config.Threads)

	if isSingleShot {
		j.processSingleShotJob(&wg, threadlimiter)
	} else {
		j.processRecursiveJob(&wg, threadlimiter)
	}
	wg.Wait()
	j.updateProgress()
}

func (j *Job) processSingleShotJob(wg *sync.WaitGroup, threadlimiter chan bool) {
	// Check if context is cancelled before processing
	select {
	case <-j.Config.Context.Done():
		return
	default:
	}

	if !j.IsRunning() {
		return
	}

	threadlimiter <- true
	wg.Add(1)
	j.Stats.Counter++

	go func() {
		defer func() { <-threadlimiter }()
		defer wg.Done()
		threadStart := time.Now()
		j.runTask(j.Input.Value(), 0, false)
		j.sleepIfNeeded()
		threadEnd := time.Now()
		j.Rate.Tick(threadStart, threadEnd)
	}()
}

func (j *Job) processRecursiveJob(wg *sync.WaitGroup, threadlimiter chan bool) {
	// Skip already-processed entries when resuming
	skipCount := j.Config.ResumePosition
	skipped := 0
	for j.Input.Next() && !j.skipQueue {
		j.CheckStop()

		select {
		case <-j.Config.Context.Done():
			return
		default:
		}

		if !j.IsRunning() {
			j.Output.Warning(j.Error)
			break
		}

		// Skip entries that were already processed before resume
		if skipped < skipCount {
			_ = j.Input.Value()
			_ = j.Input.Position()
			skipped++
			continue
		}

		j.pauseWg.Wait()
		threadlimiter <- true
		<-j.Rate.RateLimiter.C
		nextInput := j.Input.Value()
		nextPosition := j.Input.Position()
		nextInput["FFUFHASH"] = j.ffufHash(nextPosition)

		wg.Add(1)
		j.Stats.Counter++

		go func() {
			defer func() { <-threadlimiter }()
			defer wg.Done()
			threadStart := time.Now()
			j.runTask(nextInput, nextPosition, false)
			j.sleepIfNeeded()
			threadEnd := time.Now()
			j.Rate.Tick(threadStart, threadEnd)
		}()
		if !j.IsRunningJob() {
			j.Output.Warning(j.Error)
			return
		}
	}
}

func (j *Job) interruptMonitor() {
	sigChan := make(chan os.Signal, 2)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		for range sigChan {
			j.Output.Error(fmt.Sprintf("%s", j.Error))
			// resume if paused
			if j.Paused {
				j.pauseWg.Done()
			}
			j.Stop()
		}
	}()
}

func (j *Job) runBackgroundTasks(wg *sync.WaitGroup) {
	defer wg.Done()
	totalProgress := j.Input.Total()
	for j.Stats.Counter <= totalProgress && !j.skipQueue {
		// Check context
		select {
		case <-j.Config.Context.Done():
			return
		default:
		}

		j.pauseWg.Wait()
		if !j.IsRunning() {
			break
		}
		j.updateProgress()
		if j.Stats.Counter == totalProgress {
			return
		}
		if !j.IsRunningJob() {
			return
		}
		time.Sleep(time.Millisecond * time.Duration(j.Config.ProgressFrequency))
	}
}

func (j *Job) updateProgress() {
	if j.SingleShot {
		return
	}
	prog := Progress{
		StartedAt:     j.startTimeJob,
		ReqCount:      j.Stats.Counter,
		ReqTotal:      j.Input.Total(),
		ReqSec:        j.Rate.CurrentRate(),
		QueuePos:      j.queuepos,
		QueueTotal:    len(j.queuejobs),
		ErrorCount:    j.Stats.ErrorCounter,
		CurrentJobUrl: j.Config.Url,
		Finished:      !j.IsRunningJob() && j.queuepos >= len(j.queuejobs),
	}
	j.Output.Progress(prog)
}

func (j *Job) isMatch(resp Response) bool {
	matched := false
	var matchers map[string]FilterProvider
	var filters map[string]FilterProvider
	if j.Config.AutoCalibrationPerHost {
		filters = j.Config.MatcherManager.FiltersForDomain(HostURLFromRequest(*resp.Request))
	} else {
		filters = j.Config.MatcherManager.GetFilters()
	}
	matchers = j.Config.MatcherManager.GetMatchers()
	for _, m := range matchers {
		match, err := m.Filter(&resp)
		if err != nil {
			continue
		}
		if match {
			matched = true
		} else if j.Config.MatcherMode == "and" {
			// we already know this isn't "and" match
			return false

		}
	}
	// The response was not matched, return before running filters
	if !matched {
		return false
	}
	for _, f := range filters {
		fv, err := f.Filter(&resp)
		if err != nil {
			continue
		}
		if fv {
			//	return false
			if j.Config.FilterMode == "or" {
				// return early, as filter matched
				return false
			}
		} else {
			if j.Config.FilterMode == "and" {
				// return early as not all filters matched in "and" mode
				return true
			}
		}
	}
	if len(filters) > 0 && j.Config.FilterMode == "and" {
		// we did not return early, so all filters were matched
		return false
	}
	return true
}

func (j *Job) ffufHash(pos int) []byte {
	hashstring := ""
	r := []rune(j.Jobhash)
	if len(r) > 5 {
		hashstring = string(r[:5])
	}
	hashstring += fmt.Sprintf("%x", pos)
	return []byte(hashstring)
}

func (j *Job) handleScraperResult(resp *Response, sres ScraperResult) {
	for _, a := range sres.Action {
		switch a {
		case "output":
			resp.ScraperData[sres.Name] = sres.Results
		}
	}
}

// handleRecursion uses the configured strategy and coordinator to determine if recursion should happen
func (j *Job) handleRecursion(resp Response) {
	shouldRecurse, baseUrl := j.RecursionStrategy.ShouldRecurse(&resp)
	if !shouldRecurse {
		return
	}

	coordinator := j.Config.RecursionCoordinator
	if coordinator == nil {
		return
	}

	recUrl, shouldEnqueue := coordinator.TryEnqueue(baseUrl, j.currentDepth)
	if recUrl == "" {
		return // disabled or already visited
	}

	if shouldEnqueue {
		newJob := QueueJob{Url: recUrl, depth: j.currentDepth + 1, req: RecursionRequest(j.Config, recUrl), SingleShot: false}
		j.QueueMutex.Lock()
		j.queuejobs = append(j.queuejobs, newJob)
		j.QueueMutex.Unlock()
		// j.Output.Info(fmt.Sprintf("Adding a new job to the queue: %s", recUrl))
	} else {
		j.Output.Warning(fmt.Sprintf("Directory found, but recursion depth exceeded. Ignoring: %s", baseUrl))
	}
}

// CheckStop stops the job if stopping conditions are met
func (j *Job) CheckStop() {
	if j.Stats.Counter > MIN_SAMPLES_FOR_STOP_CHECK {
		// We have enough samples
		if j.Config.StopOn403 || j.Config.StopOnAll {
			if float64(j.Stats.Count403)/float64(j.Stats.Counter) > MAX_ERROR_RATIO_403 {
				// Over 95% of requests are 403
				j.Error = "Getting an unusual amount of 403 responses, exiting."
				j.Stop()
			}
		}
		if j.Config.StopOnErrors || j.Config.StopOnAll {
			if j.Stats.SpuriousErrorCounter > j.Config.Threads*MAX_SPURIOUS_ERROR_MULTIPLIER {
				// Most of the requests are erroring
				j.Error = "Receiving spurious errors, exiting."
				j.Stop()
			}
		}
		if j.Config.StopOnAll && (float64(j.Stats.Count429)/float64(j.Stats.Counter) > MAX_ERROR_RATIO_429) {
			// Over 20% of responses are 429
			j.Error = "Getting an unusual amount of 429 responses, exiting."
			j.Stop()
		}
	}

	// Check for runtime of entire process
	if j.Config.MaxTime > 0 {
		dur := time.Since(j.startTime)
		runningSecs := int(dur / time.Second)
		if runningSecs >= j.Config.MaxTime {
			j.Error = "Maximum running time for entire process reached, exiting."
			j.Stop()
		}
	}

	// Check for runtime of current job
	if j.Config.MaxTimeJob > 0 {
		dur := time.Since(j.startTimeJob)
		runningSecs := int(dur / time.Second)
		if runningSecs >= j.Config.MaxTimeJob {
			j.Error = "Maximum running time for this job reached, continuing with next job if one exists."
			j.Next()

		}
	}
}

// Stop the execution of the Job
func (j *Job) Stop() {
	j.running.Store(false)
	j.Config.Cancel()
}

// Stop current, resume to next
func (j *Job) Next() {
	j.runningJob.Store(false)
}

// ExportState returns the current job state for saving
func (j *Job) ExportState() ResumeState {
	j.QueueMutex.Lock()
	defer j.QueueMutex.Unlock()
	j.VisitedMutex.Lock()
	defer j.VisitedMutex.Unlock()

	// Clone queue
	queueCopy := make([]QueueJob, len(j.queuejobs))
	copy(queueCopy, j.queuejobs)

	// Clone visited
	visitedCopy := make(map[string]bool)
	for k, v := range j.VisitedURLs {
		visitedCopy[k] = v
	}

	return ResumeState{
		Queue:       queueCopy,
		VisitedURLs: visitedCopy,
		Position:    j.Stats.Counter,
	}
}

// ImportState loads the state functionality
func (j *Job) ImportState(state *ResumeState) {
	j.QueueMutex.Lock()
	defer j.QueueMutex.Unlock()
	j.VisitedMutex.Lock()
	defer j.VisitedMutex.Unlock()

	j.queuejobs = state.Queue
	j.VisitedURLs = state.VisitedURLs
	j.Config.ResumePosition = state.Position
}
