package gkrverifier

import (
	"errors"
	"fmt"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Point is a point of evaluation of n coordinates of E₆, each as six base cells.
type Point [][6]*wiop.Cell

// Claim is a claim that the multilinear of a wire evaluates to Value at Point. The claims that share a
// point should share the *Point, which the verifier then binds once.
type Claim struct {
	Point *Point
	Value [6]*wiop.Cell
}

// inputsModule has one row per cell handle of the inputs: the seed, each coordinate of each distinct
// point of the claims, and each claim's value. A row is equated with its handle's cells by a one-hot
// selector. The seed's row exposes its six coordinates to the bus; the others read the value that the
// layout gives them.
type inputsModule struct {
	module  *wiop.Module
	size    int
	x       [6]*wiop.Column // witness
	seed    *wiop.Column    // precomputed: 1 on the seed's row, the first
	coordID [6]*wiop.Column // the ids of the seed's coordinates
	read    *wiop.Column    // 1 on the rows of the claims
	id      *wiop.Column    // the id of the value that the row reads
	handles [][6]*wiop.Cell
}

// defineInputs binds the seed and the claims of the inputs to the values that the layout gives them.
func (v *Verifier) defineInputs(
	sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round, inputs Inputs,
) error {
	l, c := v.layout, &v.inputs
	c.handles = [][6]*wiop.Cell{inputs.Seed}
	ids := []int{-1}
	points := make(map[*Point][]int)
	for _, claim := range l.claims {
		given, err := inputs.claim(claim)
		if err != nil {
			return err
		}
		if len(*given.Point) != l.n {
			return fmt.Errorf("gkrverifier: wire %d claim %d: the point has %d coordinates, it needs %d",
				claim.wire, claim.index, len(*given.Point), l.n)
		}
		if prior, ok := points[given.Point]; ok {
			if !slices.Equal(prior, claim.point) {
				return fmt.Errorf("gkrverifier: wire %d claim %d: the point is shared with a claim of another point",
					claim.wire, claim.index)
			}
		} else {
			points[given.Point] = claim.point
			for j, cells := range *given.Point {
				if slices.Contains(cells[:], nil) {
					return fmt.Errorf("gkrverifier: wire %d claim %d: a cell of the point is nil", claim.wire, claim.index)
				}
				c.handles, ids = append(c.handles, cells), append(ids, claim.point[j])
			}
		}
		if slices.Contains(given.Value[:], nil) {
			return fmt.Errorf("gkrverifier: wire %d claim %d: a cell of the value is nil", claim.wire, claim.index)
		}
		c.handles, ids = append(c.handles, given.Value), append(ids, claim.value)
	}
	for wireI, claims := range inputs.Claims {
		if want := l.nbClaims(wireI); len(claims) != want {
			return fmt.Errorf("gkrverifier: wire %d has %d claims, the verifier returns %d", wireI, len(claims), want)
		}
	}

	nb := len(c.handles)
	c.size = moduleSize(nb)
	c.module = sys.NewSizedModule(ctx, c.size, wiop.PaddingDirectionNone)
	for k := range c.x {
		c.x[k] = c.module.NewColumn(ctx.Childf("x%d", k), round)
	}
	c.seed = precomputed(ctx.Childf("seed"), c.module, c.size, ones(1))
	for k := range c.coordID {
		c.coordID[k] = precomputed(ctx.Childf("coordinate-id%d", k), c.module, c.size, []int{l.coordinateID(l.seed, k)})
	}
	c.read, c.id = idColumns(ctx, c.module, c.size, nb, func(i int) int { return ids[i] })

	// every row is the cells of its handle, which a one-hot selector picks out
	selectors := make([]*wiop.Column, nb)
	for i := range selectors {
		oneHot := make([]int, i+1)
		oneHot[i] = 1
		selectors[i] = precomputed(ctx.Childf("handle%d", i), c.module, c.size, oneHot)
	}
	for k := range c.x {
		var sum wiop.Expression = wiop.NewConstantField(field.Zero())
		for i, cells := range c.handles {
			sum = wiop.Add(sum, wiop.Mul(selectors[i].View(), wiop.Sub(c.x[k].View(), cells[k])))
		}
		c.module.NewVanishingManual(ctx.Childf("is-input%d", k), sum)
	}
	return nil
}

// claim returns the claim of the layout that the inputs hold.
func (in *Inputs) claim(claim claimLayout) (Claim, error) {
	claims := in.Claims[claim.wire]
	if claim.index >= len(claims) {
		return Claim{}, fmt.Errorf("gkrverifier: wire %d has %d claims, the verifier returns more", claim.wire, len(claims))
	}
	if claims[claim.index].Point == nil {
		return Claim{}, fmt.Errorf("gkrverifier: wire %d claim %d has no point", claim.wire, claim.index)
	}
	return claims[claim.index], nil
}

// nbClaims is the number of claims that the verifier returns for the wire.
func (l *layout) nbClaims(wireI int) int {
	count := 0
	for _, claim := range l.claims {
		if claim.wire == wireI {
			count++
		}
	}
	return count
}

func (c *inputsModule) producers() []wiop.Table {
	producers := make([]wiop.Table, 0, len(c.coordID))
	for k := range c.coordID {
		producers = append(producers, busCoordinate(c.module, c.seed, c.coordID[k], c.x[k]))
	}
	return producers
}

func (c *inputsModule) consumers() []wiop.Table {
	return []wiop.Table{busElement(c.module, c.read, c.id, c.x)}
}

// assign assigns the rows of the inputs module from the cells that the runtime holds, and returns the
// seed.
func (c *inputsModule) assign(run *wiop.Runtime) (seed [6]field.Element, err error) {
	values := make([][6]field.Element, len(c.handles))
	for i, cells := range c.handles {
		for k, cell := range cells {
			value := run.GetCellValue(cell)
			if !value.IsBase() {
				return seed, errors.New("gkrverifier: a cell of an input is not a base field element")
			}
			values[i][k] = value.AsBase()
		}
	}
	for k, col := range c.x {
		column := make([]field.Element, len(values))
		for i := range values {
			column[i] = values[i][k]
		}
		assignColumn(run, col, c.size, column)
	}
	return values[0], nil
}
