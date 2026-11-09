package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/assert"
)

// WalkLeaves descends into each distinct node once, and a visited set shared
// between calls skips the nodes an earlier call walked.
func TestWalkLeaves_SharedVisited(t *testing.T) {
	sys := wiop.NewSystemf("walk-leaves")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 8, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), r0).View()
	b := mod.NewColumn(sys.Context.Childf("b"), r0).View()
	c := mod.NewColumn(sys.Context.Childf("c"), r0).View()

	shared := doublingDAG(wiop.Mul(a, b), 64)
	var got []wiop.Expression
	wiop.WalkLeaves(shared, nil, func(e wiop.Expression) { got = append(got, e) })
	assert.Equal(t, []wiop.Expression{a, b}, got, "a depth-64 DAG over a·b must report a and b once each")

	visited := make(map[*wiop.ArithmeticOperation]struct{})
	got = nil
	wiop.WalkLeaves(wiop.Add(shared, a), visited, func(e wiop.Expression) { got = append(got, e) })
	wiop.WalkLeaves(wiop.Add(shared, c), visited, func(e wiop.Expression) { got = append(got, e) })
	assert.Equal(t, []wiop.Expression{a, b, a, c}, got,
		"the second walk must skip the shared node the first one walked, and still report c")
}
