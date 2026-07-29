package agent

import (
	"fmt"
	"time"
)

func requestProviderWithRetry(
	maxAttempts int,
	request func() (error, string),
	wait func(time.Duration),
	onRetry func(error, int, int),
) (string, error) {
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		requestErr, response := request()
		if requestErr == nil {
			return response, nil
		}
		if attempt == maxAttempts {
			return "", fmt.Errorf(
				"request provider failed after %d attempts: %w",
				maxAttempts,
				requestErr,
			)
		}
		onRetry(requestErr, attempt, maxAttempts)
		wait(10 * time.Second)
	}

	return "", fmt.Errorf("request provider failed: no attempts configured")
}
