package utils

import "fmt"

func Panic(msg string, args ...any) {
	panic(fmt.Sprintf(msg, args...))
}

func Require(cond bool, msg string, args ...any) {
	if !cond {
		Panic(msg, args...)
	}
}

// RecoverPanic runs fn and recover/return an error if the function
// call panicked.
func RecoverPanic(fn func()) (err error) {

	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panicked with message: %v", p)
		}
	}()

	fn()

	return err
}
