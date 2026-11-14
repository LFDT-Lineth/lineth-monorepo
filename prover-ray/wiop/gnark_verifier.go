package wiop

import (
	"fmt"
	"reflect"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/circuit"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/consensys/gnark/frontend"
)

// VerifierCircuit is the gnark counterpart of [System.Verify]: a circuit whose
// witness is a [Proof] plus its [PublicInput], and whose constraints replay the
// Fiat-Shamir transcript and enforce every [GnarkVerifierAction] the compiled
// system registers. Satisfying it proves that the native verifier accepts the
// same (proof, public input) pair.
//
// What it proves: the wiop proof carried in the witness is accepted by the
// verifier of the System the circuit was allocated for. The System itself is
// compile-time metadata baked into the constraints, not a witness.
//
// Shape: the circuit, and so its constraint system and proving keys, is a
// function of the System alone and can be built before any proof exists (see
// [AllocateVerifierCircuit]). Every honest proof of the System has the same
// shape: cell field tags follow the cell declarations, commitments follow the
// rounds, and the FRI opening geometry follows the PCS parameters and the
// static batch shapes.
//
// Visibility: every field is private. Exposing the public inputs (and a digest
// of the System) is the job of the outer circuit that embeds a VerifierCircuit.
//
// Constraint budget: dominated by the PCS action (Poseidon2 Merkle paths and
// extension-field DEEP quotients, scaling with the FRI query count); the
// transcript replay and vanishing checks are comparatively small.
//
// Limitations of this first iteration, all enforced by
// [AllocateVerifierCircuit] failing closed:
//   - no dynamic-size modules (their sizes change the circuit shape): fix their
//     sizes before compiling, see [System.FixDynamicSizes];
//   - no column elision (the prover would choose the batch shapes);
//   - every verifier action must implement [GnarkVerifierAction].
type VerifierCircuit struct {
	// CellsBase holds every base-field proof cell, in round then declaration
	// order, public inputs excluded. // private
	CellsBase []circuit.Element
	// CellsExt holds every extension-field proof cell, same ordering. // private
	CellsExt []circuit.Ext
	// PublicInputsBase / PublicInputsExt hold the registered public inputs, in
	// registration order, split by field like the cells. // private
	PublicInputsBase []circuit.Element
	PublicInputsExt  []circuit.Ext
	// Commitments holds one round commitment per round flagged HasCommitment,
	// in round order. // private
	Commitments []circuit.Octuplet
	// PCSOpeningProof is the FRI opening proof; empty for a protocol that was
	// not PCS-compiled. // private
	PCSOpeningProof fri.GnarkOpeningProof

	// The metadata below is compile-time only. gnark's schema walker follows
	// every struct field, exported or not, so it has to be excluded explicitly
	// or the walk loops on the System's back-references.
	spec   *System         `gnark:"-"`
	layout *verifierLayout `gnark:"-"`
}

// verifierLayout is the witness shape shared by a circuit and its assignments:
// where every cell of the System lands in the witness slices, which rounds
// carry a commitment, and whether a PCS opening proof is present. It is a
// function of the System alone, fixed at allocation time and enforced on every
// assignment.
type verifierLayout struct {
	cells       map[ObjectID]cellSlot
	commitments []int // round IDs carrying a commitment, in order
	hasPCS      bool
	// pcsShape is a proof with the opening geometry of the System (values
	// meaningless), kept apart from the circuit fields: frontend.Compile writes
	// its variables into the circuit struct, which then no longer describes the
	// geometry alone.
	pcsShape fri.OpeningProof
}

// cellSlot locates one cell in the witness: the (isBase, isPublic) pair picks
// the slice, index the position within it.
//
// isBase is the cell's declared field ([Cell.IsExtension]). The declaration is
// binding: the runtime stores every cell value with its declared tag and
// [System.Verify] rejects a proof value tagged otherwise. It is load-bearing
// beyond slice selection: [Runtime.AdvanceRound] absorbs a base cell as one
// field element and an extension cell as six, so the tag fixes the transcript.
type cellSlot struct {
	isBase   bool
	isPublic bool
	index    int
}

