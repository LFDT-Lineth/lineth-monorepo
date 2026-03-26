package innerproduct

import (
	"sync"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
)

// ProverTask implements the [wizard.ProverAction] interface and as such
// implements the prover work of the compilation step. It works by calling
// in parallel the prover tasks of the sub-compilation steps.
type ProverTask []*ContextForSize

// Run implements the [wizard.ProverAction] interface.
func (p ProverTask) Run(run *wizard.ProverRuntime) {

	wg := &sync.WaitGroup{}
	wg.Add(len(p))

	for i := range p {
		// Passing the loop index ensures each go routine is storing the value
		// of i in a different variable so that there is no race condition over
		// i.
		go func(i int) {
			p[i].run(run)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

// run partially implements the prover runtime associated with the current
// partial compilation context. Its role is to assign Summation and its opening.
func (ctx *ContextForSize) run(run *wizard.ProverRuntime) {

	var (
		size      = ctx.Summation.Size()
		collapsed = column.EvalExprColumn(run, ctx.CollapsedBoard).IntoRegVecSaveAlloc()
		summation = make([]field.Element, size)
	)

	summation[0] = collapsed[0]

	for i := 0; i+1 < size; i++ {
		summation[i+1].Add(&summation[i], &collapsed[i+1])
	}

	run.AssignColumn(ctx.Summation.GetColID(), smartvectors.NewRegular(summation))
	run.AssignLocalPoint(ctx.SummationOpening.ID, summation[len(summation)-1])
}
