package proofserialization_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/stretchr/testify/require"
)

func TestEncodeGuest_RoundTrip(t *testing.T) {
	want := guestProof()

	image, err := ps.EncodeGuest(want)
	require.NoError(t, err)
	require.Equal(t, "LPR1", string(image[:4]))

	got, err := ps.DecodeGuest(image)
	require.NoError(t, err)
	require.Equal(t, want, got)

	again, err := ps.EncodeGuest(got)
	require.NoError(t, err)
	require.Equal(t, image, again)

	// The declared length is the image. Bytes past it belong to the guest
	// input region and must not change the decoded proof.
	padded := append(append([]byte{}, image...), 1, 2, 3, 4)
	fromPadded, err := ps.DecodeGuest(padded)
	require.NoError(t, err)
	require.Equal(t, want, fromPadded)
}

func TestEncodeGuest_NilAndEmptySlicesAreIndistinguishable(t *testing.T) {
	withNil := ps.VerifyInput{Proof: ps.Proof{Rounds: []ps.RoundMessage{{Cells: nil}}}}
	withEmpty := ps.VerifyInput{Proof: ps.Proof{Rounds: []ps.RoundMessage{{Cells: []ps.Scalar{}}}}}

	a, err := ps.EncodeGuest(withNil)
	require.NoError(t, err)
	b, err := ps.EncodeGuest(withEmpty)
	require.NoError(t, err)
	require.Equal(t, a, b)

	decoded, err := ps.DecodeGuest(b)
	require.NoError(t, err)
	require.Nil(t, decoded.Proof.Rounds[0].Cells)
}

func TestEncodeGuest_ZeroLimbsAndDuplicateRowsShrink(t *testing.T) {
	zeros, err := ps.EncodeGuest(rowProof(1000, 0, 1))
	require.NoError(t, err)
	ones, err := ps.EncodeGuest(rowProof(1000, 1, 1))
	require.NoError(t, err)
	require.Less(t, len(zeros), len(ones)/2)

	shared, err := ps.EncodeGuest(rowProof(100, 1, 8))
	require.NoError(t, err)
	distinct := distinctRowProof(100, 8)
	separate, err := ps.EncodeGuest(distinct)
	require.NoError(t, err)
	require.Less(t, len(shared), len(separate)/2)
}

func TestEncodeGuest_RejectsOversizedRow(t *testing.T) {
	wide := ps.RowOpening{Base: make([]ps.Element, 65536)}
	_, err := ps.EncodeGuest(rowWith(wide))
	require.Error(t, err)
}

func TestDecodeGuest_RejectsCastImage(t *testing.T) {
	image, err := ps.Encode(richProof(), ps.GuestBase)
	require.NoError(t, err)
	_, err = ps.DecodeGuest(image)
	require.Error(t, err)

	_, err = ps.DecodeGuest(nil)
	require.Error(t, err)
	_, err = ps.DecodeGuest([]byte("LPR1"))
	require.Error(t, err)
}

func TestEncodeGuest_RiscvFixture(t *testing.T) {
	path := filepath.Join("..", "..", "..", "verifier-ray", "testdata", "riscv_proof_image.bin")
	img, err := os.ReadFile(path)
	require.NoError(t, err)
	require.True(t, bytes.HasPrefix(img, []byte("LPR1")), "riscv_proof_image.bin must be an EncodeGuest image")

	require.Greater(t, len(img), 12_000_000)
	require.Less(t, len(img), 15_500_000)

	got, err := ps.DecodeGuest(img)
	require.NoError(t, err)
	require.Len(t, got.Proof.Rounds, 5)
	require.Len(t, got.Proof.ModuleSizes, 97)
	require.Len(t, got.PublicInputs, 337)
	require.Len(t, got.Proof.PcsOpening.InputQueries, 229)
	require.Len(t, got.Proof.PcsOpening.FriProof.RoundRoots, 15)
	require.Equal(t, uint64(0x39688720aeec5408), fingerprint(got))

	again, err := ps.EncodeGuest(got)
	require.NoError(t, err)
	require.Equal(t, img, again)
	t.Logf("riscv guest image %d bytes", len(img))
}