// GnarkOpeningProofAllocator is implemented by the verifier action that checks
// the PCS opening proof (the PCS compiler's opening action). It returns an
// opening proof with the geometry every honest proof of sys has, derived from
// the System alone; its values are meaningless. [AllocateVerifierCircuit] calls
// it; a System without such an action carries no PCS opening proof.
type GnarkOpeningProofAllocator interface {
	OpeningProofShape(sys *System) fri.OpeningProof
}

// AllocateVerifierCircuit builds the unassigned circuit for sys from the System
// alone, before any proof exists: the cell field tags come from the cell
// declarations, the commitments from the rounds flagged HasCommitment and the
// FRI opening-proof geometry from the [GnarkOpeningProofAllocator] action.
//
// It panics if sys uses a feature the circuit does not support (dynamic
// modules, a verifier action without [GnarkVerifierAction], column elision),
// or registers more than one opening-proof allocator.
func AllocateVerifierCircuit(sys *System) *VerifierCircuit {
	checkCircuitSupport(sys)
	layout := &verifierLayout{cells: make(map[ObjectID]cellSlot)}
	c := &VerifierCircuit{spec: sys, layout: layout}
	piIdx := sys.publicInputIndex()

	for _, r := range sys.Rounds {
		for _, cell := range r.Cells {
			_, isPI := piIdx[cell.Context.ID]
			layout.cells[cell.Context.ID] = c.appendCell(!cell.IsExtension(), isPI, field.Gen{}, false)
		}
		if r.HasCommitment {
			layout.commitments = append(layout.commitments, r.ID)
			c.Commitments = append(c.Commitments, circuit.Octuplet{})
		}
	}

	var allocator GnarkOpeningProofAllocator
	for _, r := range sys.Rounds {
		for _, va := range r.VerifierActions {
			a, ok := va.(GnarkOpeningProofAllocator)
			if !ok {
				continue
			}
			if allocator != nil {
				panic("wiop: VerifierCircuit: more than one verifier action allocates a PCS opening proof")
			}
			allocator = a
		}
	}
	if allocator != nil {
		layout.hasPCS = true
		layout.pcsShape = allocator.OpeningProofShape(sys)
		c.PCSOpeningProof = fri.AllocateGnarkOpeningProof(layout.pcsShape)
	}
	return c
}

// AssignVerifierCircuit returns the witness assignment of the circuit for
// (proof, pub). It panics if the proof does not have the circuit's shape: a
// missing cell, a cell value without its declared field tag, a missing
// commitment, or an opening proof of another geometry.
func (c *VerifierCircuit) AssignVerifierCircuit(proof Proof, pub PublicInput) *VerifierCircuit {
	sys := c.spec
	if len(pub) != len(sys.PublicInputs) {
		panic(fmt.Sprintf("wiop: VerifierCircuit: public inputs length mismatch: got %d, want %d",
			len(pub), len(sys.PublicInputs)))
	}
	a := &VerifierCircuit{spec: sys, layout: c.layout}
	piIdx := sys.publicInputIndex()

	for _, r := range sys.Rounds {
		for _, cell := range r.Cells {
			pos, isPI := piIdx[cell.Context.ID]
			var v field.Gen
			if isPI {
				v = pub[pos]
			} else {
				var ok bool
				if v, ok = proof.Cells[cell.Context.ID]; !ok {
					panic(fmt.Sprintf("wiop: VerifierCircuit: cell %q not found in proof", cell.Context.Path()))
				}
			}
			if !cell.hasDeclaredTag(v) {
				panic(fmt.Sprintf("wiop: VerifierCircuit: cell %q does not carry its declared field tag (extension=%v)",
					cell.Context.Path(), cell.IsExtension()))
			}
			slot := a.appendCell(v.IsBase(), isPI, v, true)
			if want := c.layout.cells[cell.Context.ID]; want != slot {
				panic(fmt.Sprintf("wiop: VerifierCircuit: cell %q lands at %+v, want %+v", cell.Context.Path(), slot, want))
			}
		}
	}
	for _, roundID := range c.layout.commitments {
		commitment, ok := proof.Commitments[roundID]
		if !ok {
			panic(fmt.Sprintf("wiop: VerifierCircuit: commitment for round %d not found in proof", roundID))
		}
		a.Commitments = append(a.Commitments, poseidon2.NewKoalagnarkOctuplet(commitment))
	}
	if len(proof.Commitments) != len(c.layout.commitments) {
		panic(fmt.Sprintf("wiop: VerifierCircuit: proof has %d commitments, want %d",
			len(proof.Commitments), len(c.layout.commitments)))
	}

	if c.layout.hasPCS != (proof.PCSOpeningProof != nil) {
		panic("wiop: VerifierCircuit: proof and circuit disagree on the presence of a PCS opening proof")
	}
	if c.layout.hasPCS {
		// Compared on fresh unassigned structs, which keep exactly the geometry.
		if !reflect.DeepEqual(fri.AllocateGnarkOpeningProof(*proof.PCSOpeningProof),
			fri.AllocateGnarkOpeningProof(c.layout.pcsShape)) {
			panic("wiop: VerifierCircuit: the PCS opening proof does not have the circuit's geometry")
		}
		a.PCSOpeningProof = fri.NewGnarkOpeningProof(*proof.PCSOpeningProof)
	}
	return a
}

