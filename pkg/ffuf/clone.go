package ffuf

// Clone creates a deep copy of the Config struct
func (c *Config) Clone() *Config {
	newC := *c

	// Deep copy slices
	if c.AutoCalibrationStrategies != nil {
		newC.AutoCalibrationStrategies = make([]string, len(c.AutoCalibrationStrategies))
		copy(newC.AutoCalibrationStrategies, c.AutoCalibrationStrategies)
	}
	if c.AutoCalibrationStrings != nil {
		newC.AutoCalibrationStrings = make([]string, len(c.AutoCalibrationStrings))
		copy(newC.AutoCalibrationStrings, c.AutoCalibrationStrings)
	}
	if c.CommandKeywords != nil {
		newC.CommandKeywords = make([]string, len(c.CommandKeywords))
		copy(newC.CommandKeywords, c.CommandKeywords)
	}
	if c.Encoders != nil {
		newC.Encoders = make([]string, len(c.Encoders))
		copy(newC.Encoders, c.Encoders)
	}
	if c.Extensions != nil {
		newC.Extensions = make([]string, len(c.Extensions))
		copy(newC.Extensions, c.Extensions)
	}
	if c.InputProviders != nil {
		newC.InputProviders = make([]InputProviderConfig, len(c.InputProviders))
		copy(newC.InputProviders, c.InputProviders)
	}
	if c.RecursionStatus != nil {
		newC.RecursionStatus = make([]string, len(c.RecursionStatus))
		copy(newC.RecursionStatus, c.RecursionStatus)
	}
	if c.Wordlists != nil {
		newC.Wordlists = make([]string, len(c.Wordlists))
		copy(newC.Wordlists, c.Wordlists)
	}

	// Deep copy maps
	if c.Headers != nil {
		newC.Headers = make(map[string]string)
		for k, v := range c.Headers {
			newC.Headers[k] = v
		}
	}

	// Note: Context and Cancel are shallow copied, which is intentional for context propagation
	if c.MatcherManager != nil {
		newC.MatcherManager = c.MatcherManager.Clone()
	}
	return &newC
}
