package utils

import (
	"errors"
	"strings"

	"golang.org/x/exp/slices"
)

func WrapErrsAlphabetically(errs []error) error {
	// Sort the string to make the error analysis simpler
	slices.SortStableFunc(
		errs,
		func(a, b error) int {
			switch {
			case a.Error() < b.Error():
				return -1
			case a.Error() == b.Error():
				return 0
			case a.Error() > b.Error():
				return 1
			default:
				panic("unexpected")
			}
		},
	)

	// Then build a the wrapped error message
	builder := strings.Builder{}
	builder.WriteString("Several errors were encountered:\n")
	for _, err := range errs {
		builder.WriteString("\t * ")
		builder.WriteString(err.Error())
		builder.WriteString("\n")
	}

	return errors.New(builder.String())
}
