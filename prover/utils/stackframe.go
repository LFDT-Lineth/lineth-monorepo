package utils

import (
	"runtime"
)

// StackFrame represents a stack frame in a user friendly way
type StackFrame struct {
	Pkg  string
	Fn   string
	File string
}

// GetCallerStackFrame returns the stack frame of the callers of the
// current call except for this one. The first frame (if none are skipped)
// if the call to [GetCallerStackFrames] itself.
func GetCallerStackFrames(skip, numFrames int) []runtime.Frame {

	// Ask runtime.Callers for up to 10 PCs, including runtime.Callers itself.
	pc := make([]uintptr, numFrames)
	n := runtime.Callers(skip+1, pc)
	if n == 0 {
		// No PCs available. This can happen if the first argument to
		// runtime.Callers is large.
		//
		// Return now to avoid processing the zero Frame that would
		// otherwise be returned by frames.Next below.
		return []runtime.Frame{}
	}

	pc = pc[:n] // pass only valid pcs to runtime.CallersFrames
	frames := runtime.CallersFrames(pc)
	res := []runtime.Frame{}

	// Loop to get frames.
	// A fixed number of PCs can expand to an indefinite number of Frames.
	for i := 0; i < numFrames; i++ {
		frame, more := frames.Next()
		res = append(res, frame)

		// Check whether there are more frames to process after this one.
		if !more {
			break
		}
	}

	return res
}
