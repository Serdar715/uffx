package config

import (
	"flag"
	"log/slog"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
	"github.com/sw33tLie/uff/v2/pkg/filter"
)

func SetupFilters(parseOpts *ffuf.ConfigOptions, conf *ffuf.Config) error {
	errs := ffuf.NewMultierror()
	conf.MatcherManager = filter.NewMatcherManager()
	// If any other matcher is set, ignore -mc default value
	matcherSet := false
	statusSet := false
	warningIgnoreBody := false
	// Configuration for flags that act as matchers
	type matcherConfig struct {
		IsMatcher   bool
		WarnIgnBody bool
	}
	matcherFlags := map[string]matcherConfig{
		"ms":  {IsMatcher: true, WarnIgnBody: true},
		"ml":  {IsMatcher: true, WarnIgnBody: true},
		"mw":  {IsMatcher: true, WarnIgnBody: true},
		"mr":  {IsMatcher: true, WarnIgnBody: false},
		"mt":  {IsMatcher: true, WarnIgnBody: false},
		"lfi": {IsMatcher: true, WarnIgnBody: false},
	}

	flag.Visit(func(flg *flag.Flag) {
		if flg.Name == "mc" {
			statusSet = true
		}
		if conf, ok := matcherFlags[flg.Name]; ok {
			if conf.IsMatcher {
				matcherSet = true
			}
			if conf.WarnIgnBody {
				warningIgnoreBody = true
			}
		}
	})
	// Only set default matchers if no
	if statusSet || !matcherSet {
		if err := conf.MatcherManager.AddMatcher("status", parseOpts.Matcher.Status); err != nil {
			errs.Add(err)
		}
	}

	if parseOpts.Filter.Status != "" {
		if err := conf.MatcherManager.AddFilter("status", parseOpts.Filter.Status, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Size != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("size", parseOpts.Filter.Size, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Regexp != "" {
		if err := conf.MatcherManager.AddFilter("regexp", parseOpts.Filter.Regexp, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Words != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("word", parseOpts.Filter.Words, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Lines != "" {
		warningIgnoreBody = true
		if err := conf.MatcherManager.AddFilter("line", parseOpts.Filter.Lines, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Filter.Time != "" {
		if err := conf.MatcherManager.AddFilter("time", parseOpts.Filter.Time, false); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Size != "" {
		if err := conf.MatcherManager.AddMatcher("size", parseOpts.Matcher.Size); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Regexp != "" {
		if err := conf.MatcherManager.AddMatcher("regexp", parseOpts.Matcher.Regexp); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Words != "" {
		if err := conf.MatcherManager.AddMatcher("word", parseOpts.Matcher.Words); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Lines != "" {
		if err := conf.MatcherManager.AddMatcher("line", parseOpts.Matcher.Lines); err != nil {
			errs.Add(err)
		}
	}
	if conf.LFI {
		if err := conf.MatcherManager.AddMatcher("lfi", ""); err != nil {
			errs.Add(err)
		}
	}
	if parseOpts.Matcher.Time != "" {
		if err := conf.MatcherManager.AddMatcher("time", parseOpts.Matcher.Time); err != nil {
			errs.Add(err)
		}
	}
	if conf.IgnoreBody && warningIgnoreBody {
		slog.Warn("Possible undesired combination of -ignore-body and the response options: fl,fs,fw,ml,ms and mw")
	}
	return errs.ErrorOrNil()
}
