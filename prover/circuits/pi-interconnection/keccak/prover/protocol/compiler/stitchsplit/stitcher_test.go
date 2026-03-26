package stitchsplit_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/accessors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/stitchsplit"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	P1, P2, P3       ifaces.ColID   = "P1", "P2", "P3"
	GLOBAL1, GLOBAL2 ifaces.QueryID = "GLOBAL1", "GLOBAL2"
	LOCAL1           ifaces.QueryID = "LOCAL1"
)

func TestLocalOpening(t *testing.T) {
	testStitcher(t, 8, 16, localOpening(4))
}

func TestStitcherFibo(t *testing.T) {
	testStitcher(t, 16, 32, singlePolyFibo(4))
	testStitcher(t, 4, 8, singlePolyFibo(8))
	testStitcher(t, 8, 16, singlePolyFibo(8))
}

func TestStitcherGlobalWithPeriodicSample(t *testing.T) {
	testStitcher(t, 16, 64, globalWithPeriodicSample(16, 8, 0))
	testStitcher(t, 64, 256, globalWithPeriodicSample(256, 8, 1))
	testStitcher(t, 64, 128, globalWithPeriodicSample(256, 8, 7))
}

func TestStitcherLocalWithPeriodicSample(t *testing.T) {
	testStitcher(t, 32, 64, localWithPeriodicSample(256, 8, 0))
	testStitcher(t, 16, 128, localWithPeriodicSample(256, 8, 1))
	testStitcher(t, 64, 256, localWithPeriodicSample(256, 8, 7))
}

func TestStitcherGlobalWithVerifColAndPeriodic(t *testing.T) {
	testStitcher(t, 8, 64, globalWithVerifColAndPeriodic(8, 4, 0))
	testStitcher(t, 64, 128, globalWithVerifColAndPeriodic(256, 8, 1))
	testStitcher(t, 8, 16, globalWithVerifColAndPeriodic(256, 8, 7))
}

func TestStitcherGlobalWithProofCol(t *testing.T) {
	testStitcher(t, 8, 64, globalWithProofCols(16))
	testStitcher(t, 64, 128, globalWithProofCols(16))
	testStitcher(t, 8, 16, globalWithProofCols(32))
}

