package wioptest

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// ActionFunc is a prover action that runs a function.
type ActionFunc func(rt *wiop.Runtime)

// Run implements [wiop.ProverAction].
func (f ActionFunc) Run(rt *wiop.Runtime) { f(rt) }

// ErrMissingRow is the error that [RecoverMissingRow] sets: the prover holds a row that a lookup reads
// and its table does not have, so it cannot build the lookup's witness and no proof exists to verify.
var ErrMissingRow = errors.New("a row of a lookup is in none of its table")

// RecoverMissingRow, called by a defer in a test that proves a system, sets *err to [ErrMissingRow],
// wrapping the panic, if the prover panicked on a lookup row that has no match in the table. Any other
// panic fails the test.
func RecoverMissingRow(tb testing.TB, err *error) {
	tb.Helper()
	if r := recover(); r != nil {
		if !strings.Contains(fmt.Sprint(r), "has no match in the lookup table") {
			tb.Fatalf("the prover panicked: %v", r)
		}
		*err = fmt.Errorf("%w: %v", ErrMissingRow, r)
	}
}

// NewE6Cells declares in round the six base cells of the coordinates of an element of E₆, named
// name-0, …, name-5 under ctx.
func NewE6Cells(ctx *wiop.ContextFrame, round *wiop.Round, name string) (cells [6]*wiop.Cell) {
	for k := range cells {
		cells[k] = round.NewCell(ctx.Childf("%s-%d", name, k), false)
	}
	return cells
}

// AssignE6 assigns the coordinates of e to the six base cells.
func AssignE6(rt *wiop.Runtime, cells [6]*wiop.Cell, e field.Ext) {
	for k, coordinate := range e.AppendBase(nil) {
		rt.AssignCell(cells[k], field.ElemFromBase(coordinate))
	}
}
