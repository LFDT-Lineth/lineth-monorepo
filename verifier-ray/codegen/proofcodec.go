package codegen

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// This file implements the binary wire format verifier-ray's Zig decoder
// (src/proof_codec.zig) reads: a schema-driven, length-prefixed encoding of
// wiop.Proof + wiop.PublicInput into the shape verifier-ray/src/verifier.zig's
// Proof/PublicInput expect (rounds/cells split, PcsOpening, entry_claims,
// module_sizes, public_inputs).
//
// It lives in verifier-ray/codegen (not prover-ray) because entry_claims —
// the prover-supplied claimed evaluations in canonical layout order — can only
// be derived via BuildPcsSystem's own canonical-ordering logic (colLoc,
// computePcsLayout, etc., all unexported), which prover-ray cannot see:
// verifier-ray/codegen is the one package that already depends on prover-ray
// AND owns that canonical ordering. Reusing BuildPcsSystem here means the
// encoder can never drift from what codegen assumed when it built
// WitnessMap/QuotientMap.
//
// No self-describing type tags are used except for Scalar (base vs extension
// field is a genuine per-cell runtime fact, not a static schema property) —
// every other field's shape is fixed at compile time and shared between this
// encoder and the Zig decoder by construction. Every variable-length list is
// a little-endian u32 count followed by that many encodings back-to-back;
// nested variable-length elements are self-delimiting recursively.
//
// Leaf encodings mirror the existing canonical byte conventions already used
// elsewhere in prover-ray/verifier-ray:
//   - field.Element: 4-byte big-endian canonical representative
//     (field.Element.Bytes(), the same convention field.OctupletToBytes and
//     verifier-ray's Element.fromBytesCanonical/toBytes already use).
//   - field.Ext: 6 consecutive field.Element encodings, in the coordinate
//     order Ext.{B0,B1,B2}.{A0,A1} — matching verifier-ray's
//     Ext{B0,B1,B2: E2{a0,a1}} layout exactly.
//   - field.Octuplet (Digest/Commitment): 8 consecutive field.Element
//     encodings in index order.

const (
	scalarTagBase = 0
	scalarTagExt  = 1
)

type proofEncoder struct {
	buf []byte
}

func (e *proofEncoder) writeByte(b byte) {
	e.buf = append(e.buf, b)
}

func (e *proofEncoder) writeBool(b bool) {
	if b {
		e.writeByte(1)
	} else {
		e.writeByte(0)
	}
}

func (e *proofEncoder) writeCount(n int) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], uint32(n))
	e.buf = append(e.buf, b[:]...)
}

func (e *proofEncoder) writeUsize(n int) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], uint64(n))
	e.buf = append(e.buf, b[:]...)
}

func (e *proofEncoder) writeElement(el field.Element) {
	b := el.Bytes()
	e.buf = append(e.buf, b[:]...)
}

func (e *proofEncoder) writeExt(x field.Ext) {
	e.writeElement(x.B0.A0)
	e.writeElement(x.B0.A1)
	e.writeElement(x.B1.A0)
	e.writeElement(x.B1.A1)
	e.writeElement(x.B2.A0)
	e.writeElement(x.B2.A1)
}

func (e *proofEncoder) writeOctuplet(o field.Octuplet) {
	for _, el := range o {
		e.writeElement(el)
	}
}

// writeScalar encodes a field.Gen as a Scalar: one tag byte, then the base or
// extension payload.
func (e *proofEncoder) writeScalar(v field.Gen) {
	if v.IsBase() {
		e.writeByte(scalarTagBase)
		e.writeElement(v.AsBase())
		return
	}
	e.writeByte(scalarTagExt)
	e.writeExt(v.AsExt())
}

func (e *proofEncoder) writeElements(els []field.Element) {
	e.writeCount(len(els))
	for _, el := range els {
		e.writeElement(el)
	}
}

func (e *proofEncoder) writeExts(xs []field.Ext) {
	e.writeCount(len(xs))
	for _, x := range xs {
		e.writeExt(x)
	}
}

func (e *proofEncoder) writeOctuplets(os []field.Octuplet) {
	e.writeCount(len(os))
	for _, o := range os {
		e.writeOctuplet(o)
	}
}

func (e *proofEncoder) writeUsizes(ns []int) {
	e.writeCount(len(ns))
	for _, n := range ns {
		e.writeUsize(n)
	}
}