// System returns the compiled system the circuit verifies.
func (c *VerifierCircuit) System() *System { return c.spec }

func checkCircuitSupport(sys *System) {
	for _, m := range sys.Modules {
		if m.IsDynamic() {
			panic(fmt.Sprintf("wiop: VerifierCircuit: dynamic module %q is not supported; "+
				"fix its size before compiling (see System.FixDynamicSizes)", m.Context.Path()))
		}
	}
	for _, r := range sys.Rounds {
		for _, va := range r.VerifierActions {
			if _, ok := va.(GnarkVerifierAction); !ok {
				panic(fmt.Sprintf(
					"wiop: VerifierCircuit: round %d registers verifier action %T which has no "+
						"in-circuit implementation; the check it enforces would be silently dropped",
					r.ID, va,
				))
			}
		}
	}
}

// appendCell appends a witness for v (a zero placeholder without values) to the
// slice its (field, visibility) pair selects and returns the cell's slot.
func (c *VerifierCircuit) appendCell(isBase, isPublic bool, v field.Gen, withValues bool) cellSlot {
	slot := cellSlot{isBase: isBase, isPublic: isPublic}
	switch {
	case isBase && isPublic:
		slot.index = len(c.PublicInputsBase)
		c.PublicInputsBase = append(c.PublicInputsBase, newElementWitness(v, withValues))
	case isBase:
		slot.index = len(c.CellsBase)
		c.CellsBase = append(c.CellsBase, newElementWitness(v, withValues))
	case isPublic:
		slot.index = len(c.PublicInputsExt)
		c.PublicInputsExt = append(c.PublicInputsExt, newExtWitness(v, withValues))
	default:
		slot.index = len(c.CellsExt)
		c.CellsExt = append(c.CellsExt, newExtWitness(v, withValues))
	}
	return slot
}

func newElementWitness(v field.Gen, withValues bool) circuit.Element {
	if !withValues {
		return circuit.Element{}
	}
	return circuit.NewElementFromKoala(v.AsBase())
}

func newExtWitness(v field.Gen, withValues bool) circuit.Ext {
	if !withValues {
		return circuit.Ext{}
	}
	return circuit.NewExt(v.AsExt())
}

