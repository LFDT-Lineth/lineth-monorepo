package collection

// Fifo is a wrapper around a slice that implements a first-in first-out queue
// that is not threadsafe. Unlike channels, the fifo does not have a maximal
// buffer size that will block insertions.
type Fifo[T any] struct {
	Inner *[]T
}

func NewFifo[T any]() *Fifo[T] {
	return &Fifo[T]{Inner: &[]T{}}
}

func (f *Fifo[T]) Push(v T) {
	*f.Inner = append(*f.Inner, v)
}

func (f *Fifo[T]) Pop() T {
	v := (*f.Inner)[0]
	*f.Inner = (*f.Inner)[1:]
	return v
}

func (f *Fifo[T]) TryPop() (T, bool) {
	if len(*f.Inner) == 0 {
		var v T
		return v, false
	}
	v := (*f.Inner)[0]
	*f.Inner = (*f.Inner)[1:]
	return v, true
}

func (f *Fifo[T]) Len() int {
	return len(*f.Inner)
}

func (f *Fifo[T]) IsEmpty() bool {
	return f.Len() == 0
}
