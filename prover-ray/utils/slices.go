package utils

// Map returns [f(x[0]), f(x[1]), ..., f(x[len(x)-1])]
func Map[X, Y any](f func(X) Y, x []X) []Y {
	y := make([]Y, len(x))
	for i, v := range x {
		y[i] = f(v)
	}
	return y
}
