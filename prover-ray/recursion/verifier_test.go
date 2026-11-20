package recursion

import (
	"fmt"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// innerCase is an inner system and an honest proof of it.
type innerCase struct {
	name  string
	sys   *wiop.System
	proof wiop.Proof
	pub   wiop.PublicInput
}

func vec(vals ...uint64) *wiop.ConcreteVector {
	xs := make([]field.Element, len(vals))
	for i, v := range vals {
		xs[i].SetUint64(v)
	}
	return &wiop.ConcreteVector{Plain: field.VecFromBase(xs)}
}

// mixedWitness breaks the mixed fixture's witness: the lookup's multiplicity
// column off by one at row 0 (the lookup's prover panics on a value missing
// from its table), a permutation broken, the bus unbalanced.
type mixedWitness struct {
	lookupMiss, permBroken, busBroken bool
}

// forgedColumn is a prover action adding one to a column's row 0, after its
// round's actions.
type forgedColumn struct{ col *wiop.Column }

func (a forgedColumn) Run(rt *wiop.Runtime) {
	xs := append([]field.Element(nil), rt.GetColumnAssignment(a.col).Plain.AsBase()...)
	xs[0].Add(&xs[0], new(field.Element).SetOne())
	rt.OverrideColumn(a.col, &wiop.ConcreteVector{Plain: field.VecFromBase(xs)})
}

// forgery overrides a cell's value on the prover's side, after its round's
// actions and before the transcript absorbs it: the rest of the proof is
// computed from the forged value.
type forgery struct {
	cell func(sys *wiop.System) *wiop.Cell
	f    func(x field.Gen) field.Gen
}

// forgedAction is the prover action of a forgery.
type forgedAction struct {
	cell *wiop.Cell
	f    func(x field.Gen) field.Gen
}

func (a forgedAction) Run(rt *wiop.Runtime) { rt.OverrideCell(a.cell, a.f(rt.GetCellValue(a.cell))) }

// forge registers the forgeries on a compiled system.
func forge(sys *wiop.System, fs []forgery) {
	for _, fg := range fs {
		c := fg.cell(sys)
		registerBeforePCS(c.Round(), forgedAction{c, fg.f})
	}
}

// registerBeforePCS registers a prover action after a round's actions but its
// PCS ones, which commit to its columns.
func registerBeforePCS(r *wiop.Round, a wiop.ProverAction) {
	i := len(r.ProverActions)
	for k, x := range r.ProverActions {
		if strings.HasPrefix(fmt.Sprintf("%T", x), "*pcs.") {
			i = k
			break
		}
	}
	r.ProverActions = append(r.ProverActions[:i], append([]wiop.ProverAction{a}, r.ProverActions[i:]...)...)
}

// mixedInner is a static system with every action type the verifier supports
// but E6Check: global quotients with shifts, cancelled rows, a constant, a
// Lagrange selector, a cell and a coin; a lookup (log-derivative sum, row
// limit); a permutation (grand product, row limit); a message bus checked in
// the shard; a public input.
func mixedInner(t *testing.T) innerCase {
	t.Helper()
	return mixedProof(t, mixedWitness{})
}

// mixedProof proves the mixed fixture with a witness, broken or not, and
// forgeries.
func mixedProof(t *testing.T, w mixedWitness, fs ...forgery) innerCase {
	t.Helper()
	sys := wiop.NewSystemf("mixed")
	r0, r1 := sys.NewRound(), sys.NewRound()
	ctx := sys.Context
	mod := sys.NewSizedModule(ctx.Childf("mod"), 8, wiop.PaddingDirectionNone)
	small := sys.NewSizedModule(ctx.Childf("small"), 4, wiop.PaddingDirectionNone)
	a, b, tab := mod.NewColumn(ctx.Childf("a"), r0), mod.NewColumn(ctx.Childf("b"), r0), mod.NewColumn(ctx.Childf("t"), r0)
	pa, pb := small.NewColumn(ctx.Childf("pa"), r0), small.NewColumn(ctx.Childf("pb"), r0)
	snd, rcv := small.NewColumn(ctx.Childf("snd"), r0), small.NewColumn(ctx.Childf("rcv"), r0)
	c := r0.NewCell(ctx.Childf("c"), true)
	coin := r1.NewCoinField(ctx.Childf("coin"))

	mod.NewVanishing(ctx.Childf("square"), wiop.Sub(b.View(), wiop.Mul(a.View(), a.View())))
	// a[i+1] = a[i] + 1, but on the last row.
	mod.NewVanishingManual(ctx.Childf("step"),
		wiop.Sub(a.View().Shift(1), wiop.Add(a.View(), wiop.NewConstantField(field.One()))), -1)
	mod.NewLocalConstraint(ctx.Childf("first"), wiop.Mul(coin, wiop.Sub(a.View(), c)), 0)
	sys.NewInclusion(ctx.Childf("lookup"), []wiop.Table{wiop.NewTable(a.View())}, []wiop.Table{wiop.NewTable(tab.View())})
	sys.NewPermutation(ctx.Childf("perm"), []wiop.Table{wiop.NewTable(pa.View())}, []wiop.Table{wiop.NewTable(pb.View())})
	sys.NewMessageBusSend(ctx.Childf("send"), "shard", "h", wiop.NewTable(snd.View()))
	sys.NewMessageBusReceive(ctx.Childf("recv"), "shard", "h", wiop.NewTable(rcv.View()))
	sys.RegisterPublicInputs("last_b", b.At(7).Open(ctx.Childf("last_b")))

	require.NoError(t, compilers.CompileFull(sys,
		compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness()),
		compilers.ConfigurePCS(pcs.WithFRINumQueries(2)),
	))
	forge(sys, fs)
	if w.lookupMiss {
		for _, col := range r0.Columns {
			if strings.HasSuffix(col.Context.Path(), "/M-0") {
				registerBeforePCS(r0, forgedColumn{col})
			}
		}
	}
	tabVals, pbVals, rcvVals := vec(8, 7, 6, 5, 4, 3, 2, 1), vec(30, 10, 40, 20), vec(8, 5, 7, 6)
	if w.permBroken {
		pbVals = vec(30, 10, 40, 21)
	}
	if w.busBroken {
		rcvVals = vec(8, 5, 7, 9)
	}
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		rt.AssignColumn(a, vec(1, 2, 3, 4, 5, 6, 7, 8))
		rt.AssignColumn(b, vec(1, 4, 9, 16, 25, 36, 49, 64))
		rt.AssignColumn(tab, tabVals)
		rt.AssignColumn(pa, vec(10, 20, 30, 40))
		rt.AssignColumn(pb, pbVals)
		rt.AssignColumn(snd, vec(5, 6, 7, 8))
		rt.AssignColumn(rcv, rcvVals)
		rt.AssignCell(c, field.ElemFromExt(field.Lift(field.One())))
	})
	if w == (mixedWitness{}) && len(fs) == 0 {
		require.NoError(t, sys.Verify(proof, pub))
	}

	// The fixture covers the action types it is meant to.
	got := map[string]bool{}
	for _, r := range sys.Rounds {
		for _, a := range r.VerifierActions {
			got[fmt.Sprintf("%T", a)] = true
		}
	}
	for _, typ := range []string{
		"*global.Verifier", "*logderivativesum.VerifierAction",
		"*lookuptologderivsum.ResultIsZeroVerifierAction", "*lookuptologderivsum.RowLimitVerifierAction",
		"*grandproduct.FinalProductCheck", "*grandproduct.CheckResultIsOne", "*grandproduct.RowLimitAction",
		"*messagebus.CheckHandleSumInShard", "*pcs.OpeningVerifierAction",
	} {
		require.True(t, got[typ], "the fixture has no %s", typ)
	}
	return innerCase{"mixed", sys, proof, pub}
}