// EncodeProof serializes proof + pub into the wire format
// verifier-ray/src/proof_codec.zig decodes into a verifier.VerifyInput.
//
// sys and rt must be the exact wiop.System/Runtime that produced proof/pub
// (the same instance passed to sys.Prove, and a runtime that has completed a
// full proving pass) — the encoder needs sys to recover the Zig-shaped
// per-round cell ordering (wiop.Proof.Cells is a flat map[ObjectID]field.Gen;
// the round/cell declaration order lives on sys, not on the proof) and needs
// rt to call BuildPcsSystem for the canonical-order entry_claims (the
// prover-supplied claimed evaluations verifier-ray's PCS sub-verifier reads
// directly, never re-derived at verify time the way prover-ray's own Go
// verifier does).
func EncodeProof(sys *wiop.System, rt *wiop.Runtime, routing CoinRouting, proof wiop.Proof, pub wiop.PublicInput) ([]byte, error) {
	e := &proofEncoder{}

	if err := e.writeRounds(sys, proof); err != nil {
		return nil, fmt.Errorf("codegen: EncodeProof: rounds: %w", err)
	}
	e.writeModuleSizes(sys, rt)
	if err := e.writePcsOpening(sys, rt, routing, proof); err != nil {
		return nil, fmt.Errorf("codegen: EncodeProof: pcs opening: %w", err)
	}
	e.writePublicInput(pub)

	return e.buf, nil
}

