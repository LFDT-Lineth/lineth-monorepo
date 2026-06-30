package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// This is intentionally a small, verifier-ray-local transport format. It is not
// meant to be the final proof serialization; it only gives native/R5 benchmark
// runs a way to load proof data at runtime instead of embedding large proof
// literals into the executable.
//
// All integers are little-endian (for consistency with zkc). Field elements are
// written as canonical u32 limbs. Extension elements use prover/verifier limb order:
// [B0.a0, B0.a1, B1.a0, B1.a1, B2.a0, B2.a1].
//
// Proof:
//
//	u32 round_count
//	round_count * RoundMessage
//	u32 witness_claim_count
//	witness_claim_count * Ext
//	u32 quotient_claim_count
//	quotient_claim_count * Ext
//	u32 module_size_count
//	module_size_count * u32
//
// RoundMessage:
//
//	u32 column_count
//	column_count * ColumnMessage
//	u32 cell_count
//	cell_count * Scalar
//
// ColumnMessage:
//
//	u8  tag
//	  0 = oracle commitment: Commitment
//	  1 = public base column: u32 len, len * Field
//	  2 = public extension column: u32 len, len * Ext
//
// Scalar:
//
//	u8  tag
//	  0 = base: Field
//	  1 = ext: Ext
//
// Commitment:
//
//	8 * Field
//
// Ext:
//
//	6 * Field
const (
	proofWireColumnOracleCommitment = byte(0)
	proofWireColumnPublicBase       = byte(1)
	proofWireColumnPublicExt        = byte(2)

	proofWireScalarBase = byte(0)
	proofWireScalarExt  = byte(1)
)

// serializeProof encodes a verifier proof view into proof wire format. The
// output contains only verifier-visible proof data: round messages, vanishing
// claims, quotient claims, and dynamic module sizes. It deliberately does not
// include the compiled spec/system; those stay in generated Zig metadata.
func serializeProof(proof vanishingProofView) ([]byte, error) {
	var w proofWireWriter
	w.writeLen(len(proof.rounds))
	for _, round := range proof.rounds {
		w.writeRound(round)
	}

	w.writeLen(len(proof.witnessClaims))
	for _, claim := range proof.witnessClaims {
		w.writeExt(claim)
	}

	w.writeLen(len(proof.quotientClaims))
	for _, claim := range proof.quotientClaims {
		w.writeExt(claim)
	}

	w.writeLen(len(proof.moduleSizes))
	for _, size := range proof.moduleSizes {
		w.writeIntAsU32(size, "module size")
	}

	if w.err != nil {
		return nil, w.err
	}
	return w.Bytes(), nil
}

// proofWireLayout mirrors the allocation shape required by the Zig decoder for
// one serialized proof. It counts the flat backing arrays needed to hold all
// nested slices after decoding, plus the exact byte length expected on the wire.
type proofWireLayout struct {
	roundCount           int
	columnCount          int
	cellCount            int
	publicBaseValueCount int
	publicExtValueCount  int
	witnessClaimCount    int
	quotientClaimCount   int
	moduleSizeCount      int
	// encodedSize is the total number of bytes that serializeProof will emit for one proof.
	encodedSize int
}

// proofLayout computes the Zig decoder layout for proof. It must stay in lock
// step with serializeProof: every byte accounted for here is emitted there, and
// every count here becomes a fixed-size backing array in generated verify.zig.
func proofLayout(proof vanishingProofView) proofWireLayout {
	layout := proofWireLayout{
		roundCount:         len(proof.rounds),
		witnessClaimCount:  len(proof.witnessClaims),
		quotientClaimCount: len(proof.quotientClaims),
		moduleSizeCount:    len(proof.moduleSizes),
		encodedSize:        4,
	}

	for _, round := range proof.rounds {
		layout.encodedSize += 4
		for _, column := range round.columns {
			switch {
			case column.commitments != nil:
				layout.columnCount += len(column.commitments)
				layout.encodedSize += len(column.commitments) * (1 + 8*4)
			case column.publicBaseValues != nil:
				layout.columnCount++
				layout.publicBaseValueCount += len(column.publicBaseValues)
				layout.encodedSize += 1 + 4 + len(column.publicBaseValues)*4
			case column.publicExtValues != nil:
				layout.columnCount++
				layout.publicExtValueCount += len(column.publicExtValues)
				layout.encodedSize += 1 + 4 + len(column.publicExtValues)*6*4
			}
		}

		layout.encodedSize += 4
		layout.cellCount += len(round.cells)
		for _, cell := range round.cells {
			switch {
			case cell.baseValue != nil:
				layout.encodedSize += 1 + 4
			case cell.extValue != nil:
				layout.encodedSize += 1 + 6*4
			}
		}
	}

	layout.encodedSize += 4 + layout.witnessClaimCount*6*4
	layout.encodedSize += 4 + layout.quotientClaimCount*6*4
	layout.encodedSize += 4 + layout.moduleSizeCount*4

	return layout
}