// machineInner is a proof of a machine program with E6 and scalar public
// inputs (publicProgram).
func machineInner(t *testing.T) innerCase {
	t.Helper()
	m, _ := publicProgram()
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	r, err := p.Execute(publicInputs())
	require.NoError(t, err)
	proof, pub := p.Prove(r)
	require.NoError(t, p.Verify(proof, pub))
	return innerCase{"machine", p.Sys, proof, pub}
}

func innerCases(t *testing.T) []innerCase {
	t.Helper()
	return []innerCase{mixedInner(t), machineInner(t)}
}

// nativeReplay is System.Verify's runtime: the proof's cells and commitments
// assigned, the transcript replayed and the coins drawn.
func nativeReplay(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) *wiop.Runtime {
	rt := wiop.NewRuntime(sys)
	for id, d := range proof.Commitments {
		rt.Commitments[id] = d
	}
	setDynamicSizes(rt, proof.DynamicSizes)
	pi := map[wiop.ObjectID]int{}
	for i, c := range sys.PublicInputs {
		pi[c.Context.ID] = i
	}
	for {
		r := rt.CurrentRound()
		for _, c := range r.Cells {
			if i, ok := pi[c.Context.ID]; ok {
				rt.AssignCell(c, pub[i])
			} else {
				rt.AssignCell(c, proof.Cells[c.Context.ID])
			}
		}
		if _, ok := r.Next(); !ok {
			return rt
		}
		rt.AdvanceRound()
	}
}

