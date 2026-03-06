package parser

import (
	"errors"
	"fmt"
)

// ErrorKind 用于标识解析失败的错误类别。
type ErrorKind string

const (
	ErrorKindInvalidInput          ErrorKind = "invalid_input"
	ErrorKindUnsupportedScheme     ErrorKind = "unsupported_scheme"
	ErrorKindMissingField          ErrorKind = "missing_field"
	ErrorKindSourceFetcherMissing  ErrorKind = "source_fetcher_missing"
	ErrorKindSourceFetchFailed     ErrorKind = "source_fetch_failed"
	ErrorKindSourceContentEmpty    ErrorKind = "source_content_empty"
	ErrorKindSourceNoSupportedNode ErrorKind = "source_no_supported_node"
)

// ParseError 为统一错误模型，便于上层做观测和分类处理。
type ParseError struct {
	Kind    ErrorKind
	Input   string
	Message string
	Cause   error
}

func (e *ParseError) Error() string {
	if e == nil {
		return "<nil>"
	}

	base := fmt.Sprintf("解析失败(kind=%s)", e.Kind)
	if e.Input != "" {
		base = fmt.Sprintf("%s, input=%q", base, e.Input)
	}
	if e.Message != "" {
		base = fmt.Sprintf("%s: %s", base, e.Message)
	}
	if e.Cause != nil {
		base = fmt.Sprintf("%s: %v", base, e.Cause)
	}
	return base
}

func (e *ParseError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func newParseError(kind ErrorKind, input, message string, cause error) error {
	return &ParseError{
		Kind:    kind,
		Input:   input,
		Message: message,
		Cause:   cause,
	}
}

// IsErrorKind 用于在测试或业务逻辑中判断错误类别。
func IsErrorKind(err error, kind ErrorKind) bool {
	if err == nil {
		return false
	}

	var parseErr *ParseError
	if !errors.As(err, &parseErr) {
		return false
	}
	return parseErr.Kind == kind
}
