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

// claimsModule has one row per cell handle of the claims: each coordinate of each distinct point, and
// each value. A row is equated with its handle's cells, and reads the value that the layout gives it.
type claimsModule struct {
	module  *wiop.Module
	size    int
	x       [6]*wiop.Column // witness
	active  *wiop.Column    // precomputed
	id      *wiop.Column
	handles [][6]*wiop.Cell
}

// defineClaims binds the claims of the inputs to the values that the layout gives them.
func (v *Verifier) defineClaims(sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round) error {
	l, c := v.layout, &v.claims
	var ids []int
	points := make(map[*Point][]int)
	for _, claim := range l.claims {
		given, err := v.inputs.claim(claim)
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
	for wireI, claims := range v.inputs.Claims {
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
	c.active = flagColumn(ctx.Childf("active"), c.module, c.size, nb, func(int) bool { return true })
	c.id = precomputed(ctx.Childf("id"), c.module, c.size, ids)

	// every row is the cells of its handle, which a one-hot selector picks out
	for k := range c.x {
		var sum wiop.Expression = wiop.NewConstantField(field.Zero())
		for i, cells := range c.handles {
			selector := flagColumn(ctx.Childf("handle%d-%d", k, i), c.module, c.size, nb, func(row int) bool { return row == i })
			sum = wiop.Add(sum, wiop.Mul(selector.View(), wiop.Sub(c.x[k].View(), cells[k])))
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

func (c *claimsModule) consumers() []wiop.Table {
	return []wiop.Table{busElement(c.module, c.active, c.id, c.x)}
}

// assign assigns the rows of the claims module from the cells that the runtime holds.
func (c *claimsModule) assign(run *wiop.Runtime, put func(*wiop.Column, []field.Element)) error {
	values := make([][6]field.Element, len(c.handles))
	for i, cells := range c.handles {
		for k, cell := range cells {
			value := run.GetCellValue(cell)
			if !value.IsBase() {
				return errors.New("gkrverifier: a cell of a claim is not a base field element")
			}
			values[i][k] = value.AsBase()
		}
	}
	for k, col := range c.x {
		column := make([]field.Element, c.size)
		for i := range values {
			column[i] = values[i][k]
		}
		put(col, column)
	}
	return nil
}
