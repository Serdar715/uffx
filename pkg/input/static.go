package input

import (
	"github.com/sw33tLie/uff/v2/pkg/ffuf"
)

// SingleInputProvider provides a single empty input for cases where no wordlist is needed
type SingleInputProvider struct {
	done   bool
	active bool
}

func NewSingleInput() *SingleInputProvider {
	return &SingleInputProvider{done: false, active: true}
}

func (s *SingleInputProvider) Next() bool {
	if s.done {
		return false
	}
	s.done = true
	return true
}

func (s *SingleInputProvider) Value() []byte {
	return []byte("")
}

func (s *SingleInputProvider) Total() int {
	return 1
}

func (s *SingleInputProvider) Position() int {
	if s.done {
		return 1
	}
	return 0
}

func (s *SingleInputProvider) ResetPosition() {
	s.done = false
}

func (s *SingleInputProvider) Active() bool {
	return s.active
}

func (s *SingleInputProvider) Enable() {
	s.active = true
}

func (s *SingleInputProvider) Disable() {
	s.active = false
}

func (s *SingleInputProvider) Keyword() string {
	return "FUZZ"
}

func (s *SingleInputProvider) SetPosition(pos int) {
	if pos >= 1 {
		s.done = true
	}
}

func (s *SingleInputProvider) IncrementPosition() {
	s.done = true
}

// Verify interface compliance
var _ ffuf.InternalInputProvider = &SingleInputProvider{}
