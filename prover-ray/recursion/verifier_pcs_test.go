package recursion

import (
	"math/bits"
	"slices"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// pcsOuter builds the full verifier, the PCS opening included.
func pcsOuter(t *testing.T, c innerCase) outer {
	t.Helper()
	m := &Machine{}
	v, err := NewVerifier(m, c.sys)
	require.NoError(t, err, "the verifier of the %s system should build", c.name)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err, "the verifier of the %s system should compile", c.name)
	return outer{v, p}
}

// pcsCases are the inner proofs of the PCS tests: the verifier's fixtures,
// and one with the shapes they lack.
func pcsCases(t *testing.T) []innerCase {
	t.Helper()
	return append(innerCases(t), shapesInner(t))
}

// TestVerifierPCS: the machine verifies honest proofs, PCS opening included.
func TestVerifierPCS(t *testing.T) {
	for _, c := range pcsCases(t) {
		t.Run(c.name, func(t *testing.T) {
			_, err := pcsOuter(t, c).run(c.proof, c.pub)
			require.NoError(t, err, "the honest %s proof should verify in the machine", c.name)
		})
	}
}

// shapesInner is a static system whose PCS opening has input trees of
// different heights (a round committing only a small module) and a level of
// size one, which joins FRI at the boundary round, after the last fold.
func shapesInner(t *testing.T) innerCase {
	t.Helper()
	return shapesProof(t, "")
}

// shapesProof proves the shapes fixture. With lie set to a column's name
// ("a", "s" or "u", at the top, a middle and the boundary level), the prover
// commits to the column, then replaces it with another witness: the claims
// and quotients are the replacement's, consistent with every constraint, but
// not with the commitment, which only the PCS opening checks.
func shapesProof(t *testing.T, lie string) innerCase {
	t.Helper()
	sys := wiop.NewSystemf("shapes")
	r0, r1 := sys.NewRound(), sys.NewRound()
	ctx := sys.Context
	big := sys.NewSizedModule(ctx.Childf("big"), 16, wiop.PaddingDirectionNone)
	small := sys.NewSizedModule(ctx.Childf("small"), 4, wiop.PaddingDirectionNone)
	one := sys.NewSizedModule(ctx.Childf("one"), 1, wiop.PaddingDirectionNone)
	a := big.NewColumn(ctx.Childf("a"), r0)
	u := one.NewColumn(ctx.Childf("u"), r0)
	s := small.NewColumn(ctx.Childf("s"), r1)

	// a[i+1] = a[i] + 1, but on the last row
	big.NewVanishingManual(ctx.Childf("step"),
		wiop.Sub(a.View().Shift(1), wiop.Add(a.View(), wiop.NewConstantField(field.One()))), -1)
	// s and u are boolean
	small.NewVanishing(ctx.Childf("bool"), wiop.Sub(s.View(), wiop.Mul(s.View(), s.View())))
	one.NewVanishing(ctx.Childf("bool-u"), wiop.Sub(u.View(), wiop.Mul(u.View(), u.View())))

	r1.RegisterAction(assignColumn{s, func(*wiop.Runtime) *wiop.ConcreteVector { return vec(0, 0, 0, 0) }})
	require.NoError(t, compilers.CompileFull(sys, compilers.ConfigurePCS(pcs.WithFRINumQueries(2))), "the inner system should compile")
	// after the round's commitment
	switch lie {
	case "a":
		r0.RegisterAction(assignColumn{a, func(*wiop.Runtime) *wiop.ConcreteVector {
			return vec(6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21)
		}})
	case "u":
		r0.RegisterAction(assignColumn{u, func(*wiop.Runtime) *wiop.ConcreteVector { return vec(0) }})
	case "s":
		r1.RegisterAction(assignColumn{s, func(*wiop.Runtime) *wiop.ConcreteVector { return vec(1, 0, 1, 1) }})
	}
	if lie != "" {
		cheatOpening(t, sys)
	}
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(a, vec(1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16))
		rt.AssignColumn(u, vec(1))
	})
	if lie == "" {
		require.NoError(t, sys.Verify(proof, pub), "the honest shapes proof should verify natively")
	}
	return innerCase{"shapes", sys, proof, pub}
}

