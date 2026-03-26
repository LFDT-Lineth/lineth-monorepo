package limbs

import (
	"math/rand/v2"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func TestGlobalSelector(t *testing.T) {

	var (
		ca   Uint128Be
		cb   Uint128Le
		cc   Uint128Le
		sel  ifaces.Column
		size = 16
		rng  = rand.NewChaCha8([32]byte{0xff})
		rnd  = rand.New(rng) // #nosec G404 -- test only
	)

	newRandByte16 := func() [16]byte {
		buf := [16]byte{}
		rng.Read(buf[:])
		return buf
	}

	define := func(b *wizard.Builder) {

		ca = NewUint128Be(b.CompiledIOP, "A", size)
		cb = NewUint128Le(b.CompiledIOP, "B", size)
		cc = NewUint128Le(b.CompiledIOP, "C", size)
		sel = b.InsertCommit(0, "SEL", size, true)

		NewGlobal(b.CompiledIOP, "G", sym.Sub(
			cc,
			sym.Mul(
				sym.Sub(1, sel),
				ca,
			),
			sym.Mul(
				sel,
				cb,
			),
		))
	}

	prove := func(run *wizard.ProverRuntime) {

		var (
			ca     = NewVectorBuilder(ca.AsDynSize())
			cb     = NewVectorBuilder(cb.AsDynSize())
			cc     = NewVectorBuilder(cc.AsDynSize())
			selAss = make([]field.Element, 0, size)
		)

		for i := 0; i < size; i++ {

			var (
				btsA = newRandByte16()
				btsB = newRandByte16()
				btsC [16]byte
				sel  = rnd.Uint64N(2)
			)

			switch sel {
			case 0:
				btsC = btsA
			case 1:
				btsC = btsB
			default:
				utils.Panic("sel should be boolean, %v", sel)
			}

			ca.PushBytes16(btsA)
			cb.PushBytes16(btsB)
			cc.PushBytes16(btsC)
			selAss = append(selAss, field.NewElement(sel))
		}

		ca.PadAndAssignZero(run)
		cb.PadAndAssignZero(run)
		cc.PadAndAssignZero(run)
		run.AssignColumn(sel.GetColID(), smartvectors.NewRegular(selAss))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatal(err)
	}
}
