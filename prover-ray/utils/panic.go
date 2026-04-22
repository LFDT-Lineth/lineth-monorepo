package utils

import "fmt"

// Panic formats msg with args using fmt.Sprintf and panics with the resulting string.
func Panic(msg string, args ...any) {
	panic(fmt.Sprintf(msg, args...))
}
