package utils

// GenSorter is a generic interface implementing [sort.Interface]
// but let the user provide the methods directly as closures.
// Without needing to implement a new custom type.
type GenSorter struct {
	LenFn  func() int
	SwapFn func(int, int)
	LessFn func(int, int) bool
}

func (s GenSorter) Len() int {
	return s.LenFn()
}

func (s GenSorter) Swap(i, j int) {
	s.SwapFn(i, j)
}

func (s GenSorter) Less(i, j int) bool {
	return s.LessFn(i, j)
}
