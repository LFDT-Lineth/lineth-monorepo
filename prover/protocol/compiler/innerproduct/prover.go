package innerproduct

import (
	"sync"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// ProverTask implements the [wizard.ProverAction] interface and as such
// implements the prover work of the compilation step. It works by calling
// in parallel the prover tasks of the sub-compilation steps.
//
// The reason we need a wrapping structure (and not just `type X []*Y“) is
// because the serializer will recognize it as an `[]*Y` and not as an `X` and
// thus not as a valid interface implementation for [wizard.ProverAction].
type ProverTask struct {
	Contexts []*ContextForSize
}

// Run implements the [wizard.ProverAction] interface.
func (p ProverTask) Run(run *wizard.ProverRuntime) {

	wg := &sync.WaitGroup{}
	wg.Add(len(p.Contexts))

	for i := range p.Contexts {
		// Passing the loop index ensures each go routine is storing the value
		// of i in a different variable so that there is no race condition over
		// i.
		go func(i int) {
			p.Contexts[i].run(run)
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
		collapsed = column.EvalExprColumn(run, ctx.CollapsedBoard).IntoRegVecSaveAllocExt()
		summation = make([]fext.Element, size)
	)

	summation[0] = collapsed[0]
	for i := 0; i+1 < size; i++ {
		summation[i+1].Add(&summation[i], &collapsed[i+1])
	}

	run.AssignColumn(ctx.Summation.GetColID(), smartvectors.NewRegularExt(summation))
	run.AssignLocalPointExt(ctx.SummationOpening.ID, summation[len(summation)-1])
}
