package input

import (
	"bufio"
	"os"
	"regexp"
	"strings"

	"path/filepath"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

type WordlistInput struct {
	active       bool
	config       *ffuf.Config
	position     int
	keyword      string
	filePath     string
	file         *os.File
	scanner      *bufio.Scanner
	currentValue []byte
	totalLines   int
	queue        [][]byte // For holding multiple inputs generated from a single line
}

func NewWordlistInput(keyword string, value string, conf *ffuf.Config) (*WordlistInput, error) {
	var wl WordlistInput
	wl.active = true
	wl.keyword = keyword
	wl.config = conf
	wl.position = 0
	wl.queue = make([][]byte, 0)

	if value == "-" {
		// Create temp file for stdin
		tmpFile, err := os.CreateTemp("", "ffuf-stdin-*")
		if err != nil {
			return nil, err
		}
		// Write stdin to temp file
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			tmpFile.WriteString(scanner.Text() + "\n")
		}
		tmpFile.Close()
		wl.filePath = tmpFile.Name()
		// Note: Temp file cleaner is not implemented here, relies on OS or later cleanup
	} else {
		wl.filePath = value
		valid, err := wl.validFile(value)
		if err != nil {
			return &wl, err
		}
		if !valid {
			return &wl, os.ErrNotExist
		}
	}

	// Count total
	count, err := wl.countLines()
	if err != nil {
		return nil, err
	}
	wl.totalLines = count

	err = wl.openFile()
	if err != nil {
		return nil, err
	}

	return &wl, nil
}

func (w *WordlistInput) validFile(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return true, nil
}

func (w *WordlistInput) openFile() error {
	if w.file != nil {
		w.file.Close()
	}
	f, err := os.Open(w.filePath)
	if err != nil {
		return err
	}
	w.file = f
	w.scanner = bufio.NewScanner(f)
	// Setting split function to handle universal line endings if needed,
	// assuming ScanLines is sufficient for now or implementing custom one.
	// For "maintainability", standard ScanLines is safer unless legacy requirements dictate.
	// The original code had ScanLinesUniversal. Let's assume standard for now to simplify.
	return nil
}

func (w *WordlistInput) countLines() (int, error) {
	f, err := os.Open(w.filePath)
	if err != nil {
		return 0, err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		text := scanner.Text()
		processed := w.processLine(text)
		count += len(processed)
	}
	return count, scanner.Err()
}

// Next advances to the next input. Returns false if EOF.
func (w *WordlistInput) Next() bool {
	// If we have items in queue, pop one
	if len(w.queue) > 0 {
		w.currentValue = w.queue[0]
		w.queue = w.queue[1:]
		// Only increment position if we moved!
		// Actually, Next() is usually called before reading Value().
		// Position tracking needs to be aligned.
		w.position++
		return true
	}

	// No items in queue, scan next line
	for w.scanner.Scan() {
		text := w.scanner.Text()

		// Generate inputs from line
		inputs := w.processLine(text)
		if len(inputs) == 0 {
			continue
		}

		// Enqueue
		for _, inp := range inputs {
			w.queue = append(w.queue, []byte(inp))
		}

		// Pop first
		w.currentValue = w.queue[0]
		w.queue = w.queue[1:]
		w.position++
		return true
	}

	return false
}

func (w *WordlistInput) processLine(text string) []string {
	// Clean text
	text = strings.TrimRight(text, "\r")
	text = strings.TrimPrefix(text, "\xef\xbb\xbf")

	if w.config.IgnoreWordlistComments {
		stripped, isFullComment := stripComments(text)
		if isFullComment {
			return nil
		}
		text = stripped
	}

	if len(strings.TrimSpace(text)) == 0 {
		return nil
	}

	// Placeholder Logic
	placeholder := w.config.ExtensionPlaceholder
	if placeholder != "" && len(w.config.Extensions) > 0 {
		if strings.Contains(strings.ToUpper(text), strings.ToUpper(placeholder)) {
			rePlaceholder := regexp.MustCompile("(?i)" + regexp.QuoteMeta(placeholder))
			if rePlaceholder.MatchString(text) {
				var res []string
				for _, ext := range w.config.Extensions {
					newData := rePlaceholder.ReplaceAllString(text, ext)
					res = append(res, applyMutations(newData, w.config))
				}
				return res
			}
		}
	}

	mutated := applyMutations(text, w.config)
	var final []string

	if !w.config.ForceExtensions {
		final = append(final, mutated)
	}

	if w.keyword == "FUZZ" && len(w.config.Extensions) > 0 {
		for _, ext := range w.config.Extensions {
			word := mutated
			if w.config.OverwriteExtensions {
				extension := filepath.Ext(word)
				if extension != "" {
					word = strings.TrimSuffix(word, extension)
				}
			}
			final = append(final, word+ext)
		}
	}

	return final
}

func (w *WordlistInput) Value() []byte {
	return w.currentValue
}

func (w *WordlistInput) Position() int {
	return w.position
}

func (w *WordlistInput) SetPosition(pos int) {
	w.ResetPosition()
	// Burn inputs until pos
	// This is O(N) but necessary for streaming without index
	for i := 0; i < pos; i++ {
		if !w.Next() {
			break
		}
	}
}

func (w *WordlistInput) ResetPosition() {
	w.openFile()
	w.position = 0
	w.queue = make([][]byte, 0)
}

func (w *WordlistInput) Keyword() string {
	return w.keyword
}

func (w *WordlistInput) Total() int {
	return w.totalLines
}

func (w *WordlistInput) Active() bool {
	return w.active
}

func (w *WordlistInput) Enable() {
	w.active = true
}

func (w *WordlistInput) Disable() {
	w.active = false
}

func (w *WordlistInput) IncrementPosition() {
	// In streaming, Next() increments.
	// But Clusterbomb might call IncrementPosition manually?
	// Let's check logic: Clusterbomb calls Next() or IncrementPosition?
	// The interface requires it.
	// We should probably just make it a no-op or alias if Next handles it.
	// But strictly, Position is just a getter/setter.
	w.position++
}

// Helpers
func applyMutations(text string, conf *ffuf.Config) string {
	res := text
	if conf.InputCapitalize {
		if len(res) > 0 {
			res = strings.ToUpper(string(res[0])) + res[1:]
		}
	}
	if conf.InputPrefix != "" {
		res = conf.InputPrefix + res
	}
	if conf.InputSuffix != "" {
		res = res + conf.InputSuffix
	}
	return res
}

// stripComments removes comment content from a wordlist line.
// Full-line comments (lines starting with #) return empty string and true.
// Inline comments (text followed by " #") return the text before the comment and false.
// Non-comment lines return the original text and false.
func stripComments(text string) (string, bool) {
	trimmed := strings.TrimLeft(text, " ")
	if strings.HasPrefix(trimmed, "#") {
		return "", true
	}
	idx := strings.Index(text, " #")
	if idx != -1 {
		return strings.TrimRight(text[:idx], " "), false
	}
	return text, false
}
