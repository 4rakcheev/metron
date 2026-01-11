package winagent

import "time"

// Clock interface abstracts time operations for testing
type Clock interface {
	// Now returns the current time
	Now() time.Time
	// NewTicker creates a new ticker that will send on its channel every d duration
	NewTicker(d time.Duration) *time.Ticker
}

// RealClock implements Clock using the real system time
type RealClock struct{}

// Now returns the current time
func (RealClock) Now() time.Time {
	return time.Now()
}

// NewTicker creates a new time.Ticker
func (RealClock) NewTicker(d time.Duration) *time.Ticker {
	return time.NewTicker(d)
}

// MockClock implements Clock for testing
type MockClock struct {
	CurrentTime time.Time
	ticker      *time.Ticker
}

// Now returns the mocked current time
func (m *MockClock) Now() time.Time {
	return m.CurrentTime
}

// NewTicker creates a ticker (for testing, this may not tick automatically)
func (m *MockClock) NewTicker(d time.Duration) *time.Ticker {
	if m.ticker == nil {
		m.ticker = time.NewTicker(d)
	}
	return m.ticker
}

// Advance moves the mocked time forward by the given duration
func (m *MockClock) Advance(d time.Duration) {
	m.CurrentTime = m.CurrentTime.Add(d)
}

// Set sets the mocked current time to a specific value
func (m *MockClock) Set(t time.Time) {
	m.CurrentTime = t
}

// Ensure implementations satisfy the interface
var (
	_ Clock = RealClock{}
	_ Clock = (*MockClock)(nil)
)
