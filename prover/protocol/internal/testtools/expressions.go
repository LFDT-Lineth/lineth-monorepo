package testtools

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
)

var (
	// _comp is just a placeholder to allow [columnA, columnB, columnC]
	// to have a space to be defined in. Without it, it would be impossible
	// to do anything with the columns: shifting or calling String() would
	// panic.
	_comp    = wizard.NewCompiledIOP()
	columnA  = _comp.InsertColumn(0, "A", 8, column.Committed, false)
	columnB  = _comp.InsertColumn(0, "B", 8, column.Committed, false)
	columnC  = _comp.InsertColumn(0, "C", 8, column.Committed, false)
	coinCoin = coin.NewInfo("coinName", coin.FieldExt, 1)
)

// ExpressionTestcase can be used to generate a global constraint
// or a local constraint testcase.
type ExpressionTestcase struct {

	// NameStr is the name of the testcase
	NameStr string

	// Expr is an expression that can be dependent on either
	// columnA, columnB, ... columnE or coinCoin. The coinstraints
	// is expected to work for random values of the coin.
	//
	// The coins will be manually given a random value from the
	// prover.
	Expr *sym.Expression

	// Columns is the list of the columns to assign and their assignment
	Columns map[ifaces.ColID]smartvectors.SmartVector

	// IsLocalConstraint is true if the testcase is a local constraint
	// and false if it is a global constraint.
	IsLocalConstraint bool

	// Query is the query that is used to generate the testcase.
	Query ifaces.Query

	// MustFailt returns an error if the testcase must fail.
	MustFailFlag bool
}

// ListOfGlobalTestcasePositive is a list of global constraints testcases
// that are expected to pass.
var ListOfGlobalTestcasePositive = []*ExpressionTestcase{

	{
		NameStr: "positive/fibonacci",
		Expr: sym.Sub(
			columnA,
			column.Shift(columnA, -1),
			column.Shift(columnA, -2),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(1, 1, 2, 3, 5, 8, 13, 21),
		},
	},

	{
		NameStr: "positive/fibonacci-over-extensions",
		Expr: sym.Sub(
			columnA,
			column.Shift(columnA, -1),
			column.Shift(columnA, -2),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTestFromQuads(
				1, 2, 5, 13,
				1, 3, 8, 21,
				2, 5, 13, 34,
				3, 8, 21, 55,
				5, 13, 34, 89,
				8, 21, 55, 144,
				13, 34, 89, 233,
				21, 55, 144, 377,
			),
		},
	},

	{
		NameStr: "positive/geometric-progression",
		Expr: sym.Sub(
			columnA,
			sym.Mul(
				2,
				column.Shift(columnA, -1),
			),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(1, 2, 4, 8, 16, 32, 64, 128),
		},
	},
	{
		NameStr: "positive/geometric-progression-over-extensions",
		Expr: sym.Sub(
			columnA,
			sym.Mul(
				2,
				column.Shift(columnA, -1),
			),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTestFromQuads(
				1, 2, 3, 4,
				2, 4, 6, 8,
				4, 8, 12, 16,
				8, 16, 24, 32,
				16, 32, 48, 64,
				32, 64, 96, 128,
				64, 128, 192, 256,
				128, 256, 384, 512,
			),
		},
	},

	{
		NameStr: "positive/random-linear-combination",
		Expr: sym.NewPolyEval(
			sym.NewVariable(coinCoin),
			[]*sym.Expression{
				sym.NewVariable(columnA),
				sym.NewVariable(columnB),
				sym.NewVariable(columnC),
			},
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.NewConstant(field.Zero(), 16),
			"B": smartvectors.NewConstant(field.Zero(), 16),
			"C": smartvectors.NewConstant(field.Zero(), 16),
		},
	},
	{
		NameStr: "positive/random-linear-combination-over-extensions",
		Expr: sym.NewPolyEval(
			sym.NewVariable(coinCoin),
			[]*sym.Expression{
				sym.NewVariable(columnA),
				sym.NewVariable(columnB),
				sym.NewVariable(columnC),
			},
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.NewConstant(field.Zero(), 16),
			"B": smartvectors.NewConstantExt(fext.Zero(), 16),
			"C": smartvectors.NewConstant(field.Zero(), 16),
		},
	},

	{
		NameStr: "positive/conditional-counter",
		Expr: sym.Sub(
			columnA,
			sym.Mul(
				sym.Sub(1, columnB),
				column.Shift(columnA, -1),
			),
			sym.Mul(
				columnB,
				sym.Add(column.Shift(columnA, -1), 1),
			),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(0, 1, 1, 1, 2, 3, 3, 3),
			"B": smartvectors.ForTest(0, 1, 0, 0, 1, 1, 0, 0),
		},
	},
	{
		NameStr: "positive/conditional-counter-extensions",
		// A = A_prev + B
		Expr: sym.Sub(
			columnA,
			sym.Mul(
				sym.Sub(1, columnB),
				column.Shift(columnA, -1),
			),
			sym.Mul(
				columnB,
				sym.Add(column.Shift(columnA, -1), 1),
			),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTestFromQuads(
				0, 3, 5, 10,
				1, 5, 8, 17,
				1, 5, 8, 17,
				1, 5, 6, 22,
				2, 6, 14, 10,
				3, 6, 15, 11,
				3, 8, 15, 11,
				3, 8, 9, 3,
			),
			"B": smartvectors.ForTestFromQuads(
				0, 3, 5, 10,
				1, 2, 3, 7,
				0, 0, 0, 0,
				0, 0, -2, 5,
				1, 1, 8, -12,
				1, 0, 1, 1,
				0, 2, 0, 0,
				0, 0, -6, -8,
			),
		},
	},

	{
		NameStr: "positive/pythagorean-triplet",
		Expr: sym.Sub(
			sym.Mul(columnA, columnA),
			sym.Mul(columnB, columnB),
			sym.Mul(columnC, columnC),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(0, 5, 1, 17, 5, 13, 0, 0),
			"B": smartvectors.ForTest(0, 3, 0, 15, 4, 5, 0, 0),
			"C": smartvectors.ForTest(0, 4, 1, 8, 3, 12, 0, 0),
		},
	},
	{
		NameStr: "positive/pythagorean-triplet-extensions",
		// A^2 = B^2 + C^2
		Expr: sym.Sub(
			sym.Mul(columnA, columnA),
			sym.Mul(columnB, columnB),
			sym.Mul(columnC, columnC),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTestFromQuads(
				0, 0, 0, 29,
				0, 5, 0, 5,
				1, 0, 1, 1,
				17, 0, 17, 0,
				5, 0, 5, 0,
				0, 13, 0, 13,
				1, 1, 1, 1,
				0, 0, 0, 0,
			),
			"B": smartvectors.ForTestFromQuads(
				0, 0, 0, 20,
				0, 3, 0, 3,
				0, 0, 0, 0,
				15, 0, 15, 0,
				4, 0, 4, 0,
				0, 5, 0, 5,
				0, 0, 0, 0,
				0, 0, 0, 0,
			),
			"C": smartvectors.ForTestFromQuads(
				0, 0, 0, 21,
				0, 4, 0, 4,
				1, 0, 1, 1,
				8, 0, 8, 0,
				3, 0, 3, 0,
				0, 12, 0, 12,
				1, 1, 1, 1,
				0, 0, 0, 0,
			),
		},
	},
}

