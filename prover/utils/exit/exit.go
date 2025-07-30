package exit

import (
	"math"
	"os"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

// issueHandlingMode indicates how the backend should behave when receiving an
// issue. With the default mode, the notification functions will panic. The
// modes can be combined by union.
type issueHandlingMode uint64

const (
	// limitOverflowExitCode is the exit code used to tell the parent process
	// know that a limit has been overflown.
	limitOverflowExitCode = 77
	// unsatisfiedConstraintExitCode is the exit code used to tell the parent
	// process know that a constraint is not satisfied.
	unsatisfiedConstraintsExitCode = 78
)

const (
	ExitOnUnsatisfiedConstraint issueHandlingMode = 1 << iota
	ExitOnLimitOverflow
	ExitAlways = math.MaxUint64
)

// LimitOverflowReport collects information related to a limit that has been
// overflown. It is used as a panic message when [PanicOnIssue] is set.
type LimitOverflowReport struct {
	Limit         int
	RequestedSize int
	Err           error
}

// UnsatisfiedConstraintError is a wrapper around an error to recognize errors
// coming from unsatisfied constraints.
type UnsatisfiedConstraintError struct {
	error
}

var (
	currentIssueHandlingMode issueHandlingMode // default to [PanicOnIssue]
)

// SetIssueHandlingMode sets the issue handling mode to the user-provided mode.
// You can pass `ExitOnLimitOverflow|ExitOnUnsatisfiedConstraint` to signify
// that the system should exit on either a limit overflow or an unsatisfied
// constraint.
func SetIssueHandlingMode(mode issueHandlingMode) {
	currentIssueHandlingMode = mode
}

// GetIssueHandlingMode returns the current handling mode
func GetIssueHandlingMode() issueHandlingMode {
	return currentIssueHandlingMode
}

// This function will exit the program with the exit code [limitOverflowExitCode]
// but only if the activateExitOnIssue flag is set to true. Otherwise, it will
// just panic.
func OnLimitOverflow(limit, requestedSize int, err error) {

	logrus.Errorf("[LIMIT OVERFLOW] A limit overflow has been detected limit=%v requested=%v err=%v", limit, requestedSize, err.Error())

	if currentIssueHandlingMode&ExitOnLimitOverflow > 0 {
		// The print stack is really useful to help locating where the problem
		// originates from.
		debug.PrintStack()
		os.Exit(limitOverflowExitCode)
	}

	panic(LimitOverflowReport{
		Limit:         limit,
		RequestedSize: requestedSize,
		Err:           err,
	})
}

// OnUnsatisfiedConstraints reports that a constraint is not satisfied.
func OnUnsatisfiedConstraints(err error) {

	debug.PrintStack()

	if currentIssueHandlingMode&ExitOnUnsatisfiedConstraint > 0 {

		logrus.Errorf("[UNSATISFIED CONSTRAINTS] An unsatisfied constraint has been report. Going to exit. err=%v", err.Error())

		// The print stack is really useful to help locating where the problem
		// originates from.
		debug.PrintStack()
		os.Exit(unsatisfiedConstraintsExitCode)
	}

	panic(UnsatisfiedConstraintError{err})
}
