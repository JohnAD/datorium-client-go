package datorium

import (
	"encoding/json"
	"fmt"
)

// APIError is one application-level error entry from a DatoriumDB envelope.
type APIError struct {
	Code     string `json:"code"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
	Expected any    `json:"expected,omitempty"`
	Actual   any    `json:"actual,omitempty"`
}

// Result is a decoded DatoriumDB response envelope.
type Result struct {
	OK     bool
	Errors []APIError
	Raw    map[string]any
}

// DecodeResult parses a JSON envelope body.
func DecodeResult(body []byte) (Result, error) {
	var raw map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return Result{}, fmt.Errorf("decode envelope: %w", err)
	}
	res := Result{Raw: raw}
	if v, ok := raw["ok"].(bool); ok {
		res.OK = v
	}
	if errList, ok := raw["errors"].([]any); ok {
		for _, item := range errList {
			m, ok := item.(map[string]any)
			if !ok {
				continue
			}
			ae := APIError{
				Code:     asString(m["code"]),
				Path:     asString(m["path"]),
				Message:  asString(m["message"]),
				Expected: m["expected"],
				Actual:   m["actual"],
			}
			res.Errors = append(res.Errors, ae)
		}
	}
	return res, nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

// FirstErrorCode returns the first application error code, or "".
func (r Result) FirstErrorCode() string {
	if len(r.Errors) == 0 {
		return ""
	}
	return r.Errors[0].Code
}

// StringField returns a top-level string field from the envelope.
func (r Result) StringField(key string) string {
	return asString(r.Raw[key])
}

// MapField returns a top-level object field.
func (r Result) MapField(key string) map[string]any {
	m, _ := r.Raw[key].(map[string]any)
	return m
}