// proofWireWriter accumulates serialized bytes while preserving the first
// validation error. The write methods can be called linearly without threading
// errors through every small field write.
type proofWireWriter struct {
	bytes.Buffer
	err error
}

func (w *proofWireWriter) writeRound(round runtimeTraceRound) {
	w.writeLen(proofWireColumnCount(round))
	for _, column := range round.columns {
		w.writeColumn(column)
	}

	w.writeLen(len(round.cells))
	for _, cell := range round.cells {
		w.writeScalar(cell)
	}
}

// proofWireColumnCount returns the number of ColumnMessage entries a round will
// occupy after expansion. Oracle columns can contain multiple commitments, and
// each commitment is serialized as its own column message.
func proofWireColumnCount(round runtimeTraceRound) int {
	count := 0
	for _, column := range round.columns {
		if column.commitments != nil {
			count += len(column.commitments)
		} else {
			count++
		}
	}
	return count
}

func (w *proofWireWriter) writeColumn(column runtimeTraceColumn) {
	switch {
	case column.commitments != nil:
		for _, commitment := range column.commitments {
			w.writeU8(proofWireColumnOracleCommitment)
			w.writeCommitment(commitment)
		}
	case column.publicBaseValues != nil:
		w.writeU8(proofWireColumnPublicBase)
		w.writeLen(len(column.publicBaseValues))
		for _, value := range column.publicBaseValues {
			w.writeField(value)
		}
	case column.publicExtValues != nil:
		w.writeU8(proofWireColumnPublicExt)
		w.writeLen(len(column.publicExtValues))
		for _, value := range column.publicExtValues {
			w.writeExt(value)
		}
	default:
		w.setErr("runtime trace column has no data variant")
	}
}

func (w *proofWireWriter) writeScalar(cell runtimeTraceCell) {
	switch {
	case cell.baseValue != nil && cell.extValue == nil:
		w.writeU8(proofWireScalarBase)
		w.writeField(*cell.baseValue)
	case cell.extValue != nil && cell.baseValue == nil:
		w.writeU8(proofWireScalarExt)
		w.writeExt(*cell.extValue)
	default:
		w.setErr("runtime trace cell must have exactly one data variant")
	}
}

func (w *proofWireWriter) writeCommitment(value field.Octuplet) {
	for _, limb := range value {
		w.writeField(limb)
	}
}

func (w *proofWireWriter) writeExt(value field.Ext) {
	a0, a1, b0, b1, c0, c1 := field.ExtToUint64s(&value)
	w.writeU32(uint32(a0))
	w.writeU32(uint32(a1))
	w.writeU32(uint32(b0))
	w.writeU32(uint32(b1))
	w.writeU32(uint32(c0))
	w.writeU32(uint32(c1))
}

func (w *proofWireWriter) writeField(value field.Element) {
	w.writeU32(uint32(u(value)))
}

func (w *proofWireWriter) writeLen(n int) {
	w.writeIntAsU32(n, "length")
}

func (w *proofWireWriter) writeIntAsU32(n int, label string) {
	if n < 0 || n > math.MaxUint32 {
		w.setErr("%s %d does not fit in u32", label, n)
		return
	}
	w.writeU32(uint32(n))
}

func (w *proofWireWriter) writeU8(value byte) {
	if w.err != nil {
		return
	}
	_ = w.WriteByte(value)
}

func (w *proofWireWriter) writeU32(value uint32) {
	if w.err != nil {
		return
	}
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], value)
	_, _ = w.Write(buf[:])
}

func (w *proofWireWriter) setErr(format string, args ...any) {
	if w.err == nil {
		w.err = fmt.Errorf(format, args...)
	}
}