// setDynamicSizes sets the dynamic modules' sizes of a fresh runtime, as
// System.Verify does from the proof. The runtime has no setter: assigning a
// column of n zeros in round 0 sizes its module n, which needs every dynamic
// module to have a column in round 0, and the sizes to be absorbed from it
// (System.DynamicSizeRound = 0).
func setDynamicSizes(rt *wiop.Runtime, sizes map[int]int) {
	sys := rt.System
	if sys.DynamicSizeRound != 0 {
		panic("setDynamicSizes: dynamic sizes after round 0")
	}
	for k, n := range sizes {
		mod := sys.Modules[k]
		for _, col := range sys.Rounds[0].Columns {
			if col.Module == mod {
				rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(make([]field.Element, n))})
				break
			}
		}
		if mod.RuntimeSize(rt) != n {
			panic(fmt.Sprintf("setDynamicSizes: module %q has no column in round 0", mod.Context.Path()))
		}
	}
}

// nativeCheck runs the inner system's verifier actions but the PCS opening,
// the ones the machine verifier checks.
func nativeCheck(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) error {
	rt := nativeReplay(sys, proof, pub)
	for _, r := range sys.Rounds {
		for _, a := range r.VerifierActions {
			if _, ok := a.(*pcs.OpeningVerifierAction); ok {
				continue
			}
			if err := a.Check(rt); err != nil {
				return err
			}
		}
	}
	return nil
}

// outer is a machine verifying proofs of an inner system.
type outer struct {
	v *Verifier
	p *Program
}

func newOuter(t *testing.T, sys *wiop.System) outer {
	t.Helper()
	m := &Machine{}
	v, err := newVerifier(m, sys, true)
	require.NoError(t, err)
	p, err := m.Compile(testPCSOpts...)
	require.NoError(t, err)
	return outer{v, p}
}

// run executes the outer program on a proof, and proves and verifies it.
func (o outer) run(proof wiop.Proof, pub wiop.PublicInput) (*Run, error) {
	in, err := o.v.Inputs(proof, pub)
	if err != nil {
		return nil, err
	}
	r, err := o.p.Execute(in)
	if err != nil {
		return nil, err
	}
	return r, proveAndVerify(o.p, r)
}

