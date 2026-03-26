package functionals

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"

	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/fft"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
)

const (
	INTERPOLATION_POLY          string = "INTERPOLATION_COEFF_EVAL"
	INTERPOLATION_LOCAL_BEGIN_0 string = "INTERPOLATION_LOCAL_BEGIN_0"
	INTERPOLATION_LOCAL_BEGIN_1 string = "INTERPOLATION_LOCAL_BEGIN_1"
	INTERPOLATION_OPEN_END      string = "INTERPOLATION_OPEN_END"
	INTERPOLATION_GLOBAL        string = "INTERPOLATION_GLOBAL"
)

type InterpolationProverAction struct {
	Name string
	A    ifaces.Accessor
	P    ifaces.Column
	N    int
}

func (a *InterpolationProverAction) Run(assi *wizard.ProverRuntime) {
	aVal := a.A.GetValExt(assi)
	one := fext.One()
	p := a.P.GetColAssignment(assi)

	omegaInv, err := fft.Generator(uint64(a.N))
	if err != nil {
		utils.Panic("the domain size is too large (%v)", err)
	}
	omegaInv.Inverse(&omegaInv)

	witi := make(extensions.Vector, a.N)

	// The first loop computes a geometric progression: witi[i] = aVal * (omegaInv)^i
	// Then it subtracts 1 from each element.
	// Since this is sequential, we keep the structure but optimize the check.
	// witi[i] = witi[i-1] * omegaInv
	// witi[i-1] = witi[i-1] - 1
	//
	// Let's unroll the logic slightly to separate generation from subtraction if possible,
	// but the dependency makes generation strictly serial.
	//
	// witi[0] = aVal
	// witi[1] = aVal * w
	// ...
	// witi[k] = aVal * w^k
	//
	// Then we subtract 1 from all.

	// 1. Generate powers
	// note: merged in the loop below.
	// witi[0] = aVal
	// for i := 1; i < a.N; i++ {
	// 	witi[i].MulByElement(&witi[i-1], &omegaInv)
	// }

	// 2. Subtract 1 from all elements (Parallelizable)
	// We also check for zero (root of unity) here.
	// Note: The original code checked IsZero() on the result of (val - 1).
	// If (val - 1) is zero, then val was 1.
	// Since we need to panic if any is zero, we can do this check during or after subtraction.
	parallel.Execute(a.N, func(start, end int) {

		var wInvStart field.Element
		wInvStart.ExpInt64(omegaInv, int64(start))

		currentWInv := wInvStart

		for i := start; i < end; i++ {
			// witi[i] = aVal * (omegaInv)^i - 1
			witi[i].MulByElement(&aVal, &currentWInv)
			currentWInv.Mul(&currentWInv, &omegaInv)

			// Subtract 1
			witi[i].Sub(&witi[i], &one)
			if witi[i].IsZero() {
				utils.Panic("detected that a is a root of unity")
			}
		}
	})

	// 4. Invert
	witi = fext.ParBatchInvert(witi, 0)

	// 5. Multiply by P[i] (Parallelizable)
	// witi[i] = witi[i] * p[i]
	if pReg, ok := p.(*smartvectors.RegularExt); ok {
		witi.Mul(witi, pReg.IntoRegVecSaveAllocExt())
	} else {
		// We need p as a vector. p is a ColumnAssignment.
		// p.GetExt(i) retrieves the value.
		parallel.Execute(a.N, func(start, end int) {
			for i := start; i < end; i++ {
				pi := p.GetExt(i)
				witi[i].Mul(&pi, &witi[i])
			}
		})
	}

	// 6. Cumulative Sum (Sequential)
	// witi[i] = witi[i] + witi[i-1]
	for i := 1; i < a.N; i++ {
		witi[i].Add(&witi[i], &witi[i-1])
	}

	assi.AssignColumn(ifaces.ColIDf("%v_%v", a.Name, INTERPOLATION_POLY), smartvectors.NewRegularExt(witi))
	assi.AssignLocalPointExt(ifaces.QueryIDf("%v_%v", a.Name, INTERPOLATION_OPEN_END), witi[a.N-1])
}

