package output

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

type StandardFormatter struct {
	config       *ffuf.Config
	fuzzkeywords []string
}

func NewStandardFormatter(conf *ffuf.Config) *StandardFormatter {
	f := &StandardFormatter{
		config: conf,
	}
	f.fuzzkeywords = make([]string, 0)
	for _, ip := range conf.InputProviders {
		f.fuzzkeywords = append(f.fuzzkeywords, ip.Keyword)
	}
	sort.Strings(f.fuzzkeywords)
	return f
}

func (f *StandardFormatter) Printf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func (f *StandardFormatter) Println(a ...interface{}) {
	fmt.Fprintln(os.Stderr, a...)
}

func (f *StandardFormatter) Banner(options map[string]string) {
	version := strings.ReplaceAll(ffuf.Version(), "<3", fmt.Sprintf("%s<3%s", ANSI_RED, ANSI_CLEAR))
	_ = version
	fmt.Fprintf(os.Stderr, "%s\n%s\n\n", BANNER_HEADER, BANNER_SEP)

	// Sort keys for consistent output
	keys := make([]string, 0, len(options))
	for k := range options {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		printOption([]byte(k), []byte(options[k]))
	}
	fmt.Fprintf(os.Stderr, "%s\n\n", BANNER_SEP)
}

func (f *StandardFormatter) Result(res ffuf.Result) {
	switch {
	case f.config.OutputLinks:
		fmt.Println(res.Url)
	case f.config.Json:
		f.resultJson(res)
	case f.config.Quiet:
		f.resultQuiet(res)
	case len(f.fuzzkeywords) > 1 || f.config.Verbose || len(f.config.OutputDirectory) > 0 || len(res.ScraperData) > 0:
		f.resultMultiline(res)
	default:
		f.resultNormal(res)
	}
}

func (f *StandardFormatter) Warning(msg string) {
	if f.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", msg)
	} else {
		if !f.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[WARN] %s\n", TERMINAL_CLEAR_LINE, msg)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sWARN%s] %s\n", TERMINAL_CLEAR_LINE, ANSI_RED, ANSI_CLEAR, msg)
		}
	}
}

func (f *StandardFormatter) Error(msg string) {
	if f.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", msg)
	} else {
		if !f.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[ERR] %s\n", TERMINAL_CLEAR_LINE, msg)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sERR%s] %s\n", TERMINAL_CLEAR_LINE, ANSI_RED, ANSI_CLEAR, msg)
		}
	}
}

func (f *StandardFormatter) Info(msg string) {
	if f.config.Quiet {
		fmt.Fprintf(os.Stderr, "%s", msg)
	} else {
		if !f.config.Colors {
			fmt.Fprintf(os.Stderr, "%s[INFO] %s\n\n", TERMINAL_CLEAR_LINE, msg)
		} else {
			fmt.Fprintf(os.Stderr, "%s[%sINFO%s] %s\n\n", TERMINAL_CLEAR_LINE, ANSI_BLUE, ANSI_CLEAR, msg)
		}
	}
}

func (f *StandardFormatter) Finalize() error {
	if !f.config.Quiet {
		fmt.Fprintf(os.Stderr, "\n")
	}
	return nil
}

// Internal helper methods copied/adapted from stdout.go
func (f *StandardFormatter) resultJson(res ffuf.Result) {
	resBytes, err := json.Marshal(res)
	if err != nil {
		f.Error(err.Error())
	} else {
		fmt.Fprint(os.Stderr, TERMINAL_CLEAR_LINE)
		fmt.Println(string(resBytes))
	}
}

func (f *StandardFormatter) resultQuiet(res ffuf.Result) {
	if f.config.LFI {
		fmt.Println(res.Url)
	} else {
		fmt.Println(f.prepareInputsOneLine(res))
	}
}

