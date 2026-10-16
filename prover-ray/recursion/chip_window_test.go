package recursion

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

var packKinds = []int{KindScalar, KindE6, KindDigest, KindE6, KindScalar}

// packProg is packProgram's program: the blocks Pack returns, its stream
// cells, and two Lanes outputs.
type packProg struct {
	m      *Machine
	blocks []int
	stream []int
	lanes  [2]int
}

// packProgram packs items of every kind after start buffered zeros, the last
// one a constant: 22 elements, so with start 0 at stream offsets 0, 1, 7
// (crossing), 15 (crossing) and 21, the last block holding 6 elements. It
// also takes two Lanes of the digest, lanes 2..7 and lane 7. With copies, the
// blocks and the first lanes are copied, so that they are read outside the
// windows too; without, only the windows check them.
func packProgram(copies bool, start int) packProg {
	m := &Machine{}
	cp := withCopy(m)
	items := []int{m.InputScalar(), m.InputE6(), m.InputDigest(), m.InputE6(), m.Const(ScalarCell(field.NewElement(3)))}
	pp := packProg{m: m, blocks: m.PackAt(start, items, packKinds)}
	pp.stream = m.packs[0].stream
	pp.lanes = [2]int{m.Lanes(items[2], 2, KindE6), m.Lanes(items[2], 7, KindScalar)}
	if copies {
		for _, c := range pp.blocks {
			cp.Copy(c)
		}
		cp.Copy(pp.lanes[0])
	}
	return pp
}

// packInputs returns the input items of packProgram: a scalar, an E6 value,
// a digest and an E6 value.
func packInputs(seed uint64) []Cell {
	r := rand.New(rand.NewPCG(seed, 6))
	in := make([]Cell, 4)
	for i, k := range packKinds[:4] {
		for j := range k {
			in[i][j].SetUint64(r.Uint64())
		}
	}
	return in
}

// packElements returns the elements packProgram packs for the inputs in.
func packElements(in []Cell) []field.Element {
	var xs []field.Element
	for i, k := range packKinds[:4] {
		xs = append(xs, in[i][:k]...)
	}
	return append(xs, field.NewElement(3))
}

// compress hashes blocks from state, as an MD hasher compresses them.
func compress(state field.Octuplet, blocks []Cell) field.Octuplet {
	for _, b := range blocks {
		state = poseidon2.Compress(state, field.Octuplet(b))
	}
	return state
}

func values(r *Run, addrs []int) []Cell {
	res := make([]Cell, len(addrs))
	for i, a := range addrs {
		res[i] = r.Value(a)
	}
	return res
}

// TestWindowChip: the blocks of a stream (after 0 or 1 buffered zero) are
// MDHasher's: they hash to its digest of the elements; the lanes hold the
// digest's; both prove, for two inputs.
func TestWindowChip(t *testing.T) {
	for _, start := range []int{0, 1} {
		pp := packProgram(true, start)
		require.Len(t, pp.blocks, 3)
		// the window rows write the Lanes outputs and the last block: the
		// witness chip only writes the inputs and the stream cells
		require.Equal(t, len(pp.m.inputs)+len(pp.stream), witnessChip{pp.m}.rows())
		p, err := pp.m.Compile()
		require.NoError(t, err)
		for _, seed := range []uint64{1, 2} {
			in := packInputs(seed)
			r, err := p.Execute(in)
			require.NoError(t, err)

			xs := packElements(in)
			blocks := values(r, pp.blocks)
			require.Equal(t, MDBlocks(start, xs), blocks)
			h := poseidon2.NewMDHasher()
			h.WriteElements(append(make([]field.Element, start), xs...)...)
			require.Equal(t, h.SumDigest(), compress(field.Octuplet{}, blocks), "MDHasher's digest")

			var want0, want1 Cell
			copy(want0[:], in[2][2:8])
			want1[0] = in[2][7]
			require.Equal(t, want0, r.Value(pp.lanes[0]), "lanes 2..7")
			require.Equal(t, want1, r.Value(pp.lanes[1]), "lane 7")
			require.NoError(t, proveAndVerify(p, r), "start %d, seed %d", start, seed)
		}
	}
}