// TestVerifierPCS_Rejects: every input of an honest proof, changed in turn,
// is rejected: by the transcript and the Merkle paths, which bind every value
// of the proof. Each input is changed at lane 0, and at lanes 5 and 7, which
// a base row's last cell may not use: there, the witness chip (lanes past the
// input's kind) or the verifier's assertions bind them to zero.
func TestVerifierPCS_Rejects(t *testing.T) {
	for _, c := range pcsCases(t) {
		t.Run(c.name, func(t *testing.T) {
			o := pcsOuter(t, c)
			in, err := o.v.Inputs(c.proof, c.pub)
			require.NoError(t, err, "the honest %s proof's inputs", c.name)
			for k := range in {
				for _, lane := range []int{0, 5, 7} {
					bad := slices.Clone(in)
					bad[k][lane].Add(&bad[k][lane], new(field.Element).SetOne())
					require.Error(t, o.runInputs(bad), "input %d changed at lane %d should be rejected", k, lane)
				}
			}
		})
	}
}

// TestVerifierPCS_RejectsForged: proofs whose claims are not the committed
// columns', which only the PCS opening rejects natively, at a column of the
// top level, of a middle one, and of the boundary one.
func TestVerifierPCS_RejectsForged(t *testing.T) {
	for _, col := range []string{"a", "s", "u"} {
		t.Run(col, func(t *testing.T) {
			c := shapesProof(t, col)
			require.Empty(t, nativeFailures(c.sys, c.proof, c.pub), "the other actions accept")
			require.ErrorContains(t, c.sys.Verify(c.proof, c.pub), "pcs.Verify")
			o := pcsOuter(t, c)
			in, err := o.v.Inputs(c.proof, c.pub)
			require.NoError(t, err, "the forged proof's inputs")
			r, err := o.p.Execute(in)
			require.NoError(t, err, "the Merkle paths hold")
			require.Error(t, proveAndVerify(o.p, r), "the forged claims of %s should be rejected", col)
		})
	}
}

// runInputs executes the outer program on inputs, and proves and verifies it.
func (o outer) runInputs(in []Cell) error {
	r, err := o.p.Execute(in)
	if err != nil {
		return err
	}
	return proveAndVerify(o.p, r)
}

// assignColumn is a prover action assigning a column, or reassigning it.
type assignColumn struct {
	col *wiop.Column
	val func(rt *wiop.Runtime) *wiop.ConcreteVector
}

func (a assignColumn) Run(rt *wiop.Runtime) {
	if rt.HasColumnAssignment(a.col) {
		rt.OverrideColumn(a.col, a.val(rt))
		return
	}
	rt.AssignColumn(a.col, a.val(rt))
}

// cheatOpening replaces the PCS opening prover action of sys by a cheating
// one: compiled.open, but sending a constant final polynomial (zero) when the
// folded codeword is not constant, where the honest prover gives up.
func cheatOpening(t *testing.T, sys *wiop.System) {
	t.Helper()
	op, err := actionOf[*pcs.OpeningVerifierAction](sys).Static(sys)
	require.NoError(t, err, "the opening's static structure")
	last := sys.Rounds[len(sys.Rounds)-1]
	for k, a := range last.ProverActions {
		if pcs.IsOpeningProverAction(a) {
			last.ProverActions[k] = cheatingOpening{op.QueryRange}
			return
		}
	}
	t.Fatal("no PCS opening prover action")
}

type cheatingOpening struct{ queryRange int }

func (a cheatingOpening) Run(rt *wiop.Runtime) {
	sys := rt.System
	batches := pcs.CommittedBatches(sys)
	manifests := make([]pcs.ColumnManifest, len(batches))
	for i, b := range batches {
		manifests[i] = make(pcs.ColumnManifest, len(b.Round.Columns))
	}
	shifts, claims, _, zeta, err := pcs.RecoverBatchClaims(rt, batches, manifests)
	if err != nil {
		panic(err)
	}
	params := pcs.FRIStaticParams(sys)
	params.NumQueries = uint(pcs.FRINumQueries(sys))
	logBlowup := bits.TrailingZeros(uint(pcs.RSBlowup(sys)))
	encoders := make([]*fri.RSEncoder, pcs.FRIMaxCommittableSizeLog2(sys)+1)
	for i := range encoders {
		enc := fri.NewEncoder(uint64(1)<<(logBlowup+i), 1<<i)
		encoders[i] = &enc
	}
	p, err := fri.NewPCS(params, encoders)
	if err != nil {
		panic(err)
	}
	for i, b := range batches {
		st, ok := pcs.CommittedState(rt, b.Round.ID)
		if b.IsPrecomp || !ok {
			panic("cheatingOpening: no committed state")
		}
		if err := p.AddOpening(*st, zeta, shifts[i], claims[i]); err != nil {
			panic(err)
		}
	}
	state, err := p.NewProverState()
	if err != nil {
		panic(err)
	}
	fs := rt.GetFS()
	for state.HasNext() {
		root := foldAnyway(state, fs.RandomFext())
		if state.HasNext() {
			fs.Update(root[:]...)
		}
	}
	fs.UpdateExt(state.FinalPoly...)
	proof := p.Open(state, fs.RandomManyIntegers(int(params.NumQueries), a.queryRange))
	rt.PCSOpeningProof = &proof
}

