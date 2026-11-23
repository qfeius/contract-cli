package openplatform

import "fmt"

type requestNotSentError struct{ cause error }

func (e *requestNotSentError) Error() string {
	return fmt.Sprintf("business request not sent: %v", e.cause)
}
func (e *requestNotSentError) Unwrap() error { return e.cause }

// RequestWasNotSent accepts the direct Client.Do error, not arbitrary nested
// causes: a later hook/refresh can fail after an earlier HTTP attempt was sent.
func RequestWasNotSent(err error) bool {
	_, ok := err.(*requestNotSentError)
	return ok
}
