package datorium

import (
	"errors"
	"fmt"
	"strings"
)

// Common stable application error codes from DatoriumDB.
const (
	CodeWrongMachine     = "wrongMachine"
	CodeVersionMismatch  = "versionMismatch"
	CodeDocumentNotFound = "documentNotFound"
	CodeDocumentExists   = "documentExists"
	CodeUnauthenticated  = "unauthenticated"
	CodeInvalidToken     = "invalidToken"
	CodeTokenExpired     = "tokenExpired"
	CodeDocumentStale    = "documentStale"
	CodeReadMemberStale  = "readMemberStale"
	CodeSearchNotFound   = "searchNotFound"
)

// AppError is an application-level failure (HTTP often still 200).
type AppError struct {
	Code          string
	Message       string
	Errors        []APIError
	Result        Result
	ShardSlot     string
	CorrectServer string
	BaseURL       string
	ConfigVersion int
	Collection    string
	ID            string
	Command       string
}

func (e *AppError) Error() string {
	if e == nil {
		return "datorium: nil error"
	}
	if e.Message != "" {
		return fmt.Sprintf("datorium: %s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("datorium: %s", e.Code)
}

// IsAppCode reports whether err is an AppError with the given code.
func IsAppCode(err error, code string) bool {
	var ae *AppError
	if !errors.As(err, &ae) {
		return false
	}
	return ae.Code == code
}

// TransportError wraps non-application HTTP/transport failures.
type TransportError struct {
	StatusCode int
	Body       string
	Err        error
}

func (e *TransportError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("datorium transport: %v", e.Err)
	}
	return fmt.Sprintf("datorium transport: HTTP %d: %s", e.StatusCode, truncate(e.Body, 200))
}

func (e *TransportError) Unwrap() error { return e.Err }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func appErrorFromResult(res Result) *AppError {
	ae := &AppError{
		Result:        res,
		Errors:        res.Errors,
		Code:          res.FirstErrorCode(),
		ShardSlot:     res.StringField("shardSlot"),
		CorrectServer: res.StringField("correctServer"),
		BaseURL:       res.StringField("baseURL"),
		Collection:    res.StringField("collection"),
		ID:            res.StringField("id"),
		Command:       res.StringField("command"),
	}
	if len(res.Errors) > 0 {
		ae.Message = res.Errors[0].Message
	}
	if v, ok := res.Raw["configVersion"].(float64); ok {
		ae.ConfigVersion = int(v)
	}
	if ae.Code == "" {
		ae.Code = "unknown"
		ae.Message = "ok:false without error codes"
	}
	return ae
}

// JoinErrors joins multiple errors for reporting (Go 1.20+).
func JoinErrors(errs ...error) error {
	var nonNil []error
	for _, e := range errs {
		if e != nil {
			nonNil = append(nonNil, e)
		}
	}
	if len(nonNil) == 0 {
		return nil
	}
	if len(nonNil) == 1 {
		return nonNil[0]
	}
	msgs := make([]string, len(nonNil))
	for i, e := range nonNil {
		msgs[i] = e.Error()
	}
	return errors.New(strings.Join(msgs, "; "))
}
