package gkrverifier_test

import (
	"fmt"
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/gkrverifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/stretchr/testify/require"
)

// e6System is a module of random rows x, y and a base element k, with the columns that
// the operations of [gkrverifier.E6] must give: x + y, x − y, xy and kx. The module constrains them.
type e6System struct {
	sys *wiop.System
	// the columns, six per element of E₆, by name, and k
	columns map[string][]*wiop.Column
	// the honest values of the columns, by name
	values map[string][][]field.Element
}

const e6Rows = 16

// e6Results are the columns that hold the results of the operations.
const (
	e6Sum        = "sum"
	e6Difference = "difference"
	e6Product    = "product"
	e6Scaled     = "scaled"
)

var e6Results = []string{e6Sum, e6Difference, e6Product, e6Scaled}

func newE6System() *e6System {
	rng := rand.New(rand.NewPCG(1, 2))
	randomE6 := func() (e extensions.E6) {
		coordinates := make([]field.Element, 6)
		for i := range coordinates {
			coordinates[i].SetUint64(uint64(rng.Uint32()))
		}
		return *e.SetBase(coordinates)
	}

	s := &e6System{sys: wiop.NewSystemf("e6"), columns: map[string][]*wiop.Column{}, values: map[string][][]field.Element{}}
	round := s.sys.NewRound()
	module := s.sys.NewSizedModule(s.sys.Context.Childf("module"), e6Rows, wiop.PaddingDirectionNone)

	declare := func(name string, width int) {
		for i := range width {
			s.columns[name] = append(s.columns[name], module.NewColumn(s.sys.Context.Childf("%s-%d", name, i), round))
		}
		s.values[name] = make([][]field.Element, width)
		for i := range width {
			s.values[name][i] = make([]field.Element, e6Rows)
		}
	}
	for _, name := range append([]string{"x", "y"}, e6Results...) {
		declare(name, 6)
	}
	declare("k", 1)

	for row := range e6Rows {
		x, y, k := randomE6(), randomE6(), randomE6().B0.A0
		var sum, difference, product, scaled extensions.E6
		sum.Add(&x, &y)
		difference.Sub(&x, &y)
		product.Mul(&x, &y)
		scaled.MulByElement(&x, &k)

		set := func(name string, e *extensions.E6) {
			for i, c := range e.AppendBase(nil) {
				s.values[name][i][row] = c
			}
		}
		set("x", &x)
		set("y", &y)
		set(e6Sum, &sum)
		set(e6Difference, &difference)
		set(e6Product, &product)
		set(e6Scaled, &scaled)
		s.values["k"][0][row] = k
	}

	view := func(name string) gkrverifier.E6 {
		var e gkrverifier.E6
		for i, c := range s.columns[name] {
			e[i] = c.View()
		}
		return e
	}
	x, y, k := view("x"), view("y"), s.columns["k"][0].View()
	for name, diffs := range map[string][6]wiop.Expression{
		e6Sum:        view(e6Sum).Equal(x.Add(y)),
		e6Difference: view(e6Difference).Add(y).Equal(x),
		e6Product:    view(e6Product).Equal(x.Mul(y)),
		e6Scaled:     view(e6Scaled).Equal(x.MulByBase(k)),
	} {
		for i, diff := range diffs {
			module.NewVanishingManual(s.sys.Context.Childf("%s-%d-constraint", name, i), diff)
		}
	}
	return s
}

// assign assigns the honest values, except that the coordinate of the column name is changed by one
// if changed >= 0.
func (s *e6System) assign(name string, changed int) func(rt *wiop.Runtime) {
	return func(rt *wiop.Runtime) {
		for column, coordinates := range s.columns {
			for i, col := range coordinates {
				values := append([]field.Element(nil), s.values[column][i]...)
				if column == name && i == changed {
					var one field.Element
					one.SetOne()
					values[3].Add(&values[3], &one)
				}
				rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(values)})
			}
		}
	}
}

// TestE6 checks the operations of E6 against gnark-crypto's extensions.E6: a module whose rows hold
// x, y and their sum, difference and product, and the product of x by a base element, constrained with
// the operations, proves and verifies. A changed coordinate is rejected: each of the six of the product,
// and one of each other result.
func TestE6(t *testing.T) {
	prove := func(t *testing.T, name string, changed int) error {
		t.Helper()
		s := newE6System()
		require.NoError(t, compilers.CompileFull(s.sys))
		proof, pub := s.sys.Prove(s.assign(name, changed))
		return s.sys.Verify(proof, pub)
	}

	t.Run("honest", func(t *testing.T) {
		require.NoError(t, prove(t, "", -1))
	})

	for name, coordinates := range map[string][]int{
		e6Product: {0, 1, 2, 3, 4, 5}, e6Sum: {2}, e6Difference: {2}, e6Scaled: {2},
	} {
		for _, coordinate := range coordinates {
			t.Run(fmt.Sprintf("changed %s %d", name, coordinate), func(t *testing.T) {
				require.Error(t, prove(t, name, coordinate))
			})
		}
	}
}
