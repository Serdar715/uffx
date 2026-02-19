package ffuf

import (
	"sync"
	"time"
)

// QueueJob defines a job that is queued for execution (e.g. recursion)
type QueueJob struct {
	Url        string
	depth      int
	req        Request
	SingleShot bool
}

// JobStats holds all the statistics for a running job
type JobStats struct {
	Counter              int
	ErrorCounter         int
	SpuriousErrorCounter int
	Total                int
	Count403             int
	Count429             int
	mu                   sync.Mutex
	StartTime            time.Time
}

// IncError increments the error counter safely
func (s *JobStats) IncError() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorCounter++
	s.SpuriousErrorCounter++
}

// Inc403 increments the 403 response counter safely
func (s *JobStats) Inc403() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Count403++
}

// Inc429 increments the 429 response counter safely
func (s *JobStats) Inc429() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Count429++
}

// ResetSpuriousErrors resets the spurious error counter safely
func (s *JobStats) ResetSpuriousErrors() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SpuriousErrorCounter = 0
}

// IncCounter increments the request counter
func (s *JobStats) IncCounter() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Counter++
}

// GetCounter returns the current request count
func (s *JobStats) GetCounter() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Counter
}
