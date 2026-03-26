package mpts

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// ShadowRowProverAction is a prover action that assigns a shadow column
// to zero
type ShadowRowProverAction struct {
	Name ifaces.ColID
	Size int
}

func (a *ShadowRowProverAction) Run(run *wizard.ProverRuntime) {
	run.AssignColumn(a.Name, smartvectors.NewConstant(field.Zero(), a.Size))
}

// A shadow row is a row filled with zeroes that we **may** add at the end of
// the rounds commitment. Its purpose is to ensure the number of "SIS limbs" in
// a row divides the degree of the ring-SIS instance.
func autoAssignedShadowRow(comp *wizard.CompiledIOP, size, round, id int) ifaces.Column {

	name := ifaces.ColIDf("MPTS_%v_SHADOW_ROUND_%v_ID_%v", comp.SelfRecursionCount, round, id)
	col := comp.InsertCommit(round, name, size, true)

	comp.RegisterProverAction(round, &ShadowRowProverAction{
		Name: name,
		Size: size,
	})

	return col
}

// precomputedShadowRow is a row filled with zeroes that we is precomputed
func precomputedShadowRow(comp *wizard.CompiledIOP, size, i int) ifaces.Column {
	name := ifaces.ColIDf("MPTS_%v_PRECOMPUTED_SHADOW_%v", comp.SelfRecursionCount, i)
	val := smartvectors.NewConstant(field.Zero(), size)
	return comp.InsertPrecomputed(name, val)
}
