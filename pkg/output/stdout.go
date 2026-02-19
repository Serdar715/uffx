package output

import (
	"crypto/md5"
	"fmt"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

const (
	BANNER_HEADER = `
  _   _  ______ ______ 
 | | | ||  ____|  ____|
 | | | || |__  | |__   
 | | | ||  __| |  __|  
 | |_| || |    | |     
  \___/ |_|    |_|       v2.1.1-uff-dev

       by sw33tLie and XBug0
`
	BANNER_SEP = "________________________________________________"
	// Minimum interval between progress redraws to avoid flickering
	ProgressThrottleInterval = 40 * time.Millisecond
)

type Stdoutput struct {
	config         *ffuf.Config
	fuzzkeywords   []string
	Results        []ffuf.Result
	CurrentResults []ffuf.Result
	Formatter      OutputFormatter // Strategy Pattern
	mutex          sync.Mutex
	activeJobs     map[string]ffuf.Progress
	linesPrinted   int
	lastPrint      time.Time
}

func NewStdoutput(conf *ffuf.Config) *Stdoutput {
	var outp Stdoutput
	outp.config = conf
	outp.Results = make([]ffuf.Result, 0)
	outp.CurrentResults = make([]ffuf.Result, 0)
	outp.fuzzkeywords = make([]string, 0)
	for _, ip := range conf.InputProviders {
		outp.fuzzkeywords = append(outp.fuzzkeywords, ip.Keyword)
	}
	sort.Strings(outp.fuzzkeywords)

	// Default to StandardFormatter
	outp.Formatter = NewStandardFormatter(conf)
	outp.activeJobs = make(map[string]ffuf.Progress)

	return &outp
}

func (s *Stdoutput) Banner() {
	options := make(map[string]string)

	if s.config.MethodAsRawRequest || s.config.RequestFile != "" {
		options["Raw Request"] = s.config.Method
	} else {
		options["Method"] = s.config.Method
	}
	options["URL"] = s.config.Url

	for _, provider := range s.config.InputProviders {
		if provider.Name == "wordlist" {
			options["Wordlist"] = provider.Keyword + ": " + provider.Value
		}
	}

	if len(s.config.Headers) > 0 {
		// If request file is used, headers will be shown individually below
		if s.config.RequestFile == "" {
			options["Headers"] = fmt.Sprintf("%d custom headers", len(s.config.Headers))
		}
	}
	if len(s.config.Data) > 0 {
		options["Data"] = s.config.Data
	}

	if len(s.config.Extensions) > 0 {
		exts := ""
		for _, ext := range s.config.Extensions {
			exts = fmt.Sprintf("%s%s ", exts, ext)
		}
		options["Extensions"] = exts
	}

	if len(s.config.OutputFile) > 0 {
		OutputFile := s.config.OutputFile
		if s.config.OutputFormat == "all" {
			OutputFile += ".{json,ejson,html,md,csv,ecsv}"
		}
		options["Output file"] = OutputFile
		options["File format"] = s.config.OutputFormat
	}

	options["Follow redirects"] = fmt.Sprintf("%t", s.config.FollowRedirects)
	options["Calibration"] = fmt.Sprintf("%t", s.config.AutoCalibration)

	if len(s.config.ProxyURL) > 0 {
		options["Proxy"] = s.config.ProxyURL
	}
	if len(s.config.ReplayProxyURL) > 0 {
		options["ReplayProxy"] = s.config.ReplayProxyURL
	}

	options["Timeout"] = fmt.Sprintf("%d", s.config.Timeout)
	options["Threads"] = fmt.Sprintf("%d", s.config.Threads)

	if s.config.Delay.HasDelay {
		if s.config.Delay.IsRange {
			options["Delay"] = fmt.Sprintf("%.2f - %.2f seconds", s.config.Delay.Min, s.config.Delay.Max)
		} else {
			options["Delay"] = fmt.Sprintf("%.2f seconds", s.config.Delay.Min)
		}
	}

	for _, f := range s.config.MatcherManager.GetMatchers() {
		options["Matcher"] = f.ReprVerbose()
	}
	for _, f := range s.config.MatcherManager.GetFilters() {
		if val, ok := options["Filter"]; ok {
			options["Filter"] = val + ", " + f.ReprVerbose()
		} else {
			options["Filter"] = f.ReprVerbose()
		}
	}

	s.Formatter.Banner(options)

	// Display individual headers when request file is used
	if s.config.RequestFile != "" && len(s.config.Headers) > 0 {
		for headerName, headerValue := range s.config.Headers {
			fmt.Printf(" :: Header           : %s: %s\n", headerName, headerValue)
		}
	}
}

func (s *Stdoutput) Reset() {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.CurrentResults = make([]ffuf.Result, 0)
}

func (s *Stdoutput) Cycle() {
	s.mutex.Lock()
	s.Results = append(s.Results, s.CurrentResults...)
	s.mutex.Unlock()
	s.Reset()
}

func (s *Stdoutput) GetCurrentResults() []ffuf.Result {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	// Return copy to be safe? Or just the slice reference?
	// Slice reference is unsafe if underlying array changes.
	// But usually this interface is for saving.
	return s.CurrentResults
}

func (s *Stdoutput) SetCurrentResults(results []ffuf.Result) {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.CurrentResults = results
}

// Multi-line Progress Logic
func (s *Stdoutput) Progress(status ffuf.Progress) {
	if s.config.Quiet {
		return
	}

	s.mutex.Lock()
	defer s.mutex.Unlock()

	if status.Finished {
		delete(s.activeJobs, status.CurrentJobUrl)
	} else {
		// Only track relevant fields to save memory/time
		// We use the URL as key
		s.activeJobs[status.CurrentJobUrl] = status
	}

	// Limit actual verify/print frequency
	if time.Since(s.lastPrint) < ProgressThrottleInterval && !status.Finished {
		return
	}
	s.lastPrint = time.Now()

	s.redrawFooter()
}

func (s *Stdoutput) redrawFooter() {
	// 1. Move cursor up by s.linesPrinted
	if s.linesPrinted > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA", s.linesPrinted)
		// 2. Clear from cursor to end of screen
		fmt.Fprintf(os.Stderr, "\033[J")
	}

	// 3. Prepare list of jobs to print
	var jobUrls []string
	for k := range s.activeJobs {
		jobUrls = append(jobUrls, k)
	}
	sort.Strings(jobUrls)

	// 4. Print each job
	for _, url := range jobUrls {
		status := s.activeJobs[url]
		dur := time.Since(status.StartedAt)
		// runningSecs := int(dur / time.Second) // unused
		var reqRate int64 = status.ReqSec

		// Custom format: [####>---] ...
		// Calculate percentage
		percent := 0
		if status.ReqTotal > 0 {
			percent = (status.ReqCount * 100) / status.ReqTotal
		}
		// Bar size 10
		barLen := 10
		doneLen := (barLen * percent) / 100
		if doneLen > barLen {
			doneLen = barLen
		}
		bar := "[" + strings.Repeat("#", doneLen)
		if doneLen < barLen {
			bar += ">" + strings.Repeat("-", barLen-doneLen-1)
		} else {
			// If full, maybe just ] or #]
			// strings.Repeat("-", -1) panics
		}
		if len(bar) < barLen+2 {
			bar += strings.Repeat("-", barLen+2-len(bar)) + "]"
		} else {
			if !strings.HasSuffix(bar, "]") {
				bar += "]"
			}
		}

		// Fixed layout
		// [##>-------] - 12s   123/4567   150/s   http://...
		line := fmt.Sprintf("%s - %-5s %6d/%-6d %5d/s   %s", bar, fmt.Sprintf("%ds", int(dur.Seconds())), status.ReqCount, status.ReqTotal, reqRate, url)

		// Truncate to avoid wrapping which breaks cursor movement
		if len(line) > 100 {
			line = line[:97] + "..."
		}
		fmt.Fprintf(os.Stderr, "%s\n", line)
	}

	s.linesPrinted = len(jobUrls)
}

