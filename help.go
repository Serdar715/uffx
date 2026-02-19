package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

type UsageSection struct {
	Name          string
	Description   string
	Flags         []UsageFlag
	Hidden        bool
	ExpectedFlags []string
}

// PrintSection prints out the section name, description and each of the flags
func (u *UsageSection) PrintSection(max_length int, extended bool) {
	// Do not print if extended usage not requested and section marked as hidden
	if !extended && u.Hidden {
		return
	}
	fmt.Printf("%s:\n", u.Name)
	for _, f := range u.Flags {
		f.PrintFlag(max_length)
	}
	fmt.Printf("\n")
}

type UsageFlag struct {
	Name        string
	Description string
	Default     string
}

// PrintFlag prints out the flag name, usage string and default value
func (f *UsageFlag) PrintFlag(max_length int) {
	// Create format string, used for padding
	format := fmt.Sprintf("  -%%-%ds %%s", max_length)
	if f.Default != "" {
		format = format + " (default: %s)\n"
		fmt.Printf(format, f.Name, f.Description, f.Default)
	} else {
		format = format + "\n"
		fmt.Printf(format, f.Name, f.Description)
	}
}

func Usage() {
	u_http := UsageSection{
		Name:          "HTTP OPTIONS",
		Description:   "Options controlling the HTTP request and its parts.",
		Flags:         make([]UsageFlag, 0),
		Hidden:        false,
		ExpectedFlags: []string{"cc", "ck", "H", "X", "b", "d", "r", "u", "opaque", "raw", "recursion", "recursion-depth", "recursion-strategy", "recursion-status", "replay-proxy", "timeout", "ignore-body", "x", "sni", "http2", "dns"},
	}
	u_general := UsageSection{
		Name:          "GENERAL OPTIONS",
		Description:   "",
		Flags:         make([]UsageFlag, 0),
		Hidden:        false,
		ExpectedFlags: []string{"ac", "acc", "ack", "ach", "acs", "c", "db", "discover-backup", "collect-backups", "backup-level", "diff", "no-content-length", "method-as-raw-request", "config", "json", "maxtime", "maxtime-job", "noninteractive", "p", "rate", "scraperfile", "scrapers", "search", "s", "sa", "se", "sf", "sr", "smart-404", "t", "v", "V", "lfi", "autotune", "batch", "l", "random-agent", "auto-file", "resume"},
	}
	u_compat := UsageSection{
		Name:          "COMPATIBILITY OPTIONS",
		Description:   "Options to ensure compatibility with other pieces of software.",
		Flags:         make([]UsageFlag, 0),
		Hidden:        true,
		ExpectedFlags: []string{"compressed", "cookie", "data", "data-ascii", "data-binary", "i", "k", "req"},
	}
	u_matcher := UsageSection{
		Name:          "MATCHER OPTIONS",
		Description:   "Matchers for the response filtering.",
		Flags:         make([]UsageFlag, 0),
		Hidden:        false,
		ExpectedFlags: []string{"mmode", "mc", "ml", "mr", "ms", "mt", "mw"},
	}
	u_filter := UsageSection{
		Name:          "FILTER OPTIONS",
		Description:   "Filters for the response filtering.",
		Flags:         make([]UsageFlag, 0),
		Hidden:        false,
		ExpectedFlags: []string{"fmode", "fc", "fl", "fr", "fs", "ft", "fw", "min-lines", "max-lines", "min-length", "max-length", "min-words", "max-words"},
	}
	u_input := UsageSection{
		Name:          "INPUT OPTIONS",
		Description:   "Options for input data for fuzzing. Wordlists and input generators.",
		Flags:         make([]UsageFlag, 0),
		Hidden:        false,
		ExpectedFlags: []string{"D", "enc", "ic", "input-cmd", "input-num", "input-shell", "mode", "request", "request-proto", "request-keepalive", "e", "w", "auto", "spider", "range", "force-extensions", "overwrite-extensions", "ext-placeholder", "prefix", "suffix", "capitalize"},
	}
	u_output := UsageSection{
		Name:          "OUTPUT OPTIONS",
		Description:   "Options for output. Output file formats, file names and debug file locations.",
		Flags:         make([]UsageFlag, 0),
		Hidden:        false,
		ExpectedFlags: []string{"audit-log", "debug-log", "o", "ol", "of", "od", "or"},
	}
	sections := []UsageSection{u_http, u_general, u_compat, u_matcher, u_filter, u_input, u_output}

	// Populate the flag sections
	max_length := 0
	flag.VisitAll(func(f *flag.Flag) {
		found := false
		for i, section := range sections {
			if ffuf.StrInSlice(f.Name, section.ExpectedFlags) {
				sections[i].Flags = append(sections[i].Flags, UsageFlag{
					Name:        f.Name,
					Description: f.Usage,
					Default:     f.DefValue,
				})
				found = true
			}
		}
		if !found {
			fmt.Printf("DEBUG: Flag %s was found but not defined in help.go.\n", f.Name)
			os.Exit(1)
		}
		if len(f.Name) > max_length {
			max_length = len(f.Name)
		}
	})

	fmt.Printf("uff - v%s\n\n", ffuf.Version())

	// Print out the sections
	for _, section := range sections {
		section.PrintSection(max_length, false)
	}

	// Usage examples.
	fmt.Printf("EXAMPLE USAGE:\n")

	fmt.Printf("  Auto-Fuzz all parameters in a URL (WAF Evasion + Smart LFI enabled):\n")
	fmt.Printf("    uff -auto \"http://target.com/page?id=1&user=admin\" -w wordlist.txt -autotune -lfi\n\n")

	fmt.Printf("  Batch Scan from a list of targets (File or Stdin):\n")
	fmt.Printf("    uff -l targets.txt -w wordlist.txt -u FUZZ/admin\n")
	fmt.Printf("    cat targets.txt | uff -batch -w wordlist.txt -u FUZZ/admin\n\n")

	fmt.Printf("  Spider a site and fuzz discovered links:\n")
	fmt.Printf("    uff -u https://example.org -spider -w wordlist.txt\n\n")

	fmt.Printf("  Standard Fuzzing (Status 200 only):\n")
	fmt.Printf("    uff -w wordlist.txt -u https://example.org/FUZZ -mc 200\n\n")

	fmt.Printf("  Post JSON Fuzzing:\n")
	fmt.Printf("    uff -w entries.txt -u https://example.org/ -X POST -H \"Content-Type: application/json\" \\\n")
	fmt.Printf("      -d '{\"name\": \"FUZZ\", \"key\": \"val\"}'\n\n")

	fmt.Printf("  More information and examples: https://github.com/Serdar715/uffx\n\n")
}
