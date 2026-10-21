package consumer

import "time"

// SetRetryBackoff shortens Run's backoff for a test and returns a func that
// restores the production values.
func SetRetryBackoff(min, max time.Duration) (restore func()) {
	oldMin, oldMax := minRetryBackoff, maxRetryBackoff
	minRetryBackoff, maxRetryBackoff = min, max
	return func() { minRetryBackoff, maxRetryBackoff = oldMin, oldMax }
}

// IsPermanent exposes the permanent-error classification to tests.
var IsPermanent = isPermanent