func TestLocalEvalWithStatus(t *testing.T) {
	// Set log level to Info to capture detailed logs during the test
	logrus.SetLevel(logrus.DebugLevel)

	var a, b, c, d ifaces.Column
	var q1, q2, q3, q4, q5, q6, q7, q8, q9, q10, q11, q12 query.LocalOpening

	define := func(builder *wizard.Builder) {
		logrus.Info("Defining columns and local opening queries")
		// Declare columns of different sizes
		a = builder.RegisterCommit("A", 2)
		b = builder.RegisterCommit("B", 4)
		c = builder.RegisterCommit("C", 8)
		d = builder.RegisterCommit("D", 16)

		// Local opening at zero
		q1 = builder.LocalOpening("Q00", a)
		q2 = builder.LocalOpening("Q01", b)
		q3 = builder.LocalOpening("Q02", c)
		q4 = builder.LocalOpening("Q03", d)

		// Local opening but shifted by one
		q5 = builder.LocalOpening("Q10", column.Shift(a, 1))
		q6 = builder.LocalOpening("Q11", column.Shift(b, 1))
		q7 = builder.LocalOpening("Q12", column.Shift(c, 1))
		q8 = builder.LocalOpening("Q13", column.Shift(d, 1))

		// Local opening but shifted by negative one
		q9 = builder.LocalOpening("Q20", column.Shift(a, -1))
		q10 = builder.LocalOpening("Q21", column.Shift(b, -1))
		q11 = builder.LocalOpening("Q22", column.Shift(c, -1))
		q12 = builder.LocalOpening("Q23", column.Shift(d, -1))
	}

	logrus.Info("Starting compilation with Stitcher")
	comp := wizard.Compile(define, stitchsplit.Stitcher(4, 8))
	logrus.Info("Compilation completed")

	// After stitching compilation, check column statuses
	logrus.WithFields(logrus.Fields{
		"A": comp.Columns.Status("A").String(),
		"B": comp.Columns.Status("B").String(),
		"C": comp.Columns.Status("C").String(),
		"D": comp.Columns.Status("D").String(),
	}).Info("Column statuses after stitching")

	assert.Equal(t, column.Proof.String(), comp.Columns.Status("A").String())
	assert.Equal(t, column.Ignored.String(), comp.Columns.Status("B").String())
	assert.Equal(t, column.Committed.String(), comp.Columns.Status("C").String())
	assert.Equal(t, column.Committed.String(), comp.Columns.Status("D").String())

	// Check query ignored statuses
	logrus.WithFields(logrus.Fields{
		"Q00": comp.QueriesParams.IsIgnored(q1.ID),
		"Q01": comp.QueriesParams.IsIgnored(q2.ID),
		"Q02": comp.QueriesParams.IsIgnored(q3.ID),
		"Q03": comp.QueriesParams.IsIgnored(q4.ID),
		"Q10": comp.QueriesParams.IsIgnored(q5.ID),
		"Q11": comp.QueriesParams.IsIgnored(q6.ID),
		"Q12": comp.QueriesParams.IsIgnored(q7.ID),
		"Q13": comp.QueriesParams.IsIgnored(q8.ID),
		"Q20": comp.QueriesParams.IsIgnored(q9.ID),
		"Q21": comp.QueriesParams.IsIgnored(q10.ID),
		"Q22": comp.QueriesParams.IsIgnored(q11.ID),
		"Q23": comp.QueriesParams.IsIgnored(q12.ID),
	}).Info("Query ignored statuses after stitching")

	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q1.ID))
	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q2.ID))
	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q3.ID))
	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q4.ID))
	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q5.ID))
	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q6.ID))
	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q7.ID))
	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q8.ID))
	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q9.ID))
	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q10.ID))
	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q11.ID))
	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q12.ID))

	logrus.Info("Manually compiling with dummy compiler")
	dummy.Compile(comp)
	logrus.Info("Dummy compilation completed")

	logrus.Info("Generating proof")
	proof := wizard.Prove(comp, func(assi *wizard.ProverRuntime) {
		logrus.Info("Assigning columns")
		// Assigns all the columns
		assi.AssignColumn(a.GetColID(), smartvectors.ForTest(0, 1))
		logrus.WithField("A", smartvectors.ForTest(0, 1)).Debug("Assigned column A")
		assi.AssignColumn(b.GetColID(), smartvectors.ForTest(2, 3, 4, 5))
		logrus.WithField("B", smartvectors.ForTest(2, 3, 4, 5)).Debug("Assigned column B")
		assi.AssignColumn(c.GetColID(), smartvectors.ForTest(6, 7, 8, 9, 10, 11, 12, 13))
		logrus.WithField("C", smartvectors.ForTest(6, 7, 8, 9, 10, 11, 12, 13)).Debug("Assigned column C")
		assi.AssignColumn(d.GetColID(), smartvectors.ForTest(15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30))
		logrus.WithField("D", smartvectors.ForTest(15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30)).Debug("Assigned column D")

		logrus.Info("Assigning local points")
		// Assign the alleged results
		assi.AssignLocalPoint("Q00", field.NewElement(0))
		logrus.WithField("Q00", field.NewElement(0)).Debug("Assigned local point Q00")
		assi.AssignLocalPoint("Q01", field.NewElement(2))
		logrus.WithField("Q01", field.NewElement(2)).Debug("Assigned local point Q01")
		assi.AssignLocalPoint("Q02", field.NewElement(6))
		logrus.WithField("Q02", field.NewElement(6)).Debug("Assigned local point Q02")
		assi.AssignLocalPoint("Q03", field.NewElement(15))
		logrus.WithField("Q03", field.NewElement(15)).Debug("Assigned local point Q03")
		assi.AssignLocalPoint("Q10", field.NewElement(1))
		logrus.WithField("Q10", field.NewElement(1)).Debug("Assigned local point Q10")
		assi.AssignLocalPoint("Q11", field.NewElement(3))
		logrus.WithField("Q11", field.NewElement(3)).Debug("Assigned local point Q11")
		assi.AssignLocalPoint("Q12", field.NewElement(7))
		logrus.WithField("Q12", field.NewElement(7)).Debug("Assigned local point Q12")
		assi.AssignLocalPoint("Q13", field.NewElement(16))
		logrus.WithField("Q13", field.NewElement(16)).Debug("Assigned local point Q13")
		assi.AssignLocalPoint("Q20", field.NewElement(1))
		logrus.WithField("Q20", field.NewElement(1)).Debug("Assigned local point Q20")
		assi.AssignLocalPoint("Q21", field.NewElement(5))
		logrus.WithField("Q21", field.NewElement(5)).Debug("Assigned local point Q21")
		assi.AssignLocalPoint("Q22", field.NewElement(13))
		logrus.WithField("Q22", field.NewElement(13)).Debug("Assigned local point Q22")
		assi.AssignLocalPoint("Q23", field.NewElement(30))
		logrus.WithField("Q23", field.NewElement(30)).Debug("Assigned local point Q23")
	})
	logrus.Info("Proof generation completed")

	logrus.Info("Verifying proof")
	err := wizard.Verify(comp, proof)
	if err != nil {
		logrus.WithError(err).Error("Proof verification failed")
	} else {
		logrus.Info("Proof verification succeeded")
	}
	require.NoError(t, err)
}