// foldAnyway folds, and when the final layer is not constant, sends zero.
func foldAnyway(state *fri.ProverState, alpha field.Ext) (root field.Octuplet) {
	defer func() {
		if recover() != nil {
			state.FinalPoly = []field.Ext{{}}
		}
	}()
	return state.Fold(alpha)
}

// TestVerifierPCS_Unsupported: the verifier fails closed on the PCS
// configurations it does not handle.
func TestVerifierPCS_Unsupported(t *testing.T) {
	for _, tc := range []struct {
		opt  pcs.Option
		want string
	}{
		{pcs.WithMerkleCapHeight(1), "the Merkle caps"},
		{pcs.WithModuleGroups(), "grouped"},
		{pcs.WithColumnElision(), "elision"},
		{pcs.WithFixedFRIDomain(), "fixed FRI domain"},
		{pcs.WithMerkleHash(fri.MerkleHashBNPacked), "Merkle hash"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			sys := wiop.NewSystemf("unsupported")
			r0 := sys.NewRound()
			mod := sys.NewSizedModule(sys.Context.Childf("mod"), 16, wiop.PaddingDirectionNone)
			small := sys.NewSizedModule(sys.Context.Childf("small"), 4, wiop.PaddingDirectionNone)
			a, s := mod.NewColumn(sys.Context.Childf("a"), r0), small.NewColumn(sys.Context.Childf("s"), r0)
			mod.NewVanishing(sys.Context.Childf("a"), wiop.Sub(a.View(), wiop.Mul(a.View(), a.View())))
			small.NewVanishing(sys.Context.Childf("s"), wiop.Sub(s.View(), wiop.Mul(s.View(), s.View())))
			require.NoError(t, compilers.CompileFull(sys, compilers.ConfigurePCS(pcs.WithFRINumQueries(2), tc.opt)), "the inner system should compile")
			_, err := NewVerifier(&Machine{}, sys)
			require.ErrorContains(t, err, tc.want, "the verifier should reject %s", tc.want)
		})
	}

	// Only one-row modules: round 0 folds the zero codeword, and the only
	// level joins at round 1.
	t.Run("one-row modules", func(t *testing.T) {
		sys := wiop.NewSystemf("one-row")
		r0 := sys.NewRound()
		mod := sys.NewSizedModule(sys.Context.Childf("mod"), 1, wiop.PaddingDirectionNone)
		a := mod.NewColumn(sys.Context.Childf("a"), r0)
		mod.NewVanishing(sys.Context.Childf("a"), wiop.Sub(a.View(), wiop.Mul(a.View(), a.View())))
		require.NoError(t, compilers.CompileFull(sys, compilers.ConfigurePCS(pcs.WithFRINumQueries(2))), "the inner system should compile")
		_, err := NewVerifier(&Machine{}, sys)
		require.ErrorContains(t, err, "no level at the first FRI round", "the verifier should reject an opening of one-row columns")
	})
}

