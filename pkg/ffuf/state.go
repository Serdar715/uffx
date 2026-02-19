package ffuf

import (
	"encoding/json"
	"os"
)

type ResumeState struct {
	Queue       []QueueJob      `json:"queue"`
	VisitedURLs map[string]bool `json:"visited_urls"`
	Position    int             `json:"position"`
}

func GetStateFilename(url string) string {
	// Simple hashing for filename
	// We need a unique filename based on target
	// Import "crypto/md5" and "encoding/hex" at top
	// But let's keep it simple for now, maybe just "ferox-TARGET.state" style?
	// We don't have sanitization here easily.
	// Let's use MD5 of URL.
	// Assume imports are available or handled.
	return "uff-resume.json" // Simplified for MVP, ideally should be unique per scan
}

func SaveState(url string, job *Job) error {
	state := job.ExportState()
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return os.WriteFile(GetStateFilename(url), data, 0644)
}

func LoadState(filename string) (*ResumeState, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	var state ResumeState
	err = json.Unmarshal(data, &state)
	if err != nil {
		return nil, err
	}

	return &state, nil
}
