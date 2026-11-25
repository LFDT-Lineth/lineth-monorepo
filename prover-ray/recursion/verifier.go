package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// Verifier is the verifier of an inner wiop system's proofs, built in a
// Machine. It mirrors System.Verify: it replays the Fiat–Shamir transcript
// round by round to derive the coins, then checks every verifier action of the
// system. The proof's values are inputs of the machine (see Inputs); the
// coins, and every intermediate value, are computed by the machine.
//
// The verifier is built for one system, and takes every one of its shape
// choices from it: a cell's encoding in the transcript, for instance, is its
// declared one (a scalar for a base cell, six elements for an extension one).
// Native verification takes it from the proof's value, so Inputs rejects a
// proof whose values do not follow the declarations.
//
// Dynamic modules are supported (see verifier_sizes.go): the sizes are
// inputs, so one verifier checks the proofs of every size. The PCS opening is
// checked in the machine too (see verifier_pcs.go), for the PCS's default
// configuration and sized modules only.
type Verifier struct {
	m   *Machine
	sys *wiop.System

	inputs      []verifierInput // the machine inputs the verifier allocated, in order
	cells       map[wiop.ObjectID]int
	coins       map[wiop.ObjectID]int
	commitments map[int]int  // round ID → digest cell
	public      []int        // the public inputs' cells, in sys.PublicInputs order
	t           *transcript  // the inner transcript, after the replay of the rounds
	pcs         *pcsVerifier // the PCS opening, nil if skipped

	sizes      map[int]*dynSize // module index → the dynamic module's size
	sizeOrder  []int            // the dynamic modules' indices, in module order
	sizeTables map[sizeTableKey]Table
	sizeCells  map[sizeCellKey]int // the size constants looked up, per module
	powTables  map[int]Table       // x's cell → the table of x^(2^i)

	// skipPCS skips the PCS opening (tests only): the verifier then checks a
	// proof's values and transcript, not that they are bound to the committed
	// columns, so that tests can compare it with the other native actions.
	skipPCS bool
}

// verifierInput is one machine input: a proof cell, a round's commitment, a
// dynamic module's log size, or a value of the PCS opening proof.
type verifierInput struct {
	cell   *wiop.Cell
	module *wiop.Module // the module whose log size it is
	round  int          // the commitment's round, if cell, module and pcs are nil
	pub    int          // the cell's index in sys.PublicInputs, -1 if it is not a public input
	// pcs reads the value from the opening proof, whose shape Inputs checks
	// first.
	pcs func(*fri.OpeningProof) Cell
}

// NewVerifier builds in m the verifier of sys's proofs. sys must be compiled
// (compilers.CompileFull). It allocates the machine inputs of a proof
// contiguously: see Inputs for their values.
func NewVerifier(m *Machine, sys *wiop.System) (*Verifier, error) {
	return newVerifier(m, sys, false)
}

func newVerifier(m *Machine, sys *wiop.System, skipPCS bool) (*Verifier, error) {
	// Fail closed on a system the verifier would check only in part: without
	// a PCS, or with queries no compiler reduced, no verifier action binds the
	// proof's values to the committed columns.
	if len(sys.Rounds) == 0 {
		return nil, fmt.Errorf("recursion: system %q has no rounds", sys.Context.Path())
	}
	if !skipPCS && sys.PCSMode() != 1 {
		return nil, fmt.Errorf("recursion: system %q is not compiled with a PCS", sys.Context.Path())
	}
	if unreduced := sys.UnreducedQueries(); len(unreduced) != 0 {
		return nil, fmt.Errorf("recursion: system %q has %d unreduced queries (%q, ...)",
			sys.Context.Path(), len(unreduced), unreduced[0].Context().Path())
	}
	for _, mod := range sys.Modules {
		if !mod.IsDynamic() && !mod.IsSized() {
			return nil, fmt.Errorf("recursion: module %q has no size", mod.Context.Path())
		}
	}
	v := &Verifier{
		m: m, sys: sys, skipPCS: skipPCS,
		cells:       map[wiop.ObjectID]int{},
		coins:       map[wiop.ObjectID]int{},
		commitments: map[int]int{},
		public:      make([]int, len(sys.PublicInputs)),
		sizes:       map[int]*dynSize{},
		sizeTables:  map[sizeTableKey]Table{},
		sizeCells:   map[sizeCellKey]int{},
		powTables:   map[int]Table{},
	}
	if err := v.replay(); err != nil {
		return nil, err
	}
	for _, r := range sys.Rounds {
		for _, a := range r.VerifierActions {
			if err := v.check(a); err != nil {
				return nil, fmt.Errorf("recursion: round %d: %w", r.ID, err)
			}
		}
	}
	return v, nil
}

// PublicInput returns the cell of the i-th public input of the inner system
// (in sys.PublicInputs order), for the caller to expose or check.
func (v *Verifier) PublicInput(i int) int { return v.public[i] }

