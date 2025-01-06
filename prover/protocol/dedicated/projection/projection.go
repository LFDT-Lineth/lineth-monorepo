/*
Package projection implements the utilities for the projection query.

A projection query between sets (columnsA,filterA) and (columnsB,filterB) asserts
whether the columnsA filtered by filterA is the same as columnsB filtered by
filterB, preserving the order.

Example:

FilterA = (1,0,0,1,1), ColumnA := (aO,a1,a2,a3,a4)

FiletrB := (0,0,1,0,0,0,0,0,1,1), ColumnB :=(b0,b1,b2,b3,b4,b5,b6,b7,b8,b9)

Thus we have,

ColumnA filtered by FilterA = (a0,a3,a4)

ColumnB filtered by FilterB  = (b2,b8,b9)

The projection query checks if a0 = b2, a3 = b8, a4 = b9

Note that the query imposes that:
  - the number of 1 in the filters are equal
  - the order of filtered elements is preserved
*/
package projection

import (
	"fmt"
	"strings"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/protocol/wizardutils"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
)

// projectionProverAction is a compilation artefact generated during the
// execution of the [InsertProjection] and which implements the
// [wizard.ProverAction]. It is meant to compute to assign the "Horner" columns
// and their respective local opening queries.
type projectionProverAction struct {
	Name               ifaces.QueryID
	FilterA, FilterB   ifaces.Column
	ColA, ColB         []ifaces.Column
	ABoard, BBoard     sym.ExpressionBoard
	EvalCoin           coin.Info
	HornerA, HornerB   ifaces.Column
	HornerA0, HornerB0 query.LocalOpening
}

// projectionVerifierAction is a compilation artifact generated during the
// execution of the [InsertProjection] and which implements the [wizard.VerifierAction]
// interface. It is meant to perform the verifier checks that the first values
// of the two Horner are equals.
type projectionVerifierAction struct {
	Name               ifaces.QueryID
	HornerA0, HornerB0 query.LocalOpening
	skipped            bool
}