// TestVerifier: the machine verifies honest proofs, and its coins are the
// native verifier's.
func TestVerifier(t *testing.T) {
	for _, c := range innerCases(t) {
		t.Run(c.name, func(t *testing.T) {
			o := newOuter(t, c.sys)
			r, err := o.run(c.proof, c.pub)
			require.NoError(t, err)
			rt := nativeReplay(c.sys, c.proof, c.pub)
			require.NotEmpty(t, o.v.coins)
			for _, rd := range c.sys.Rounds {
				for _, coin := range rd.Coins {
					a, ok := o.v.coins[coin.Context.ID]
					if !ok {
						continue // a coin of the first round, never drawn
					}
					require.Equal(t, rt.GetCoinValue(coin).Ext, CellE6(r.Value(a)), "coin %s", coin.Context.Path())
				}
			}
			for i := range c.sys.PublicInputs {
				x := c.pub[i]
				want := E6Cell(x.Ext)
				if x.IsBase() {
					want = ScalarCell(x.AsBase())
				}
				require.Equal(t, want, r.Value(o.v.PublicInput(i)), "public input %d", i)
			}
		})
	}
}

// TestVerifierRejects changes one value of an honest proof (a cell, a public
// input or a commitment): the machine rejects it iff the native verifier's
// actions but the PCS opening do. A changed value of the transcript changes
// the coins, so every check is exercised with other coins.
func TestVerifierRejects(t *testing.T) {
	for _, c := range innerCases(t) {
		t.Run(c.name, func(t *testing.T) {
			o := newOuter(t, c.sys)
			type tamper struct {
				name string
				f    func(proof *wiop.Proof, pub wiop.PublicInput)
			}
			var tampers []tamper
			bump := func(x field.Gen) field.Gen { return x.Add(field.ElemOne()) }
			for _, rd := range c.sys.Rounds {
				for _, cell := range rd.Cells {
					id := cell.Context.ID
					if _, ok := c.proof.Cells[id]; !ok {
						continue // a public input
					}
					tampers = append(tampers, tamper{cell.Context.Path(), func(p *wiop.Proof, _ wiop.PublicInput) {
						p.Cells[id] = bump(p.Cells[id])
					}})
				}
				if rd.HasCommitment {
					id := rd.ID
					tampers = append(tampers, tamper{fmt.Sprintf("commitment %d", id), func(p *wiop.Proof, _ wiop.PublicInput) {
						d := p.Commitments[id]
						d[3].Add(&d[3], new(field.Element).SetOne())
						p.Commitments[id] = d
					}})
				}
			}
			for i := range c.pub {
				tampers = append(tampers, tamper{fmt.Sprintf("public input %d", i), func(_ *wiop.Proof, pub wiop.PublicInput) {
					pub[i] = bump(pub[i])
				}})
			}
			if testing.Short() && len(tampers) > 12 {
				tampers = tampers[:12]
			}
			rejected := 0
			for _, tp := range tampers {
				proof := c.proof
				proof.Cells = make(map[wiop.ObjectID]field.Gen, len(c.proof.Cells))
				for k, x := range c.proof.Cells {
					proof.Cells[k] = x
				}
				proof.Commitments = make(map[int]field.Octuplet, len(c.proof.Commitments))
				for k, d := range c.proof.Commitments {
					proof.Commitments[k] = d
				}
				pub := append(wiop.PublicInput(nil), c.pub...)
				tp.f(&proof, pub)

				native := nativeCheck(c.sys, proof, pub)
				_, err := o.run(proof, pub)
				require.Equal(t, native != nil, err != nil, "%s: native %v, machine %v", tp.name, native, err)
				if err != nil {
					rejected++
				}
			}
			require.Positive(t, rejected)
			t.Logf("%d of %d changed values rejected", rejected, len(tampers))
		})
	}
}

