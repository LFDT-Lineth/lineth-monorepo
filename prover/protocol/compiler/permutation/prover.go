package permutation

import (
	"runtime"
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// ProverTaskAtRound implements the [wizard.ProverAction] interface and is
// responsible for assigning the Z polynomials of all the queries for which the
// Z polynomial needs to be assigned in the current round
type ProverTaskAtRound struct {
	Zs []*ZCtx
}

// Run implements the [wizard.ProverAction interface]. The tasks will spawn
// a goroutine for each tasks and wait for all of them to finish. The approach
// for parallelization can be justified if the number of go-routines stays low
// (e.g. less than 1000s).
func (p ProverTaskAtRound) Run(run *wizard.ProverRuntime) {
	wg := &sync.WaitGroup{}
	wg.Add(len(p.Zs))

	for i := range p.Zs {
		go func(i int) {
			p.Zs[i].run(run)
			wg.Done()
		}(i)
	}

	wg.Wait()
}

// run assigns all the Zs in parallel and set the parameters for their
// corresponding last values openings.

// run assigns all the Zs in parallel and set the parameters for their
// corresponding last values openings.
func (z *ZCtx) run(run *wizard.ProverRuntime) {
	for i := range z.Zs {
		var (
			numerator   smartvectors.SmartVector
			denominator smartvectors.SmartVector
		)

		if packingArity*i < len(z.NumeratorFactors) {
			numerator = column.EvalExprColumn(run, z.NumeratorFactorsBoarded[i])
		} else {
			numerator = smartvectors.NewConstant(field.One(), z.Size)
		}

		if packingArity*i < len(z.DenominatorFactors) {
			denominator = column.EvalExprColumn(run, z.DenominatorFactorsBoarded[i])
		} else {
			denominator = smartvectors.NewConstant(field.One(), z.Size)
		}

		// This case does not corresponds to actual production use of the compiler
		// because due to how grand-product queries are constructed, the Zs
		// column is always dependant on a randomness and is therefore over a
		// field extension.
		if smartvectors.IsBase(numerator) && smartvectors.IsBase(denominator) {
			// If both numerator and denominator are base
			denominatorSlice, _ := denominator.IntoRegVecSaveAllocBase()
			denominatorSlice = field.ParBatchInvert(denominatorSlice, runtime.NumCPU()/4)

			numeratorSlice, _ := numerator.IntoRegVecSaveAllocBase()

			vNum := field.Vector(numeratorSlice)
			vNum.Mul(vNum, field.Vector(denominatorSlice))
			for i := 1; i < len(numeratorSlice); i++ {
				numeratorSlice[i].Mul(&numeratorSlice[i], &numeratorSlice[i-1])
			}

			run.AssignColumn(z.Zs[i].GetColID(), smartvectors.NewRegular(numeratorSlice))

			// Regardless of the assignment, the local opening will always be
			// defined as a field extension column.
			run.AssignLocalPointExt(
				z.ZOpenings[i].Name(),
				fext.Lift(numeratorSlice[len(numeratorSlice)-1]),
			)
		} else {
			// at least one of the numerator or denominator is over field extensions
			denominatorSlice := denominator.IntoRegVecSaveAllocExt()
			denominatorSlice = fext.ParBatchInvert(denominatorSlice, runtime.NumCPU()/4)

			numeratorSlice := numerator.IntoRegVecSaveAllocExt()

			vNum := extensions.Vector(numeratorSlice)
			vNum.Mul(vNum, extensions.Vector(denominatorSlice))
			vNum.PrefixProduct()

			run.AssignColumn(z.Zs[i].GetColID(), smartvectors.NewRegularExt(numeratorSlice))
			run.AssignLocalPointExt(z.ZOpenings[i].Name(), numeratorSlice[len(numeratorSlice)-1])
		}

	}
}

// AssignPermutationGranddProduct assigns the grand product query
type AssignPermutationGrandProduct struct {
	Query *query.GrandProduct
	// IsPartial indicates that the permuation queries contains public
	// terms to evaluate explictly by the verifier. In that case, the
	// result of the query is not one and must be computed explicitly.
	IsPartial bool
}

func (a AssignPermutationGrandProduct) Run(run *wizard.ProverRuntime) {
	y := fext.GenericFieldOne()
	if a.IsPartial {
		res := a.Query.Compute(run)
		y = res
	}
	if y.GetIsBase() {
		baseRes, _ := y.GetBase()
		run.AssignGrandProduct(a.Query.ID, baseRes)
	} else {
		extRes := y.GetExt()
		run.AssignGrandProductExt(a.Query.ID, extRes)
	}
}