// ListOfLocalTestcasePositive is a list of global constraints testcases
// that are expected to pass. They are essentially, the global constraints
// written in sych a way that the corresponding local-constraint works.
var ListOfLocalTestcasePositive = []*ExpressionTestcase{

	{
		NameStr: "positive/fibonacci",
		Expr: sym.Sub(
			column.Shift(columnA, 2),
			column.Shift(columnA, 1),
			columnA,
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(1, 1, 2, 3, 5, 8, 13, 21),
		},
		IsLocalConstraint: true,
	},

	{
		NameStr: "positive/geometric-progression",
		Expr: sym.Sub(
			column.Shift(columnA, 1),
			sym.Mul(2, columnA),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(1, 2, 4, 8, 16, 32, 64, 128),
		},
		IsLocalConstraint: true,
	},

	{
		NameStr: "positive/random-linear-combination",
		Expr: sym.NewPolyEval(
			sym.NewVariable(coinCoin),
			[]*sym.Expression{
				sym.NewVariable(columnA),
				sym.NewVariable(columnB),
				sym.NewVariable(columnC),
			},
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.NewConstant(field.Zero(), 16),
			"B": smartvectors.NewConstant(field.Zero(), 16),
			"C": smartvectors.NewConstant(field.Zero(), 16),
		},
		IsLocalConstraint: true,
	},

	{
		NameStr: "positive/conditional-counter",
		Expr: sym.Sub(
			column.Shift(columnA, 1),
			sym.Mul(
				column.Shift(columnB, 1),
				columnA,
			),
			sym.Mul(
				sym.Sub(1, column.Shift(columnB, 1)),
				sym.Add(columnA, 1),
			),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(0, 1, 1, 1, 2, 3, 3, 3),
			"B": smartvectors.ForTest(0, 1, 0, 0, 1, 1, 0, 0),
		},
		IsLocalConstraint: true,
	},
}

