//go:build trace

package serde

import (
	"fmt"
	"reflect"
	"strings"
	"sync/atomic"
)

var debugDepth int32

func traceEnter(op string, v reflect.Value, extra ...any) {
	d := atomic.AddInt32(&debugDepth, 1)
	pad := strings.Repeat("|  ", int(d-1))

	typeStr := "nil"
	if v.IsValid() {
		typeStr = v.Type().String()
	}

	ptrInfo := ""
	if v.IsValid() && v.Kind() == reflect.Ptr {
		ptrInfo = fmt.Sprintf(" [Ptr: %p]", v.Interface())
	}

	fmt.Printf("%s-> %s: Type=%s%s %v\n", pad, op, typeStr, ptrInfo, extra)
}

func traceExit(op string, err error, extra ...any) {
	d := atomic.LoadInt32(&debugDepth)
	pad := strings.Repeat("|  ", int(d-1))

	status := "OK"
	if err != nil {
		status = fmt.Sprintf("ERR: %v", err)
	}

	fmt.Printf("%s<- %s [%s] %v\n", pad, op, status, extra)
	atomic.AddInt32(&debugDepth, -1)
}

func traceLog(msg string, args ...any) {
	d := atomic.LoadInt32(&debugDepth)
	pad := strings.Repeat("|  ", int(d))
	fmt.Printf("%s[LOG] "+msg+"\n", append([]any{pad}, args...)...)
}

func traceOffset(component string, offset int64) {
	d := atomic.LoadInt32(&debugDepth)
	pad := strings.Repeat("|  ", int(d))
	fmt.Printf("%s[@OFFSET %d] %s\n", pad, offset, component)
}
