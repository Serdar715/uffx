package ffuf

import (
	"path/filepath"

	"github.com/adrg/xdg"
)

var (
	//VERSION holds the current version number
	VERSION = "2.1.1"
	//VERSION_APPENDIX holds additional version definition
	VERSION_APPENDIX = "-uff-dev"
	CONFIGDIR        = filepath.Join(xdg.ConfigHome, "ffuf")
	HISTORYDIR       = filepath.Join(CONFIGDIR, "history")
	SCRAPERDIR       = filepath.Join(CONFIGDIR, "scraper")
	AUTOCALIBDIR     = filepath.Join(CONFIGDIR, "autocalibration")
	// Magic Numbers used in CheckStop
	MAX_ERROR_RATIO_403           = 0.95
	MAX_SPURIOUS_ERROR_MULTIPLIER = 2
	MAX_ERROR_RATIO_429           = 0.2
	MIN_SAMPLES_FOR_STOP_CHECK    = 50
	// Magic Numbers used in Runner
	MAX_REDIRECTS_FOLLOW = 10 // Assuming default or common sense value, original code might have used constants.MaxRedirects which is good.
	// Actually, original code used constants.MaxRedirects.
	// But simple.go loop 2 iteration needs constant.
	KEYWORD_REPLACEMENT_ITERATIONS = 2
	// DefaultRecursionLimit defines the semaphore size for parallel recursion
	DefaultRecursionLimit = 128
)
