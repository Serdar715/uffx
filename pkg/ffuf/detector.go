package ffuf

import (
	"bytes"
	"fmt"
	"strings"
	"sync"
	"time"
)

type FalsePositiveDetector interface {
	// Calibrate performs any necessary initial requests to establish a baseline
	Calibrate(runner RunnerProvider, request *Request) error
	// IsFalsePositive checks if the response looks like a false positive (e.g. soft 404)
	IsFalsePositive(resp *Response) bool
}

// Smart404Detector uses Jaccard index similarity against a 404 baseline to filter responses
type Smart404Detector struct {
	Config           *Config
	CalibrationWords map[string]bool
	mutex            sync.Mutex
}

func NewSmart404Detector(conf *Config) *Smart404Detector {
	return &Smart404Detector{
		Config:           conf,
		CalibrationWords: make(map[string]bool),
	}
}

func (d *Smart404Detector) Calibrate(runner RunnerProvider, basereq *Request) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	// Create a random path for calibration
	randomPath := fmt.Sprintf("random-%d", time.Now().UnixNano())

	// Prepare calibration request
	// We clone the base request to avoid modifying the original
	calibReq := *basereq
	calibReq.Url = strings.Replace(calibReq.Url, "FUZZ", randomPath, 1)
	if !strings.Contains(calibReq.Url, randomPath) {
		// If no FUZZ keyword, append to end
		if !strings.HasSuffix(calibReq.Url, "/") {
			calibReq.Url += "/"
		}
		calibReq.Url += randomPath
	}

	// Execute request
	resp, err := runner.Execute(&calibReq)
	if err != nil {
		return err
	}

	// Tokenize response — use bytes.Fields to avoid string copy
	wordBytes := bytes.Fields(resp.Data)
	for _, w := range wordBytes {
		d.CalibrationWords[string(w)] = true
	}
	return nil
}

func (d *Smart404Detector) IsFalsePositive(resp *Response) bool {
	if len(d.CalibrationWords) == 0 {
		return false
	}

	distance := d.calculateDistance(resp)
	resp.Distance = distance

	// Threshold 0.15 means > 85% similarity
	if d.Config.Smart404 && distance < 0.15 {
		return true
	}
	return false
}

func (d *Smart404Detector) calculateDistance(resp *Response) float64 {
	wordBytes := bytes.Fields(resp.Data)
	respWords := make(map[string]bool, len(wordBytes))
	for _, w := range wordBytes {
		respWords[string(w)] = true
	}

	intersection := 0
	for w := range respWords {
		if d.CalibrationWords[w] {
			intersection++
		}
	}

	union := len(d.CalibrationWords) + len(respWords) - intersection
	if union == 0 {
		return 0.0
	}

	jaccardIndex := float64(intersection) / float64(union)
	return 1.0 - jaccardIndex
}

// NoopDetector does nothing (used when features are disabled)
type NoopDetector struct{}

func (d *NoopDetector) Calibrate(runner RunnerProvider, req *Request) error { return nil }
func (d *NoopDetector) IsFalsePositive(resp *Response) bool                 { return false }
