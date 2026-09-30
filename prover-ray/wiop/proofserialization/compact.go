package proofserialization

import (
	"encoding/binary"
	"fmt"
)

// Guest image consumed by verifier-ray's loaders (main.zig, the R5 input
// region, and test/riscv_proof_image_test.zig).
//
// This is not the cast layout Encode writes. Encode remains the pointer image
// that proof_abi.zig and image_relocation.zig pin; a guest has to walk that
// image by absolute address. EncodeGuest is pointer-free and self-delimiting,
// so the same bytes are valid at any host address and at _in_start. The
// verifier expands them back into a VerifyInput before hashing, which keeps
// every opened row — including its zero limbs — in the Merkle preimage.
//
// What the expansion is allowed to omit, because the verifier rebuilds it:
//   - a zero base limb or an all-zero extension, replaced by a presence bit
//   - a second copy of a RowOpening already stored in the unique-row pool
//   - null RowPair slots and null auxiliary digests, replaced by a presence bit
//   - slice headers, because every width is written next to its payload
//
// Layout, little-endian. Multi-byte integers are unsigned. A bitset is
// ceil(n/8) bytes, bit i in byte i>>3 at mask 1<<(i&7), and the unused high
// bits of a partial last byte are zero. A count is a u32 and must be <=
// maxGuestItems. A zero count occupies no further bytes and decodes as a nil
// slice. The declared image length may be shorter than the buffer that holds
// it (the guest input region is larger than the image); bytes past it are
// ignored. Bytes inside it must be consumed exactly.
//
//	u32 magic = 'LPR1'
//	u32 image_len            // total bytes, including this 8-byte header
//	u32 n_rows               // unique RowOpenings, pool index order
//	repeated n_rows:
//	    u16 n_base, u16 n_ext
//	    bitset n_base        // 1 = nonzero limb follows
//	    u32 * popcount
//	    bitset n_ext         // 1 = nonzero extension follows
//	    6*u32 * popcount
//	u32 n_rounds
//	repeated:
//	    scalar_slice         // cells
//	    u8 commitment_flag   // 0 absent, 1 present
//	    if present: digest
//	u32 n_module_sizes
//	repeated: u64
//	scalar_slice             // public inputs
//	u32 n_queries
//	repeated query:
//	    u32 n_trees
//	    repeated tree:
//	        digest_slice     // siblings
//	        u32 n_levels     // full height, including leading nulls
//	        bitset n_levels  // 1 = RowPair present
//	        repeated present: u32 half0, u32 half1  // indexes into n_rows
//	u32 n_caps
//	repeated cap:
//	    digest_slice         // nodes
//	    u32 n_tables
//	    repeated table:
//	        u8 size_log2
//	        u32 n_rows
//	        repeated: u32 row index
//	digest_slice             // FRI round roots
//	u32 n_fri_caps
//	repeated:
//	    digest_slice         // nodes
//	    optional_digests     // aux
//	u32 n_final
//	repeated: 6*u32
//	u32 n_running_queries
//	repeated:
//	    u32 n_branches
//	    repeated branch:
//	        digest_slice     // siblings
//	        digest           // leaf
//
// scalar_slice is: u32 n, bitset n (1 = extension), then n times 6*u32.
// A base scalar still stores all six limbs, matching the cast image, where
// the high limbs are padding the verifier does not read.
// digest is 8*u32. digest_slice is u32 n, then n digests.
// optional_digests is u32 n, bitset n, then a digest for each set bit.
const guestMagic = "LPR1"

// maxGuestItems bounds every u32 count in a guest image. The RISC-V proof
// sits far under this; the cap is what stops a hostile header from forcing
// a multi-gigabyte allocation.
const maxGuestItems = 1 << 20

