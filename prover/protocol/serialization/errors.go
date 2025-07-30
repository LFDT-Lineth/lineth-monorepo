package serialization

import "fmt"

// error is a package-level error structure wrapping a list of errors and a list
// of paths for each error to help identify the location of each errors.
type serdeError struct {
	err  []error
	path []string
}

func (e *serdeError) Error() string {
	res := ""
	for i := range e.err {
		res += "\n" + fmt.Sprintf("error=%v, path=%v", e.err[i], e.path[i])
	}
	return res
}

// newSerdeError creates a new error without any path
func newSerdeErrorf(s string, args ...any) *serdeError {
	return &serdeError{
		err:  []error{fmt.Errorf(s, args...)},
		path: []string{""},
	}
}

// WrapPath prepends the path of the error with the provided string
func (e *serdeError) wrapPath(s string) *serdeError {
	for i := range e.path {
		e.path[i] = s + e.path[i]
	}
	return e
}

// AppendError appends an error to the list of errors
func (e *serdeError) appendError(err *serdeError) {
	e.err = append(e.err, err.err...)
	e.path = append(e.path, err.path...)
}

// IsEmpty returns true if the list of errors is empty
func (e *serdeError) isEmpty() bool {
	return len(e.err) == 0
}
