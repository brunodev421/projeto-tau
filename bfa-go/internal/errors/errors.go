package errors

import (
	"errors"
	"net/http"
)

type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
}

func (e APIError) Error() string {
	return e.Message
}

func New(code, message string, statusCode int) APIError {
	return APIError{Code: code, Message: message, StatusCode: statusCode}
}

var (
	ErrBadRequest         = New("bad_request", "request is invalid", http.StatusBadRequest)
	ErrCustomerNotFound   = New("customer_not_found", "customer not found", http.StatusNotFound)
	ErrDependencyFailure  = New("dependency_failure", "one or more downstream services are unavailable", http.StatusServiceUnavailable)
	ErrUpstreamTimeout    = New("dependency_timeout", "a downstream service timed out", http.StatusGatewayTimeout)
	ErrInternalFailure    = New("internal_error", "internal server error", http.StatusInternalServerError)
	ErrResponseValidation = New("invalid_agent_response", "agent returned an invalid response", http.StatusBadGateway)
)

func StatusCode(err error) int {
	var apiErr APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode
	}
	return http.StatusInternalServerError
}

func Code(err error) string {
	var apiErr APIError
	if errors.As(err, &apiErr) {
		return apiErr.Code
	}
	return ErrInternalFailure.Code
}