func (f *StandardFormatter) resultMultiline(res ffuf.Result) {
	var res_hdr, res_str string
	res_str = "%s%s    * %s: %s\n"
	if f.config.ShowDiff {
		res_hdr = fmt.Sprintf("%s%s[Status: %d, Size: %d, Words: %d, Lines: %d, Duration: %dms, Diff: %.2f%%]%s", TERMINAL_CLEAR_LINE, f.colorize(res.StatusCode), res.StatusCode, res.ContentLength, res.ContentWords, res.ContentLines, res.Duration.Milliseconds(), res.Distance*100, ANSI_CLEAR)
	} else {
		res_hdr = fmt.Sprintf("%s%s[Status: %d, Size: %d, Words: %d, Lines: %d, Duration: %dms]%s", TERMINAL_CLEAR_LINE, f.colorize(res.StatusCode), res.StatusCode, res.ContentLength, res.ContentWords, res.ContentLines, res.Duration.Milliseconds(), ANSI_CLEAR)
	}
	reslines := ""
	if f.config.Verbose || len(res.ScraperData) > 0 {
		displayUrl := res.Url
		if res.Host != "" {
			u, err := url.Parse(displayUrl)
			if err == nil {
				if u.Hostname() != res.Host {
					u.Host = res.Host
					displayUrl = u.String()
				}
			}
		}

		reslines = fmt.Sprintf("%s%s| URL | %s\n", reslines, TERMINAL_CLEAR_LINE, displayUrl)
		redirectLocation := res.RedirectLocation
		if redirectLocation != "" {
			reslines = fmt.Sprintf("%s%s| --> | %s\n", reslines, TERMINAL_CLEAR_LINE, redirectLocation)
		}
	}
	if res.ResultFile != "" {
		reslines = fmt.Sprintf("%s%s| RES | %s\n", reslines, TERMINAL_CLEAR_LINE, res.ResultFile)
	}
	for _, k := range f.fuzzkeywords {
		if ffuf.StrInSlice(k, f.config.CommandKeywords) {
			reslines = fmt.Sprintf(res_str, reslines, TERMINAL_CLEAR_LINE, k, strconv.Itoa(res.Position))
		} else {
			reslines = fmt.Sprintf(res_str, reslines, TERMINAL_CLEAR_LINE, k, res.Input[k])
		}
	}
	if len(res.ScraperData) > 0 {
		reslines = fmt.Sprintf("%s%s| SCR |\n", reslines, TERMINAL_CLEAR_LINE)
		for k, vslice := range res.ScraperData {
			for _, v := range vslice {
				reslines = fmt.Sprintf(res_str, reslines, TERMINAL_CLEAR_LINE, k, v)
			}
		}
	}
	fmt.Printf("%s\n%s\n", res_hdr, reslines)
}

func (f *StandardFormatter) resultNormal(res ffuf.Result) {
	var resnormal string
	redirectInfo := ""

	// Construct display URL based on Host header if needed
	displayUrl := res.Url
	if res.Host != "" {
		u, err := url.Parse(displayUrl)
		if err == nil {
			if u.Hostname() != res.Host {
				u.Host = res.Host
				displayUrl = u.String()
			}
		}
	}

	if f.config.ShowRedirect && res.RedirectLocation != "" && res.StatusCode >= 300 && res.StatusCode < 400 {
		if f.config.Colors {
			redirectInfo = fmt.Sprintf(" %s--> %s (%d) --> %s%s", ANSI_BLUE, displayUrl, res.StatusCode, res.RedirectLocation, ANSI_CLEAR)
		} else {
			redirectInfo = fmt.Sprintf(" --> %s (%d) --> %s", displayUrl, res.StatusCode, res.RedirectLocation)
		}
	} else {
		// Show URL for non-redirects
		if f.config.Colors {
			redirectInfo = fmt.Sprintf(" %s--> %s%s", ANSI_BLUE, displayUrl, ANSI_CLEAR)
		} else {
			redirectInfo = fmt.Sprintf(" --> %s", displayUrl)
		}
	}

	if f.config.ShowDiff {
		resnormal = fmt.Sprintf("%s%s%-23s [Status: %d, Size: %d, Words: %d, Lines: %d, Duration: %dms, Diff: %.2f%%]%s%s", TERMINAL_CLEAR_LINE, f.colorize(res.StatusCode), f.prepareInputsOneLine(res), res.StatusCode, res.ContentLength, res.ContentWords, res.ContentLines, res.Duration.Milliseconds(), res.Distance*100, ANSI_CLEAR, redirectInfo)
	} else {
		resnormal = fmt.Sprintf("%s%s%-23s [Status: %d, Size: %d, Words: %d, Lines: %d, Duration: %dms]%s%s", TERMINAL_CLEAR_LINE, f.colorize(res.StatusCode), f.prepareInputsOneLine(res), res.StatusCode, res.ContentLength, res.ContentWords, res.ContentLines, res.Duration.Milliseconds(), ANSI_CLEAR, redirectInfo)
	}
	fmt.Println(resnormal)
}

func (f *StandardFormatter) prepareInputsOneLine(res ffuf.Result) string {
	inputs := ""
	if len(f.fuzzkeywords) > 1 {
		for _, k := range f.fuzzkeywords {
			if ffuf.StrInSlice(k, f.config.CommandKeywords) {
				inputs = fmt.Sprintf("%s%s : %s ", inputs, k, strconv.Itoa(res.Position))
			} else {
				inputs = fmt.Sprintf("%s%s : %s ", inputs, k, res.Input[k])
			}
		}
	} else {
		for _, k := range f.fuzzkeywords {
			if ffuf.StrInSlice(k, f.config.CommandKeywords) {
				inputs = strconv.Itoa(res.Position)
			} else {
				val := string(res.Input[k])
				if val == "" {
					inputs = "<EMPTY>"
				} else {
					inputs = val
				}
			}
		}
	}
	return inputs
}

func (f *StandardFormatter) colorize(status int64) string {
	if !f.config.Colors {
		return ""
	}
	colorCode := ANSI_CLEAR
	if status >= 200 && status < 300 {
		colorCode = ANSI_GREEN
	}
	if status >= 300 && status < 400 {
		colorCode = ANSI_BLUE
	}
	if status >= 400 && status < 500 {
		colorCode = ANSI_YELLOW
	}
	if status >= 500 && status < 600 {
		colorCode = ANSI_RED
	}
	return colorCode
}
