package profiling

import (
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
)

// TimeIt runs "f" and returns the runtime duration of the call to "f"
func TimeIt(f func()) time.Duration {
	t := time.Now()
	f()
	return time.Since(t)
}

// LogTime registers the current time and returns a closure logging the
// duration since [LogTimer] was called. The function will mute the
// log if the duration is less than 1ms to prevent flooding.
func LogTimer(msg string, args ...any) func() {
	msgString := fmt.Sprintf(msg, args...)
	timer := time.Now()
	return func() {

		duration := time.Since(timer)
		if duration < time.Millisecond {
			return
		}

		logrus.Infof("[logtimer] operation=%q duration=%v", msgString, duration)
	}
}
