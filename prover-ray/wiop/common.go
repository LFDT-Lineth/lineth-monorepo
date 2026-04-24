package wiop

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover-ray/maths/koalabear/field"
)

// Annotations is a string-keyed map of arbitrary values. Both compiler passes
// and user code may attach metadata to any annotatable object by setting
// entries in its Annotations map. The label string conventionally uses
// lower-case, dot-separated namespacing (e.g. "compiler.committed").
type Annotations map[string]any

// Visibility represents the visibility scope of an object (column or value)
// within the protocol transcript. It controls which queries may reference the
// object and whether the verifier can observe it directly.
type Visibility int

const (
	// VisibilityInternal marks an object as purely prover-internal. Internal
	// objects may not appear in active queries (i.e. queries that have not yet
	// been compiled away) and are invisible to the verifier.
	VisibilityInternal Visibility = iota
	// VisibilityOracle marks an object as usable in active queries but not
	// directly sent to the verifier. For columns, this implicitly signals an
	// intent to commit to the column. SingleValues assertedly cannot carry
	// this visibility.
	VisibilityOracle
	// VisibilityPublic marks an object as both queryable and verifier-visible.
	// Public objects are part of the protocol transcript and may be read by
	// verifier execution steps.
	VisibilityPublic
)

// String implements [fmt.Stringer].
func (v Visibility) String() string {
	switch v {
	case VisibilityInternal:
		return "Internal"
	case VisibilityOracle:
		return "Oracle"
	case VisibilityPublic:
		return "Public"
	default:
		return fmt.Sprintf("Visibility(%d)", int(v))
	}
}

// PaddingDirection specifies how a column's assignment vector is padded to
// match the declared module size when the provided data is shorter than
// expected.
type PaddingDirection int

const (
	// PaddingDirectionNone indicates the column cannot be padded; its
	// assignment must already have exactly the module size, which must be
	// non-zero at assignment time.
	PaddingDirectionNone PaddingDirection = iota
	// PaddingDirectionLeft pads the assignment by prepending a constant value
	// on the left until the module size is reached.
	PaddingDirectionLeft
	// PaddingDirectionRight pads the assignment by appending a constant value
	// on the right until the module size is reached.
	PaddingDirectionRight
)

// String implements [fmt.Stringer].
func (pd PaddingDirection) String() string {
	switch pd {
	case PaddingDirectionNone:
		return "None"
	case PaddingDirectionLeft:
		return "Left"
	case PaddingDirectionRight:
		return "Right"
	default:
		return fmt.Sprintf("PaddingDirection(%d)", int(pd))
	}
}

// ConcreteVector is the evaluated assignment of a [VectorPromise]. Plain holds
// the field elements in their natural order; Padding is the constant value
// prepended or appended when the assignment is shorter than the module size
// (see [PaddingDirection]).
//
// promise back-references the symbolic [VectorPromise] this vector fulfils so
// that consumers can recover size and padding semantics without carrying them
// separately.
type ConcreteVector struct {
	// Plain holds the field elements of the evaluated vector.
	Plain []field.Vec
	// Padding is the constant fill value used when the assignment is padded.
	Padding field.Element
	// promise is the symbolic vector-promise this concrete value fulfils.
	// Set once at construction, never mutated.
	promise VectorPromise
}

// ElementAt returns the field element at logical row pos within the concrete
// vector, accounting for the owning module's padding direction.
//
// The three cases are:
//   - [PaddingDirectionNone]: Plain[0] is the full-sized vector; pos indexes it directly.
//   - [PaddingDirectionLeft]: the full column is [Padding × gap] + Plain[0];
//     rows 0..gap-1 are the padding value, the rest index Plain[0].
//   - [PaddingDirectionRight]: the full column is Plain[0] + [Padding × gap];
//     rows 0..plainLen-1 index Plain[0], the rest are the padding value.
//
// Panics if pos is out of bounds for the module size, or if m is unsized.
// For dynamic modules use [ConcreteVector.ElementAtN] with the size obtained
// from [Module.RuntimeSize].
func (cv *ConcreteVector) ElementAt(m *Module, pos int) field.Gen {
	return cv.ElementAtN(m.Padding, m.Size(), pos)
}

// ElementAtN is like [ConcreteVector.ElementAt] but takes the domain size and
// padding direction as explicit arguments. Use this when the size has already
// been resolved (e.g. via [Module.RuntimeSize]) so that dynamic modules are
// handled correctly without re-deriving the size from the module.
func (cv *ConcreteVector) ElementAtN(padding PaddingDirection, n, pos int) field.Gen {
	if pos < 0 || pos >= n {
		panic(fmt.Sprintf("wiop: ConcreteVector.ElementAtN: pos %d out of bounds [0, %d)", pos, n))
	}

	vec := cv.Plain[0]
	plainLen := vec.Len()

	idx, inPadding := pos, false
	switch padding {
	case PaddingDirectionNone:
		// plain is full-sized; direct index.
	case PaddingDirectionLeft:
		gap := n - plainLen
		if pos < gap {
			inPadding = true
		} else {
			idx = pos - gap
		}
	case PaddingDirectionRight:
		if pos >= plainLen {
			inPadding = true
		}
	}

	if inPadding {
		return field.ElemFromBase(cv.Padding)
	}
	if vec.IsBase() {
		return field.ElemFromBase(vec.AsBase()[idx])
	}
	return field.ElemFromExt(vec.AsExt()[idx])
}

// ConcreteField is the evaluated value of a [FieldPromise]. Value holds the
// single field element produced by the evaluation.
//
// promise back-references the symbolic [FieldPromise] this value fulfils so
// that consumers can recover metadata (e.g. extension-field flag) without
// carrying it separately.
type ConcreteField struct {
	// Value is the evaluated field element.
	Value field.Gen
	// promise is the symbolic field-promise this concrete value fulfils.
	// Set once at construction, never mutated.
	promise FieldPromise
}
