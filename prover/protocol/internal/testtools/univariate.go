package testtools

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/coin"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

// UnivariateTestcase represents a univariate relationship and and its assignment.
// The instances are used to generate testcases.
type UnivariateTestcase struct {
	NameStr string
	Polys   []smartvectors.SmartVector
	QueryXs []fext.Element
	// QueryPols[i] indexes the polynomials in [Polys] that are queried. In the i-th query.
	QueryPols [][]int
	// This parameter is optionally set. If not set, the test case computes the
	// correct value.
	QueryYs [][]fext.Element
	// Round indicates the round definition of [Polys]. -1 indicates that the column
	// is precomputed. If the field is empty, the testcase assumes that the all
	// the columns are for round zero.
	RoundOfPolys []int
}

// ListOfUnivariateTestcasesPositive lists standard univariate testcases
// that are supposed to pass.
var ListOfUnivariateTestcasesPositive = []*UnivariateTestcase{
	{
		NameStr: "constant-poly",
		Polys: []smartvectors.SmartVector{
			smartvectors.ForTest(10, 10, 10, 10, 10, 10, 10, 10),
		},
		QueryXs: []fext.Element{
			fext.PseudoRand(rng),
		},
		QueryPols: [][]int{
			{0},
		},
	},
	{
		NameStr: "one-poly-one-point-non-simple-values",
		Polys: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 0, 0, 0, 0, 1, 0, 0),
		},
		QueryXs: []fext.Element{
			fext.Zero(),
		},
		QueryPols: [][]int{
			{0},
		},
	},
	{
		NameStr: "one-poly-one-point",
		Polys: []smartvectors.SmartVector{
			RandomVec(8),
		},
		QueryXs: []fext.Element{
			fext.Zero(),
		},
		QueryPols: [][]int{
			{0},
		},
	},
	{
		NameStr: "one-poly-one-point-precomputed",
		Polys: []smartvectors.SmartVector{
			RandomVec(8),
		},
		QueryXs: []fext.Element{
			fext.Zero(),
		},
		QueryPols: [][]int{
			{0},
		},
		RoundOfPolys: []int{-1},
	},
	{
		NameStr: "one-poly-one-point-round-k",
		Polys: []smartvectors.SmartVector{
			RandomVec(8),
		},
		QueryXs: []fext.Element{
			fext.Zero(),
		},
		QueryPols: [][]int{
			{0},
		},
		RoundOfPolys: []int{3},
	},
	{
		NameStr: "two-poly-one-point-same-simple-values",
		Polys: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 0, 0, 0, 0, 1, 0, 0),
			smartvectors.ForTest(0, 0, 0, 0, 0, 1, 0, 0),
		},
		QueryXs: []fext.Element{
			fext.Zero(),
		},
		QueryPols: [][]int{
			{0, 1},
		},
	},
	{
		NameStr: "two-poly-one-point-same-simple-values-one-precomputed",
		Polys: []smartvectors.SmartVector{
			smartvectors.ForTest(0, 0, 0, 0, 0, 1, 0, 0),
			smartvectors.ForTest(0, 0, 0, 0, 0, 1, 0, 0),
		},
		QueryXs: []fext.Element{
			fext.Zero(),
		},
		QueryPols: [][]int{
			{0, 1},
		},
		RoundOfPolys: []int{-1, 0},
	},
	{
		NameStr: "two-poly-one-point",
		Polys: []smartvectors.SmartVector{
			RandomVec(8),
			RandomVec(8),
		},
		QueryXs: []fext.Element{
			fext.PseudoRand(rng),
		},
		QueryPols: [][]int{
			{0, 1},
		},
	},
	{
		NameStr: "one-poly-two-points",
		Polys: []smartvectors.SmartVector{
			RandomVec(8),
		},
		QueryXs: []fext.Element{
			fext.PseudoRand(rng),
			fext.PseudoRand(rng),
		},
		QueryPols: [][]int{
			{0},
			{0},
		},
	},
	{
		NameStr: "two-poly-two-points",
		Polys: []smartvectors.SmartVector{
			RandomVec(8),
			RandomVec(8),
		},
		QueryXs: []fext.Element{
			fext.PseudoRand(rng),
			fext.PseudoRand(rng),
		},
		QueryPols: [][]int{
			{0, 1},
			{0, 1},
		},
	},
	{
		NameStr: "complex-multi-round",
		Polys: []smartvectors.SmartVector{
			RandomVec(8),
			RandomVec(8),
			RandomVec(8),
			RandomVec(8),
			RandomVec(8),
			RandomVec(8),
			RandomVec(8),
		},
		QueryXs: []fext.Element{
			fext.PseudoRand(rng),
			fext.PseudoRand(rng),
			fext.PseudoRand(rng),
			fext.PseudoRand(rng),
			fext.PseudoRand(rng),
		},
		QueryPols: [][]int{
			{0, 1, 2, 3},
			{2, 3, 4, 5},
			{0, 1, 4},
			{0},
			{6},
		},
		RoundOfPolys: []int{-1, 0, 1, 2, 2, 3, 3},
	},
}

