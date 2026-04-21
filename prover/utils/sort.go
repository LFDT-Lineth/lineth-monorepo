package utils

// GenSorter is a generic interface implementing [sort.Interface]
// but let the user provide the methods directly as closures.
// Without needing to implement a new custom type.
type GenSorter struct {
	LenFn  func() int
	SwapFn func(int, int)
	LessFn func(int, int) bool
}