// Define implements [frontend.Circuit]. It mirrors [System.Verify]: replay the
// transcript round by round, deriving every coin in-circuit, then run every
// verifier action's CheckGnark with the round pointer set like the native
// verifier does.
func (c *VerifierCircuit) Define(api frontend.API) error {
	run := c.newGnarkRuntime(api)
	sys := c.spec

	for _, r := range sys.Rounds {
		if r.ID == len(sys.Rounds)-1 {
			break
		}
		run.advanceRound(r)
	}

	for _, r := range sys.Rounds {
		run.currentRound = r
		for _, va := range r.VerifierActions {
			gva, ok := va.(GnarkVerifierAction)
			if !ok {
				panic(fmt.Sprintf("wiop: VerifierCircuit: verifier action %T has no in-circuit implementation", va))
			}
			gva.CheckGnark(api, run)
		}
	}
	return nil
}

// GnarkRuntime is the in-circuit mirror of [Runtime] for the verifier side: it
// exposes the proof cells, the re-derived coins, the round commitments and the
// Fiat-Shamir transcript to [GnarkVerifierAction] implementations. All values
// are circuit variables; extension elements are returned for every cell so
// callers need not care about the base/extension split.
type GnarkRuntime struct {
	// System is the protocol specification being verified.
	System *System
	// PCSOpeningProof is the FRI opening proof carried by the witness, or nil
	// when the protocol was not PCS-compiled.
	PCSOpeningProof *fri.GnarkOpeningProof

	api          *circuit.KoalaBearAPI
	fs           *fiatshamir.GnarkFiatShamir
	cellsBase    map[ObjectID]circuit.Element
	cells        map[ObjectID]circuit.Ext
	coins        map[ObjectID]circuit.Ext
	commitments  map[int]circuit.Octuplet
	currentRound *Round
}

func (c *VerifierCircuit) newGnarkRuntime(api frontend.API) *GnarkRuntime {
	fs := fiatshamir.NewGnarkFiatShamir(api)
	run := &GnarkRuntime{
		System:      c.spec,
		api:         fs.API(),
		fs:          fs,
		cellsBase:   make(map[ObjectID]circuit.Element),
		cells:       make(map[ObjectID]circuit.Ext),
		coins:       make(map[ObjectID]circuit.Ext),
		commitments: make(map[int]circuit.Octuplet),
	}
	if c.layout.hasPCS {
		run.PCSOpeningProof = &c.PCSOpeningProof
	}
	for id, slot := range c.layout.cells {
		switch {
		case slot.isBase && slot.isPublic:
			run.setBaseCell(id, c.PublicInputsBase[slot.index])
		case slot.isBase:
			run.setBaseCell(id, c.CellsBase[slot.index])
		case slot.isPublic:
			run.cells[id] = c.PublicInputsExt[slot.index]
		default:
			run.cells[id] = c.CellsExt[slot.index]
		}
	}
	for i, roundID := range c.layout.commitments {
		run.commitments[roundID] = c.Commitments[i]
	}
	if len(c.spec.Rounds) > 0 {
		run.currentRound = c.spec.Rounds[0]
	}
	return run
}

func (run *GnarkRuntime) setBaseCell(id ObjectID, e circuit.Element) {
	run.cellsBase[id] = e
	run.cells[id] = run.api.FromBaseExt(e)
}

// advanceRound mirrors [Runtime.AdvanceRound]: absorb the current round's
// commitment and cells, move to the next round, derive its coins.
func (run *GnarkRuntime) advanceRound(r *Round) {
	next, ok := r.Next()
	if !ok {
		panic("wiop: GnarkRuntime.advanceRound: already at the last round")
	}
	if r.HasCommitment {
		run.fs.UpdateOctuplet(run.GetCommitment(r.ID))
	}
	for _, cell := range r.Cells {
		if base, isBase := run.cellsBase[cell.Context.ID]; isBase {
			run.fs.Update(base)
			continue
		}
		run.fs.UpdateExt(run.GetCellValue(cell))
	}
	run.currentRound = next
	for _, coin := range next.Coins {
		run.coins[coin.Context.ID] = run.fs.RandomFext()
	}
}

// API returns the koalagnark arithmetic API.
func (run *GnarkRuntime) API() *circuit.KoalaBearAPI { return run.api }