func (u *UnivariateTestcase) Name() string {
	return u.NameStr
}

func (u *UnivariateTestcase) MustFail() bool { return false }

func (u *UnivariateTestcase) Define(comp *wizard.CompiledIOP) {

	var (
		polys    = make([]ifaces.Column, len(u.Polys))
		maxRound = 0
	)

	for i := range polys {

		var (
			name  = formatName[ifaces.ColID]("Univariate", u.NameStr, "Poly", i)
			round = 0
		)

		if len(u.RoundOfPolys) > i {
			round = u.RoundOfPolys[i]
		}

		if round < 0 {
			polys[i] = comp.InsertPrecomputed(name, u.Polys[i])
			continue
		}
		maxRound = max(maxRound, round)
		polys[i] = comp.InsertCommit(round, name, u.Polys[i].Len(), smartvectors.IsBase(u.Polys[i]))

		if round > 0 {
			comp.RegisterProverAction(round, autoAssignColumn{
				col: polys[i],
				sv:  u.Polys[i],
			})
		}
	}

	for round := 1; round <= maxRound; round++ {
		_ = comp.InsertCoin(round, formatName[coin.Name]("Univariate", u.NameStr, "Coin", round), coin.FieldExt)
	}

	for i := range u.QueryXs {

		queryPols := make([]ifaces.Column, len(u.QueryPols[i]))
		for j := range queryPols {
			queryPols[j] = polys[u.QueryPols[i][j]]
		}

		comp.InsertUnivariate(
			maxRound,
			formatName[ifaces.QueryID]("Univariate", u.NameStr, "Query", i),
			queryPols,
		)
		if maxRound > 0 {
			comp.RegisterProverAction(maxRound, assignUnivariatePA{u, i})
		}
	}
}

func (u *UnivariateTestcase) Assign(run *wizard.ProverRuntime) {

	maxRound := 0
	if len(u.RoundOfPolys) > 0 {
		for _, r := range u.RoundOfPolys {
			maxRound = max(maxRound, r)
		}
	}
	for i := range u.Polys {

		round := 0
		if len(u.RoundOfPolys) > i {
			round = u.RoundOfPolys[i]
		}

		// The columns with a round > 0 are assigned via a registered prover
		// action. And those with a round < 0 are precomputed. So we only
		// need to assign the columns with a round == 0.
		if round != 0 {
			continue
		}

		run.AssignColumn(
			formatName[ifaces.ColID]("Univariate", u.NameStr, "Poly", i),
			u.Polys[i],
			wizard.DisableAssignmentSizeReduction,
		)
	}

	for i := range u.QueryXs {
		if maxRound == 0 {
			u.assignUnivariate(run, i)
		}
	}
}

func (u *UnivariateTestcase) assignUnivariate(run *wizard.ProverRuntime, i int) {

	var (
		name = formatName[ifaces.QueryID]("Univariate", u.NameStr, "Query", i)
		ys   = make([]fext.Element, len(u.QueryPols[i]))
		x    = u.QueryXs[i]
		q    = run.Spec.QueriesParams.Data(name).(query.UnivariateEval)
	)

	if len(u.QueryYs) > 0 {
		run.AssignUnivariateExt(name, x, u.QueryYs[i]...)
		return
	}

	for j := range q.Pols {
		p := q.Pols[j].GetColAssignment(run)
		ys[j] = smartvectors.EvaluateBasePolyLagrange(p, x)
	}

	run.AssignUnivariateExt(q.QueryID, x, ys...)
}

// assignUnivariatePA is a ProverAction to assign a univariate query.
type assignUnivariatePA struct {
	u *UnivariateTestcase
	i int
}

func (pa assignUnivariatePA) Run(run *wizard.ProverRuntime) { pa.u.assignUnivariate(run, pa.i) }
