package dedicated

import (
	"crypto/rand"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/stretchr/testify/assert"
)

func makeTestCaseLengthConsistency() (
	define wizard.DefineFunc,
	prover wizard.MainProverStep,
) {
	lc := LengthConsistencyCtx{}
	size := 8
	maxLen := 2

	define = func(build *wizard.Builder) {
		var (
			comp = build.CompiledIOP

			t    = make([]ifaces.Column, 2)
			tLen = make([]ifaces.Column, 2)

			inp = LcInputs{
				Table:    t,
				TableLen: tLen,
				MaxLen:   maxLen,
			}
		)

		t[0] = comp.InsertCommit(0, "Table_0", size, true)
		t[1] = comp.InsertCommit(0, "Table_1", size, true)

		tLen[0] = comp.InsertCommit(0, "TableLen_0", size, true)
		tLen[1] = comp.InsertCommit(0, "TableLen_1", size, true)

		lc = *LengthConsistency(comp, inp)
	}
	prover = func(run *wizard.ProverRuntime) {

		lc.assignTable(run)
		lc.Run(run)

	}
	return define, prover
}

func TestLengthConsistency(t *testing.T) {
	define, prover := makeTestCaseLengthConsistency()
	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	assert.NoErrorf(t, wizard.Verify(comp, proof), "invalid proof")
}

func (lc *LengthConsistencyCtx) assignTable(run *wizard.ProverRuntime) {
	var (
		table    = make([]*common.VectorBuilder, 2)
		tableLen = make([]*common.VectorBuilder, 2)
		f        field.Element
	)
	for i := range table {
		table[i] = common.NewVectorBuilder(lc.Inp.Table[i])

		tableLen[i] = common.NewVectorBuilder(lc.Inp.TableLen[i])

		for row := 0; row < lc.Size; row++ {
			token := make([]byte, row%lc.Inp.MaxLen)
			rand.Read(token)
			table[i].PushField(*f.SetBytes(token))
			tableLen[i].PushInt(row % lc.Inp.MaxLen)
		}
		table[i].PadAndAssign(run, field.Zero())
		tableLen[i].PadAndAssign(run, field.Zero())
	}

}
