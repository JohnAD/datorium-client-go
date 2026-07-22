package datorium

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/JohnAD/datorium-client-go/refs"
	"github.com/JohnAD/ojson"
)

const (
	formatDatoriumDirectRef = "DatoriumDirectRef"
	formatDatoriumCachedRef = "DatoriumCachedRef"
)

var (
	datoriumFormatsOnce sync.Once
	datoriumFormats     *ojson.StringFormatRegistry
)

// datoriumStringFormats returns the ojson string-format registry used when
// compiling establishment schemas. DatoriumDB schemas use DatoriumDirectRef /
// DatoriumCachedRef; ojson requires those formats to be registered at compile time.
func datoriumStringFormats() *ojson.StringFormatRegistry {
	datoriumFormatsOnce.Do(func() {
		reg := ojson.NewStringFormatRegistry()
		stringType := reflect.TypeOf("")
		_ = reg.Register(formatDatoriumDirectRef, ojson.StringFormatFunc(validateDatoriumDirectRef), stringType)
		_ = reg.Register(formatDatoriumCachedRef, ojson.StringFormatFunc(validateDatoriumCachedRef), stringType)
		datoriumFormats = reg
	})
	return datoriumFormats
}

func validateDatoriumDirectRef(value string) error {
	r, ok, err := refs.Parse(value)
	if err != nil {
		return err
	}
	if !ok || r.Kind != refs.Direct {
		return fmt.Errorf("expected %s (@__Collection__id)", formatDatoriumDirectRef)
	}
	return nil
}

func validateDatoriumCachedRef(value string) error {
	r, ok, err := refs.Parse(value)
	if err != nil {
		return err
	}
	if !ok || r.Kind != refs.Cached {
		return fmt.Errorf("expected %s (@@__Collection__id)", formatDatoriumCachedRef)
	}
	return nil
}
