package smartvectors

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCircularIntervalConstructors(t *testing.T) {

	t.Run("for a normal interval", func(t *testing.T) {
		i := IvalWithStartLen(2, 5, 10)
		assert.Equal(t, 2, i.Start(), "Start")
		assert.Equal(t, 7, i.Stop(), "Stop")
		assert.Equal(t, 5, i.IntervalLen, "interval length")
		assert.False(t, i.DoesWrapAround(), "wrap around")
		assert.False(t, i.IsFullCircle(), "full circle")

		assert.True(t, i.DoesInclude(5), "in the middle of the interval")
		assert.True(t, i.DoesInclude(2), "it should be closed on the left")
		assert.False(t, i.DoesInclude(7), "it should be open on the right")

		assert.False(t, i.DoesInclude(0), "point on the left")
		assert.False(t, i.DoesInclude(8), "point on the right")

		assert.Equal(t, IvalWithStartStop(2, 7, 10), i)
	})

	t.Run("for a wrapped around vector", func(t *testing.T) {

		i := IvalWithStartLen(7, 5, 10)

		assert.Equal(t, 7, i.Start(), "Start")
		assert.Equal(t, 2, i.Stop(), "Stop")
		assert.Equal(t, 5, i.IntervalLen, "interval length")
		assert.True(t, i.DoesWrapAround(), "wrap around")
		assert.False(t, i.IsFullCircle(), "full circle")

		assert.False(t, i.DoesInclude(5), "in the middle of the interval")
		assert.True(t, i.DoesInclude(7), "it should be closed on the left")
		assert.False(t, i.DoesInclude(2), "it should be open on the right")

		assert.True(t, i.DoesInclude(0), "point on the left")
		assert.True(t, i.DoesInclude(8), "point on the right")

		assert.Equal(t, IvalWithStartStop(7, 2, 10), i)
	})

	t.Run("for a full vector", func(t *testing.T) {
		i := IvalWithFullLen(10)
		assert.Equal(t, 0, i.Start(), "Start")
		assert.Equal(t, 0, i.Stop(), "Stop")
		assert.Equal(t, 10, i.IntervalLen, "interval length")
		assert.False(t, i.DoesWrapAround(), "wrap around")
		assert.True(t, i.IsFullCircle(), "full circle")

		assert.True(t, i.DoesInclude(5), "in the middle of the interval")
		assert.True(t, i.DoesInclude(7), "it should be closed on the left")
		assert.True(t, i.DoesInclude(2), "it should be open on the right")

		assert.True(t, i.DoesInclude(0), "point on the left")
		assert.True(t, i.DoesInclude(8), "point on the right")
	})
}

func TestDoesFullyContain(t *testing.T) {

	t.Run("for a normal vector", func(t *testing.T) {
		i := IvalWithStartStop(5, 10, 15)

		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 3, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 5, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 8, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 10, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 13, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 1, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(5, 8, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(5, 10, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(5, 13, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(5, 3, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(7, 8, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(7, 10, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 13, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 3, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 5, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 6, 15)))

		assert.False(t, i.DoesFullyContain(IvalWithStartStop(10, 13, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(10, 3, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(10, 5, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(10, 8, 15)))

		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 13, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 3, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 5, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 8, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 10, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 11, 15)))

		assert.False(t, i.DoesFullyContain(IvalWithFullLen(15)))
	})

	t.Run("for a wrap around", func(t *testing.T) {
		i := IvalWithStartStop(10, 5, 15)

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(2, 3, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(2, 5, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 8, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 10, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 13, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(2, 1, 15)))

		assert.False(t, i.DoesFullyContain(IvalWithStartStop(5, 8, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(5, 10, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(5, 13, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(5, 3, 15)))

		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 8, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 10, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 13, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 3, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 5, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(7, 6, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(10, 13, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(10, 3, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(10, 5, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(10, 8, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 13, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 3, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 5, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 8, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 10, 15)))
		assert.False(t, i.DoesFullyContain(IvalWithStartStop(12, 11, 15)))

		assert.False(t, i.DoesFullyContain(IvalWithFullLen(15)))

	})

	t.Run("for a wrap around", func(t *testing.T) {
		i := IvalWithFullLen(15)

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(2, 3, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(2, 5, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(2, 8, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(2, 10, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(2, 13, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(2, 1, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(5, 8, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(5, 10, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(5, 13, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(5, 3, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(7, 8, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(7, 10, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(7, 13, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(7, 3, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(7, 5, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(7, 6, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(10, 13, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(10, 3, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(10, 5, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(10, 8, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 13, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 3, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 5, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 8, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 10, 15)))
		assert.True(t, i.DoesFullyContain(IvalWithStartStop(12, 11, 15)))

		assert.True(t, i.DoesFullyContain(IvalWithFullLen(15)))

	})

}

func TestTryOverlap(t *testing.T) {

	var ok bool
	var res CircularInterval

	t.Run("for a normal vector", func(t *testing.T) {
		i := IvalWithStartStop(5, 10, 15)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 3, 15))
		assert.False(t, ok)
		assert.Equal(t, CircularInterval{}, res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(2, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(2, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(2, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(2, 13, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 1, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(2, 1, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 13, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 3, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 13, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 3, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 6, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 13, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(5, 3, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 13, 15))
		assert.False(t, ok)
		assert.Equal(t, CircularInterval{}, res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 3, 15))
		assert.False(t, ok)
		assert.Equal(t, CircularInterval{}, res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(12, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(12, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(12, 10, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 11, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(12, 11, 15), res)

		ok, res = i.TryOverlapWith(IvalWithFullLen(15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)
	})

	t.Run("for a wrap around", func(t *testing.T) {
		i := IvalWithStartStop(10, 5, 15)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 8, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 1, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 8, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 8, 15))
		assert.False(t, ok)
		assert.Equal(t, CircularInterval{}, res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(7, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(7, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(7, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(7, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 6, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(7, 6, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 8, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 5, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithStartStop(10, 8, 15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 11, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithFullLen(15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

	})

	t.Run("for a wrap around", func(t *testing.T) {
		i := IvalWithFullLen(15)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(2, 1, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(5, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(7, 6, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(10, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 13, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 3, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 5, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 8, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 10, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithStartStop(12, 11, 15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

		ok, res = i.TryOverlapWith(IvalWithFullLen(15))
		assert.True(t, ok)
		assert.Equal(t, IvalWithFullLen(15), res)

	})

}
