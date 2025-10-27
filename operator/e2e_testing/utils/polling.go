package utils

import (
	"context"
	"fmt"
	"time"
)

// PollForCondition repeatedly evaluates a condition function at the specified interval
// until it returns true or the timeout is reached. Returns an error if the condition fails,
// returns an error, or the timeout expires.
func PollForCondition(ctx context.Context, timeout, interval time.Duration, condition func() (bool, error)) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check immediately first
	if satisfied, err := condition(); err != nil {
		return err
	} else if satisfied {
		return nil
	}

	for {
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("condition not met within timeout of %v", timeout)
		case <-ticker.C:
			if satisfied, err := condition(); err != nil {
				return err
			} else if satisfied {
				return nil
			}
		}
	}
}
