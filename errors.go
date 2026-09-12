package qrocodile

import "fmt"

// APIError is returned by the [Client] methods when the API answers with an error response
// (non-2xx). The TypeScript client (@qrocodile/api) calls the equivalent type QrApiError; this
// one drops the redundant "Qr" since it is already package-qualified as qrocodile.APIError.
//
// A genuine network failure (offline, DNS, aborted, timeout) is returned as the underlying
// error instead of an *APIError — it never reached the API, so there is no status or code to
// attach. Check for this type with errors.As when you need to branch on it specifically.
type APIError struct {
	// Status is the HTTP status of the error response.
	Status int
	// Code is the machine-readable failure code from the API's {error:{code,message}} body.
	// Stable, and the field to branch on — Message is written for a human and may be reworded.
	Code ErrorCode
	// Message is what went wrong, in English, for a human to read. Do not match on it.
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("qrocodile: %s (code %s, HTTP %d)", e.Message, e.Code, e.Status)
}
