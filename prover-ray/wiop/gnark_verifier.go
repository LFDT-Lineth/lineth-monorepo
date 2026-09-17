package wiop

import (
	"fmt"

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
// Visibility: every field is private. Exposing the public inputs (and a digest
// of the System) is the job of the outer circuit that embeds a VerifierCircuit.
//
// Constraint budget: dominated by the PCS action (Poseidon2 Merkle paths and
// extension-field DEEP quotients, scaling with the FRI query count); the
// transcript replay and vanishing checks are comparatively small.
//
// Limitations of this first iteration, all enforced by
// [AllocateVerifierCircuit] failing closed:
//   - no dynamic-size modules (their sizes change the circuit shape);
//   - no pre-sampling hooks;
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
	Commitments []poseidon2.KoalagnarkOctuplet
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
// carry a commitment, and whether a PCS opening proof is present. It is
// derived from a template proof at allocation time and enforced on every
// assignment.
type verifierLayout struct {
	cells       map[ObjectID]cellSlot
	commitments []int // round IDs carrying a commitment, in order
	hasPCS      bool
}

// cellSlot locates one cell in the witness: the (isBase, isPublic) pair picks
// the slice, index the position within it.
//
// isBase is a property of the proof value (see [field.Gen.IsBase]), not of the
// cell declaration, which is why the layout cannot be derived from the System
// alone. It is load-bearing beyond slice selection: [Runtime.AdvanceRound]
// absorbs a base cell as one field element and an extension cell as six, so a
// proof whose tags differ from the template's would replay a different
// transcript.
type cellSlot struct {
	isBase   bool
	isPublic bool
	index    int
}

// AllocateVerifierCircuit builds the unassigned circuit for sys, using
// template (an honest proof of sys with its public inputs) to fix the witness
// shape: cell field tags, commitment count and FRI opening-proof geometry.
//
// It panics if sys uses a feature the circuit does not support yet, or if a
// registered verifier action does not implement [GnarkVerifierAction]: the
// check it enforces would otherwise be silently dropped from the circuit.
func AllocateVerifierCircuit(sys *System, template Proof, templatePub PublicInput) *VerifierCircuit {
	checkCircuitSupport(sys)
	return buildVerifierCircuit(sys, nil, template, templatePub, false)
}

// AssignVerifierCircuit returns the witness assignment of the circuit produced
// by [AllocateVerifierCircuit] for (proof, pub). It panics if the proof does
// not have the shape the circuit was allocated with.
func (c *VerifierCircuit) AssignVerifierCircuit(proof Proof, pub PublicInput) *VerifierCircuit {
	return buildVerifierCircuit(c.spec, c.layout, proof, pub, true)
}

// System returns the compiled system the circuit verifies.
func (c *VerifierCircuit) System() *System { return c.spec }

func checkCircuitSupport(sys *System) {
	for _, m := range sys.Modules {
		if m.IsDynamic() {
			panic(fmt.Sprintf("wiop: VerifierCircuit: dynamic module %q is not supported", m.Context.Path()))
		}
	}
	for _, r := range sys.Rounds {
		if len(r.PreSamplingHooks) > 0 {
			panic(fmt.Sprintf("wiop: VerifierCircuit: round %d has pre-sampling hooks, not supported", r.ID))
		}
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

// buildVerifierCircuit walks the system exactly like [System.Verify] and fills
// the witness slices. With layout == nil the layout is derived from the proof
// (allocation); otherwise the proof is checked against it (assignment).
// withValues selects between zero placeholders and actual assignments.
func buildVerifierCircuit(
	sys *System, layout *verifierLayout, proof Proof, pub PublicInput, withValues bool,
) *VerifierCircuit {
	c := &VerifierCircuit{spec: sys}
	allocating := layout == nil
	if allocating {
		layout = &verifierLayout{cells: make(map[ObjectID]cellSlot)}
	}
	c.layout = layout

	if len(pub) != len(sys.PublicInputs) {
		panic(fmt.Sprintf("wiop: VerifierCircuit: public inputs length mismatch: got %d, want %d",
			len(pub), len(sys.PublicInputs)))
	}
	piIdx := sys.publicInputIndex()

	for _, r := range sys.Rounds {
		for _, cell := range r.Cells {
			var v field.Gen
			pos, isPI := piIdx[cell.Context.ID]
			if isPI {
				v = pub[pos]
			} else {
				var ok bool
				if v, ok = proof.Cells[cell.Context.ID]; !ok {
					panic(fmt.Sprintf("wiop: VerifierCircuit: cell %q not found in proof", cell.Context.Path()))
				}
			}
			c.placeCell(cell, v, isPI, allocating, withValues)
		}
		if r.HasCommitment {
			commitment, ok := proof.Commitments[r.ID]
			if !ok {
				panic(fmt.Sprintf("wiop: VerifierCircuit: commitment for round %d not found in proof", r.ID))
			}
			if allocating {
				layout.commitments = append(layout.commitments, r.ID)
			}
			if withValues {
				c.Commitments = append(c.Commitments, poseidon2.NewKoalagnarkOctuplet(commitment))
			} else {
				c.Commitments = append(c.Commitments, poseidon2.KoalagnarkOctuplet{})
			}
		}
	}

	if allocating {
		layout.hasPCS = proof.PCSOpeningProof != nil
	}
	if layout.hasPCS != (proof.PCSOpeningProof != nil) {
		panic("wiop: VerifierCircuit: proof and circuit disagree on the presence of a PCS opening proof")
	}
	if layout.hasPCS {
		if withValues {
			c.PCSOpeningProof = fri.NewGnarkOpeningProof(*proof.PCSOpeningProof)
		} else {
			c.PCSOpeningProof = fri.AllocateGnarkOpeningProof(*proof.PCSOpeningProof)
		}
	}
	return c
}

// placeCell appends v to the slice its (field, visibility) pair selects and
// records or checks the cell's slot.
func (c *VerifierCircuit) placeCell(cell *Cell, v field.Gen, isPublic, allocating, withValues bool) {
	id := cell.Context.ID
	slot := cellSlot{isBase: v.IsBase(), isPublic: isPublic}
	switch {
	case slot.isBase && isPublic:
		slot.index = len(c.PublicInputsBase)
		c.PublicInputsBase = append(c.PublicInputsBase, newElementWitness(v, withValues))
	case slot.isBase:
		slot.index = len(c.CellsBase)
		c.CellsBase = append(c.CellsBase, newElementWitness(v, withValues))
	case isPublic:
		slot.index = len(c.PublicInputsExt)
		c.PublicInputsExt = append(c.PublicInputsExt, newExtWitness(v, withValues))
	default:
		slot.index = len(c.CellsExt)
		c.CellsExt = append(c.CellsExt, newExtWitness(v, withValues))
	}

	if allocating {
		c.layout.cells[id] = slot
		return
	}
	if want := c.layout.cells[id]; want != slot {
		panic(fmt.Sprintf(
			"wiop: VerifierCircuit: cell %q does not match the allocated layout (base=%v public=%v index=%d), "+
				"got (base=%v public=%v index=%d)",
			cell.Context.Path(), want.isBase, want.isPublic, want.index, slot.isBase, slot.isPublic, slot.index,
		))
	}
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

	api          *circuit.API
	fs           *fiatshamir.GnarkFiatShamir
	cellsBase    map[ObjectID]circuit.Element
	cells        map[ObjectID]circuit.Ext
	coins        map[ObjectID]circuit.Ext
	commitments  map[int]poseidon2.KoalagnarkOctuplet
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
		commitments: make(map[int]poseidon2.KoalagnarkOctuplet),
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
func (run *GnarkRuntime) API() *circuit.API { return run.api }

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
func (run *GnarkRuntime) GetCommitment(roundID int) poseidon2.KoalagnarkOctuplet {
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