// TestVerifierPCS_Inputs: Inputs rejects an opening proof of another shape.
func TestVerifierPCS_Inputs(t *testing.T) {
	c := shapesInner(t)
	v, err := NewVerifier(&Machine{}, c.sys)
	require.NoError(t, err, "the verifier of the shapes system")
	for _, tc := range []struct {
		name   string
		mutate func(p *fri.OpeningProof)
	}{
		{"a query less", func(p *fri.OpeningProof) { p.InputQueries = p.InputQueries[1:] }},
		{"a round root less", func(p *fri.OpeningProof) { p.FRIProof.RoundRoots = p.FRIProof.RoundRoots[1:] }},
		{"a sibling less", func(p *fri.OpeningProof) { p.InputQueries[0][0].Siblings = p.InputQueries[0][0].Siblings[1:] }},
		{"a row level missing", func(p *fri.OpeningProof) { p.InputQueries[0][0].Leaves[0] = nil }},
		{"a row too short", func(p *fri.OpeningProof) {
			bottom := p.InputQueries[0][0].Leaves[len(p.InputQueries[0][0].Leaves)-1]
			bottom[1].Base = bottom[1].Base[1:]
		}},
		{"a running leaf of 8 elements", func(p *fri.OpeningProof) { p.FRIProof.RunningQueries[0][0].Leaf[7].SetOne() }},
		{"an injection in a running layer", func(p *fri.OpeningProof) {
			p.FRIProof.RunningQueries[0][0].AuxSiblings[0] = []field.Octuplet{{}}
		}},
		{"a Merkle cap", func(p *fri.OpeningProof) { p.InputCaps[0].Nodes = []field.Octuplet{{}} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			proof := c.proof
			op := cloneOpening(*c.proof.PCSOpeningProof)
			tc.mutate(&op)
			proof.PCSOpeningProof = &op
			_, err := v.Inputs(proof, c.pub)
			require.ErrorContains(t, err, "PCS opening", "Inputs should reject %s", tc.name)
		})
	}
	proof := c.proof
	proof.PCSOpeningProof = nil
	_, err = v.Inputs(proof, c.pub)
	require.ErrorContains(t, err, "no PCS opening proof", "Inputs should reject a proof without opening")
}

// cloneOpening deep-copies the parts of an opening proof the tests mutate.
func cloneOpening(p fri.OpeningProof) fri.OpeningProof {
	p.InputQueries = slices.Clone(p.InputQueries)
	for q := range p.InputQueries {
		p.InputQueries[q] = slices.Clone(p.InputQueries[q])
		for t := range p.InputQueries[q] {
			o := &p.InputQueries[q][t]
			o.Siblings = slices.Clone(o.Siblings)
			o.Leaves = slices.Clone(o.Leaves)
			for l, pair := range o.Leaves {
				if pair != nil {
					cp := *pair
					cp[1].Base = slices.Clone(cp[1].Base)
					o.Leaves[l] = &cp
				}
			}
		}
	}
	p.InputCaps = slices.Clone(p.InputCaps)
	p.FRIProof.RoundRoots = slices.Clone(p.FRIProof.RoundRoots)
	p.FRIProof.RunningQueries = slices.Clone(p.FRIProof.RunningQueries)
	for q := range p.FRIProof.RunningQueries {
		p.FRIProof.RunningQueries[q] = slices.Clone(p.FRIProof.RunningQueries[q])
		for j := range p.FRIProof.RunningQueries[q] {
			b := &p.FRIProof.RunningQueries[q][j]
			b.AuxSiblings = slices.Clone(b.AuxSiblings)
		}
	}
	return p
}

// TestVerifierPCS_LastRoundCell: a cell in the last round, the PCS opening's,
// is a proof value but is never absorbed natively (no AdvanceRound follows
// it): the machine's PCS transcript must not absorb it either.
func TestVerifierPCS_LastRoundCell(t *testing.T) {
	sys := wiop.NewSystemf("last-round-cell")
	r0 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	a := mod.NewColumn(sys.Context.Childf("a"), r0)
	mod.NewVanishing(sys.Context.Childf("bool"), wiop.Sub(a.View(), wiop.Mul(a.View(), a.View())))
	addCell := func(sys *wiop.System) error {
		last := sys.Rounds[len(sys.Rounds)-1]
		last.NewLazyCell(sys.Context.Childf("late"), false, func(*wiop.Runtime) field.Gen {
			return field.ElemFromBase(field.NewElement(7))
		})
		return nil
	}
	require.NoError(t, compilers.CompileFull(sys,
		compilers.ConfigurePCS(pcs.WithFRINumQueries(2)),
		compilers.WithPostHook(compilers.PCS, addCell)), "the inner system should compile")
	require.NotEmpty(t, sys.Rounds[len(sys.Rounds)-1].Cells, "the last round has a cell")
	proof, pub := sys.Prove(func(rt *wiop.Runtime) { rt.AssignColumn(a, vec(0, 1, 1, 0)) })
	require.NoError(t, sys.Verify(proof, pub), "the native verifier accepts the proof")
	_, err := pcsOuter(t, innerCase{"last-round-cell", sys, proof, pub}).run(proof, pub)
	require.NoError(t, err, "the machine accepts the proof")
}