// TestPackFiatShamir: a transcript of segments packed with Pack, then
// PackAt(1, …) after each challenge, and hashed from the previous digest,
// gives the challenges of ray's Fiat–Shamir transcript. The segments end with
// a partial block, a full one, and a single safeguard zero (two challenges
// in a row).
func TestPackFiatShamir(t *testing.T) {
	m := &Machine{}
	a, b, c, d, e := m.InputScalar(), m.InputE6(), m.InputDigest(), m.InputScalar(), m.InputE6()
	segs := [][]int{
		m.Pack([]int{a, b}, []int{KindScalar, KindE6}),                     // 7 elements
		m.PackAt(1, []int{c, d}, []int{KindDigest, KindScalar}),            // 1 + 9
		m.PackAt(1, []int{e, d}, []int{KindE6, KindScalar}),                // 1 + 7: a full block
		m.PackAt(1, nil, nil),                                              // the safeguard zero only
		m.PackAt(1, []int{b, c, a}, []int{KindE6, KindDigest, KindScalar}), // 1 + 15
	}
	p, err := m.Compile()
	require.NoError(t, err)

	rnd := rand.New(rand.NewPCG(3, 4))
	in := make([]Cell, 5)
	for i, k := range []int{KindScalar, KindE6, KindDigest, KindScalar, KindE6} {
		for j := range k {
			in[i][j].SetUint64(rnd.Uint64())
		}
	}
	r, err := p.Execute(in)
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r))

	fs := fiatshamir.NewFiatShamir()
	elems := [][]field.Element{
		append(in[0][:1:1], in[1][:6]...),
		append(in[2][:8:8], in[3][0]),
		append(in[4][:6:6], in[3][0]),
		nil,
		append(append(in[1][:6:6], in[2][:]...), in[0][0]),
	}
	var state field.Octuplet
	for i, seg := range segs {
		fs.Update(elems[i]...)
		want := fs.RandomDigest()
		state = compress(state, values(r, seg))
		require.Equal(t, want, state, "challenge %d", i)
	}
}

// rotateLasts recomputes every Pack's last block from its stream: a cheat on
// a stream cell, consistent with the last block's row.
func rotateLasts(m *Machine, r *Run) {
	for _, pk := range m.packs {
		if pk.last >= 0 {
			n := pk.start
			for _, k := range pk.kinds {
				n += k
			}
			r.values[pk.last] = rotate(r.values[pk.stream[len(pk.stream)-1]], n%CellWidth)
		}
	}
}

// TestWindowChipRejects: a wrong stream cell (a hint), last block or Lanes
// output (written by their window rows), consistent on the bus (and the last
// block consistent with a cheated stream), is rejected by the windows: item
// lanes, zero-pinned lanes (the buffered start, the last cell's right
// padding), the last block's rotation, and a Lanes output beyond its kind.
// Nothing but the windows reads the stream and the lanes.
func TestWindowChipRejects(t *testing.T) {
	for name, tamper := range map[string]struct {
		start int
		cell  func(pp packProg) int
		lane  int
	}{
		// the scalar item, at offset 0
		"scalar item": {0, func(pp packProg) int { return pp.stream[0] }, 0},
		// lane 3 of the E6 item, which starts at offset 1 of the first cell
		"item lane": {0, func(pp packProg) int { return pp.stream[0] }, 4},
		// the digest crosses from the first cell (lane 7) into the second
		"crossing item, first cell":  {0, func(pp packProg) int { return pp.stream[0] }, 7},
		"crossing item, second cell": {0, func(pp packProg) int { return pp.stream[1] }, 2},
		// the constant, the stream's last item
		"constant item": {0, func(pp packProg) int { return pp.stream[2] }, 5},
		// the buffered zero, and the last cell's right padding (lanes 6, 7)
		"start lane":    {1, func(pp packProg) int { return pp.stream[0] }, 0},
		"right padding": {0, func(pp packProg) int { return pp.stream[2] }, 7},
		// the last block's left padding (lanes 0, 1), and an element
		"last block, padding": {0, func(pp packProg) int { return pp.blocks[2] }, 1},
		"last block, element": {0, func(pp packProg) int { return pp.blocks[2] }, 4},
		"lanes output, E6":    {0, func(pp packProg) int { return pp.lanes[0] }, 3},
		"lanes output, one":   {0, func(pp packProg) int { return pp.lanes[1] }, 0},
		// beyond the output's kind, zero by its row's strict form
		"lanes output, E6, lane 6":     {0, func(pp packProg) int { return pp.lanes[0] }, 6},
		"lanes output, scalar, lane 3": {0, func(pp packProg) int { return pp.lanes[1] }, 3},
	} {
		t.Run(name, func(t *testing.T) {
			pp := packProgram(false, tamper.start)
			p, err := pp.m.Compile()
			require.NoError(t, err)
			r, err := p.Execute(packInputs(1))
			require.NoError(t, err)
			c := tamper.cell(pp)
			r.values[c][tamper.lane].SetUint64(5)
			if c != pp.blocks[len(pp.blocks)-1] {
				rotateLasts(pp.m, r)
			}
			require.NoError(t, p.trace(r))
			require.Error(t, proveAndVerify(p, r), "wrong cell accepted")
		})
	}
}

// TestPackItemKind: an E6 cell packed as a scalar proves only if its lanes
// 1..5 are zero: its window reads it zero beyond the packed kind, since the
// stream (and its hash) holds lane 0 only.
func TestPackItemKind(t *testing.T) {
	m := &Machine{}
	e := m.InputE6()
	m.Pack([]int{e}, []int{KindScalar})
	p, err := m.Compile()
	require.NoError(t, err)

	var x Cell
	x[0].SetUint64(3)
	r, err := p.Execute([]Cell{x})
	require.NoError(t, err)
	require.NoError(t, proveAndVerify(p, r), "an E6 value in the base field")

	x[1].SetUint64(4)
	r, err = p.Execute([]Cell{x})
	require.NoError(t, err)
	require.Error(t, proveAndVerify(p, r), "lane 1 of an E6 cell packed as a scalar")
}
