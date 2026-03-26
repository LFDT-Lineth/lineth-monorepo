package keccakfkoalabear

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/stretchr/testify/assert"
)

const (
	limit = 214358881 // 11^8
)

func TestConvertAndClean(t *testing.T) {
	var (
		size                   = 1024
		stateCurr              [5][5][8]ifaces.Column
		fromBaseX              *BackToThetaOrOutput
		isActive, isFirstBlock ifaces.Column
		period                 = 4
	)

	define := func(b *wizard.Builder) {
		comp := b.CompiledIOP
		/// commit to the input state
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				for z := 0; z < 8; z++ {
					stateCurr[x][y][z] = comp.InsertCommit(0, ifaces.ColIDf("BC_STATE_CURR_%v_%v_%v", x, y, z), size, true)

				}
			}
		}

		isActive = comp.InsertCommit(0, ifaces.ColID("BC_IS_ACTIVE"), size, true)
		isFirstBlock = comp.InsertCommit(0, ifaces.ColID("BC_IS_FIRST_BLOCK"), size, true)

		fromBaseX = newBackToThetaOrOutput(b.CompiledIOP, stateCurr, isActive, isFirstBlock)
	}
	prover := func(run *wizard.ProverRuntime) {
		// assign values to input state
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				for z := 0; z < 8; z++ {
					var col = make([]field.Element, size)
					for i := 0; i < size; i++ {
						var a uint64
						if i%2 == 0 {
							a = 10*14641 + 19487171 // 11^7+ 10 * 11^4  ---> 0*4^7+1*4^4 conversion in base 4 clean.
						} else {
							a = 7*14641 + 2*1771561 // 2*11^6 +7*11^4  ---> 1*4^6+1*4^4 conversion in base 4 clean.
						}
						col[i] = field.NewElement(uint64(a))
					}
					run.AssignColumn(stateCurr[x][y][z].GetColID(), smartvectors.NewRegular(col))
				}
			}
		}
		run.AssignColumn(isActive.GetColID(), smartvectors.RightZeroPadded(
			vector.Repeat(field.NewElement(1), size), size))

		run.AssignColumn(isFirstBlock.GetColID(),
			smartvectors.RightZeroPadded(vector.PeriodicOne(period, size-2), size))

		// assign the base conversion module
		fromBaseX.Run(run)
		expected00 := uint64(256)  // 4^4
		expected01 := uint64(16)   // 2^4
		expected10 := uint64(4352) // 1*4^6 +1*4^4
		expected11 := uint64(80)   // 1*2^6 +1*2^4
		var expected uint64

		isActive := run.GetColumn(fromBaseX.IsActive.GetColID()).IntoRegVecSaveAlloc()
		for x := 0; x < 5; x++ {
			for y := 0; y < 5; y++ {
				for z := 0; z < 8; z++ {
					// verify the output values are correct
					actualState := fromBaseX.StateNext[x][y][z].GetColAssignment(run).IntoRegVecSaveAlloc()
					for i := 0; i < size; i++ {
						switch {

						case isActive[i].IsZero():
							expected = 0
						case i%2 == 0 && (i+1)%period == 0:
							expected = expected01 // base2
						case i%2 != 0 && (i+1)%period == 0:
							expected = expected11 // base2
						case i+1 < size && isActive[i].IsOne() && isActive[i+1].IsZero() && i%2 == 0:
							expected = expected01 // base2
						case i+1 < size && isActive[i].IsOne() && isActive[i+1].IsZero() && i%2 != 0:
							expected = expected11 // base2
						case i+1 == size && i%2 == 0: //
							expected = expected01 // base2
						case i+1 == size && i%2 != 0: //
							expected = expected11 // base2
						case i%2 == 0 && (i+1)%period != 0:
							expected = expected00 // base Theta
						case i%2 != 0 && (i+1)%period != 0:
							expected = expected10 // base Theta
						}

						if actualState[i].Uint64() != expected {
							t.Fatalf("Base conversion failed at position (%v,%v,%v) at row %v: expected %v, got %v", x, y, z, i, expected, actualState[i].Uint64())
						}
					}
				}
			}
		}
	}

	compiled := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(compiled, prover)
	assert.NoErrorf(t, wizard.Verify(compiled, proof), "verifier failed")

}
