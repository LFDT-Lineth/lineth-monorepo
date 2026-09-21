package gkr

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"io"
	"maps"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// Encoding of a compiled circuit, following gnark's internal/gkr/gkrcore/serialize.go.
//
// This is write-only and lossy by design: its sole purpose is to give the calling
// protocol a digest to bind the statement into its transcript. Nothing in
// prover-ray persists a compiled protocol — wiop rebuilds its System in-process at
// startup and emits the prover sequence through wiop/codegen — so there is no
// counterpart reader, and derived metadata that can be recovered from the bytecode
// (a gate's degree and input count) is omitted.
//
// The encoding is compact (uint16 for counts and indices) and little-endian for
// scalars; extension constants are written as canonical big-endian coefficients.
//
//	Compiled:    [circuit] [schedule] [positions]
//
//	Circuit:     [wire_count:u16] [wire...]
//	Wire:        [input_count:u16] [input_indices:u16...] [exported:bool] [gate]
//	Gate:        (non-input wires only)
//	             [const_count:u16] [constant...] [inst_count:u16] [instruction...]
//	Constant:    [b0a0:u32] [b0a1:u32] [b1a0:u32] [b1a1:u32] [b2a0:u32] [b2a1:u32]
//	Instruction: [op:u8] [input_count:u16] [input_indices:u16...]
//
//	Schedule:    [level_count:u16] [level...]
//	Level:       [level_type:u8] [group_count:u16] [claim_group...]
//	             level_type: 0=sumcheck, 1=skip, 2=singleSourceZeroCheck
//	ClaimGroup:  [wire_count:u16] [wire_indices:u16...] [source_count:u16] [claim_source...]
//	ClaimSource: [level:u16] [outgoing_claim_index:u16]
//
//	Positions:   [count:u16] [entry...]
//	Entry:       [identifier:u64] [variable:u16]
//
// The identifier table is included because it is part of the statement: the same
// topology with different identifiers binds different columns to different wires.

func writeUint8(w io.Writer, x int) error {
	if x >= 256 || x < 0 {
		return fmt.Errorf("%d out of range", x)
	}
	_, err := w.Write([]byte{byte(x)})
	return err
}

func writeUint16[T int | uint16](w io.Writer, x T) error {
	var buf [2]byte
	if int(x) >= 65536 || x < 0 {
		return fmt.Errorf("%d out of range", x)
	}
	binary.LittleEndian.PutUint16(buf[:], uint16(x))
	_, err := w.Write(buf[:])
	return err
}

func writeUint64(w io.Writer, x uint64) error {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], x)
	_, err := w.Write(buf[:])
	return err
}

func writeBool(w io.Writer, b bool) error {
	var v byte
	if b {
		v = 1
	}
	_, err := w.Write([]byte{v})
	return err
}

func writeUint16Slice[T int | uint16](w io.Writer, s []T) error {
	if err := writeUint16(w, len(s)); err != nil {
		return err
	}
	for _, v := range s {
		if err := writeUint16(w, v); err != nil {
			return err
		}
	}
	return nil
}

// writeExt writes the six canonical base coefficients of x, four bytes each.
func writeExt(w io.Writer, x field.Ext) error {
	for _, c := range [...]field.Element{x.B0.A0, x.B0.A1, x.B1.A0, x.B1.A1, x.B2.A0, x.B2.A1} {
		b := c.Bytes()
		if _, err := w.Write(b[:]); err != nil {
			return err
		}
	}
	return nil
}

// countingWriter tracks how many bytes were written, so WriteTo can report it.
type countingWriter struct {
	w io.Writer
	n int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	n, err := c.w.Write(p)
	c.n += int64(n)
	return n, err
}

// WriteTo writes the deterministic encoding of c. It is the input to Digest and is
// not readable back into a Compiled.
func (c *Compiled) WriteTo(w io.Writer) (int64, error) {
	cw := countingWriter{w: w}
	if err := writeCircuit(&cw, c.circuit); err != nil {
		return cw.n, err
	}
	if err := writeSchedule(&cw, c.schedule); err != nil {
		return cw.n, err
	}
	err := writePositions(&cw, c.positions)
	return cw.n, err
}

