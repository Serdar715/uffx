package filter

import (
	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

func (f *MatcherManager) Clone() ffuf.MatcherManager {
	f.Mutex.Lock()
	defer f.Mutex.Unlock()

	newM := &MatcherManager{
		IsCalibrated:     f.IsCalibrated,
		Matchers:         make(map[string]ffuf.FilterProvider),
		Filters:          make(map[string]ffuf.FilterProvider),
		PerDomainFilters: make(map[string]*PerDomainFilter),
	}

	for k, v := range f.Matchers {
		newM.Matchers[k] = v
	}

	for k, v := range f.Filters {
		newM.Filters[k] = v
	}

	for k, v := range f.PerDomainFilters {
		newPD := &PerDomainFilter{
			IsCalibrated: v.IsCalibrated,
			Filters:      make(map[string]ffuf.FilterProvider),
		}
		for fk, fv := range v.Filters {
			newPD.Filters[fk] = fv
		}
		newM.PerDomainFilters[k] = newPD
	}

	return newM
}