func fingerprint(in ps.VerifyInput) uint64 {
	var x uint64 = 0x6a09e667f3bcc909
	mix := func(v uint32) { x = x*0x9e3779b185ebca87 + uint64(v) }
	mixU8 := func(v byte) { mix(uint32(v)) }
	mixDig := func(d ps.Digest) {
		for _, limb := range d {
			mix(uint32(limb))
		}
	}
	mixExt := func(e ps.Ext) {
		for _, limb := range e {
			mix(uint32(limb))
		}
	}
	mixScalar := func(s ps.Scalar) {
		if s.IsExt {
			mixU8(1)
		} else {
			mixU8(0)
		}
		mixExt(s.Value)
	}
	mixRow := func(row ps.RowOpening) {
		mix(uint32(len(row.Base)))
		for _, e := range row.Base {
			mix(uint32(e))
		}
		mix(uint32(len(row.Ext)))
		for _, e := range row.Ext {
			mixExt(e)
		}
	}
	for _, round := range in.Proof.Rounds {
		mix(uint32(len(round.Cells)))
		for _, cell := range round.Cells {
			mixScalar(cell)
		}
		if round.Commitment == nil {
			mixU8(0)
			continue
		}
		mixU8(1)
		mixDig(*round.Commitment)
	}
	mix(uint32(len(in.Proof.ModuleSizes)))
	for _, n := range in.Proof.ModuleSizes {
		mix(uint32(n))
		mix(uint32(n >> 32))
	}
	for _, s := range in.PublicInputs {
		mixScalar(s)
	}
	for _, query := range in.Proof.PcsOpening.InputQueries {
		mix(uint32(len(query)))
		for _, tree := range query {
			mix(uint32(len(tree.Siblings)))
			for _, d := range tree.Siblings {
				mixDig(d)
			}
			mix(uint32(len(tree.Leaves)))
			for _, leaf := range tree.Leaves {
				if leaf == nil {
					mixU8(0)
					continue
				}
				mixU8(1)
				mixRow(leaf[0])
				mixRow(leaf[1])
			}
		}
	}
	for _, cap := range in.Proof.PcsOpening.InputCaps {
		mix(uint32(len(cap.Nodes)))
		for _, d := range cap.Nodes {
			mixDig(d)
		}
		mix(uint32(len(cap.Tables)))
		for _, table := range cap.Tables {
			mixU8(table.SizeLog2)
			mix(uint32(len(table.Rows)))
			for _, row := range table.Rows {
				mixRow(row)
			}
		}
	}
	fri := in.Proof.PcsOpening.FriProof
	mix(uint32(len(fri.RoundRoots)))
	for _, d := range fri.RoundRoots {
		mixDig(d)
	}
	mix(uint32(len(fri.RoundCaps)))
	for _, cap := range fri.RoundCaps {
		mix(uint32(len(cap.Nodes)))
		for _, d := range cap.Nodes {
			mixDig(d)
		}
		mix(uint32(len(cap.Aux)))
		for _, d := range cap.Aux {
			if d == nil {
				mixU8(0)
				continue
			}
			mixU8(1)
			mixDig(*d)
		}
	}
	mix(uint32(len(fri.FinalPoly)))
	for _, e := range fri.FinalPoly {
		mixExt(e)
	}
	mix(uint32(len(fri.RunningQueries)))
	for _, query := range fri.RunningQueries {
		mix(uint32(len(query)))
		for _, branch := range query {
			mix(uint32(len(branch.Siblings)))
			for _, d := range branch.Siblings {
				mixDig(d)
			}
			mixDig(branch.Leaf)
		}
	}
	return x
}

func guestProof() ps.VerifyInput {
	p := richProof()
	dig := p.Proof.Rounds[0].Commitment
	zeroRow := ps.RowOpening{
		Base: []ps.Element{0, 0, 1, 0},
		Ext:  []ps.Ext{{}, {9}},
	}
	shared := p.Proof.PcsOpening.InputQueries[0][0].Leaves[1][0]
	p.Proof.PcsOpening.InputCaps = []ps.InputCap{{
		Nodes: []ps.Digest{*dig},
		Tables: []ps.InputCapTable{{
			SizeLog2: 3,
			Rows:     []ps.RowOpening{shared, zeroRow, shared},
		}},
	}}
	p.Proof.PcsOpening.FriProof.RoundCaps = []ps.MerkleCap{{
		Nodes: []ps.Digest{*dig},
		Aux:   []*ps.Digest{nil, dig},
	}}
	return p
}

func rowProof(width int, fill ps.Element, copies int) ps.VerifyInput {
	base := make([]ps.Element, width)
	for i := range base {
		base[i] = fill
	}
	return repeatedRowProof(ps.RowOpening{Base: base}, copies)
}

func distinctRowProof(width, n int) ps.VerifyInput {
	leaves := make([]*ps.RowPair, n)
	for i := range leaves {
		base := make([]ps.Element, width)
		for j := range base {
			base[j] = ps.Element(i + 1)
		}
		row := ps.RowOpening{Base: base}
		pair := ps.RowPair{row, row}
		leaves[i] = &pair
	}
	return ps.VerifyInput{Proof: ps.Proof{PcsOpening: ps.OpeningProof{
		InputQueries: [][]ps.InputTreeOpening{{{Leaves: leaves}}},
	}}}
}

func repeatedRowProof(row ps.RowOpening, copies int) ps.VerifyInput {
	leaves := make([]*ps.RowPair, copies)
	for i := range leaves {
		pair := ps.RowPair{row, row}
		leaves[i] = &pair
	}
	return ps.VerifyInput{Proof: ps.Proof{PcsOpening: ps.OpeningProof{
		InputQueries: [][]ps.InputTreeOpening{{{Leaves: leaves}}},
	}}}
}

func rowWith(row ps.RowOpening) ps.VerifyInput {
	return repeatedRowProof(row, 1)
}