// replay allocates the proof's inputs and derives the coins, as
// Runtime.AdvanceRound does between rounds: each round but the last absorbs
// the dynamic sizes (from System.DynamicSizeRound on), its commitment (if
// committed) and its cells, then the next round's coins are squeezed, one
// challenge each. The last round's commitment and cells are inputs, but not
// absorbed: the transcript left in v.t, which the PCS opening continues, is
// the native one after the last AdvanceRound.
func (v *Verifier) replay() error {
	pi := map[wiop.ObjectID]int{}
	for i, c := range v.sys.PublicInputs {
		pi[c.Context.ID] = i
	}
	t := &transcript{m: v.m, state: -1}
	v.t = t
	v.allocSizes()
	for i, r := range v.sys.Rounds {
		// Runtime.AdvanceRound draws coins only for the rounds after the
		// first.
		if i == 0 && len(r.Coins) > 0 {
			return fmt.Errorf("recursion: round 0 declares %d coins", len(r.Coins))
		}
		next, hasNext := r.Next()
		if hasNext && r.ID >= v.sys.DynamicSizeRound {
			v.absorbSizes()
		}
		absorb := func(addr, kind int) {
			if hasNext {
				t.absorb(addr, kind)
			}
		}
		if r.HasCommitment {
			d := v.m.InputDigest()
			v.inputs = append(v.inputs, verifierInput{round: r.ID, pub: -1})
			v.commitments[r.ID] = d
			absorb(d, KindDigest)
		}
		for _, c := range r.Cells {
			kind := KindScalar
			if c.IsExtension() {
				kind = KindE6
			}
			a := v.m.input(kind)
			in := verifierInput{cell: c, pub: -1}
			if i, ok := pi[c.Context.ID]; ok {
				v.public[i] = a
				in.pub = i
			}
			v.inputs = append(v.inputs, in)
			v.cells[c.Context.ID] = a
			absorb(a, kind)
		}
		if !hasNext {
			break
		}
		for _, coin := range next.Coins {
			v.coins[coin.Context.ID] = v.m.Lanes(t.squeeze(), 0, KindE6)
		}
	}
	for i, c := range v.sys.PublicInputs {
		if _, ok := v.cells[c.Context.ID]; !ok {
			return fmt.Errorf("recursion: public input %d (%q) is in no round", i, c.Context.Path())
		}
	}
	return nil
}

// transcript is ray's Fiat–Shamir transcript (fiatshamir.FiatShamir over
// poseidon2.MDHasher), in the machine. The hasher buffers the absorbed
// elements and compresses them on a challenge, the last block left-padded with
// zeros; after a challenge, its buffer holds the safeguard zero.
type transcript struct {
	m            *Machine
	state        int // the digest cell of the last challenge, -1 before the first one
	items, kinds []int
}

func (t *transcript) absorb(addr, kind int) {
	t.items = append(t.items, addr)
	t.kinds = append(t.kinds, kind)
}

// squeeze returns the digest cell of a challenge: the hasher's digest of the
// buffered elements, from the zero state for the first challenge.
func (t *transcript) squeeze() int {
	start, state := 1, t.state
	if state < 0 {
		start, state = 0, t.m.Const(Cell{})
	}
	t.state = t.m.MDHashFrom(state, t.m.PackAt(start, t.items, t.kinds))
	t.items, t.kinds = nil, nil
	return t.state
}

// Inputs returns the values of the verifier's machine inputs for a proof and
// its public inputs. It rejects a proof that misses a value, whose dynamic
// sizes are not one power of two per dynamic module (the machine checks their
// bounds), or holds a value whose field differs from its cell's declaration:
// an extension value in a base cell, or a base value in an extension cell,
// which the native transcript would absorb as one element instead of six.
func (v *Verifier) Inputs(proof wiop.Proof, pub wiop.PublicInput) ([]Cell, error) {
	if len(pub) != len(v.sys.PublicInputs) {
		return nil, fmt.Errorf("recursion: %d public inputs, want %d", len(pub), len(v.sys.PublicInputs))
	}
	if v.pcs != nil {
		if err := v.pcs.checkOpeningShape(proof.PCSOpeningProof); err != nil {
			return nil, fmt.Errorf("recursion: PCS opening: %w", err)
		}
	}
	res := make([]Cell, len(v.inputs))
	if err := v.sizeInputs(proof.DynamicSizes, res); err != nil {
		return nil, err
	}
	for k, in := range v.inputs {
		if in.module != nil {
			continue // set by sizeInputs
		}
		if in.pcs != nil {
			res[k] = in.pcs(proof.PCSOpeningProof)
			continue
		}
		if in.cell == nil {
			d, ok := proof.Commitments[in.round]
			if !ok {
				return nil, fmt.Errorf("recursion: no commitment for round %d", in.round)
			}
			res[k] = Cell(d)
			continue
		}
		var x field.Gen
		var ok bool
		if in.pub >= 0 {
			x = pub[in.pub]
		} else if x, ok = proof.Cells[in.cell.Context.ID]; !ok {
			return nil, fmt.Errorf("recursion: cell %q is not in the proof", in.cell.Context.Path())
		}
		if x.IsBase() == in.cell.IsExtension() {
			return nil, fmt.Errorf("recursion: cell %q: the value's field (base: %v) is not the declared one",
				in.cell.Context.Path(), x.IsBase())
		}
		if x.IsBase() {
			res[k] = ScalarCell(x.AsBase())
		} else {
			res[k] = E6Cell(x.Ext)
		}
	}
	return res, nil
}

