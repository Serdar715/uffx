package features

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
)

// BackupConfig holds the extensions for different levels
type BackupConfig struct {
	Basic      []string `json:"basic"`
	Common     []string `json:"common"`
	Aggressive []string `json:"aggressive"`
}

// LoadBackupExtensions reads the backup extensions from a JSON file.
// If the file does not exist or fails to parse, it returns an error.
func LoadBackupExtensions(path string) (*BackupConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config BackupConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// LoadLFISignatures reads LFI regex patterns from a JSON file.
func LoadLFISignatures(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var sigs []string
	if err := json.Unmarshal(data, &sigs); err != nil {
		return nil, err
	}

	return sigs, nil
}

// CompileSignatures compiles a list of regex strings into regex objects.
func CompileSignatures(patterns []string) ([]*regexp.Regexp, error) {
	var compiled []*regexp.Regexp
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return nil, err
		}
		compiled = append(compiled, re)
	}
	return compiled, nil
}

// GetDefaultSignaturePath returns the path for signature files.
// Search order: 1) CWD/resources/signatures  2) binary dir/resources/signatures  3) relative fallback
func GetDefaultSignaturePath(filename string) string {
	// 1. Check current working directory
	cwd, err := os.Getwd()
	if err == nil {
		localPath := filepath.Join(cwd, "resources", "signatures", filename)
		if _, err := os.Stat(localPath); err == nil {
			return localPath
		}
	}

	// 2. Check next to the executable binary
	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		binPath := filepath.Join(exeDir, "resources", "signatures", filename)
		if _, err := os.Stat(binPath); err == nil {
			return binPath
		}
	}

	// 3. Relative fallback
	return filepath.Join("resources", "signatures", filename)
}