func (s *Stdoutput) Info(infostring string) {
	s.Formatter.Info(infostring)
}

func (s *Stdoutput) Error(errstring string) {
	s.Formatter.Error(errstring)
}

func (s *Stdoutput) Warning(warnstring string) {
	s.Formatter.Warning(warnstring)
}

func (s *Stdoutput) Raw(output string) {
	fmt.Fprintf(os.Stderr, "%s%s", TERMINAL_CLEAR_LINE, output)
}

func (s *Stdoutput) writeToAll(filename string, config *ffuf.Config, res []ffuf.Result) error {
	var err error
	var BaseFilename string = s.config.OutputFile

	s.config.OutputFile = BaseFilename + ".json"
	err = writeJSON(s.config.OutputFile, s.config, res)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".ejson"
	err = writeEJSON(s.config.OutputFile, s.config, res)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".html"
	err = writeHTML(s.config.OutputFile, s.config, res)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".md"
	err = writeMarkdown(s.config.OutputFile, s.config, res)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".csv"
	err = writeCSV(s.config.OutputFile, s.config, res, false)
	if err != nil {
		s.Error(err.Error())
	}

	s.config.OutputFile = BaseFilename + ".ecsv"
	err = writeCSV(s.config.OutputFile, s.config, res, true)
	if err != nil {
		s.Error(err.Error())
	}

	return nil
}

