//go:build !bufpooldebug

package bufpool

// poisonEnabled is off outside the bufpooldebug build: the calls to poison
// compile away.
const poisonEnabled = false

func poison[T any](_ []T) {}