// See the explainer here : https://hackmd.io/S78bJUa0Tk-T256iduE22g#Evaluate-in-Lagrange-form
// The variable names are the same as the one in the hackmd
func Interpolation(comp *wizard.CompiledIOP, name string, a ifaces.Accessor, p ifaces.Column) ifaces.Accessor {

	length := p.Size()
	maxRound := utils.Max(a.Round(), p.Round())

	i := comp.InsertCommit(
		maxRound,
		ifaces.ColIDf("%v_%v", name, INTERPOLATION_POLY),
		length,
		false,
	)

	/*
		Common variables
	*/
	aV := a.AsVariable()
	pPrev := ifaces.ColumnAsVariable(column.Shift(p, -1))
	pV := ifaces.ColumnAsVariable(p)
	pNext := ifaces.ColumnAsVariable(column.Shift(p, 1))
	iPrevPrev := ifaces.ColumnAsVariable(column.Shift(i, -2))
	iPrev := ifaces.ColumnAsVariable(column.Shift(i, -1))
	iV := ifaces.ColumnAsVariable(i)
	iNext := ifaces.ColumnAsVariable(column.Shift(i, 1))
	one := symbolic.NewConstant(1)
	omegainit, err := fft.Generator(uint64(p.Size()))
	if err != nil {
		panic(err)
	}
	omega := symbolic.NewConstant(omegainit)
	omegaMin1 := omega.Sub(one)

	/*
		For the global constraint
			(ω−1)Δ[i]Δ[i−1] + ωΔ[i−1]P[i] − Δ[i]P[i−1] == 0
	*/
	delta := iV.Sub(iPrev)
	deltaPrev := iPrev.Sub(iPrevPrev)
	t1 := omegaMin1.Mul(delta).Mul(deltaPrev)
	t2 := omega.Mul(deltaPrev).Mul(ifaces.ColumnAsVariable(p))
	t3 := pPrev.Mul(delta)

	comp.InsertGlobal(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, INTERPOLATION_GLOBAL),
		t1.Add(t2).Sub(t3),
	)

	/*
		For the local constraint at the beginning
			(a − 1)I[0] = P[0]
	*/

	comp.InsertLocal(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, INTERPOLATION_LOCAL_BEGIN_0),
		aV.Sub(one).Mul(iV).Sub(pV),
	)

	/*
		For the the second local constraint
			(a − ω)(I[1] − I[0]) = ωP[1]
	*/

	comp.InsertLocal(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, INTERPOLATION_LOCAL_BEGIN_1),
		aV.Sub(omega).Mul(iNext.Sub(iV)).Sub(pNext.Mul(omega)),
	)

	/*
		Local opening at the end
	*/

	localOpenEnd := comp.InsertLocalOpening(
		maxRound,
		ifaces.QueryIDf("%v_%v", name, INTERPOLATION_OPEN_END),
		column.Shift(i, -1),
	)

	comp.RegisterProverAction(maxRound, &InterpolationProverAction{
		Name: name,
		A:    a,
		P:    p,
		N:    length,
	})

	// Since the symbolic package does not support inversion, we have to compute
	// n**(-1) outside of the expression that we use to instantiate the returned
	// accessor.
	nInv := field.NewElement(uint64(p.Size()))
	nInv.Inverse(&nInv)

	// Finally we return the accessor that will read the interpolation result
	return accessors.NewFromExpression(
		symbolic.Mul(
			symbolic.Sub(
				symbolic.Pow(a, p.Size()),
				1,
			),
			accessors.NewLocalOpeningAccessor(localOpenEnd, maxRound),
			nInv,
		),
		fmt.Sprintf("INTERPOLATION_RES_%v", name),
	)
}