// TestVerifierInputs: Inputs rejects a proof whose values do not follow the
// cells' declarations, or misses one.
func TestVerifierInputs(t *testing.T) {
	c := mixedInner(t)
	v, err := newVerifier(&Machine{}, c.sys, true)
	require.NoError(t, err)
	_, err = v.Inputs(c.proof, c.pub)
	require.NoError(t, err)

	var ext, base wiop.ObjectID
	var hasExt, hasBase bool
	for id, x := range c.proof.Cells {
		if x.IsBase() {
			base, hasBase = id, true
		} else {
			ext, hasExt = id, true
		}
	}
	require.True(t, hasExt && hasBase)
	for name, f := range map[string]func(p *wiop.Proof){
		"a base value in an extension cell": func(p *wiop.Proof) { p.Cells[ext] = field.ElemFromBase(field.One()) },
		"an extension value in a base cell": func(p *wiop.Proof) { p.Cells[base] = field.ElemFromExt(field.Lift(field.One())) },
		"a missing cell":                    func(p *wiop.Proof) { delete(p.Cells, base) },
		"a missing commitment":              func(p *wiop.Proof) { p.Commitments = nil },
	} {
		proof := c.proof
		proof.Cells = map[wiop.ObjectID]field.Gen{}
		for k, x := range c.proof.Cells {
			proof.Cells[k] = x
		}
		f(&proof)
		_, err := v.Inputs(proof, c.pub)
		require.Error(t, err, name)
	}
	_, err = v.Inputs(c.proof, nil)
	require.Error(t, err, "missing public inputs")
}

// TestVerifierUnsupported: the verifier fails closed on what it does not
// handle yet.
func TestVerifierUnsupported(t *testing.T) {
	sys := wiop.NewSystemf("dynamic")
	r0 := sys.NewRound()
	mod := sys.NewDynamicModule(sys.Context.Childf("mod"), wiop.PaddingDirectionRight)
	col := mod.NewColumn(sys.Context.Childf("a"), r0)
	mod.NewVanishing(sys.Context.Childf("v"), wiop.Mul(col.View(), col.View()))
	require.NoError(t, compilers.CompileFull(sys, compilers.ConfigurePCS(pcs.WithFRINumQueries(2))))
	_, err := NewVerifier(&Machine{}, sys)
	require.ErrorContains(t, err, "dynamic", "the PCS opening of a dynamic module")
	_, err = newVerifier(&Machine{}, sys, true)
	require.NoError(t, err, "a dynamic module, the PCS skipped")

	// Without a PCS, nothing binds the proof's values to the columns.
	noPCS := wiop.NewSystemf("no-pcs")
	rNoPCS := noPCS.NewRound()
	mNoPCS := noPCS.NewSizedModule(noPCS.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)
	cNoPCS := mNoPCS.NewColumn(noPCS.Context.Childf("a"), rNoPCS)
	mNoPCS.NewVanishing(noPCS.Context.Childf("v"), wiop.Mul(cNoPCS.View(), cNoPCS.View()))
	_, err = NewVerifier(&Machine{}, noPCS)
	require.ErrorContains(t, err, "PCS", "an uncompiled system")
	_, err = newVerifier(&Machine{}, noPCS, true)
	require.ErrorContains(t, err, "unreduced", "an uncompiled system, the PCS skipped")
	require.NoError(t, compilers.CompileFull(noPCS, compilers.WithoutPCS()))
	_, err = NewVerifier(&Machine{}, noPCS)
	require.ErrorContains(t, err, "PCS", "a system compiled without a PCS")

	// No coin is drawn before the first round.
	early := wiop.NewSystemf("early-coin")
	early.NewRound().NewCoinField(early.Context.Childf("coin"))
	_, err = newVerifier(&Machine{}, early, true)
	require.ErrorContains(t, err, "round 0")
}

// nativeFailures returns the types of the inner system's verifier actions but
// the PCS opening that reject a proof.
func nativeFailures(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) []string {
	rt := nativeReplay(sys, proof, pub)
	var res []string
	for _, r := range sys.Rounds {
		for _, a := range r.VerifierActions {
			if _, ok := a.(*pcs.OpeningVerifierAction); ok {
				continue
			}
			if a.Check(rt) != nil {
				res = append(res, fmt.Sprintf("%T", a))
			}
		}
	}
	return res
}