// ListOfGlobalTestcaseNegative is a list of global constraints testcases
// that are expected to pass.
var ListOfGlobalTestcaseNegative = []*ExpressionTestcase{

	{
		NameStr: "negative/fibonacci/wrong-last-value",
		Expr: sym.Sub(
			columnA,
			column.Shift(columnA, -1),
			column.Shift(columnA, -2),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(1, 1, 2, 3, 5, 8, 13, 22),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/fibonacci/wrong-first-value",
		Expr: sym.Sub(
			columnA,
			column.Shift(columnA, -1),
			column.Shift(columnA, -2),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(0, 1, 2, 3, 5, 8, 13, 21),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/fibonacci/full-random",
		Expr: sym.Sub(
			columnA,
			column.Shift(columnA, -1),
			column.Shift(columnA, -2),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": RandomVec(8),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/geometric-progression/wrong-coeff",
		Expr: sym.Sub(
			columnA,
			sym.Mul(
				2,
				column.Shift(columnA, -1),
			),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(1, 3, 9, 27, 81, 243, 729, 2187),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/geometric-progression/wrong-first-value",
		Expr: sym.Sub(
			columnA,
			sym.Mul(
				2,
				column.Shift(columnA, -1),
			),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(2, 2, 4, 8, 16, 32, 64, 128),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/random-linear-combination/first-value-is-bad",
		Expr: sym.NewPolyEval(
			sym.NewVariable(coinCoin),
			[]*sym.Expression{
				sym.NewVariable(columnA),
				sym.NewVariable(columnB),
				sym.NewVariable(columnC),
			},
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0),
			"B": smartvectors.NewConstant(field.Zero(), 16),
			"C": smartvectors.NewConstant(field.Zero(), 16),
		},
		MustFailFlag: true,
	},

	{
		NameStr: "negative/conditional-counter/skip-2",
		Expr: sym.Sub(
			columnA,
			sym.Mul(
				columnB,
				column.Shift(columnA, -1),
			),
			sym.Mul(
				sym.Sub(1, columnB),
				sym.Add(column.Shift(columnA, -1), 1),
			),
		),
		Columns: map[ifaces.ColID]smartvectors.SmartVector{
			"A": smartvectors.ForTest(0, 1, 1, 1, 1, 3, 3, 3),
			"B": smartvectors.ForTest(0, 1, 0, 0, 1, 1, 0, 0),
		},
		MustFailFlag: true,
	},
}

// Define registers the necessary columns and the global/local constraint
// needed to instantiate the testcase.
func (etc *ExpressionTestcase) Define(comp *wizard.CompiledIOP) {

	var (
		board          = etc.Expr.Board()
		metas          = board.ListVariableMetadata()
		translationMap = map[ifaces.ColID]ifaces.Column{}
		round          = 0
	)

	for _, meta := range metas {
		switch m := meta.(type) {

		case ifaces.Column:
			// the root col is more of less a column that does not belong
			// to the compiled IOP. The real one is the one has a prefix.
			rootCol := column.RootParents(m)
			realName := formatName[ifaces.ColID]("Column", etc.NameStr, rootCol.GetColID())

			// Save the registration if the column has already been reached.
			if comp.Columns.Exists(realName) {
				continue
			}

			size := etc.Columns[m.GetColID()].Len()
			col := comp.InsertCommit(0, realName, size, smartvectors.IsBase(etc.Columns[m.GetColID()]))
			translationMap[rootCol.GetColID()] = col

		case coin.Info:
			// the coin is inserted at round 1. Since there is only one coin
			// we already know which one is needed.
			if !comp.Coins.Exists(coinCoin.Name) {
				comp.InsertCoin(1, coinCoin.Name, coinCoin.Type)
			}
			round = max(round, 1)

		default:
			panic("unknown type")
		}
	}

	// This reconstruct the expression by replacing all the columns by those who
	// are actually declared. The coin does not need to be readded since it does
	// not have private fields linking to the compiled-IOP.
	expr := etc.Expr.ReconstructBottomUp(func(e *sym.Expression, children []*sym.Expression) (new *sym.Expression) {

		vari, isVar := e.Operator.(sym.Variable)
		if !isVar {
			return e.SameWithNewChildren(children)
		}

		if coin, isCoin := vari.Metadata.(coin.Info); isCoin {
			return sym.NewVariable(coin)
		}

		col, isCol := vari.Metadata.(ifaces.Column)
		if !isCol {
			return e.SameWithNewChildren(children)
		}

		switch c := col.(type) {
		case column.Natural:
			return sym.NewVariable(translationMap[c.GetColID()])
		case column.Shifted:
			return sym.NewVariable(column.Shift(translationMap[c.Parent.GetColID()], c.Offset))
		default:
			return e
		}
	})

	if etc.IsLocalConstraint {
		etc.Query = comp.InsertLocal(
			round,
			formatName[ifaces.QueryID]("Local", etc.NameStr, "Query"),
			expr,
		)
		return
	}

	etc.Query = comp.InsertGlobal(
		round,
		formatName[ifaces.QueryID]("Global", etc.NameStr, "Query"),
		expr,
	)
}

func (etc *ExpressionTestcase) Assign(run *wizard.ProverRuntime) {
	for colID, vector := range etc.Columns {
		run.AssignColumn(formatName[ifaces.ColID]("Column", etc.NameStr, colID), vector)
	}
}

func (etc *ExpressionTestcase) MustFail() bool {
	return etc.MustFailFlag
}

func (etc *ExpressionTestcase) Name() string {
	return etc.NameStr
}