func testStitcher(t *testing.T, minSize, maxSize int, gen func() (wizard.DefineFunc, wizard.MainProverStep)) {

	// Activates the logs for easy debugging
	logrus.SetLevel(logrus.TraceLevel)

	builder, prover := gen()
	comp := wizard.Compile(builder, stitchsplit.Stitcher(minSize, maxSize), dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err := wizard.Verify(comp, proof)

	require.NoError(t, err)

	for _, qName := range comp.QueriesNoParams.AllKeysAt(0) {

		switch q := comp.QueriesNoParams.Data(qName).(type) {

		case query.GlobalConstraint:
			board := q.Expression.Board()
			metadatas := board.ListVariableMetadata()
			metadataNames := []string{}
			for i := range metadatas {
				metadataNames = append(metadataNames, metadatas[i].String())
			}
			t.Logf("query %v - with metadata %v", q.ID, metadataNames)
		case query.LocalConstraint:
			board := q.Expression.Board()
			metadatas := board.ListVariableMetadata()
			metadataNames := []string{}
			for i := range metadatas {
				metadataNames = append(metadataNames, metadatas[i].String())
			}
			t.Logf("query %v - with metadata %v", q.ID, metadataNames)
		}
	}
}

func localOpening(n int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {
		definer := func(build *wizard.Builder) {
			P1 := build.RegisterCommit(P1, n)
			_ = build.LocalOpening("O1", P1)
			_ = build.LocalOpening("O2", column.Shift(P1, 3))
			_ = build.LocalOpening("O3", column.Shift(P1, 4))
			_ = build.LocalOpening("O4", column.Shift(P1, -1))
		}

		prover := func(run *wizard.ProverRuntime) {
			p1_ := make([]field.Element, n)
			for i := range p1_ {
				p1_[i].SetUint64(uint64(i))
			}
			p1 := smartvectors.NewRegular(p1_)
			run.AssignColumn(P1, p1)
			run.AssignLocalPoint("O1", p1.Get(0%n))
			run.AssignLocalPoint("O2", p1.Get(3%n))
			run.AssignLocalPoint("O3", p1.Get(4%n))
			run.AssignLocalPoint("O4", p1.Get(n-1))
		}

		return definer, prover
	}
}

func singlePolyFibo(size int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {
		builder := func(build *wizard.Builder) {
			// Number of rows
			P1 := build.RegisterCommit(P1, size) // overshadows P
			P2 := build.RegisterCommit(P2, size)

			// P(X) = P(X/w) + P(X/w^2)
			// i.e., P[i+2] = P[i+1] + P[i]
			expr1 := sym.Sub(
				sym.Add(column.Shift(P1, 1), P1),
				column.Shift(P1, 2))

			expr2 := sym.Sub(
				sym.Add(column.Shift(P2, 1), P2),
				column.Shift(P2, 2))

			_ = build.GlobalConstraint(GLOBAL1, expr1)
			_ = build.GlobalConstraint(GLOBAL2, expr2)
			// 	_ = build.LocalConstraint(LOCAL1, sym.Sub(P1, 1))
		}

		prover := func(run *wizard.ProverRuntime) {
			x := make([]field.Element, size)
			x[0].SetOne()
			x[1].SetOne()
			for i := 2; i < size; i++ {
				x[i].Add(&x[i-1], &x[i-2])
			}
			run.AssignColumn(P1, smartvectors.NewRegular(x))
			run.AssignColumn(P2, smartvectors.NewRegular(x))
		}

		return builder, prover
	}
}

func globalWithPeriodicSample(size, period, offset int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {

		builder := func(build *wizard.Builder) {
			P1 := build.RegisterCommit(P1, size) // overshadows P
			_ = build.GlobalConstraint(GLOBAL1, sym.Mul(P1, variables.NewPeriodicSample(period, offset)))
		}

		prover := func(run *wizard.ProverRuntime) {
			v := vector.Repeat(field.One(), size)
			for i := 0; i < size; i++ {
				if i%period == offset {
					v[i].SetZero()
				}
			}
			run.AssignColumn(P1, smartvectors.NewRegular(v))
		}

		return builder, prover
	}
}

func globalWithProofCols(size int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {

		builder := func(build *wizard.Builder) {
			P1 := build.RegisterCommit(P1, size)
			P2 := build.CompiledIOP.InsertProof(0, P2, size)
			P3 := build.RegisterCommit(P3, size) // overshadows P1
			_ = build.GlobalConstraint(GLOBAL1, sym.Mul(P1, P2, P3))
		}

		prover := func(run *wizard.ProverRuntime) {
			v1 := vector.Repeat(field.Zero(), size)
			v2 := vector.Repeat(field.Zero(), size)
			v3 := vector.Repeat(field.Zero(), size)
			for i := 0; i < size; i++ {
				if i%2 == 0 {
					v1[i].SetZero()
					v2[i].SetOne()
					v3[i].SetOne()
				} else {
					v1[i].SetOne()
					v2[i].SetZero()
					v3[i].SetOne()
				}
			}
			run.AssignColumn(P1, smartvectors.NewRegular(v1))
			run.AssignColumn(P2, smartvectors.NewRegular(v2))
			run.AssignColumn(P3, smartvectors.NewRegular(v3))
		}

		return builder, prover
	}
}

func localWithPeriodicSample(size, period, offset int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {

		builder := func(build *wizard.Builder) {
			P1 := build.RegisterCommit(P1, size) // overshadows P
			_ = build.LocalConstraint(LOCAL1, sym.Mul(P1, variables.NewPeriodicSample(period, offset)))
		}

		prover := func(run *wizard.ProverRuntime) {
			v := vector.Repeat(field.One(), size)
			for i := 0; i < size; i++ {
				if i%period == offset {
					v[i].SetZero()
				}
			}
			run.AssignColumn(P1, smartvectors.NewRegular(v))
		}

		return builder, prover
	}
}

func globalWithVerifColAndPeriodic(size, period, offset int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {

		builder := func(build *wizard.Builder) {
			P1 := build.RegisterCommit(P1, size)
			verifcol1 := verifiercol.NewFromAccessors(genAccessors(0, size), field.Zero(), size)
			verifcol2 := verifiercol.NewFromAccessors(genAccessors(2, size), field.Zero(), size)
			_ = build.GlobalConstraint(LOCAL1,
				sym.Sub(

					sym.Mul(sym.Sub(1, P1),
						verifcol2),

					sym.Mul(variables.NewPeriodicSample(period, offset),
						sym.Add(2, verifcol1))),
			)
		}

		prover := func(run *wizard.ProverRuntime) {
			v := vector.Repeat(field.One(), size)
			for i := 0; i < size; i++ {
				if i%period == offset {
					v[i].SetZero()
				}
			}
			run.AssignColumn(P1, smartvectors.NewRegular(v))
		}

		return builder, prover
	}
}

func genAccessors(start, size int) (res []ifaces.Accessor) {
	for i := start; i < size+start; i++ {
		t := accessors.NewConstant(field.NewElement(uint64(i)))
		res = append(res, t)
	}
	return res
}