// Digest is a SHA-256 commitment to the circuit, its proving schedule and its
// identifier table, reduced into eight field elements. The calling protocol must
// absorb it into the transcript before drawing any GKR challenge, so that the
// challenges are bound to the statement:
//
//	d := compiled.Digest()
//	fs.Update(d[:]...)
//
// A plain hash suffices because the digest is never recomputed under a proof: the
// prover derives it from the compiled circuit and the verifier carries it as a
// build-time constant.
func (c *Compiled) Digest() field.Octuplet {
	h := sha256.New()
	if _, err := c.WriteTo(h); err != nil {
		panic(fmt.Sprintf("gkr: digest: %v", err))
	}
	sum := h.Sum(nil)

	var res field.Octuplet
	for i := range res {
		res[i].SetBytes(sum[i*4 : (i+1)*4])
	}
	return res
}

func writeCircuit(w io.Writer, c Circuit) error {
	if len(c) >= 1<<16 {
		return fmt.Errorf("circuit length too large: %d", len(c))
	}
	if err := writeUint16(w, len(c)); err != nil {
		return err
	}

	for i := range c {
		wire := &c[i]

		if err := writeUint16Slice(w, wire.Inputs); err != nil {
			return err
		}
		if err := writeBool(w, wire.Exported); err != nil {
			return err
		}
		if wire.IsInput() {
			continue
		}

		bytecode := &wire.Gate.Evaluate

		if err := writeUint16(w, len(bytecode.Constants)); err != nil {
			return err
		}
		for _, constant := range bytecode.Constants {
			if err := writeExt(w, constant); err != nil {
				return err
			}
		}
		if err := writeUint16(w, len(bytecode.Instructions)); err != nil {
			return err
		}
		for _, inst := range bytecode.Instructions {
			if _, err := w.Write([]byte{byte(inst.Op)}); err != nil {
				return err
			}
			if err := writeUint16Slice(w, inst.Inputs); err != nil {
				return err
			}
		}
	}

	return nil
}

func writeSchedule(w io.Writer, s ProvingSchedule) error {
	if len(s) >= 1<<16 {
		return fmt.Errorf("schedule length too large: %d", len(s))
	}

	writeClaimGroup := func(cg ClaimGroup) error {
		if err := writeUint16Slice(w, cg.Wires); err != nil {
			return err
		}
		if err := writeUint16(w, len(cg.ClaimSources)); err != nil {
			return err
		}
		for _, src := range cg.ClaimSources {
			if err := writeUint16(w, src.Level); err != nil {
				return err
			}
			if err := writeUint16(w, src.OutgoingClaimIndex); err != nil {
				return err
			}
		}
		return nil
	}

	if err := writeUint16(w, len(s)); err != nil {
		return err
	}

	for _, level := range s {
		var levelType int
		switch level.(type) {
		case *SumcheckLevel:
			levelType = 0
		case *SkipLevel:
			levelType = 1
		case *SingleSourceZeroCheckLevel:
			levelType = 2
		default:
			return fmt.Errorf("unknown proving level type %T", level)
		}
		if err := writeUint8(w, levelType); err != nil {
			return err
		}
		groups := level.ClaimGroups()
		if err := writeUint16(w, len(groups)); err != nil {
			return err
		}
		for _, cg := range groups {
			if err := writeClaimGroup(cg); err != nil {
				return err
			}
		}
	}

	return nil
}

// writePositions emits the identifier table in ascending identifier order, so that
// the encoding does not depend on Go's map iteration order.
func writePositions(w io.Writer, positions map[Identifier]Variable) error {
	if len(positions) >= 1<<16 {
		return fmt.Errorf("too many identifiers: %d", len(positions))
	}
	if err := writeUint16(w, len(positions)); err != nil {
		return err
	}
	for _, id := range slices.Sorted(maps.Keys(positions)) {
		if err := writeUint64(w, uint64(id)); err != nil {
			return err
		}
		if err := writeUint16(w, int(positions[id])); err != nil {
			return err
		}
	}
	return nil
}