// check builds the check of one verifier action. Every action type is handled
// explicitly: an unknown one is an error, rather than a check silently
// dropped.
func (v *Verifier) check(a wiop.VerifierAction) error {
	switch a := a.(type) {
	case *global.Verifier:
		return v.checkGlobal(a)
	case *logderivativesum.VerifierAction:
		v.checkLogDerivativeSum(a)
	case *lookuptologderivsum.ResultIsZeroVerifierAction:
		v.assertEq(v.cell(a.LogDerivativeSum.Result), v.scalar(field.Zero()))
	case *lookuptologderivsum.RowLimitVerifierAction:
		if err := v.checkRowLimit("lookup row limit, A side", a.IncludedModules, wiop.MaxLookupRows); err != nil {
			return err
		}
		return v.checkRowLimit("lookup row limit, B side", a.IncludingsModules, wiop.MaxLookupRows)
	case *grandproduct.FinalProductCheck:
		v.checkFinalProduct(a)
	case *grandproduct.CheckResultIsOne:
		v.assertEq(v.cell(a.GrandProduct.Result), v.scalar(field.One()))
	case *grandproduct.RowLimitAction:
		if err := v.checkRowLimit("permutation row limit, A side", tableModules(a.Query.A), a.Limit); err != nil {
			return err
		}
		return v.checkRowLimit("permutation row limit, B side", tableModules(a.Query.B), a.Limit)
	case *messagebus.CheckHandleSumInShard:
		v.assertEq(v.cell(a.Cell), v.m.Const(E6Cell(a.Expected.Ext)))
	case *E6Check:
		v.checkE6(a)
	case *pcs.OpeningVerifierAction:
		if !v.skipPCS {
			return v.checkPCS(a)
		}
	default:
		return fmt.Errorf("verifier action %T is not supported", a)
	}
	return nil
}

// cell returns the cell holding a proof cell's value.
func (v *Verifier) cell(c *wiop.Cell) int {
	a, ok := v.cells[c.Context.ID]
	if !ok {
		panic(fmt.Sprintf("recursion: cell %q is in no round", c.Context.Path()))
	}
	return a
}

// coin returns the cell holding a coin's value.
func (v *Verifier) coin(c *wiop.CoinField) int {
	a, ok := v.coins[c.Context.ID]
	if !ok {
		panic(fmt.Sprintf("recursion: coin %q is not drawn after a round", c.Context.Path()))
	}
	return a
}

func (v *Verifier) scalar(x field.Element) int { return v.m.Const(ScalarCell(x)) }

func (v *Verifier) scalarInt(n int) int {
	var x field.Element
	x.SetUint64(uint64(n))
	return v.scalar(x)
}

func (v *Verifier) assertEq(a, b int) { v.m.AssertEqE6(a, b) }

// checkLogDerivativeSum: Σ n·C over the Z columns, n the size of the column's
// module and C its average cell, equals the Result cell.
func (v *Verifier) checkLogDerivativeSum(a *logderivativesum.VerifierAction) {
	as, ns := make([]int, len(a.Entries)), make([]int, len(a.Entries))
	for i, e := range a.Entries {
		as[i], ns[i] = v.cell(e.Average), v.size(e.Module())
	}
	sums := v.m.MulAddChain(v.scalar(field.Zero()), as, ns)
	v.assertEq(sums[len(sums)-1], v.cell(a.LogDerivativeSum.Result))
}

// checkFinalProduct: the product of the Z columns' last values is the Result
// cell.
func (v *Verifier) checkFinalProduct(a *grandproduct.FinalProductCheck) {
	prod := v.scalar(field.One())
	for _, e := range a.Entries {
		prod = v.m.Mul(prod, v.cell(e.ZFinal))
	}
	v.assertEq(prod, v.cell(a.GrandProduct.Result))
}

// checkE6: the E6 public input is Σ_l lane_l·e_l, e_l the basis element of
// lane l. The lanes are base cells, so scalar inputs.
func (v *Verifier) checkE6(a *E6Check) {
	if len(a.Lanes) != KindE6 {
		panic(fmt.Sprintf("recursion: E6Check of %q has %d lanes", a.Cell.Context.Path(), len(a.Lanes)))
	}
	lanes, basis := make([]int, KindE6), make([]int, KindE6)
	for l, lane := range a.Lanes {
		var e Cell
		e[l] = field.One()
		lanes[l], basis[l] = v.cell(lane), v.m.Const(e)
	}
	sums := v.m.MulAddChain(v.scalar(field.Zero()), lanes, basis)
	v.assertEq(sums[len(sums)-1], v.cell(a.Cell))
}

// tableModules returns the module of each table, as the row limits count
// them.
func tableModules(ts []wiop.Table) []*wiop.Module {
	res := make([]*wiop.Module, len(ts))
	for i, t := range ts {
		res[i] = t.Module()
	}
	return res
}
