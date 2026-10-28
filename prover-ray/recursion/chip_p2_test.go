package recursion

import (
	"math/rand/v2"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// TestPoseidon2AIR: the AIR's trace of a permutation ends with gnark-crypto's
// permutation of the inputs (so the matrices and round keys agree), and its
// compression tuple is poseidon2.Compress's.
func TestPoseidon2AIR(t *testing.T) {
	rnd := rand.New(rand.NewPCG(1, 2))
	for range 8 {
		var in [P2Width]field.Element
		for i := range in {
			in[i].SetUint64(rnd.Uint64())
		}
		want := in
		require.NoError(t, p2Perm.Permutation(want[:]))
		tr := make([]field.Element, len(p2Cols))
		p2Trace(in, tr)
		for i := range P2Width {
			require.Equal(t, want[i], tr[p2OutIdx[i]], "out%d", i)
		}
		var st, blk field.Octuplet
		copy(st[:], in[:p2Half])
		copy(blk[:], in[p2Half:])
		c := poseidon2.Compress(st, blk)
		for i := range p2Half {
			var x field.Element
			x.Add(&want[p2Half+i], &in[p2Half+i])
			require.Equal(t, c[i], x, "compression lane %d", i)
		}
	}
}

// mdInputs returns n inputs for mdProgram: a digest, an E6 value, a scalar,
// and again.
func mdInputs(seed uint64, n int) []Cell {
	rnd := rand.New(rand.NewPCG(seed, 3))
	in := make([]Cell, n)
	kinds := []int{KindDigest, KindE6, KindScalar}
	for i := range in {
		for j := range kinds[i%3] {
			in[i][j].SetUint64(rnd.Uint64())
		}
	}
	return in
}

// mdProgram packs n items (digest, E6, scalar, …) and hashes them; its digest
// is copied, so that it is read on the bus.
func mdProgram(n int) (m *Machine, kinds []int, digest int) {
	m = &Machine{}
	cp := withCopy(m)
	items := make([]int, n)
	kinds = make([]int, n)
	for i := range items {
		kinds[i] = []int{KindDigest, KindE6, KindScalar}[i%3]
		items[i] = m.input(kinds[i])
	}
	digest = m.MDHash(m.Pack(items, kinds))
	cp.Copy(digest)
	return m, kinds, digest
}

// TestMDHash: the digest of packed items is poseidon2.MDHasher's digest of
// their elements, for streams ending with a partial block, a full one, and a
// single element; and it proves.
func TestMDHash(t *testing.T) {
	for _, n := range []int{1, 2, 3, 6} { // 8, 14, 15 and 45 elements
		m, kinds, digest := mdProgram(n)
		p, err := m.Compile(testPCSOpts...)
		require.NoError(t, err)
		in := mdInputs(uint64(n), n)
		r, err := p.Execute(in)
		require.NoError(t, err)
		h := poseidon2.NewMDHasher()
		for i, c := range in {
			h.WriteElements(c[:kinds[i]]...)
		}
		require.Equal(t, Cell(h.SumDigest()), r.Value(digest), "%d items", n)
		require.NoError(t, proveAndVerify(p, r), "%d items", n)
	}
}

// TestMDFiatShamir: a transcript hashed in the machine, a first segment with
// MDHash, then MDHashFrom the previous digest of PackAt(1, …) after each
// challenge, gives ray's Fiat–Shamir challenges; two challenges in a row hash
// the safeguard zero alone.
func TestMDFiatShamir(t *testing.T) {
	m := &Machine{}
	cp := withCopy(m)
	a, b, c := m.InputE6(), m.InputDigest(), m.InputScalar()
	d1 := m.MDHash(m.Pack([]int{a, c}, []int{KindE6, KindScalar}))
	d2 := m.MDHashFrom(d1, m.PackAt(1, []int{b, a, c}, []int{KindDigest, KindE6, KindScalar}))
	d3 := m.MDHashFrom(d2, m.PackAt(1, nil, nil))
	for _, d := range []int{d1, d2, d3} {
		cp.Copy(d)
	}
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	in := mdInputs(9, 3)
	in[0], in[1] = in[1], in[0] // E6, digest, scalar
	r, err := p.Execute(in)
	require.NoError(t, err)

	fs := fiatshamir.NewFiatShamir()
	fs.Update(append(in[0][:KindE6:KindE6], in[2][0])...)
	require.Equal(t, Cell(fs.RandomDigest()), r.Value(d1), "challenge 1")
	fs.Update(append(append(in[1][:KindDigest:KindDigest], in[0][:KindE6]...), in[2][0])...)
	require.Equal(t, Cell(fs.RandomDigest()), r.Value(d2), "challenge 2")
	require.Equal(t, Cell(fs.RandomDigest()), r.Value(d3), "challenge 3")
	require.NoError(t, proveAndVerify(p, r))
}

// TestMDHashRejects: cheats on a 2-block hash (n = 3 items, 15 elements),
// consistent on the memory bus (the digest's copy reads the cheated value):
// a digest that is not the compression of its state and block (only the P2
// lookup sees it), a second block hashed from another state than the first
// one's digest (with that compression in the P2 core: only the chaining
// constraint sees it), a P2 internal column, and the initial state.
func TestMDHashRejects(t *testing.T) {
	setLanes := func(p *Program, r *Run, chip, prefix string, row int, v Cell) {
		for i := range CellWidth {
			r.Witness(p, chip, laneName(prefix, i))[row] = v[i]
		}
	}
	for name, cheat := range map[string]func(p *Program, r *Run, digest int){
		"digest": func(p *Program, r *Run, digest int) {
			bad := r.values[digest]
			bad[2].SetUint64(5)
			r.values[digest] = bad
			require.NoError(t, p.trace(r))
			setLanes(p, r, mdMod, mdD, 1, bad)
		},
		"chaining": func(p *Program, r *Run, digest int) {
			var other Cell // the second block from the zero state
			b1 := cellAt(r, p, mdMod, mdB, 1)
			r.p2[1] = p2Input(other, b1)
			out := compressCell(other, b1)
			r.values[digest] = out
			require.NoError(t, p.trace(r))
			setLanes(p, r, mdMod, mdSt, 1, other)
			setLanes(p, r, mdMod, mdD, 1, out)
		},
		"P2 internal": func(p *Program, r *Run, _ int) {
			r.Witness(p, p2Mod, p2SBoxName(p2HalfFullRounds+3))[0].SetUint64(5)
		},
		"initial state": func(p *Program, r *Run, _ int) {
			r.Witness(p, mdMod, laneName(mdSt, 0))[0].SetUint64(5)
		},
	} {
		t.Run(name, func(t *testing.T) {
			m, _, digest := mdProgram(3)
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			r, err := p.Execute(mdInputs(3, 3))
			require.NoError(t, err)
			require.NoError(t, proveAndVerify(p, r), "honest")
			cheat(p, r, digest)
			require.Error(t, proveAndVerify(p, r), "cheat accepted")
		})
	}
}

// cellAt returns the 8 lanes of a chip's witness columns prefix at row.
func cellAt(r *Run, p *Program, chip, prefix string, row int) Cell {
	var c Cell
	for i := range CellWidth {
		c[i] = r.Witness(p, chip, laneName(prefix, i))[row]
	}
	return c
}

// BenchmarkP2Execute: executing (and tracing) one MDHash of 4096 blocks, so
// 4096 rows of the P2 core.
func BenchmarkP2Execute(b *testing.B) {
	const n = 4096
	m := &Machine{}
	blocks := make([]int, n)
	in := make([]Cell, n)
	for i := range blocks {
		blocks[i] = m.InputDigest()
		in[i] = cell(uint64(i))
	}
	m.MDHash(blocks)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(b, err)
	for b.Loop() {
		if _, err := p.Execute(in); err != nil {
			b.Fatal(err)
		}
	}
}