func (s *Stdoutput) SaveFile(filename, format string) error {
	var err error
	s.mutex.Lock()
	allResults := append(s.Results, s.CurrentResults...)
	s.mutex.Unlock()

	if s.config.OutputSkipEmptyFile && len(allResults) == 0 {
		s.Info("No results and -or defined, output file not written.")
		return err
	}
	switch format {
	case "all":
		err = s.writeToAll(filename, s.config, allResults)
	case "json":
		err = writeJSON(filename, s.config, allResults)
	case "ejson":
		err = writeEJSON(filename, s.config, allResults)
	case "html":
		err = writeHTML(filename, s.config, allResults)
	case "md":
		err = writeMarkdown(filename, s.config, allResults)
	case "csv":
		err = writeCSV(filename, s.config, allResults, false)
	case "ecsv":
		err = writeCSV(filename, s.config, allResults, true)
	}
	return err
}

func (s *Stdoutput) Finalize() error {
	var err error
	// -ol flag: save links to file
	if len(s.config.OutputLinksFile) > 0 {
		err = s.saveLinksToFile(s.config.OutputLinksFile)
		if err != nil {
			s.Error(err.Error())
		}
	} else if s.config.OutputFile != "" {
		// Normal output save
		err = s.SaveFile(s.config.OutputFile, s.config.OutputFormat)
		if err != nil {
			s.Error(err.Error())
		}
	}
	s.Formatter.Finalize()
	return nil
}

// saveLinksToFile saves only URLs to the output file when -ol flag is used
func (s *Stdoutput) saveLinksToFile(filename string) error {
	s.mutex.Lock()
	allResults := append(s.Results, s.CurrentResults...)
	s.mutex.Unlock()

	if s.config.OutputSkipEmptyFile && len(allResults) == 0 {
		s.Info("No results and -or defined, output file not written.")
		return nil
	}

	// Create or truncate the file
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write each URL on a separate line
	for _, res := range allResults {
		_, err := file.WriteString(res.Url + "\n")
		if err != nil {
			return err
		}
	}

	s.Info(fmt.Sprintf("Saved %d links to %s", len(allResults), filename))
	return nil
}

func (s *Stdoutput) PrintResult(res ffuf.Result) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Clear footer before printing result
	if s.linesPrinted > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA", s.linesPrinted)
		fmt.Fprintf(os.Stderr, "\033[J")
	}

	s.Formatter.Result(res)

	// Redraw footer
	// We need to call redrawFooter here but s.redrawFooter() is not safe to call recursively if it locked,
	// but here we already hold the lock.
	// So we need to copy redrawFooter logic or extract it to a private method that assumes lock is held.
	// Since redrawFooter in my implementation doesn't lock itself (it assumes caller holds lock), strict speaking:
	// Wait, did I make redrawFooter lock?
	// In the previous replacement chunk, redrawFooter does NOT lock.
	// So we can call it here.

	// BUT, redrawFooter relies on s.activeJobs which is protected by mutex.
	// Since we are holding the lock, it is safe.

	// Re-draw immediately to keep UI responsive
	// However, redrawFooter assumes it's called from Progress which does rate limiting.
	// Here we force it because we just printed a line and broke the visual layout.
	s.forceRedrawFooter()
}

