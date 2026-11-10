package fri

import (
	"fmt"
	"sort"
)

// deepPowers assigns the alpha_DEEP power of every committed column and the
// power of the running codeword of every level. A level (one per size) enters
// FRI at its round j, with a = alpha_j²:
//
//	F_j(X) = a^R · running_j(X) + Σ_c a^{p(c)} · Σ_z (f_c(X) − y_{c,z}) / (X − z)
//
// where p(c) is column c's power and R the level's running power, above every
// p(c) of the level, so that every term of F_j has its own power of a.
//
// Ungrouped commitments number each level on its own: the level's columns in
// order (batch order, then base rows, then extension rows), p(c) = 0, 1, …,
// and R = C, the level's width.
//
// Grouped commitments (see [SizedTable.Groups]) number all the columns of all
// the batches once: batch order, then the batch's groups by increasing
// [GroupShape.Ordinal] whatever their sizes, then within a group its base rows,
// then its extension rows; R = G, the total number of columns. In a grouped
// commitment (any shape declaring groups), every size with columns, of every
// batch, must declare its groups, and the ordinals of a batch must be distinct
// (the PCS compiler uses the module index). A group's powers then depend on the
// widths and ordinals of the groups, not on their sizes: a module keeps its
// powers when its size changes, and only its round (hence its a) moves, which a
// size-oblivious verifier needs. Groups of the same size, joining FRI at the
// same round, have disjoint powers.
type deepPowers struct {
	base, ext [][][]int // [batch][size][row] → power
	running   []int     // [size] → running power of the level
}

// newDeepPowers numbers the columns of shapes (see deepPowers). It rejects a
// grouped commitment with a size that does not declare its groups, or with two
// groups of the same ordinal in one batch.
func newDeepPowers(shapes []Shape) (deepPowers, error) {
	grouped := false
	for _, shape := range shapes {
		if shape.grouped() {
			grouped = true
		}
	}

	maxSizes := 0
	for _, shape := range shapes {
		maxSizes = max(maxSizes, len(shape))
	}
	d := deepPowers{
		base:    make([][][]int, len(shapes)),
		ext:     make([][][]int, len(shapes)),
		running: make([]int, maxSizes),
	}
	for b, shape := range shapes {
		d.base[b] = make([][]int, len(shape))
		d.ext[b] = make([][]int, len(shape))
		for s, sized := range shape {
			d.base[b][s] = make([]int, sized.BaseWidth)
			d.ext[b][s] = make([]int, sized.ExtWidth)
		}
	}

	if !grouped {
		for b, shape := range shapes {
			for s, sized := range shape {
				for row := range sized.BaseWidth {
					d.base[b][s][row] = d.running[s]
					d.running[s]++
				}
				for row := range sized.ExtWidth {
					d.ext[b][s][row] = d.running[s]
					d.running[s]++
				}
			}
		}
		return d, nil
	}

	// a group of a batch, with where its rows start in its size
	type group struct {
		ordinal, size, baseStart, extStart int
		shape                              GroupShape
	}
	next := 0
	for b, shape := range shapes {
		var groups []group
		for s, sized := range shape {
			if sized.BaseWidth+sized.ExtWidth == 0 {
				continue
			}
			if sized.Groups == nil {
				return deepPowers{}, fmt.Errorf("fri: batch %d size %d declares no groups, in a grouped commitment", b, s)
			}
			var base, ext int
			for _, g := range sized.Groups {
				groups = append(groups, group{g.Ordinal, s, base, ext, g})
				base += g.BaseWidth
				ext += g.ExtWidth
			}
		}
		sort.Slice(groups, func(i, j int) bool { return groups[i].ordinal < groups[j].ordinal })
		for i, g := range groups {
			if i > 0 && groups[i-1].ordinal == g.ordinal {
				return deepPowers{}, fmt.Errorf("fri: batch %d has two groups of ordinal %d", b, g.ordinal)
			}
			for k := range g.shape.BaseWidth {
				d.base[b][g.size][g.baseStart+k] = next
				next++
			}
			for k := range g.shape.ExtWidth {
				d.ext[b][g.size][g.extStart+k] = next
				next++
			}
		}
	}
	for s := range d.running {
		d.running[s] = next
	}
	return d, nil
}

// column returns the power of a committed column.
func (d deepPowers) column(batch, sizeLog2 int, isExt bool, row int) int {
	if isExt {
		return d.ext[batch][sizeLog2][row]
	}
	return d.base[batch][sizeLog2][row]
}
