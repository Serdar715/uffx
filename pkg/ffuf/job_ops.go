package ffuf

import (
	"fmt"
	"log"
	"os"
	"time"
)

// runTask executes a single task (implementation of TaskExecutor logic)
func (j *Job) runTask(input map[string][]byte, position int, retried bool) {
	req, err := j.prepareRequest(input, position)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while preparing request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return
	}

	resp, err := j.executeRequest(req)
	if err != nil {
		j.handleExecutionError(err, input, position, retried)
		return
	}

	j.handleResponse(resp, input, position)
}

// prepareRequest handles the request preparation logic
func (j *Job) prepareRequest(input map[string][]byte, position int) (Request, error) {
	basereq := j.queuejobs[j.queuepos-1].req
	req, err := j.Runner.Prepare(input, &basereq)
	if err != nil {
		return Request{}, err
	}
	req.Timestamp = time.Now()
	req.Position = position
	return req, nil
}

// executeRequest handles the request execution and audit logging
func (j *Job) executeRequest(req Request) (Response, error) {
	// Audit the request
	if j.AuditLogger != nil {
		if err := j.AuditLogger.Write(&req); err != nil {
			j.Output.Error(fmt.Sprintf("Encountered error while writing request audit log: %s\n", err))
		}
	}

	resp, err := j.Runner.Execute(&req)
	if err != nil {
		req.Error = err.Error()
		return resp, err
	}

	// Audit the response
	if j.AuditLogger != nil {
		if err := j.AuditLogger.Write(&resp); err != nil {
			j.Output.Error(fmt.Sprintf("Encountered error while writing response audit log: %s\n", err))
		}
	}

	return resp, nil
}

// handleExecutionError deals with errors during request execution
func (j *Job) handleExecutionError(err error, input map[string][]byte, position int, retried bool) {
	if retried {
		j.incError()
		log.Printf("%s", err)
	} else {
		j.runTask(input, position, true)
	}

	if os.IsTimeout(err) {
		j.checkForTimeoutMatchers(input)
	}
}

// checkForTimeoutMatchers checks if a timeout matcher or filter is active and logs it
func (j *Job) checkForTimeoutMatchers(input map[string][]byte) {
	inputmsg := ""
	for k, v := range input {
		inputmsg += fmt.Sprintf("%s : %s  // ", k, v)
	}

	for name := range j.Config.MatcherManager.GetMatchers() {
		if name == "time" {
			j.Output.Info("Timeout while 'time' matcher is active: " + inputmsg)
			return
		}
	}
	for name := range j.Config.MatcherManager.GetFilters() {
		if name == "time" {
			j.Output.Info("Timeout while 'time' filter is active: " + inputmsg)
			return
		}
	}
}

// handleResponse processes the response including error counting, scraping, and result output
func (j *Job) handleResponse(resp Response, input map[string][]byte, position int) {
	// Spurious Error Handling
	if j.Stats.SpuriousErrorCounter > 0 {
		j.resetSpuriousErrors()
	}

	// Stop conditions handling
	if j.Config.StopOn403 || j.Config.StopOnAll {
		if resp.StatusCode == 403 {
			j.inc403()
		}
	}
	if j.Config.StopOnAll {
		if resp.StatusCode == 429 {
			j.inc429()
		}
	}

	j.pauseWg.Wait()

	// Auto-calibration
	_ = j.CalibrateIfNeeded(HostURLFromRequest(*resp.Request), input)

	// Scraper
	if j.Scraper != nil {
		for _, sres := range j.Scraper.Execute(&resp, j.isMatch(resp)) {
			resp.ScraperData[sres.Name] = sres.Results
			j.handleScraperResult(&resp, sres)
		}
	}

	// Matching logic
	if j.isMatch(resp) {
		if j.Detector.IsFalsePositive(&resp) {
			return
		}

		// Replay Proxy
		if j.ReplayRunner != nil {
			// We need to reconstruct basereq potentially or just pass what we have
			// ReplayRunner Prepare needs basereq.
			// In original runTask: basereq := j.queuejobs[j.queuepos-1].req
			// We can get it again
			basereq := j.queuejobs[j.queuepos-1].req
			replayreq, err := j.ReplayRunner.Prepare(input, &basereq)
			replayreq.Position = position
			if err != nil {
				j.Output.Error(fmt.Sprintf("Encountered an error while preparing replayproxy request: %s\n", err))
				j.incError()
				log.Printf("%s", err)
			} else {
				_, _ = j.ReplayRunner.Execute(&replayreq)
			}
		}

		j.Output.Result(resp)
		j.updateProgress()

		// Recursion
		if j.Config.Recursion {
			j.handleRecursion(resp)
		}

		// Hooks targets
		j.handleNewTargets(resp.NewTargets, false)
	}
	// Removed: else block that was showing filtered results when ScraperData exists
	// This was causing -fl filter to not work properly

	// SingleShot targets check
	if j.IsRunning() && len(resp.CheckTargets) > 0 {
		j.handleNewTargets(resp.CheckTargets, true)
	}
}

func (j *Job) handleNewTargets(targets []string, singleShot bool) {
	for _, target := range targets {
		if !j.IsRunning() {
			break
		}

		j.VisitedMutex.Lock()
		if j.VisitedURLs[target] {
			j.VisitedMutex.Unlock()
			continue
		}
		j.VisitedURLs[target] = true
		j.VisitedMutex.Unlock()

		req := RecursionRequest(j.Config, target)
		j.QueueMutex.Lock()
		j.queuejobs = append(j.queuejobs, QueueJob{Url: target, depth: j.currentDepth, req: req, SingleShot: singleShot})
		j.QueueMutex.Unlock()
		if !singleShot {
			j.Output.Info(fmt.Sprintf("Queuing new target: %s", target))
		}
	}
}