// WriteProof is EncodeProof followed by a write to w, for callers (e.g. a
// driver program) that want the bytes written directly to a file.
func WriteProof(w io.Writer, sys *wiop.System, rt *wiop.Runtime, routing CoinRouting, proof wiop.Proof, pub wiop.PublicInput) error {
	b, err := EncodeProof(sys, rt, routing, proof, pub)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// writeRounds encodes proof.rounds: one RoundMessage per sys.Rounds entry,
// mirroring prover-ray's own Prove/Verify per-round cell ordering (walk
// round.Cells in declaration order, skipping cells promoted to public inputs
// — the same skip logic Prove itself applies when filling Proof.Cells) and
// testdata/generate/main.go's runtimeTraceRoundFromRuntime, which the fixture
// generator already uses for the identical purpose.
func (e *proofEncoder) writeRounds(sys *wiop.System, proof wiop.Proof) error {
	piIdx := publicInputIndexForEncoding(sys)

	// The last round is never replayed: protocol.replayWithTranscript expects
	// exactly len(spec.round_coin_counts)-1 rounds (the last slot holds the
	// PCS opening machinery, not transcript cells), and
	// codegen.BuildPublicInputSystem's RoundCellCounts is sized identically
	// (len(sys.Rounds)-1). Mirror that exclusion here so the encoded round
	// count matches what the comptime system/spec expect.
	replayedRounds := sys.Rounds
	if len(replayedRounds) > 0 {
		replayedRounds = replayedRounds[:len(replayedRounds)-1]
	}

	e.writeCount(len(replayedRounds))
	for _, round := range replayedRounds {
		if round.HasCommitment {
			commitment, ok := proof.Commitments[round.ID]
			if !ok {
				return fmt.Errorf("round %d has HasCommitment but no commitment in proof.Commitments", round.ID)
			}
			e.writeBool(true)
			e.writeOctuplet(commitment)
		} else {
			e.writeBool(false)
		}

		cells := make([]field.Gen, 0, len(round.Cells))
		for _, cell := range round.Cells {
			if _, isPI := piIdx[cell.Context.ID]; isPI {
				continue
			}
			v, ok := proof.Cells[cell.Context.ID]
			if !ok {
				return fmt.Errorf("round %d: cell %q has no value in proof.Cells", round.ID, cell.Context.Path())
			}
			cells = append(cells, v)
		}
		e.writeCount(len(cells))
		for _, v := range cells {
			e.writeScalar(v)
		}
	}
	return nil
}

// publicInputIndexForEncoding mirrors wiop.System's own unexported
// publicInputIndex: a map from each registered public-input cell's ObjectID
// to its position in sys.PublicInputs. Re-derived here (rather than imported)
// because the original is unexported and this package cannot reach into wiop
// internals.
func publicInputIndexForEncoding(sys *wiop.System) map[wiop.ObjectID]int {
	idx := make(map[wiop.ObjectID]int, len(sys.PublicInputs))
	for i, cell := range sys.PublicInputs {
		idx[cell.Context.ID] = i
	}
	return idx
}

// writeModuleSizes writes rt's dynamic-module sizes in DynamicModuleOrder —
// the same order every dynamic column's DynamicIndex refers to — via
// DynamicModuleSizes, which reads them straight off rt (this proof's own
// runtime), rather than re-deriving them from wiop.Proof.DynamicSizes by hand.
func (e *proofEncoder) writeModuleSizes(sys *wiop.System, rt *wiop.Runtime) {
	e.writeUsizes(DynamicModuleSizes(sys, rt))
}

// writePcsOpening encodes proof.PCSOpeningProof as the PcsOpening shape
// verifier.zig expects: entry_claims (from ExtractPcsOpening, in this proof's
// canonical layout order) followed by the recursive OpeningProof/fri.Proof/
// merkle.InputTreeOpening tree.
func (e *proofEncoder) writePcsOpening(sys *wiop.System, rt *wiop.Runtime, routing CoinRouting, proof wiop.Proof) error {
	if proof.PCSOpeningProof == nil {
		return fmt.Errorf("proof has no PCSOpeningProof (was pcs.Compile run on this system?)")
	}

	pcsSys, err := BuildPcsSystem(sys, routing)
	if err != nil {
		return fmt.Errorf("BuildPcsSystem: %w", err)
	}
	entryClaims, err := ExtractPcsOpening(pcsSys, rt)
	if err != nil {
		return fmt.Errorf("ExtractPcsOpening: %w", err)
	}

	e.writeCount(len(entryClaims))
	for _, row := range entryClaims {
		e.writeExts(row)
	}

	op := proof.PCSOpeningProof
	e.writeInputQueries(op.InputQueries)
	return e.writeFriProof(op.FRIProof)
}

// writeInputQueries encodes []fri.InputQuery ([query][distinct_tree]) as
// verifier-ray's input_queries: []const []const merkle.InputTreeOpening.
func (e *proofEncoder) writeInputQueries(queries []fri.InputQuery) {
	e.writeCount(len(queries))
	for _, q := range queries {
		e.writeCount(len(q))
		for _, opening := range q {
			e.writeInputTreeOpening(opening)
		}
	}
}

// writeInputTreeOpening encodes fri.InputTreeOpening as merkle.InputTreeOpening
// { siblings: []Digest, leaves: []?RowPair }.
func (e *proofEncoder) writeInputTreeOpening(opening fri.InputTreeOpening) {
	e.writeOctuplets(opening.Siblings)

	e.writeCount(len(opening.Leaves))
	for _, leaf := range opening.Leaves {
		if leaf == nil {
			e.writeBool(false)
			continue
		}
		e.writeBool(true)
		e.writeRowPair(*leaf)
	}
}

// writeRowPair encodes fri.RowPair ([2]RowOpening) as merkle.RowPair.
func (e *proofEncoder) writeRowPair(pair fri.RowPair) {
	e.writeRowOpening(pair[0])
	e.writeRowOpening(pair[1])
}

// writeRowOpening encodes fri.RowOpening as merkle.RowOpening
// { base: []Element, ext: []Ext }.
func (e *proofEncoder) writeRowOpening(row fri.RowOpening) {
	e.writeElements(row.Base)
	e.writeExts(row.Ext)
}

// writeFriProof encodes fri.Proof as verifier-ray's fri.Proof
// { round_roots: []Digest, final_poly: []Ext, running_queries: [][]Branch }.
//
// Go's RunningQueries is [query][round]QueryLayer where QueryLayer = []Branch
// — an extra nesting level shared with the general multi-tree Level
// construct. For the running-polynomial path specifically, openRunningQueryExt
// always constructs QueryLayer{path}, a single-element slice (fri.go:691), so
// each [query][round] slot holds exactly one Branch; that single Branch is
// what verifier-ray's running_queries: []const []const merkle.Branch expects
// at [query][round].
func (e *proofEncoder) writeFriProof(p fri.Proof) error {
	e.writeOctuplets(p.RoundRoots)
	e.writeExts(p.FinalPoly)

	e.writeCount(len(p.RunningQueries))
	for qi, rq := range p.RunningQueries {
		e.writeCount(len(rq))
		for ri, layer := range rq {
			if len(layer) != 1 {
				return fmt.Errorf("running query %d round %d: expected exactly one branch, got %d", qi, ri, len(layer))
			}
			e.writeBranch(layer[0])
		}
	}
	return nil
}

// writeBranch encodes fri.Branch as merkle.Branch { leaf: Digest, siblings: []Digest }.
func (e *proofEncoder) writeBranch(b fri.Branch) {
	e.writeOctuplet(b.Leaf)
	e.writeOctuplets(b.Siblings)
}

// writePublicInput encodes the flat public-input statement as []Scalar.
func (e *proofEncoder) writePublicInput(pub wiop.PublicInput) {
	e.writeCount(len(pub))
	for _, v := range pub {
		e.writeScalar(v)
	}
}
