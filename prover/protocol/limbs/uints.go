package limbs

import (
	"slices"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
)

type (
	Uint256Be  = Uint[S256, BigEndian]
	EthAddress = Uint[S160, BigEndian]
	Uint128Be  = Uint[S128, BigEndian]
	Uint64Be   = Uint[S64, BigEndian]
	Uint32Be   = Uint[S32, BigEndian]
	Uint16Be   = Uint[S16, BigEndian]
	Uint256Le  = Uint[S256, LittleEndian]
	Uint160Le  = Uint[S160, LittleEndian]
	Uint128Le  = Uint[S128, LittleEndian]
	Uint64Le   = Uint[S64, LittleEndian]
	Uint32Le   = Uint[S32, LittleEndian]
	Uint16Le   = Uint[S16, LittleEndian]
)

var (
	NewUint256Be  = NewUint[S256, BigEndian]
	NewEthAddress = NewUint[S160, BigEndian]
	NewUint128Be  = NewUint[S128, BigEndian]
	NewUint64Be   = NewUint[S64, BigEndian]
	NewUint32Be   = NewUint[S32, BigEndian]
	NewUint16Be   = NewUint[S16, BigEndian]
	NewUint256Le  = NewUint[S256, LittleEndian]
	NewEthAddreLe = NewUint[S160, LittleEndian]
	NewUint128Le  = NewUint[S128, LittleEndian]
	NewUint64Le   = NewUint[S64, LittleEndian]
	NewUint32Le   = NewUint[S32, LittleEndian]
	NewUint16Le   = NewUint[S16, LittleEndian]
)

// Uint[S, E] represents a register represented by a list of columns.
type Uint[S BitSize, E Endianness] struct {
	Limbs[E]
}

// NewUint[S, E] creates a new [Uints] registering all its components.
func NewUint[S BitSize, E Endianness](comp *wizard.CompiledIOP, name ifaces.ColID, size int, prags ...pragmas.PragmaPair) Uint[S, E] {
	numLimbs := utils.DivExact(uintSize[S](), limbBitWidth)
	limbs := NewLimbs[E](comp, name, numLimbs, size, prags...)
	return Uint[S, E]{limbs}
}

// AsDynSize returns the underlying limbs.
func (u Uint[S, E]) AsDynSize() Limbs[E] {
	return u.Limbs
}

// ToBigEndianUint converts (if needed) to a big-endian uint register.
func (u Uint[S, E]) ToBigEndianUint() Uint[S, BigEndian] {
	return Uint[S, BigEndian]{
		u.ToBigEndianLimbs(),
	}
}

// ToLittleEndianUint converts (if needed) to a little-endian uint register.
func (u Uint[S, E]) ToLittleEndianUint() Uint[S, LittleEndian] {
	return Uint[S, LittleEndian]{
		u.ToLittleEndianLimbs(),
	}
}

// FromSliceUnsafe creates a [Uint] object from a slice of bytes. The function
// trusts the user is providing limbs in the right direction.
func FromSliceUnsafe[S BitSize, E Endianness](name ifaces.ColID, cols []ifaces.Column) Uint[S, E] {
	numLimbs := utils.DivExact(uintSize[S](), limbBitWidth)
	if len(cols) != numLimbs {
		utils.Panic("number of columns must be equal to the number of limbs, got %v and %v", len(cols), numLimbs)
	}
	return Uint[S, E]{Limbs[E]{C: cols, Name: name}}
}

// AssertUint128 converts the slice into a [Uint128] object and panicks if the size
// is not the expected one.
func (limbs Limbs[E]) AssertUint128() Uint[S128, E] {
	if limbs.NumLimbs() != NumLimbsOf[S128]() {
		utils.Panic("number of columns must be equal to the number of limbs, got %v and %v", limbs.NumLimbs(), NumLimbsOf[S128]())
	}
	return Uint[S128, E]{limbs}
}

// AssertUint160 converts the slice into a Uint160 object and panicks if the size
// is not the expected one.
func (limbs Limbs[E]) AssertUint160() Uint[S160, E] {
	if limbs.NumLimbs() != NumLimbsOf[S160]() {
		utils.Panic("number of columns must be equal to the number of limbs, got %v and %v", limbs.NumLimbs(), NumLimbsOf[S160]())
	}
	return Uint[S160, E]{limbs}
}

// AssertUint256 converts the slice into a Uint256 object and panicks if the size
// is not the expected one.
func (limbs Limbs[E]) AssertUint256() Uint[S256, E] {
	if limbs.NumLimbs() != NumLimbsOf[S256]() {
		utils.Panic("number of columns must be equal to the number of limbs, got %v and %v", limbs.NumLimbs(), NumLimbsOf[S256]())
	}
	return Uint[S256, E]{limbs}
}

// ZeroExtendToSize extends the provided limbs to the provided size.
func (limbs Limbs[E]) ZeroExtendToSize(size int) Limbs[E] {

	if size < limbs.NumLimbs() {
		utils.Panic("size must be greater than or equal to the number of limbs, %d < %d", size, limbs.NumLimbs())
	}

	if size == limbs.NumLimbs() {
		return limbs
	}

	numLimbsToAdd := size - limbs.NumLimbs()

	newLimbs := make([]ifaces.Column, numLimbsToAdd)
	for i := range newLimbs {
		newLimbs[i] = verifiercol.NewConstantCol(field.Zero(), limbs.NumRow(), "0")
	}

	if isBigEndian[E]() {
		newLimbs = append(newLimbs, limbs.C...)
	} else {
		c := slices.Clone(limbs.C)
		newLimbs = append(c, newLimbs...)
	}

	return Limbs[E]{C: newLimbs, Name: limbs.Name}
}

// LeastSignificantLimb returns the least significant limb.
func (u Uint[S, E]) LeastSignificantLimb() ifaces.Column {
	if isBigEndian[E]() {
		return u.Limbs.C[len(u.Limbs.C)-1]
	}
	return u.Limbs.C[0]
}
