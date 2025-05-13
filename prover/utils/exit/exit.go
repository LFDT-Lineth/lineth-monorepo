package exit

import (
	"os"
	"runtime/debug"
)

var (
	activateExitOnIssue bool
)

// ActivateExitOnIssue tells the program to actually exit when [OnLimitOverflow]
// or [OnSatisfiedConstraints] are called. This has to be manually called at the
// beginning of the program if we want the behavior to take place. This is
// to avoid having it running in the tests.
func ActivateExitOnIssue() {
	activateExitOnIssue = true
}

const (
	limitOverflowExitCode          = 77
	unsatisfiedConstraintsExitCode = 78
)

// This function will exit the program with the exit code [limitOverflowExitCode]
// but only if the activateExitOnIssue flag is set to true. Otherwise, it will
// just panic.
func OnLimitOverflow() {

	debug.PrintStack()

	if !activateExitOnIssue {
		panic("limit overflow")
	}
	os.Exit(limitOverflowExitCode)
}

func OnUnsatisfiedConstraints() {

	debug.PrintStack()

	if !activateExitOnIssue {
		panic("unsatisfied constraints")
	}
	os.Exit(unsatisfiedConstraintsExitCode)
}
