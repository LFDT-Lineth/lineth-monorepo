package utils

// Map returns [f(x[0]), f(x[1]), ..., f(x[len(x)-1])]
// If x is empty, nil is returned.
func Map[X, Y any](f func(X) Y, x []X) []Y {
	if len(x) == 0 {
		return nil
	}
	y := make([]Y, len(x))
	for i, v := range x {
		y[i] = f(v)
	}
	return y
}
