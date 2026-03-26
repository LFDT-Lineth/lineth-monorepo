//go:build !trace

package serde

import "reflect"

// These are empty "no-op" functions.
// In a release build, the compiler removes these calls entirely.

func traceEnter(op string, v reflect.Value, extra ...any) {}
func traceExit(op string, err error, extra ...any)        {}
func traceLog(msg string, args ...any)                    {}
func traceOffset(component string, offset int64)          {}
