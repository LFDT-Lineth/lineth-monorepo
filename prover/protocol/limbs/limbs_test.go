package limbs

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

func TestLimbSplitOnBit(t *testing.T) {

	var (
		comp     = wizard.NewCompiledIOP()
		a        = NewLimbs[LittleEndian](comp, "A", 10, 32)
		aHi, aLo = a.SplitOnBit(128)
		b        = NewLimbs[BigEndian](comp, "B", 10, 32)
		bHi, bLo = b.SplitOnBit(128)
	)

	assert.Equal(t, "A_2", string(aHi.ToRawUnsafe()[0].GetColID()))
	assert.Equal(t, "A_0", string(aLo.ToRawUnsafe()[0].GetColID()))
	assert.Equal(t, "B_0", string(bHi.ToRawUnsafe()[0].GetColID()))
	assert.Equal(t, "B_8", string(bLo.ToRawUnsafe()[0].GetColID()))
}
