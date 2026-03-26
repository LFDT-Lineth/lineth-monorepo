package functionals

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
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
	aVal := a.A.GetVal(assi)
	one := field.One()
	p := a.P.GetColAssignment(assi)

	omegaInv := fft.GetOmega(a.N)
	omegaInv.Inverse(&omegaInv)

	witi := make([]field.Element, a.N)
	witi[0] = aVal

	aRootOfUnityFlag := false
	for i := 1; i < a.N; i++ {
		witi[i].Mul(&witi[i-1], &omegaInv)
		witi[i-1].Sub(&witi[i-1], &one)
		if witi[i-1].IsZero() {
			aRootOfUnityFlag = true
		}
	}
	witi[a.N-1].Sub(&witi[a.N-1], &one)

	if witi[a.N-1].IsZero() || aRootOfUnityFlag {
		utils.Panic("detected that a is a root of unity")
	}

	witi = field.BatchInvert(witi)

	for i := range witi {
		pi := p.Get(i)
		witi[i].Mul(&pi, &witi[i])
		if i > 0 {
			witi[i].Add(&witi[i], &witi[i-1])
		}
	}

	assi.AssignColumn(ifaces.ColIDf("%v_%v", a.Name, INTERPOLATION_POLY), smartvectors.NewRegular(witi))
	assi.AssignLocalPoint(ifaces.QueryIDf("%v_%v", a.Name, INTERPOLATION_OPEN_END), witi[a.N-1])
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
	omega := symbolic.NewConstant(fft.GetOmega(p.Size()))
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