// InsertProjection applies a projection query between sets (columnsA, filterA)
// and (columnsB,filterB).
//
// Note: The filters are supposed to be binary.
// These binary constraints are not handled here and should have been imposed
// before calling the function.
func InsertProjection(
	comp *wizard.CompiledIOP,
	queryName ifaces.QueryID,
	columnsA, columnsB []ifaces.Column,
	filterA, filterB ifaces.Column,
) {

	var (
		sizeA  = filterA.Size()
		sizeB  = filterB.Size()
		numCol = len(columnsA)
		round  = max(
			wizardutils.MaxRound(columnsA...),
			wizardutils.MaxRound(columnsB...),
			filterA.Round(),
			filterB.Round(),
		)

		// a and b are storing the columns used to compute the linear combination
		// of the columnsA and columnsB. The initial assignment is for the case
		// where there is only a single column. If there is more than one column
		// then they will store an expression computing a random linear
		// combination of the columns.
		a, b any = columnsA[0], columnsB[0]

		// af and bf are as a and b but shifted by -1. They are initially
		// assigned assuming the case where the number of column is 1 and
		// replaced later by a random linear combination if not. They are meant
		// to be used in the local constraints to point to the last entry of the
		// "a" and "b".
		af, bf any = column.Shift(columnsA[0], -1), column.Shift(columnsB[0], -1)
	)

	if len(columnsB) != numCol {
		utils.Panic("A and B must have the same number of columns")
	}

	if ifaces.AssertSameLength(columnsA...) != sizeA {
		utils.Panic("A and its filter do not have the same column sizes")
	}

	if ifaces.AssertSameLength(columnsB...) != sizeB {
		utils.Panic("B and its filter do not have the same column sizes")
	}

	if numCol > 0 {
		round++
		alpha := comp.InsertCoin(round, coin.Namef("%v_MERGING_COIN", queryName), coin.Field)
		a = wizardutils.RandLinCombColSymbolic(alpha, columnsA)
		b = wizardutils.RandLinCombColSymbolic(alpha, columnsB)

		afs := make([]ifaces.Column, numCol)
		bfs := make([]ifaces.Column, numCol)

		for i := range afs {
			afs[i] = column.Shift(columnsA[i], -1)
			bfs[i] = column.Shift(columnsB[i], -1)
		}

		af = wizardutils.RandLinCombColSymbolic(alpha, afs)
		bf = wizardutils.RandLinCombColSymbolic(alpha, bfs)
	}

	var (
		aExpr, _, _ = wizardutils.AsExpr(a)
		bExpr, _, _ = wizardutils.AsExpr(b)
		pa          = projectionProverAction{
			Name:     queryName,
			EvalCoin: comp.InsertCoin(round, coin.Namef("%v_EVAL_COIN", queryName), coin.Field),
			FilterA:  filterA,
			FilterB:  filterB,
			ColA:     columnsA,
			ColB:     columnsB,
			ABoard:   aExpr.Board(),
			BBoard:   bExpr.Board(),
			HornerA:  comp.InsertCommit(round, ifaces.ColIDf("%v_HORNER_A", queryName), sizeA),
			HornerB:  comp.InsertCommit(round, ifaces.ColIDf("%v_HORNER_B", queryName), sizeB),
		}
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%v_HORNER_A_GLOBAL", queryName),
		sym.Sub(
			pa.HornerA,
			sym.Mul(
				sym.Sub(1, pa.FilterA),
				column.Shift(pa.HornerA, 1),
			),
			sym.Mul(
				pa.FilterA,
				sym.Add(
					a,
					sym.Mul(
						pa.EvalCoin,
						column.Shift(pa.HornerA, 1),
					),
				),
			),
		),
	)

	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%v_HORNER_B_GLOBAL", queryName),
		sym.Sub(
			pa.HornerB,
			sym.Mul(
				sym.Sub(1, pa.FilterB),
				column.Shift(pa.HornerB, 1),
			),
			sym.Mul(
				pa.FilterB,
				sym.Add(b, sym.Mul(pa.EvalCoin, column.Shift(pa.HornerB, 1))),
			),
		),
	)

	comp.InsertLocal(
		0,
		ifaces.QueryIDf("%v_HORNER_A_LOCAL_END", queryName),
		sym.Sub(
			column.Shift(pa.HornerA, -1),
			sym.Mul(column.Shift(pa.FilterA, -1), af),
		),
	)

	comp.InsertLocal(
		0,
		ifaces.QueryIDf("%v_HORNER_B_LOCAL_END", queryName),
		sym.Sub(
			column.Shift(pa.HornerB, -1),
			sym.Mul(column.Shift(pa.FilterB, -1), bf),
		),
	)

	pa.HornerA0 = comp.InsertLocalOpening(round, ifaces.QueryIDf("%v_HORNER_A0", queryName), pa.HornerA)
	pa.HornerB0 = comp.InsertLocalOpening(round, ifaces.QueryIDf("%v_HORNER_B0", queryName), pa.HornerB)

	comp.RegisterProverAction(round, pa)
	comp.RegisterVerifierAction(round, &projectionVerifierAction{HornerA0: pa.HornerA0, HornerB0: pa.HornerB0, Name: queryName})
}