// actionOf returns the first verifier action of type T of sys.
func actionOf[T wiop.VerifierAction](sys *wiop.System) T {
	for _, r := range sys.Rounds {
		for _, a := range r.VerifierActions {
			if x, ok := a.(T); ok {
				return x
			}
		}
	}
	panic(fmt.Sprintf("no %T action", *new(T)))
}

func setTo(v field.Element) func(field.Gen) field.Gen {
	return func(field.Gen) field.Gen { return field.ElemFromExt(field.Lift(v)) }
}

func plusOne(x field.Gen) field.Gen { return x.Add(field.ElemOne()) }

// TestVerifierRejectsForged: proofs of false statements, or with a forged
// value, that a single native verifier action rejects (the PCS opening aside):
// the machine rejects each, so each check of the machine is exercised alone.
func TestVerifierRejectsForged(t *testing.T) {
	lookupResult := func(sys *wiop.System) *wiop.Cell {
		return actionOf[*lookuptologderivsum.ResultIsZeroVerifierAction](sys).LogDerivativeSum.Result
	}
	permResult := func(sys *wiop.System) *wiop.Cell {
		return actionOf[*grandproduct.CheckResultIsOne](sys).GrandProduct.Result
	}
	quotientShare := func(sys *wiop.System) *wiop.Cell {
		return actionOf[*global.Verifier](sys).Buckets[0].QuotientClaims[0]
	}
	for _, tc := range []struct {
		name  string
		inner func(t *testing.T) innerCase
		want  string
	}{
		{"a lookup multiplicity off by one", func(t *testing.T) innerCase {
			t.Helper()
			return mixedProof(t, mixedWitness{lookupMiss: true})
		}, "*lookuptologderivsum.ResultIsZeroVerifierAction"},
		{"a lookup multiplicity off by one, its sum forged to zero", func(t *testing.T) innerCase {
			t.Helper()
			return mixedProof(t, mixedWitness{lookupMiss: true}, forgery{lookupResult, setTo(field.Zero())})
		}, "*logderivativesum.VerifierAction"},
		{"a broken permutation", func(t *testing.T) innerCase {
			t.Helper()
			return mixedProof(t, mixedWitness{permBroken: true})
		}, "*grandproduct.CheckResultIsOne"},
		{"a broken permutation, its product forged to one", func(t *testing.T) innerCase {
			t.Helper()
			return mixedProof(t, mixedWitness{permBroken: true}, forgery{permResult, setTo(field.One())})
		}, "*grandproduct.FinalProductCheck"},
		{"an unbalanced bus", func(t *testing.T) innerCase {
			t.Helper()
			return mixedProof(t, mixedWitness{busBroken: true})
		}, "*messagebus.CheckHandleSumInShard"},
		{"a forged quotient share", func(t *testing.T) innerCase {
			t.Helper()
			// The share is a column opening: the inner prover cannot open a
			// forged one, so it is changed in the proof. The transcript then
			// changes after the global check's coins.
			c := mixedInner(t)
			id := quotientShare(c.sys).Context.ID
			c.proof.Cells[id] = plusOne(c.proof.Cells[id])
			return c
		}, "*global.Verifier"},
		{"a forged E6 public input", func(t *testing.T) innerCase {
			t.Helper()
			m, _ := publicProgram()
			p, err := m.Compile(testPCSOpts...)
			require.NoError(t, err)
			forge(p.Sys, []forgery{{func(sys *wiop.System) *wiop.Cell { return actionOf[*E6Check](sys).Cell }, plusOne}})
			r, err := p.Execute(publicInputs())
			require.NoError(t, err)
			proof, pub := p.Prove(r)
			return innerCase{"machine", p.Sys, proof, pub}
		}, "*recursion.E6Check"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := tc.inner(t)
			require.Equal(t, []string{tc.want}, nativeFailures(c.sys, c.proof, c.pub))
			_, err := newOuter(t, c.sys).run(c.proof, c.pub)
			require.Error(t, err)
		})
	}
}