// EncodeGuest serializes input as the pointer-free guest image described above.
func EncodeGuest(input VerifyInput) ([]byte, error) {
	pool, err := collectRows(input)
	if err != nil {
		return nil, err
	}
	var w guestWriter
	if err := w.rowSection(pool.rows); err != nil {
		return nil, err
	}
	if err := w.rounds(input.Proof.Rounds); err != nil {
		return nil, err
	}
	if err := w.moduleSizes(input.Proof.ModuleSizes); err != nil {
		return nil, err
	}
	if err := w.scalarSlice(input.PublicInputs); err != nil {
		return nil, err
	}
	if err := w.queries(input.Proof.PcsOpening.InputQueries, pool); err != nil {
		return nil, err
	}
	if err := w.inputCaps(input.Proof.PcsOpening.InputCaps, pool); err != nil {
		return nil, err
	}
	if err := w.fri(input.Proof.PcsOpening.FriProof); err != nil {
		return nil, err
	}
	if len(w.buf) > MaxImageSize-8 {
		return nil, checkImageSize(len(w.buf) + 8)
	}
	out := make([]byte, 8+len(w.buf))
	copy(out, guestMagic)
	binary.LittleEndian.PutUint32(out[4:8], uint32(len(out)))
	copy(out[8:], w.buf)
	return out, nil
}

// DecodeGuest parses a guest image. The buffer may extend past the declared
// image length; those bytes are not read. Decoding a cast-layout image fails
// the magic check.
func DecodeGuest(image []byte) (VerifyInput, error) {
	if len(image) < 8 {
		return VerifyInput{}, fmt.Errorf("proofserialization: guest image truncated")
	}
	if string(image[:4]) != guestMagic {
		return VerifyInput{}, fmt.Errorf("proofserialization: guest image magic is %q, want %q", image[:4], guestMagic)
	}
	n := binary.LittleEndian.Uint32(image[4:8])
	if n < 8 || uint64(n) > uint64(len(image)) || n > MaxImageSize {
		return VerifyInput{}, fmt.Errorf("proofserialization: guest image length %d is not in [8, %d]",
			n, min(len(image), MaxImageSize))
	}
	r := guestReader{buf: image[8:n]}
	rows, err := r.rowSection()
	if err != nil {
		return VerifyInput{}, err
	}
	rounds, err := r.rounds()
	if err != nil {
		return VerifyInput{}, err
	}
	sizes, err := r.moduleSizes()
	if err != nil {
		return VerifyInput{}, err
	}
	public, err := r.scalarSlice()
	if err != nil {
		return VerifyInput{}, err
	}
	queries, err := r.queries(rows)
	if err != nil {
		return VerifyInput{}, err
	}
	caps, err := r.inputCaps(rows)
	if err != nil {
		return VerifyInput{}, err
	}
	fri, err := r.fri()
	if err != nil {
		return VerifyInput{}, err
	}
	if r.off != len(r.buf) {
		return VerifyInput{}, fmt.Errorf("proofserialization: guest image has %d trailing bytes inside its declared length",
			len(r.buf)-r.off)
	}
	return VerifyInput{
		Proof: Proof{
			Rounds:      rounds,
			ModuleSizes: sizes,
			PcsOpening: OpeningProof{
				InputQueries: queries,
				InputCaps:    caps,
				FriProof:     fri,
			},
		},
		PublicInputs: public,
	}, nil
}

type rowPool struct {
	rows  []RowOpening
	index map[string]uint32
}