// Run implements the [wizard.ProverAction] interface.
func (pa projectionProverAction) Run(run *wizard.ProverRuntime) {

	var (
		a       = wizardutils.EvalExprColumn(run, pa.ABoard).IntoRegVecSaveAlloc()
		b       = wizardutils.EvalExprColumn(run, pa.BBoard).IntoRegVecSaveAlloc()
		fA      = pa.FilterA.GetColAssignment(run).IntoRegVecSaveAlloc()
		fB      = pa.FilterB.GetColAssignment(run).IntoRegVecSaveAlloc()
		x       = run.GetRandomCoinField(pa.EvalCoin.Name)
		hornerA = cmptHorner(a, fA, x)
		hornerB = cmptHorner(b, fB, x)
	)

	run.AssignColumn(pa.HornerA.GetColID(), smartvectors.NewRegular(hornerA))
	run.AssignColumn(pa.HornerB.GetColID(), smartvectors.NewRegular(hornerB))
	run.AssignLocalPoint(pa.HornerA0.ID, hornerA[0])
	run.AssignLocalPoint(pa.HornerB0.ID, hornerB[0])

	if hornerA[0] != hornerB[0] {

		var (
			colA  = make([][]field.Element, len(pa.ColA))
			colB  = make([][]field.Element, len(pa.ColB))
			cntA  = 0
			cntB  = 0
			rowsA = [][]string{}
			rowsB = [][]string{}
		)

		for c := range pa.ColA {
			colA[c] = pa.ColA[c].GetColAssignment(run).IntoRegVecSaveAlloc()
			colB[c] = pa.ColB[c].GetColAssignment(run).IntoRegVecSaveAlloc()
		}

		for i := range fA {

			if fA[i].IsZero() {
				continue
			}

			row := make([]string, len(pa.ColA))

			for c := range pa.ColA {
				fString := colA[c][i].Text(16)
				if colA[c][i].IsUint64() && colA[c][i].Uint64() < 1000000 {
					fString = colA[c][i].String()
				}
				row[c] = fmt.Sprintf("%v=%v", pa.ColA[c].GetColID(), fString)
			}

			rowsA = append(rowsA, row)
			cntA++
		}

		for i := range fB {

			if fB[i].IsZero() {
				continue
			}

			row := make([]string, len(pa.ColB))

			for c := range pa.ColB {
				fString := colB[c][i].Text(16)
				if colB[c][i].IsUint64() && colB[c][i].Uint64() < 1000000 {
					fString = colB[c][i].String()
				}
				row[c] = fmt.Sprintf("%v=%v", pa.ColB[c].GetColID(), fString)
			}

			rowsB = append(rowsB, row)
			cntB++
		}

		larger := max(len(rowsA), len(rowsB))

		for i := 0; i < larger; i++ {

			var (
				fa = "* * * * * *"
				fb = "* * * * * *"
			)

			if i < len(rowsA) {
				fa = strings.Join(rowsA[i], " ")
			}

			if i < len(rowsB) {
				fb = strings.Join(rowsB[i], " ")
			}

			fmt.Printf("row=%v %v %v\n", i, fa, fb)
		}

		logrus.Errorf("projection query %v failed", pa.Name)
	}
}

// Run implements the [wizard.VerifierAction] interface.
func (va *projectionVerifierAction) Run(run *wizard.VerifierRuntime) error {

	var (
		a = run.GetLocalPointEvalParams(va.HornerA0.ID).Y
		b = run.GetLocalPointEvalParams(va.HornerB0.ID).Y
	)

	if a != b {
		return fmt.Errorf("the horner check of the projection query `%v` did not pass", va.Name)
	}

	return nil
}

// RunGnark implements the [wizard.VerifierAction] interface.
func (va *projectionVerifierAction) RunGnark(api frontend.API, run *wizard.WizardVerifierCircuit) {

	var (
		a = run.GetLocalPointEvalParams(va.HornerA0.ID).Y
		b = run.GetLocalPointEvalParams(va.HornerB0.ID).Y
	)

	api.AssertIsEqual(a, b)
}

func (va *projectionVerifierAction) Skip() {
	va.skipped = true
}

func (va *projectionVerifierAction) IsSkipped() bool {
	return va.skipped
}

// cmptHorner computes a random Horner accumulation of the filtered elements
// starting from the last entry down to the first entry. The final value is
// stored in the last entry of the returned slice.
func cmptHorner(c, fC []field.Element, x field.Element) []field.Element {

	var (
		horner = make([]field.Element, len(c))
		prev   field.Element
	)

	for i := len(horner) - 1; i >= 0; i-- {

		if !fC[i].IsZero() && !fC[i].IsOne() {
			utils.Panic("we expected the filter to be binary")
		}

		if fC[i].IsOne() {
			prev.Mul(&prev, &x)
			prev.Add(&prev, &c[i])
		}

		horner[i] = prev
	}

	return horner
}
