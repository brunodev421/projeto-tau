package resilience

import "fmt"

type HTTPStatusError struct {
	StatusCode int
	Body       string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected downstream status: %d", e.StatusCode)
}
