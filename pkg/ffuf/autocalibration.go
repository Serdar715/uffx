package ffuf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type AutocalibrationStrategy map[string][]string

func (j *Job) autoCalibrationStrings() map[string][]string {
	rand.Seed(time.Now().UnixNano())
	cInputs := make(map[string][]string)

	if len(j.Config.AutoCalibrationStrings) > 0 {
		cInputs["custom"] = append(cInputs["custom"], j.Config.AutoCalibrationStrings...)
		return cInputs

	}

	for _, strategy := range j.Config.AutoCalibrationStrategies {
		jsonStrategy, err := os.ReadFile(filepath.Join(AUTOCALIBDIR, strategy+".json"))
		if err != nil {
			j.Output.Warning(fmt.Sprintf("Skipping strategy \"%s\" because of error: %s\n", strategy, err))
			continue
		}

		tmpStrategy := AutocalibrationStrategy{}
		err = json.Unmarshal(jsonStrategy, &tmpStrategy)
		if err != nil {
			j.Output.Warning(fmt.Sprintf("Skipping strategy \"%s\" because of error: %s\n", strategy, err))
			continue
		}

		cInputs = mergeMaps(cInputs, tmpStrategy)
	}

	return cInputs
}

func setupDefaultAutocalibrationStrategies() error {
	const (
		RandomSuffixLenLong  = 16
		RandomSuffixLenShort = 8
	)

	basic_strategy := AutocalibrationStrategy{
		"basic_admin":  []string{"admin" + RandomString(RandomSuffixLenLong), "admin" + RandomString(RandomSuffixLenShort)},
		"htaccess":     []string{".htaccess" + RandomString(RandomSuffixLenLong), ".htaccess" + RandomString(RandomSuffixLenShort)},
		"basic_random": []string{RandomString(RandomSuffixLenLong), RandomString(RandomSuffixLenShort)},
		"sensitive_files": []string{
			".env" + RandomString(RandomSuffixLenShort),
			"web.config" + RandomString(RandomSuffixLenShort),
			"wp-config.php" + RandomString(RandomSuffixLenShort),
			"config" + RandomString(RandomSuffixLenShort) + ".json",
		},
	}
	basic_strategy_json, err := json.Marshal(basic_strategy)
	if err != nil {
		return err
	}

	advanced_strategy := AutocalibrationStrategy{
		"basic_admin":  []string{"admin" + RandomString(RandomSuffixLenLong), "admin" + RandomString(RandomSuffixLenShort)},
		"htaccess":     []string{".htaccess" + RandomString(RandomSuffixLenLong), ".htaccess" + RandomString(RandomSuffixLenShort)},
		"basic_random": []string{RandomString(RandomSuffixLenLong), RandomString(RandomSuffixLenShort)},
		"admin_dir":    []string{"admin" + RandomString(RandomSuffixLenLong) + "/", "admin" + RandomString(RandomSuffixLenShort) + "/"},
		"random_dir":   []string{RandomString(RandomSuffixLenLong) + "/", RandomString(RandomSuffixLenShort) + "/"},
		"sensitive_files": []string{
			".env" + RandomString(RandomSuffixLenShort),
			"web.config" + RandomString(RandomSuffixLenShort),
			"wp-config.php" + RandomString(RandomSuffixLenShort),
			"config" + RandomString(RandomSuffixLenShort) + ".json",
		},
	}
	advanced_strategy_json, err := json.Marshal(advanced_strategy)
	if err != nil {
		return err
	}

	basic_strategy_file := filepath.Join(AUTOCALIBDIR, "basic.json")
	// Always overwrite to ensure updates are applied
	err = os.WriteFile(basic_strategy_file, basic_strategy_json, 0640)
	if err != nil {
		return err
	}
	advanced_strategy_file := filepath.Join(AUTOCALIBDIR, "advanced.json")
	// Always overwrite to ensure updates are applied
	err = os.WriteFile(advanced_strategy_file, advanced_strategy_json, 0640)
	if err != nil {
		return err
	}

	return nil
}

func (j *Job) calibrationRequest(inputs map[string][]byte) (Response, error) {
	basereq := BaseRequest(j.Config)
	req, err := j.Runner.Prepare(inputs, &basereq)
	if err != nil {
		j.Output.Error(fmt.Sprintf("Encountered an error while preparing autocalibration request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return Response{}, err
	}
	resp, err := j.Runner.Execute(&req)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return Response{}, err
		}
		j.Output.Error(fmt.Sprintf("Encountered an error while executing autocalibration request: %s\n", err))
		j.incError()
		log.Printf("%s", err)
		return Response{}, err
	}
	// Only calibrate on responses that would be matched otherwise
	if j.isMatch(resp) {
		return resp, nil
	}
	return resp, fmt.Errorf("Response wouldn't be matched")
}

// createCalibrationInput creates a shallow copy of input map for calibration
// This prevents modifications during calibration from affecting the original fuzzing input
func createCalibrationInput(baseInput map[string][]byte) map[string][]byte {
	calibInput := make(map[string][]byte, len(baseInput))
	for k, v := range baseInput {
		calibInput[k] = v
	}
	return calibInput
}

