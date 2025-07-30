package stitchsplit

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	P1      ifaces.ColID   = "P1"
	GLOBAL1 ifaces.QueryID = "GLOBAL1"
	LOCAL1  ifaces.QueryID = "LOCAL1"
)

func TestSplitterWithFixedPointOpening(t *testing.T) {
	testSplitter(t, 16, fixedPointOpening)
}

func TestSplitterFibo(t *testing.T) {
	testSplitter(t, 16, singlePolyFibo(4))
	testSplitter(t, 4, singlePolyFibo(8))
	testSplitter(t, 4, singlePolyFibo(16))
}

func TestSplitterGlobalWithPeriodicSample(t *testing.T) {
	testSplitter(t, 64, globalWithPeriodicSample(16, 8, 0))
	testSplitter(t, 64, globalWithPeriodicSample(256, 8, 1))
	testSplitter(t, 64, globalWithPeriodicSample(256, 8, 7))
}

func TestSplitterLocalWithPeriodicSample(t *testing.T) {
	testSplitter(t, 64, localWithPeriodicSample(256, 8, 0))
	testSplitter(t, 64, localWithPeriodicSample(256, 8, 1))
	testSplitter(t, 64, localWithPeriodicSample(256, 8, 7))
}

func fixedPointOpening() (wizard.DefineFunc, wizard.MainProverStep) {
	n := 1 << 6
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
		run.AssignLocalPoint("O1", p1.Get(0))
		run.AssignLocalPoint("O2", p1.Get(3))
		run.AssignLocalPoint("O3", p1.Get(4))
		run.AssignLocalPoint("O4", p1.Get(n-1))
	}

	return definer, prover
}

func singlePolyFibo(size int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {
		builder := func(build *wizard.Builder) {
			// Number of rows
			P1 := build.RegisterCommit(P1, size) // overshadows P

			// P(X) = P(X/w) + P(X/w^2)
			expr := sym.Sub(
				P1,
				column.Shift(P1, -1),
				column.Shift(P1, -2),
			)

			_ = build.GlobalConstraint(GLOBAL1, expr)
			_ = build.LocalConstraint(LOCAL1, sym.Sub(P1, 1))
		}

		prover := func(run *wizard.ProverRuntime) {
			x := make([]field.Element, size)
			x[0].SetOne()
			x[1].SetOne()
			for i := 2; i < size; i++ {
				x[i].Add(&x[i-1], &x[i-2])
			}
			run.AssignColumn(P1, smartvectors.NewRegular(x))
		}

		return builder, prover
	}
}

func globalWithPeriodicSample(size, period, offset int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {

		builder := func(build *wizard.Builder) {
			P1 := build.RegisterCommit(P1, size) // overshadows P
			_ = build.GlobalConstraint(GLOBAL1, variables.NewPeriodicSample(period, offset).Mul(ifaces.ColumnAsVariable(P1)))
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

func localWithPeriodicSample(size, period, offset int) func() (wizard.DefineFunc, wizard.MainProverStep) {
	return func() (wizard.DefineFunc, wizard.MainProverStep) {

		builder := func(build *wizard.Builder) {
			P1 := build.RegisterCommit(P1, size) // overshadows P
			_ = build.LocalConstraint(GLOBAL1, variables.NewPeriodicSample(period, offset).Mul(ifaces.ColumnAsVariable(P1)))
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

func testSplitter(t *testing.T, splitSize int, gen func() (wizard.DefineFunc, wizard.MainProverStep)) {

	// Activates the logs for easy debugging
	logrus.SetLevel(logrus.TraceLevel)

	builder, prover := gen()
	comp := wizard.Compile(builder, Stitcher(splitSize/2, splitSize), Splitter(splitSize), dummy.Compile)
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

func TestLocalEvalWithStatus(t *testing.T) {

	var b, c ifaces.Column
	var q2, q3, q6, q7, q10, q11 query.LocalOpening

	define := func(builder *wizard.Builder) {
		// declare columns of different sizes
		b = builder.RegisterCommit("B", 4)
		c = builder.RegisterCommit("C", 8)

		// Local opening at zero
		q2 = builder.LocalOpening("Q01", b)
		q3 = builder.LocalOpening("Q02", c)

		// Local opening at but shifted by one
		q6 = builder.LocalOpening("Q11", column.Shift(b, 1))
		q7 = builder.LocalOpening("Q12", column.Shift(c, 1))

		// Local opening  but shifted by -1
		q10 = builder.LocalOpening("Q21", column.Shift(b, -1))
		q11 = builder.LocalOpening("Q22", column.Shift(c, -1))

	}

	comp := wizard.Compile(define, Splitter(4))

	//after splitting-compilation we expect that the eligible columns and their relevant queries be ignored
	assert.Equal(t, column.Committed.String(), comp.Columns.Status("B").String())
	assert.Equal(t, column.Ignored.String(), comp.Columns.Status("C").String())

	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q2.ID))
	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q3.ID))

	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q6.ID))
	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q7.ID))

	assert.Equal(t, false, comp.QueriesParams.IsIgnored(q10.ID))
	assert.Equal(t, true, comp.QueriesParams.IsIgnored(q11.ID))

	// manually compiles the comp
	dummy.Compile(comp)

	proof := wizard.Prove(comp, func(assi *wizard.ProverRuntime) {
		// Assigns all the columns
		assi.AssignColumn(b.GetColID(), smartvectors.ForTest(2, 3, 4, 5))
		assi.AssignColumn(c.GetColID(), smartvectors.ForTest(6, 7, 8, 9, 10, 11, 12, 13))

		// And the alleged results
		assi.AssignLocalPoint("Q01", field.NewElement(2))
		assi.AssignLocalPoint("Q02", field.NewElement(6))

		assi.AssignLocalPoint("Q11", field.NewElement(3))
		assi.AssignLocalPoint("Q12", field.NewElement(7))

		assi.AssignLocalPoint("Q21", field.NewElement(5))
		assi.AssignLocalPoint("Q22", field.NewElement(13))

	})

	err := wizard.Verify(comp, proof)
	require.NoError(t, err)

}