// FS returns the in-circuit Fiat-Shamir transcript, positioned exactly where
// the native verifier's transcript is when verifier actions run.
func (run *GnarkRuntime) FS() *fiatshamir.GnarkFiatShamir { return run.fs }

// CurrentRound mirrors [Runtime.CurrentRound].
func (run *GnarkRuntime) CurrentRound() *Round { return run.currentRound }

// GetCellValue returns the cell's value lifted to the extension field.
func (run *GnarkRuntime) GetCellValue(cell *Cell) circuit.Ext {
	v, ok := run.cells[cell.Context.ID]
	if !ok {
		panic(fmt.Sprintf("wiop: GnarkRuntime: cell %q is not in the witness", cell.Context.Path()))
	}
	return v
}

// GetCellBase returns the cell's base-field value and true, or false if the
// proof carries the cell as an extension element.
func (run *GnarkRuntime) GetCellBase(cell *Cell) (circuit.Element, bool) {
	v, ok := run.cellsBase[cell.Context.ID]
	return v, ok
}

// GetCoinValue mirrors [Runtime.GetCoinValue].
func (run *GnarkRuntime) GetCoinValue(coin *CoinField) circuit.Ext {
	v, ok := run.coins[coin.Context.ID]
	if !ok {
		panic(fmt.Sprintf("wiop: GnarkRuntime: coin %q has not been sampled yet", coin.Context.Path()))
	}
	return v
}

// GetCommitment returns the transported commitment of round roundID.
func (run *GnarkRuntime) GetCommitment(roundID int) circuit.Octuplet {
	v, ok := run.commitments[roundID]
	if !ok {
		panic(fmt.Sprintf("wiop: GnarkRuntime: commitment for round %d not found", roundID))
	}
	return v
}

// EvaluateSingle evaluates a scalar expression in-circuit. Cells, coins,
// scalar constants and arithmetic nodes are handled here; any other leaf is
// delegated to resolveLeaf, which returns (value, true) when it recognises
// the node. It is the counterpart of [Expression.EvaluateSingle] for the
// verifier circuit.
func (run *GnarkRuntime) EvaluateSingle(
	expr Expression, resolveLeaf func(Expression) (circuit.Ext, bool),
) circuit.Ext {
	if resolveLeaf != nil {
		if v, ok := resolveLeaf(expr); ok {
			return v
		}
	}
	api := run.api
	switch e := expr.(type) {
	case *Constant:
		if e.IsMultiValued() {
			panic("wiop: GnarkRuntime.EvaluateSingle: vector constant in a scalar expression")
		}
		return api.ConstExt(field.Lift(e.Value))
	case *Cell:
		return run.GetCellValue(e)
	case *CoinField:
		return run.GetCoinValue(e)
	case *ArithmeticOperation:
		eval := func(i int) circuit.Ext { return run.EvaluateSingle(e.Operands[i], resolveLeaf) }
		switch e.Operator {
		case ArithmeticOperatorAdd:
			return api.AddExt(eval(0), eval(1))
		case ArithmeticOperatorSub:
			return api.SubExt(eval(0), eval(1))
		case ArithmeticOperatorMul:
			return api.MulExt(eval(0), eval(1))
		case ArithmeticOperatorDiv:
			return api.DivExt(eval(0), eval(1))
		case ArithmeticOperatorDouble:
			return api.DoubleExt(eval(0))
		case ArithmeticOperatorSquare:
			return api.SquareExt(eval(0))
		case ArithmeticOperatorNegate:
			return api.NegExt(eval(0))
		case ArithmeticOperatorInverse:
			return api.InverseExt(eval(0))
		default:
			panic(fmt.Sprintf("wiop: GnarkRuntime.EvaluateSingle: unknown operator %v", e.Operator))
		}
	default:
		panic(fmt.Sprintf("wiop: GnarkRuntime.EvaluateSingle: unsupported expression type %T", expr))
	}
}
