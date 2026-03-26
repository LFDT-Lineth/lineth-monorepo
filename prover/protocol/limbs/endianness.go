package limbs

import "slices"

// LittleEndian is an empty type that can be used as type parameter to specify
// the endianness of a limb.
type LittleEndian struct{}

// BigEndian is an empty type that can be used as type parameter to specify
// the endianness of a limb.
type BigEndian struct{}

// Endianness indicates whether a limb structure is in little or big endian.
type Endianness interface {
	BigEndian | LittleEndian
}

// isLittleEndian is a utility function returning true if its type parameters
// E is LittleEndian and false otherwise.
func isLittleEndian[E Endianness]() bool {
	_, ok := any(E{}).(LittleEndian)
	return ok
}

// isBigEndian is a utility function returning true if its type parameters
// E is BigEndian and false otherwise.
func isBigEndian[E Endianness]() bool {
	_, ok := any(E{}).(BigEndian)
	return ok
}

// ConvertSlice converts a e1-endian ordered slice into an e2-endian ordered
// slice. The input slice is deep-copied in all cases.
func ConvertSlice[E1, E2 Endianness, T any](s []T) []T {
	s = slices.Clone(s)
	if isLittleEndian[E1]() != isLittleEndian[E2]() {
		slices.Reverse(s)
	}
	return s
}
