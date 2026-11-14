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
// selector. The seed's row exposes the seed to the bus; the others read the value that the layout gives
// them.
type inputsModule struct {
	module  *wiop.Module
	x       [6]*wiop.Column // witness
	seed    *wiop.Column    // precomputed: 1 on the seed's row, the first
	seedID  *wiop.Column    // the id of the seed
	read    *wiop.Column    // 1 on the rows of the claims
	id      *wiop.Column    // the id of the value that the row reads
	handles [][6]*wiop.Cell
}

// defineInputs binds the seed and the claims of the inputs to the values that the layout gives them.
func (v *Verifier) defineInputs(
	sys *wiop.System, ctx *wiop.ContextFrame, round *wiop.Round, inputs Inputs,
) error {
	l, in := v.layout, &v.inputs
	if slices.Contains(inputs.Seed[:], nil) {
		return errors.New("gkrverifier: a cell of the seed is nil")
	}
	for wireI, want := range l.claimCounts {
		if got := len(inputs.Claims[wireI]); got != want {
			return fmt.Errorf("gkrverifier: wire %d has %d claims, the verifier returns %d", wireI, got, want)
		}
	}
	for wireI, claims := range inputs.Claims {
		if want := l.claimCounts[wireI]; len(claims) != want {
			return fmt.Errorf("gkrverifier: wire %d has %d claims, the verifier returns %d", wireI, len(claims), want)
		}
	}

	in.handles = [][6]*wiop.Cell{inputs.Seed}
	ids := []int{-1}
	points := make(map[*Point][]int)
	for _, claim := range l.claims {
		given := inputs.Claims[claim.wire][claim.index]
		if given.Point == nil {
			return fmt.Errorf("gkrverifier: wire %d claim %d has no point", claim.wire, claim.index)
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
				in.handles, ids = append(in.handles, cells), append(ids, claim.point[j])
			}
		}
		if slices.Contains(given.Value[:], nil) {
			return fmt.Errorf("gkrverifier: wire %d claim %d: a cell of the value is nil", claim.wire, claim.index)
		}
		in.handles, ids = append(in.handles, given.Value), append(ids, claim.value)
	}

	nb := len(in.handles)
	in.module = sys.NewSizedModule(ctx, moduleSize(nb), wiop.PaddingDirectionNone)
	for k := range in.x {
		in.x[k] = in.module.NewColumn(ctx.Childf("x%d", k), round)
	}
	in.seed = precomputed(ctx.Childf("seed"), in.module, []int{1})
	in.seedID = precomputed(ctx.Childf("seed-id"), in.module, []int{l.seed})
	in.read, in.id = idColumns(ctx, in.module, ids)

	// every row is the cells of its handle, which a one-hot selector picks out
	selectors := make([]*wiop.Column, nb)
	for i := range selectors {
		oneHot := make([]int, i+1)
		oneHot[i] = 1
		selectors[i] = precomputed(ctx.Childf("handle%d", i), in.module, oneHot)
	}
	for k := range in.x {
		sum := baseConstant(0)
		for i, cells := range in.handles {
			sum = wiop.Add(sum, wiop.Mul(selectors[i].View(), wiop.Sub(in.x[k].View(), cells[k])))
		}
		in.module.NewVanishingManual(ctx.Childf("is-input%d", k), sum)
	}
	return nil
}

func (in *inputsModule) producers() []wiop.Table {
	return []wiop.Table{busElement(in.module, in.seed, in.seedID, in.x)}
}

func (in *inputsModule) consumers() []wiop.Table {
	return []wiop.Table{busElement(in.module, in.read, in.id, in.x)}
}

// assign assigns the rows of the inputs module from the cells that the runtime holds, and returns the
// seed.
func (in *inputsModule) assign(run *wiop.Runtime) (seed [6]field.Element, err error) {
	values := make([][6]field.Element, len(in.handles))
	for i, cells := range in.handles {
		for k, cell := range cells {
			value := run.GetCellValue(cell)
			if !value.IsBase() {
				return seed, errors.New("gkrverifier: a cell of an input is not a base field element")
			}
			values[i][k] = value.AsBase()
		}
	}
	for k, col := range in.x {
		column := make([]field.Element, len(values))
		for i := range values {
			column[i] = values[i][k]
		}
		assignColumn(run, col, column)
	}
	return values[0], nil
}