func (s *Stdoutput) forceRedrawFooter() {
	// Same as redrawFooter but without internal checks if any
	// ... Actually my redrawFooter implementation above is pure logic, no checks.
	// So we can just call the logic.
	// Copy-paste for safety to avoid method signature issues in `replace_content`?
	// No, better to define `redrawFooter` as private helper `_redrawFooter` and call it.
	// But `redrawFooter` is already added as a method.
	// Let's just duplicate logic slightly or trust the method availability.
	// Refactoring note: I will use the code directly here to be safe and atomic.

	// 3. Prepare list of jobs to print
	var jobUrls []string
	for k := range s.activeJobs {
		jobUrls = append(jobUrls, k)
	}
	sort.Strings(jobUrls)

	// 4. Print each job
	for _, url := range jobUrls {
		status := s.activeJobs[url]
		dur := time.Since(status.StartedAt)
		var reqRate int64 = status.ReqSec
		percent := 0
		if status.ReqTotal > 0 {
			percent = (status.ReqCount * 100) / status.ReqTotal
		}
		barLen := 10
		doneLen := (barLen * percent) / 100
		if doneLen > barLen {
			doneLen = barLen
		}
		bar := "[" + strings.Repeat("#", doneLen)
		if doneLen < barLen {
			bar += ">" + strings.Repeat("-", barLen-doneLen-1)
		}
		if len(bar) < barLen+2 {
			bar += strings.Repeat("-", barLen+2-len(bar)) + "]"
		} else {
			if !strings.HasSuffix(bar, "]") {
				bar += "]"
			}
		}
		fmt.Fprintf(os.Stderr, "%s - %-5s %6d/%-6d %5d/s   %s\n", bar, fmt.Sprintf("%ds", int(dur.Seconds())), status.ReqCount, status.ReqTotal, reqRate, url)
	}
	s.linesPrinted = len(jobUrls)
}

func (s *Stdoutput) Result(resp ffuf.Response) {
	if len(s.config.OutputDirectory) > 0 {
		resp.ResultFile = s.writeResultToFile(resp)
	}

	inputs := make(map[string][]byte, len(resp.Request.Input))
	for k, v := range resp.Request.Input {
		inputs[k] = v
	}
	sResult := ffuf.Result{
		Input:            inputs,
		Position:         resp.Request.Position,
		StatusCode:       resp.StatusCode,
		ContentLength:    resp.ContentLength,
		ContentWords:     resp.ContentWords,
		ContentLines:     resp.ContentLines,
		ContentType:      resp.ContentType,
		RedirectLocation: resp.GetRedirectLocation(true),
		ScraperData:      resp.ScraperData,
		Url:              resp.Request.Url,
		Duration:         resp.Duration,
		ResultFile:       resp.ResultFile,
		Host:             resp.Host,
		Distance:         resp.Distance,
	}

	s.mutex.Lock()
	s.CurrentResults = append(s.CurrentResults, sResult)

	// Clear footer, print result, redraw footer — all under mutex
	// to prevent race condition with Progress() which also holds mutex
	if s.linesPrinted > 0 {
		fmt.Fprintf(os.Stderr, "\033[%dA", s.linesPrinted)
		fmt.Fprintf(os.Stderr, "\033[J")
	}

	s.Formatter.Result(sResult)
	s.linesPrinted = 0

	// Redraw footer
	s.forceRedrawFooter()
	s.mutex.Unlock()
}

func (s *Stdoutput) writeResultToFile(resp ffuf.Response) string {
	var fileContent, fileName, filePath string
	if s.config.OutputDirectory != "" {
		err := os.MkdirAll(s.config.OutputDirectory, 0750)
		if err != nil {
			if !os.IsExist(err) {
				s.Error(err.Error())
				return ""
			}
		}
	}
	fileContent = fmt.Sprintf("%s\n---- ↑ Request ---- Response ↓ ----\n\n%s", resp.Request.Raw, resp.Raw)
	fileName = fmt.Sprintf("%x", md5.Sum([]byte(fileContent)))

	filePath = path.Join(s.config.OutputDirectory, fileName)
	err := os.WriteFile(filePath, []byte(fileContent), 0640)
	if err != nil {
		s.Error(err.Error())
	}
	return fileName
}

func printOption(name []byte, value []byte) {
	if strings.HasSuffix(string(value), "NOCOLON") {
		value = value[:len(value)-9]
	}
	fmt.Fprintf(os.Stderr, " :: %-16s : %s\n", name, value)
}
