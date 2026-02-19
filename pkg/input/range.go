package input

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

type RangeType int

const (
	RangeInt RangeType = iota
	RangeHex
	RangeDate
	RangeAlpha
)

type RangeInput struct {
	config   *ffuf.Config
	keyword  string
	value    string
	active   bool
	position int

	// State
	rType RangeType
	start int64
	end   int64
	step  int64

	// Date specific
	startDate time.Time
	endDate   time.Time

	// Formatting
	width  int    // zero padding width
	layout string // date layout
}

func NewRangeInput(keyword, value string, conf *ffuf.Config) (*RangeInput, error) {
	r := &RangeInput{
		config:  conf,
		keyword: keyword,
		value:   value,
		active:  true,
		layout:  "2006-01-02",
	}
	err := r.parse()
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (r *RangeInput) parse() error {
	// Date
	if strings.Contains(r.value, "..") {
		parts := strings.Split(r.value, "..")
		if len(parts) == 2 {
			s, err1 := time.Parse(r.layout, strings.TrimSpace(parts[0]))
			e, err2 := time.Parse(r.layout, strings.TrimSpace(parts[1]))
			if err1 == nil && err2 == nil {
				r.rType = RangeDate
				r.startDate = s
				r.endDate = e
				r.step = 1 // Days
				return nil
			}
		}
	}

	// Step parsing
	step := int64(1)
	valPart := r.value
	if strings.Contains(strings.ToLower(r.value), " step ") {
		parts := strings.Split(strings.ToLower(r.value), " step ")
		if len(parts) == 2 {
			valPart = strings.TrimSpace(parts[0])
			s, err := strconv.ParseInt(strings.TrimSpace(parts[1]), 10, 64)
			if err == nil && s > 0 {
				step = s
			}
		}
	}
	r.step = step

	parts := strings.Split(valPart, "-")
	if len(parts) != 2 {
		return fmt.Errorf("invalid range format: %s", r.value)
	}
	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	// Hex
	if strings.HasPrefix(startStr, "0x") && strings.HasPrefix(endStr, "0x") {
		s, err1 := strconv.ParseInt(startStr[2:], 16, 64)
		e, err2 := strconv.ParseInt(endStr[2:], 16, 64)
		if err1 == nil && err2 == nil {
			r.rType = RangeHex
			r.start = s
			r.end = e
			return nil
		}
	}

	// Int (padding)
	s, err1 := strconv.Atoi(startStr)
	e, err2 := strconv.Atoi(endStr)
	if err1 == nil && err2 == nil {
		r.rType = RangeInt
		r.start = int64(s)
		r.end = int64(e)
		if strings.HasPrefix(startStr, "0") && len(startStr) > 1 {
			r.width = len(startStr)
		}
		return nil
	}

	// Alpha
	if len(startStr) == 1 && len(endStr) == 1 {
		sr := []rune(startStr)[0]
		er := []rune(endStr)[0]
		if sr <= er {
			r.rType = RangeAlpha
			r.start = int64(sr)
			r.end = int64(er)
			return nil
		}
	}

	return fmt.Errorf("unsupported range format: %s", r.value)
}

func (r *RangeInput) Keyword() string     { return r.keyword }
func (r *RangeInput) Position() int       { return r.position }
func (r *RangeInput) SetPosition(pos int) { r.position = pos }
func (r *RangeInput) IncrementPosition()  { r.position++ }
func (r *RangeInput) ResetPosition()      { r.position = 0 }
func (r *RangeInput) Active() bool        { return r.active }
func (r *RangeInput) Enable()             { r.active = true }
func (r *RangeInput) Disable()            { r.active = false }

func (r *RangeInput) Total() int {
	switch r.rType {
	case RangeDate:
		// approximate diff in days + 1
		diff := r.endDate.Sub(r.startDate).Hours() / 24
		return int(diff) + 1
	default:
		// (end - start) / step + 1
		count := (r.end - r.start) / r.step
		return int(count) + 1
	}
}

func (r *RangeInput) Value() []byte {
	// Calculate value based on position
	// Val = Start + (Pos * Step)

	if r.position >= r.Total() {
		return []byte{}
	}

	offset := int64(r.position) * r.step

	switch r.rType {
	case RangeInt:
		val := r.start + offset
		if r.width > 0 {
			format := fmt.Sprintf("%%0%dd", r.width)
			return []byte(fmt.Sprintf(format, val))
		}
		return []byte(strconv.FormatInt(val, 10))

	case RangeHex:
		val := r.start + offset
		return []byte(fmt.Sprintf("0x%x", val))

	case RangeAlpha:
		// Alpha step is usually 1, but we support step N too
		val := r.start + offset
		return []byte(string(rune(val)))

	case RangeDate:
		// Date offset is in days
		// Ignore step for date? Or treat step as days?
		// Previous implementation iterated `d = d.AddDate(0, 0, 1)`.
		// Let's assume step=1 day for now as per `parse` logic above setting step=1
		d := r.startDate.AddDate(0, 0, r.position) // Position days
		return []byte(d.Format(r.layout))
	}

	return []byte{}
}

func (r *RangeInput) Next() bool {
	return r.position < r.Total()
}