// CalibrateForHost runs autocalibration for a specific host
func (j *Job) CalibrateForHost(host string, baseinput map[string][]byte) error {
	if j.Config.MatcherManager.CalibratedForDomain(host) {
		return nil
	}
	if baseinput[j.Config.AutoCalibrationKeyword] == nil {
		return fmt.Errorf("Autocalibration keyword \"%s\" not found in the request.", j.Config.AutoCalibrationKeyword)
	}
	cStrings := j.autoCalibrationStrings()
	input := createCalibrationInput(baseinput)
	for _, v := range cStrings {
		responses := make([]Response, 0)
		for _, cs := range v {
			input[j.Config.AutoCalibrationKeyword] = []byte(cs)
			resp, err := j.calibrationRequest(input)
			if err != nil {
				continue
			}
			responses = append(responses, resp)
			err = j.calibrateFilters(responses, true)
			if err != nil {
				j.Output.Error(fmt.Sprintf("%s", err))
			}
		}
	}
	j.Config.MatcherManager.SetCalibratedForHost(host, true)
	return nil
}

// CalibrateResponses returns slice of Responses for randomly generated filter autocalibration requests
func (j *Job) Calibrate(input map[string][]byte) error {
	if j.Config.MatcherManager.Calibrated() {
		return nil
	}
	cInputs := j.autoCalibrationStrings()

	// Create a protected copy to avoid modifying the original fuzzing input
	calibrationInput := createCalibrationInput(input)

	for _, v := range cInputs {
		responses := make([]Response, 0)
		for _, cs := range v {
			calibrationInput[j.Config.AutoCalibrationKeyword] = []byte(cs)
			resp, err := j.calibrationRequest(calibrationInput)
			if err != nil {
				continue
			}
			responses = append(responses, resp)
		}
		_ = j.calibrateFilters(responses, false)
	}
	j.Config.MatcherManager.SetCalibrated(true)
	return nil
}

// CalibrateIfNeeded runs a self-calibration task for filtering options (if needed) by requesting random resources and
//
//	configuring the filters accordingly
func (j *Job) CalibrateIfNeeded(host string, input map[string][]byte) error {
	j.calibMutex.Lock()
	defer j.calibMutex.Unlock()
	if !j.Config.AutoCalibration {
		return nil
	}
	if j.Config.AutoCalibrationPerHost {
		return j.CalibrateForHost(host, input)
	}
	return j.Calibrate(input)
}

func (j *Job) calibrateFilters(responses []Response, perHost bool) error {
	// Capture words for diffing from the first response
	// Capture words for diffing is now handled by the Smart404Detector if enabled.
	// We no longer populate j.CalibrationWords here.

	// Work down from the most specific common denominator
	if len(responses) > 0 {
		// Content length
		baselineSize := responses[0].ContentLength
		sizeMatch := true
		for _, r := range responses {
			if baselineSize != r.ContentLength {
				sizeMatch = false
			}
		}
		if sizeMatch {
			if perHost {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.FiltersForDomain(HostURLFromRequest(*responses[0].Request)) {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddPerDomainFilter(HostURLFromRequest(*responses[0].Request), "size", strconv.FormatInt(baselineSize, 10))
				return nil
			} else {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.GetFilters() {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddFilter("size", strconv.FormatInt(baselineSize, 10), false)
				return nil
			}
		}

		// Content words
		baselineWords := responses[0].ContentWords
		wordsMatch := true
		for _, r := range responses {
			if baselineWords != r.ContentWords {
				wordsMatch = false
			}
		}
		if wordsMatch {
			if perHost {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.FiltersForDomain(HostURLFromRequest(*responses[0].Request)) {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddPerDomainFilter(HostURLFromRequest(*responses[0].Request), "word", strconv.FormatInt(baselineWords, 10))
				return nil
			} else {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.GetFilters() {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddFilter("word", strconv.FormatInt(baselineWords, 10), false)
				return nil
			}
		}

		// Content lines
		baselineLines := responses[0].ContentLines
		linesMatch := true
		for _, r := range responses {
			if baselineLines != r.ContentLines {
				linesMatch = false
			}
		}
		if linesMatch {
			if perHost {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.FiltersForDomain(HostURLFromRequest(*responses[0].Request)) {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddPerDomainFilter(HostURLFromRequest(*responses[0].Request), "line", strconv.FormatInt(baselineLines, 10))
				return nil
			} else {
				// Check if already filtered
				for _, f := range j.Config.MatcherManager.GetFilters() {
					match, _ := f.Filter(&responses[0])
					if match {
						// Already filtered
						return nil
					}
				}
				_ = j.Config.MatcherManager.AddFilter("line", strconv.FormatInt(baselineLines, 10), false)
				return nil
			}
		}
	}
	return fmt.Errorf("No common filtering values found")
}
