package limbs

// BitSize is a pseudo const-generics that specifies the size of a limb
type BitSize interface {
	S16 | S32 | S48 | S64 | S128 | S160 | S256
}

type (
	// S16 is an abstract representation of the [Size] 16
	S16 struct{}
	// S32 is an abstract representation of the [Size] 32
	S32 struct{}
	// S48 is an abstract representation of the [Size] 48
	S48 struct{}
	// S64 is an abstract representation of the [Size] 64
	S64 struct{}
	// S128 is an abstract representation of the [Size] 128
	S128 struct{}
	// S16 0is an abstract representation of the [Size] 160
	S160 struct{}
	// S256 is an abstract representation of the [Size] 256
	S256 struct{}
)

// uintSize returns the bit size of a Uint[S]
func uintSize[S BitSize]() int {
	switch any(S{}).(type) {
	case S16:
		return 16
	case S32:
		return 32
	case S48:
		return 48
	case S64:
		return 64
	case S128:
		return 128
	case S160:
		return 160
	case S256:
		return 256
	default:
		panic("unreachable")
	}
}

// bytesSize returns the size of a Bytes[S]
func bytesSize[S BitSize]() int {
	return uintSize[S]() / 8
}
