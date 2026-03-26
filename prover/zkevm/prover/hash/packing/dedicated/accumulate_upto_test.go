package dedicated

import (
	"crypto/rand"
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/stretchr/testify/assert"
)

// It generates Define and Assign function of Packing module, for testing
func makeTestCaseLaneAlloc() (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	var (
		//  the total value stored in colA should be a factor of maxValue
		n    = 20
		size = 256
	)
	acc := &AccumulateUpToMaxCtx{}
	maxValue := 4

	define = func(build *wizard.Builder) {
		comp := build.CompiledIOP

		colA := comp.InsertCommit(0, ifaces.ColIDf("COL_A"), size, true)

		acc = AccumulateUpToMax(comp, maxValue, colA, verifiercol.NewConstantCol(field.One(), size, "accumulate-up-to-max"))

	}
	prover = func(run *wizard.ProverRuntime) {

		// assigning cldLenSpaghetti, and isActive
		acc.assignNonNativeColumns(run, n, size)

		// assigning the submodule
		acc.Run(run)
	}
	return define, prover
}

func TestLaneAlloc(t *testing.T) {
	define, prover := makeTestCaseLaneAlloc()
	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

// it assigns ColA
func (acc *AccumulateUpToMaxCtx) assignNonNativeColumns(run *wizard.ProverRuntime, n, size int) {
	var (
		max  = acc.Inputs.MaxValue
		colA = acc.Inputs.ColA
		col  = common.NewVectorBuilder(colA)
	)

	b, _ := cutUpToMax(max, n, size)

	for j := 0; j < len(b); j++ {
		col.PushInt(b[j])
	}
	col.PadAndAssign(run, field.Zero())
}

// it generates nBytes and then cut each nByte up to the given max.
func cutUpToMax(max int, times, size int) (b, c []int) {
	total := max * times
	t := total / size
	remain := total
	var nByte []int
	currRow := 0
	for remain > 0 {
		if currRow == size-1 {
			nByte = append(nByte, remain)
		} else {
			nBig, _ := rand.Int(rand.Reader, big.NewInt(int64(t+5)))
			n := int(nBig.Int64()) + 1
			if n > remain {
				n = remain
			}
			nByte = append(nByte, n)
			remain = remain - n
			currRow++
		}
	}

	s := 0
	for i := 0; i < min(size, len(nByte)); i++ {
		s = s + nByte[i]
	}
	if s != total {
		utils.Panic("nByte is not generated correctly")
	}

	missing := max
	for i := range nByte {
		var a []int
		curr := nByte[i]
		for curr != 0 {
			if curr >= missing {
				a = append(a, missing)
				c = append(c, 0)
				curr = curr - missing
				missing = max
			} else {
				a = append(a, curr)
				missing = missing - curr
				curr = 0
			}
		}
		b = append(b, a...)
	}
	return b, c
}
