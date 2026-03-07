package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// ErrorCode identifies the type of domain error.
type ErrorCode string

const (
	ErrCodeNotFound          ErrorCode = "NOT_FOUND"
	ErrCodeAlreadyExists     ErrorCode = "ALREADY_EXISTS"
	ErrCodeInvalidInput      ErrorCode = "INVALID_INPUT"
	ErrCodeUnauthorized      ErrorCode = "UNAUTHORIZED"
	ErrCodeForbidden         ErrorCode = "FORBIDDEN"
	ErrCodeInternalError     ErrorCode = "INTERNAL_ERROR"
	ErrCodeRepoTooLarge      ErrorCode = "REPO_TOO_LARGE"
	ErrCodeRepoPrivate       ErrorCode = "REPO_PRIVATE"
	ErrCodeRepoNotFound      ErrorCode = "REPO_NOT_FOUND"
	ErrCodeAnalysisInProgress ErrorCode = "ANALYSIS_IN_PROGRESS"
	ErrCodeAnalysisFailed    ErrorCode = "ANALYSIS_FAILED"
	ErrCodeGitHubRateLimit   ErrorCode = "GITHUB_RATE_LIMIT"
	ErrCodeParseError        ErrorCode = "PARSE_ERROR"
	ErrCodeGraphError        ErrorCode = "GRAPH_ERROR"
	ErrCodeCacheError        ErrorCode = "CACHE_ERROR"
	ErrCodeAIServiceError    ErrorCode = "AI_SERVICE_ERROR"
	ErrCodeTimeout           ErrorCode = "TIMEOUT"
)

// DomainError is the standard error type used throughout the application.
type DomainError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Detail  string    `json:"detail,omitempty"`
	Err     error     `json:"-"`
}

func (e *DomainError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *DomainError) Unwrap() error {
	return e.Err
}

// New creates a new DomainError.
func New(code ErrorCode, message string) *DomainError {
	return &DomainError{Code: code, Message: message}
}

// Newf creates a new DomainError with a formatted message.
func Newf(code ErrorCode, format string, args ...any) *DomainError {
	return &DomainError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap wraps an existing error with a domain error.
func Wrap(code ErrorCode, message string, err error) *DomainError {
	return &DomainError{Code: code, Message: message, Err: err, Detail: err.Error()}
}

// Wrapf wraps an existing error with a formatted domain error.
func Wrapf(code ErrorCode, err error, format string, args ...any) *DomainError {
	return &DomainError{
		Code:    code,
		Message: fmt.Sprintf(format, args...),
		Err:     err,
		Detail:  err.Error(),
	}
}

// Is checks if the error matches the given error code.
func Is(err error, code ErrorCode) bool {
	var de *DomainError
	if errors.As(err, &de) {
		return de.Code == code
	}
	return false
}

// HTTPStatus maps a domain error code to an HTTP status code.
func HTTPStatus(err error) int {
	var de *DomainError
	if !errors.As(err, &de) {
		return http.StatusInternalServerError
	}

	switch de.Code {
	case ErrCodeNotFound, ErrCodeRepoNotFound:
		return http.StatusNotFound
	case ErrCodeAlreadyExists:
		return http.StatusConflict
	case ErrCodeInvalidInput:
		return http.StatusBadRequest
	case ErrCodeUnauthorized:
		return http.StatusUnauthorized
	case ErrCodeForbidden, ErrCodeRepoPrivate:
		return http.StatusForbidden
	case ErrCodeRepoTooLarge:
		return http.StatusRequestEntityTooLarge
	case ErrCodeAnalysisInProgress:
		return http.StatusAccepted
	case ErrCodeGitHubRateLimit:
		return http.StatusTooManyRequests
	case ErrCodeTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// Predefined common errors.
var (
	ErrNotFound       = New(ErrCodeNotFound, "resource not found")
	ErrUnauthorized   = New(ErrCodeUnauthorized, "unauthorized")
	ErrInternalServer = New(ErrCodeInternalError, "internal server error")
)