package datorium

import (
	"fmt"

	"github.com/JohnAD/ojson"
)

// APIError is one application-level error entry from a DatoriumDB envelope.
type APIError struct {
	Code     string
	Path     string
	Message  string
	Expected ojson.JSONValue
	Actual   ojson.JSONValue
}

// Result is a decoded DatoriumDB response envelope.
type Result struct {
	OK     bool
	Errors []APIError
	// Env is the full response object parsed with ojson (ordered).
	Env ojson.JSONValue
	// Body is the original response bytes.
	Body []byte
}

// DecodeResult parses a JSON envelope body with ojson.
func DecodeResult(body []byte) (Result, error) {
	env, err := ojson.ReadBytesNoSchema(body)
	if err != nil {
		return Result{}, fmt.Errorf("decode envelope: %w", err)
	}
	if !env.IsObject() {
		return Result{}, fmt.Errorf("decode envelope: expected object, got %s", env.Kind())
	}
	res := Result{
		Env:  env,
		Body: append([]byte(nil), body...),
	}
	if ok := env.Get("ok"); ok.IsBoolean() {
		res.OK = ok.ToBoolOrDefault(false)
	}
	if errList := env.Get("errors"); errList.IsArray() {
		for _, item := range errList.Items() {
			if !item.IsObject() {
				continue
			}
			res.Errors = append(res.Errors, APIError{
				Code:     item.Get("code").ToStringOrEmpty(),
				Path:     item.Get("path").ToStringOrEmpty(),
				Message:  item.Get("message").ToStringOrEmpty(),
				Expected: item.Get("expected"),
				Actual:   item.Get("actual"),
			})
		}
	}
	return res, nil
}

// FirstErrorCode returns the first application error code, or "".
func (r Result) FirstErrorCode() string {
	if len(r.Errors) == 0 {
		return ""
	}
	return r.Errors[0].Code
}

// StringField returns a top-level string (or stringified number) field.
func (r Result) StringField(key string) string {
	return jsonValueAsString(r.Env.Get(key))
}

// ValueField returns a top-level field as an ojson value (Void if missing).
func (r Result) ValueField(key string) ojson.JSONValue {
	if r.Env.IsMissing() || !r.Env.IsObject() {
		return ojson.NewVoid()
	}
	return r.Env.Get(key)
}

// IntField returns a top-level integer field, or 0 if absent/invalid.
func (r Result) IntField(key string) int {
	v := r.Env.Get(key)
	if v.IsMissing() {
		return 0
	}
	n, err := v.ToIntTry()
	if err != nil {
		return 0
	}
	return n
}

func jsonValueAsString(v ojson.JSONValue) string {
	if v.IsMissing() || v.IsNull() {
		return ""
	}
	if v.IsString() {
		return v.ToStringOrEmpty()
	}
	if v.IsNumber() {
		return v.ToJSON()
	}
	if v.IsBoolean() {
		return fmt.Sprint(v.ToBoolOrDefault(false))
	}
	return v.ToJSON()
}