func collectRows(in VerifyInput) (*rowPool, error) {
	p := &rowPool{index: make(map[string]uint32)}
	for _, query := range in.Proof.PcsOpening.InputQueries {
		for _, tree := range query {
			for _, leaf := range tree.Leaves {
				if leaf == nil {
					continue
				}
				if _, err := p.add(leaf[0]); err != nil {
					return nil, err
				}
				if _, err := p.add(leaf[1]); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, cap := range in.Proof.PcsOpening.InputCaps {
		for _, table := range cap.Tables {
			for _, row := range table.Rows {
				if _, err := p.add(row); err != nil {
					return nil, err
				}
			}
		}
	}
	return p, nil
}

func (p *rowPool) add(row RowOpening) (uint32, error) {
	if len(row.Base) > 65535 || len(row.Ext) > 65535 {
		return 0, fmt.Errorf("proofserialization: guest row width base=%d ext=%d exceeds 65535", len(row.Base), len(row.Ext))
	}
	key := rowKey(row)
	if i, ok := p.index[key]; ok {
		return i, nil
	}
	if len(p.rows) >= maxGuestItems {
		return 0, fmt.Errorf("proofserialization: guest unique-row count exceeds %d", maxGuestItems)
	}
	i := uint32(len(p.rows))
	p.rows = append(p.rows, row)
	p.index[key] = i
	return i, nil
}

func (p *rowPool) lookup(row RowOpening) (uint32, error) {
	i, ok := p.index[rowKey(row)]
	if !ok {
		return 0, fmt.Errorf("proofserialization: guest row missing from the unique-row pool")
	}
	return i, nil
}

func rowKey(row RowOpening) string {
	b := make([]byte, 4+4*len(row.Base)+24*len(row.Ext))
	binary.LittleEndian.PutUint16(b[0:2], uint16(len(row.Base)))
	binary.LittleEndian.PutUint16(b[2:4], uint16(len(row.Ext)))
	off := 4
	for _, e := range row.Base {
		binary.LittleEndian.PutUint32(b[off:], uint32(e))
		off += 4
	}
	for _, e := range row.Ext {
		for _, limb := range e {
			binary.LittleEndian.PutUint32(b[off:], uint32(limb))
			off += 4
		}
	}
	return string(b)
}

func extZero(e Ext) bool {
	for _, limb := range e {
		if limb != 0 {
			return false
		}
	}
	return true
}

type guestWriter struct {
	buf []byte
}

func (w *guestWriter) u8(v byte) { w.buf = append(w.buf, v) }

func (w *guestWriter) u16(v uint16) {
	var b [2]byte
	binary.LittleEndian.PutUint16(b[:], v)
	w.buf = append(w.buf, b[:]...)
}

func (w *guestWriter) u32(v uint32) {
	var b [4]byte
	binary.LittleEndian.PutUint32(b[:], v)
	w.buf = append(w.buf, b[:]...)
}

func (w *guestWriter) u64(v uint64) {
	var b [8]byte
	binary.LittleEndian.PutUint64(b[:], v)
	w.buf = append(w.buf, b[:]...)
}

func (w *guestWriter) count(n int) error {
	if n < 0 || n > maxGuestItems {
		return fmt.Errorf("proofserialization: guest image count %d exceeds %d", n, maxGuestItems)
	}
	w.u32(uint32(n))
	return nil
}

func (w *guestWriter) bits(flags []bool) {
	if len(flags) == 0 {
		return
	}
	n := (len(flags) + 7) / 8
	start := len(w.buf)
	w.buf = append(w.buf, make([]byte, n)...)
	for i, on := range flags {
		if on {
			w.buf[start+i>>3] |= byte(1 << (i & 7))
		}
	}
}

func (w *guestWriter) ext(e Ext) {
	for _, limb := range e {
		w.u32(uint32(limb))
	}
}

func (w *guestWriter) digest(d Digest) {
	for _, limb := range d {
		w.u32(uint32(limb))
	}
}

func (w *guestWriter) digests(ds []Digest) error {
	if err := w.count(len(ds)); err != nil {
		return err
	}
	for _, d := range ds {
		w.digest(d)
	}
	return nil
}

func (w *guestWriter) optionalDigests(xs []*Digest) error {
	if err := w.count(len(xs)); err != nil {
		return err
	}
	present := make([]bool, len(xs))
	for i, d := range xs {
		present[i] = d != nil
	}
	w.bits(present)
	for _, d := range xs {
		if d != nil {
			w.digest(*d)
		}
	}
	return nil
}

func (w *guestWriter) scalarSlice(xs []Scalar) error {
	if err := w.count(len(xs)); err != nil {
		return err
	}
	flags := make([]bool, len(xs))
	for i, s := range xs {
		flags[i] = s.IsExt
	}
	w.bits(flags)
	for _, s := range xs {
		w.ext(s.Value)
	}
	return nil
}

func (w *guestWriter) packedRow(row RowOpening) {
	w.u16(uint16(len(row.Base)))
	w.u16(uint16(len(row.Ext)))
	baseOn := make([]bool, len(row.Base))
	for i, e := range row.Base {
		baseOn[i] = e != 0
	}
	w.bits(baseOn)
	for _, e := range row.Base {
		if e != 0 {
			w.u32(uint32(e))
		}
	}
	extOn := make([]bool, len(row.Ext))
	for i, e := range row.Ext {
		extOn[i] = !extZero(e)
	}
	w.bits(extOn)
	for _, e := range row.Ext {
		if !extZero(e) {
			w.ext(e)
		}
	}
}

func (w *guestWriter) rowSection(rows []RowOpening) error {
	if err := w.count(len(rows)); err != nil {
		return err
	}
	for _, row := range rows {
		w.packedRow(row)
	}
	return nil
}

func (w *guestWriter) rounds(rs []RoundMessage) error {
	if err := w.count(len(rs)); err != nil {
		return err
	}
	for _, round := range rs {
		if err := w.scalarSlice(round.Cells); err != nil {
			return err
		}
		if round.Commitment == nil {
			w.u8(0)
			continue
		}
		w.u8(1)
		w.digest(*round.Commitment)
	}
	return nil
}

func (w *guestWriter) moduleSizes(xs []uint64) error {
	if err := w.count(len(xs)); err != nil {
		return err
	}
	for _, n := range xs {
		w.u64(n)
	}
	return nil
}

func (w *guestWriter) queries(qs [][]InputTreeOpening, pool *rowPool) error {
	if err := w.count(len(qs)); err != nil {
		return err
	}
	for _, query := range qs {
		if err := w.count(len(query)); err != nil {
			return err
		}
		for _, tree := range query {
			if err := w.digests(tree.Siblings); err != nil {
				return err
			}
			if err := w.count(len(tree.Leaves)); err != nil {
				return err
			}
			present := make([]bool, len(tree.Leaves))
			for i, leaf := range tree.Leaves {
				present[i] = leaf != nil
			}
			w.bits(present)
			for _, leaf := range tree.Leaves {
				if leaf == nil {
					continue
				}
				i0, err := pool.lookup(leaf[0])
				if err != nil {
					return err
				}
				i1, err := pool.lookup(leaf[1])
				if err != nil {
					return err
				}
				w.u32(i0)
				w.u32(i1)
			}
		}
	}
	return nil
}

func (w *guestWriter) inputCaps(caps []InputCap, pool *rowPool) error {
	if err := w.count(len(caps)); err != nil {
		return err
	}
	for _, cap := range caps {
		if err := w.digests(cap.Nodes); err != nil {
			return err
		}
		if err := w.count(len(cap.Tables)); err != nil {
			return err
		}
		for _, table := range cap.Tables {
			w.u8(table.SizeLog2)
			if err := w.count(len(table.Rows)); err != nil {
				return err
			}
			for _, row := range table.Rows {
				idx, err := pool.lookup(row)
				if err != nil {
					return err
				}
				w.u32(idx)
			}
		}
	}
	return nil
}

func (w *guestWriter) fri(proof FriProof) error {
	if err := w.digests(proof.RoundRoots); err != nil {
		return err
	}
	if err := w.count(len(proof.RoundCaps)); err != nil {
		return err
	}
	for _, cap := range proof.RoundCaps {
		if err := w.digests(cap.Nodes); err != nil {
			return err
		}
		if err := w.optionalDigests(cap.Aux); err != nil {
			return err
		}
	}
	if err := w.count(len(proof.FinalPoly)); err != nil {
		return err
	}
	for _, e := range proof.FinalPoly {
		w.ext(e)
	}
	if err := w.count(len(proof.RunningQueries)); err != nil {
		return err
	}
	for _, query := range proof.RunningQueries {
		if err := w.count(len(query)); err != nil {
			return err
		}
		for _, branch := range query {
			if err := w.digests(branch.Siblings); err != nil {
				return err
			}
			w.digest(branch.Leaf)
		}
	}
	return nil
}

type guestReader struct {
	buf []byte
	off int
}

func (r *guestReader) need(n int) error {
	if n < 0 || n > len(r.buf)-r.off {
		return fmt.Errorf("proofserialization: guest image truncated")
	}
	return nil
}

func (r *guestReader) u8() (byte, error) {
	if err := r.need(1); err != nil {
		return 0, err
	}
	v := r.buf[r.off]
	r.off++
	return v, nil
}

func (r *guestReader) u16() (uint16, error) {
	if err := r.need(2); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint16(r.buf[r.off:])
	r.off += 2
	return v, nil
}

func (r *guestReader) u32() (uint32, error) {
	if err := r.need(4); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint32(r.buf[r.off:])
	r.off += 4
	return v, nil
}

func (r *guestReader) u64() (uint64, error) {
	if err := r.need(8); err != nil {
		return 0, err
	}
	v := binary.LittleEndian.Uint64(r.buf[r.off:])
	r.off += 8
	return v, nil
}

func (r *guestReader) count() (int, error) {
	n, err := r.u32()
	if err != nil {
		return 0, err
	}
	if n > maxGuestItems {
		return 0, fmt.Errorf("proofserialization: guest image count %d exceeds %d", n, maxGuestItems)
	}
	return int(n), nil
}

func (r *guestReader) bits(n int) ([]byte, error) {
	if n == 0 {
		return nil, nil
	}
	nb := (n + 7) / 8
	if err := r.need(nb); err != nil {
		return nil, err
	}
	raw := r.buf[r.off : r.off+nb]
	r.off += nb
	return raw, nil
}

func bitOn(raw []byte, i int) bool {
	return raw[i>>3]&(1<<uint(i&7)) != 0
}

func (r *guestReader) ext() (Ext, error) {
	var e Ext
	for i := range e {
		v, err := r.u32()
		if err != nil {
			return Ext{}, err
		}
		e[i] = Element(v)
	}
	return e, nil
}

func (r *guestReader) digest() (Digest, error) {
	var d Digest
	for i := range d {
		v, err := r.u32()
		if err != nil {
			return Digest{}, err
		}
		d[i] = Element(v)
	}
	return d, nil
}

func (r *guestReader) digests() ([]Digest, error) {
	n, err := r.count()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	out := make([]Digest, n)
	for i := range out {
		d, err := r.digest()
		if err != nil {
			return nil, err
		}
		out[i] = d
	}
	return out, nil
}

func (r *guestReader) optionalDigests() ([]*Digest, error) {
	n, err := r.count()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	raw, err := r.bits(n)
	if err != nil {
		return nil, err
	}
	out := make([]*Digest, n)
	for i := range out {
		if !bitOn(raw, i) {
			continue
		}
		d, err := r.digest()
		if err != nil {
			return nil, err
		}
		out[i] = &d
	}
	return out, nil
}

func (r *guestReader) scalarSlice() ([]Scalar, error) {
	n, err := r.count()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	raw, err := r.bits(n)
	if err != nil {
		return nil, err
	}
	out := make([]Scalar, n)
	for i := range out {
		e, err := r.ext()
		if err != nil {
			return nil, err
		}
		out[i] = Scalar{Value: e, IsExt: bitOn(raw, i)}
	}
	return out, nil
}

func (r *guestReader) packedRow() (RowOpening, error) {
	nb, err := r.u16()
	if err != nil {
		return RowOpening{}, err
	}
	ne, err := r.u16()
	if err != nil {
		return RowOpening{}, err
	}
	baseBits, err := r.bits(int(nb))
	if err != nil {
		return RowOpening{}, err
	}
	var base []Element
	if nb != 0 {
		base = make([]Element, nb)
		for i := range base {
			if !bitOn(baseBits, i) {
				continue
			}
			v, err := r.u32()
			if err != nil {
				return RowOpening{}, err
			}
			base[i] = Element(v)
		}
	}
	extBits, err := r.bits(int(ne))
	if err != nil {
		return RowOpening{}, err
	}
	var exts []Ext
	if ne != 0 {
		exts = make([]Ext, ne)
		for i := range exts {
			if !bitOn(extBits, i) {
				continue
			}
			e, err := r.ext()
			if err != nil {
				return RowOpening{}, err
			}
			exts[i] = e
		}
	}
	return RowOpening{Base: base, Ext: exts}, nil
}

func (r *guestReader) rowSection() ([]RowOpening, error) {
	n, err := r.count()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	rows := make([]RowOpening, n)
	for i := range rows {
		row, err := r.packedRow()
		if err != nil {
			return nil, err
		}
		rows[i] = row
	}
	return rows, nil
}

func (r *guestReader) rounds() ([]RoundMessage, error) {
	n, err := r.count()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	out := make([]RoundMessage, n)
	for i := range out {
		cells, err := r.scalarSlice()
		if err != nil {
			return nil, err
		}
		flag, err := r.u8()
		if err != nil {
			return nil, err
		}
		switch flag {
		case 0:
			out[i] = RoundMessage{Cells: cells}
		case 1:
			d, err := r.digest()
			if err != nil {
				return nil, err
			}
			out[i] = RoundMessage{Cells: cells, Commitment: &d}
		default:
			return nil, fmt.Errorf("proofserialization: guest commitment flag %d", flag)
		}
	}
	return out, nil
}

func (r *guestReader) moduleSizes() ([]uint64, error) {
	n, err := r.count()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	out := make([]uint64, n)
	for i := range out {
		v, err := r.u64()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func (r *guestReader) rowIndex(n int) (int, error) {
	v, err := r.u32()
	if err != nil {
		return 0, err
	}
	if int(v) >= n {
		return 0, fmt.Errorf("proofserialization: guest row index %d out of range %d", v, n)
	}
	return int(v), nil
}

func (r *guestReader) queries(rows []RowOpening) ([][]InputTreeOpening, error) {
	nq, err := r.count()
	if err != nil {
		return nil, err
	}
	if nq == 0 {
		return nil, nil
	}
	out := make([][]InputTreeOpening, nq)
	for i := range out {
		nt, err := r.count()
		if err != nil {
			return nil, err
		}
		if nt == 0 {
			continue
		}
		trees := make([]InputTreeOpening, nt)
		for t := range trees {
			sibs, err := r.digests()
			if err != nil {
				return nil, err
			}
			nl, err := r.count()
			if err != nil {
				return nil, err
			}
			present, err := r.bits(nl)
			if err != nil {
				return nil, err
			}
			var leaves []*RowPair
			if nl != 0 {
				leaves = make([]*RowPair, nl)
				for lv := range leaves {
					if !bitOn(present, lv) {
						continue
					}
					i0, err := r.rowIndex(len(rows))
					if err != nil {
						return nil, err
					}
					i1, err := r.rowIndex(len(rows))
					if err != nil {
						return nil, err
					}
					pair := RowPair{rows[i0], rows[i1]}
					leaves[lv] = &pair
				}
			}
			trees[t] = InputTreeOpening{Siblings: sibs, Leaves: leaves}
		}
		out[i] = trees
	}
	return out, nil
}

func (r *guestReader) inputCaps(rows []RowOpening) ([]InputCap, error) {
	n, err := r.count()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	out := make([]InputCap, n)
	for i := range out {
		nodes, err := r.digests()
		if err != nil {
			return nil, err
		}
		nt, err := r.count()
		if err != nil {
			return nil, err
		}
		var tables []InputCapTable
		if nt != 0 {
			tables = make([]InputCapTable, nt)
			for t := range tables {
				log2, err := r.u8()
				if err != nil {
					return nil, err
				}
				nr, err := r.count()
				if err != nil {
					return nil, err
				}
				var capRows []RowOpening
				if nr != 0 {
					capRows = make([]RowOpening, nr)
					for row := range capRows {
						idx, err := r.rowIndex(len(rows))
						if err != nil {
							return nil, err
						}
						capRows[row] = rows[idx]
					}
				}
				tables[t] = InputCapTable{SizeLog2: log2, Rows: capRows}
			}
		}
		out[i] = InputCap{Nodes: nodes, Tables: tables}
	}
	return out, nil
}

func (r *guestReader) fri() (FriProof, error) {
	roots, err := r.digests()
	if err != nil {
		return FriProof{}, err
	}
	nc, err := r.count()
	if err != nil {
		return FriProof{}, err
	}
	var caps []MerkleCap
	if nc != 0 {
		caps = make([]MerkleCap, nc)
		for i := range caps {
			nodes, err := r.digests()
			if err != nil {
				return FriProof{}, err
			}
			aux, err := r.optionalDigests()
			if err != nil {
				return FriProof{}, err
			}
			caps[i] = MerkleCap{Nodes: nodes, Aux: aux}
		}
	}
	nf, err := r.count()
	if err != nil {
		return FriProof{}, err
	}
	var final []Ext
	if nf != 0 {
		final = make([]Ext, nf)
		for i := range final {
			e, err := r.ext()
			if err != nil {
				return FriProof{}, err
			}
			final[i] = e
		}
	}
	nq, err := r.count()
	if err != nil {
		return FriProof{}, err
	}
	var running [][]Branch
	if nq != 0 {
		running = make([][]Branch, nq)
		for i := range running {
			nb, err := r.count()
			if err != nil {
				return FriProof{}, err
			}
			if nb == 0 {
				continue
			}
			branches := make([]Branch, nb)
			for b := range branches {
				sibs, err := r.digests()
				if err != nil {
					return FriProof{}, err
				}
				leaf, err := r.digest()
				if err != nil {
					return FriProof{}, err
				}
				branches[b] = Branch{Siblings: sibs, Leaf: leaf}
			}
			running[i] = branches
		}
	}
	return FriProof{
		RoundRoots:     roots,
		RoundCaps:      caps,
		FinalPoly:      final,
		RunningQueries: running,
	}, nil
}
