// Package proofserialization turns a [wiop.Proof] into the byte image the Zig
// verifier casts straight out of its input region, with no decode step.
//
//   - [Project] maps a [wiop.Proof] onto the verifier's round-major shape.
//   - [Encode] lays that out as the image, relocated for a given base address.
//   - [Decode] and [Validate] read an image back, host-side.
//   - [Measure] reports what an image would cost without building one.
//
// The layout is not a free choice: it mirrors the Zig ABI of verifier-ray's
// verifier.Proof, pinned by verifier-ray/src/proof_abi.zig. README.md documents
// the format and the reasoning; sections 5 to 7 are the ones that matter when
// changing either side.
package proofserialization

import (
	"fmt"
	"sort"
	"strings"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Section is one named group of image bytes, so a report can show where the size
// goes rather than only a total.
type Section struct {
	Name  string
	Bytes int
	Note  string
}

// Stats is the measured shape and size of the image for one proof.
type Stats struct {
	Rounds       int
	RoundCells   []int // per round, excluding public inputs
	Cells        int
	BaseCells    int
	ExtCells     int
	PublicInputs int
	Commitments  int
	DynamicSizes int

	// HasPCS is false for a protocol that was not PCS-compiled, in which case
	// every field below is zero and the total covers only the round messages.
	HasPCS            bool
	Queries           int
	TreesPerQuery     int
	InputTreeSiblings int
	LeafSlots         int
	PresentLeaves     int
	RowBaseElements   int
	RowExtElements    int
	OpeningDepths     map[int]int
	RoundRoots        int
	FinalPolyCoeffs   int
	LayersPerQuery    int
	Branches          int
	BranchSiblings    int
	AuxSlots          int
	AuxNonNil         int

	Sections []Section
	// Total is the image size in bytes. Payload is the part that is irreducible
	// content — field elements and Merkle digests — so Total-Payload is what the
	// format costs over a packed encoding.
	Total   int
	Payload int
}

// Measure computes the image size for proof without encoding it.
//
// It deliberately counts what the VERIFIER reads, not what Go carries: one
// merkle.Branch per fold round rather than the whole QueryLayer, and no
// AuxSiblings, since the Zig type has no such field. AuxNonNil records whether
// dropping those would actually lose anything.
func Measure(sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) Stats {
	s := Stats{
		Rounds:        len(sys.Rounds),
		Cells:         len(proof.Cells),
		PublicInputs:  len(pub),
		Commitments:   len(proof.Commitments),
		DynamicSizes:  len(proof.DynamicSizes),
		RoundCells:    make([]int, len(sys.Rounds)),
		OpeningDepths: map[int]int{},
	}

	for _, g := range proof.Cells {
		if g.IsBase() {
			s.BaseCells++
		} else {
			s.ExtCells++
		}
	}

	publicInput := make(map[wiop.ObjectID]bool, len(sys.PublicInputs))
	for _, c := range sys.PublicInputs {
		publicInput[c.Context.ID] = true
	}
	for _, r := range sys.Rounds {
		for _, c := range r.Cells {
			if !publicInput[c.Context.ID] {
				s.RoundCells[r.ID]++
			}
		}
	}

	add := func(name string, n int, note string) {
		s.Sections = append(s.Sections, Section{name, n, note})
	}

	add("root", SizeProof, "verifier.Proof")
	add("rounds array", s.Rounds*SizeRoundMessage, fmt.Sprintf("%d x RoundMessage", s.Rounds))
	add("cells", s.Cells*SizeScalar, fmt.Sprintf("%d x Scalar(28)", s.Cells))
	// A committed round carries its Merkle root inline in the RoundMessage, so
	// the commitments cost nothing beyond the rounds array itself.
	add("oracle commitments", 0,
		fmt.Sprintf("%d, stored inline in RoundMessage", s.Commitments))
	add("module_sizes", s.DynamicSizes*SizeUsize, "")
	// No entry for PcsOpening / OpeningProof / fri.Proof: each is stored INLINE in
	// its parent, so all three already sit inside the root's SizeProof bytes.
	// Counting them again over-stated every image by a fixed 192 bytes until
	// TestMeasureAgreesWithEncode compared the model against the real encoder.

	// A cell's 24-byte value is content; its tag and padding are not.
	s.Payload += s.Cells * SizeExt
	s.Payload += s.Commitments * SizeDigest

	if op := proof.PCSOpeningProof; op != nil {
		s.HasPCS = true
		s.Queries = len(op.InputQueries)

		for _, iq := range op.InputQueries {
			s.TreesPerQuery = len(iq)
			for _, open := range iq {
				s.InputTreeSiblings += len(open.Siblings)
				s.LeafSlots += len(open.Leaves)
				s.OpeningDepths[len(open.Leaves)]++
				for _, l := range open.Leaves {
					if l == nil {
						continue
					}
					s.PresentLeaves++
					for _, row := range l {
						s.RowBaseElements += len(row.Base)
						s.RowExtElements += len(row.Ext)
					}
				}
			}
		}

		add("input_queries outer", s.Queries*SizeSlice, "")
		add("input_queries inner", s.Queries*s.TreesPerQuery*SizeInputTreeOpen, "InputTreeOpening structs")
		add("input-tree siblings", s.InputTreeSiblings*SizeDigest, "Merkle path digests")
		// A ?RowPair is 72 B: a 64 B payload -- which IS the two RowOpenings, i.e.
		// four slice headers -- plus the presence flag and padding. Counting
		// RowOpening separately would double-count those 64 bytes.
		add("input-tree leaf slots", s.LeafSlots*SizeOptRowPair,
			fmt.Sprintf("%d x ?RowPair(72) = 2 x RowOpening(32) + flag; %d null",
				s.LeafSlots, s.LeafSlots-s.PresentLeaves))
		rowData := s.RowBaseElements*SizeElement + s.RowExtElements*SizeExt
		add("row data", rowData, "opened witness rows -- actual field elements")

		s.Payload += s.InputTreeSiblings * SizeDigest
		s.Payload += rowData

		fp := op.FRIProof
		s.RoundRoots = len(fp.RoundRoots)
		s.FinalPolyCoeffs = len(fp.FinalPoly)
		for _, rq := range fp.RunningQueries {
			s.LayersPerQuery = len(rq)
			for _, layer := range rq {
				if len(layer) == 0 {
					continue
				}
				s.Branches++
				s.BranchSiblings += len(layer[0].Siblings)
				// AuxSiblings is contractually as long as Siblings, with nil where
				// a level has no aux node. Only non-nil entries are real data, and
				// the Zig merkle.Branch has no field for them -- so a non-zero
				// AuxNonNil is a prover/verifier disagreement, not a saving.
				for _, aux := range layer[0].AuxSiblings {
					s.AuxSlots++
					if aux != nil {
						s.AuxNonNil++
					}
				}
			}
		}

		add("round_roots", s.RoundRoots*SizeDigest, "")
		add("final_poly", s.FinalPolyCoeffs*SizeExt, "")
		add("running_queries outer", len(fp.RunningQueries)*SizeSlice, "")
		add("running_queries branches", s.Branches*SizeBranch, "")
		add("branch siblings", s.BranchSiblings*SizeDigest, "Merkle path digests")

		s.Payload += s.RoundRoots * SizeDigest
		s.Payload += s.FinalPolyCoeffs * SizeExt
		s.Payload += s.BranchSiblings * SizeDigest
	}

	for _, sec := range s.Sections {
		s.Total += sec.Bytes
	}

	return s
}

// Overhead is the bytes the format costs over a packed encoding.
func (s Stats) Overhead() int { return s.Total - s.Payload }

// String renders the measurement as a report.
func (s Stats) String() string {
	var b strings.Builder

	fmt.Fprintf(&b, "rounds                     %d\n", s.Rounds)
	fmt.Fprintf(&b, "cells (in proof)           %d  (base %d, ext %d)\n", s.Cells, s.BaseCells, s.ExtCells)
	fmt.Fprintf(&b, "public inputs              %d  (carried separately, not in the image today)\n", s.PublicInputs)
	fmt.Fprintf(&b, "oracle commitments         %d  (one per committed round)\n", s.Commitments)
	fmt.Fprintf(&b, "dynamic modules            %d\n", s.DynamicSizes)

	fmt.Fprintf(&b, "\nper-round cell counts (excluding public inputs):\n")
	for id, n := range s.RoundCells {
		if n > 0 {
			fmt.Fprintf(&b, "  round %-3d %d\n", id, n)
		}
	}

	if !s.HasPCS {
		fmt.Fprintf(&b, "\nNO PCS OPENING PROOF — not PCS-compiled, so this covers only the\n")
		fmt.Fprintf(&b, "round messages.\n")
	} else {
		fmt.Fprintf(&b, "\nFRI / PCS opening:\n")
		fmt.Fprintf(&b, "  queries                  %d\n", s.Queries)
		fmt.Fprintf(&b, "  input trees per query    %d\n", s.TreesPerQuery)
		fmt.Fprintf(&b, "  sibling digests          %d\n", s.InputTreeSiblings)
		fmt.Fprintf(&b, "  leaf slots               %d  (present %d, null %d)\n",
			s.LeafSlots, s.PresentLeaves, s.LeafSlots-s.PresentLeaves)
		fmt.Fprintf(&b, "  row elements             base %d, ext %d\n", s.RowBaseElements, s.RowExtElements)
		fmt.Fprintf(&b, "  opening depths           %s\n", histogram(s.OpeningDepths))
		fmt.Fprintf(&b, "  round roots              %d  (fri rounds = %d)\n", s.RoundRoots, s.RoundRoots+1)
		fmt.Fprintf(&b, "  final poly coeffs        %d\n", s.FinalPolyCoeffs)
		fmt.Fprintf(&b, "  branches                 %d, %d sibling digests, %d layers per query\n",
			s.Branches, s.BranchSiblings, s.LayersPerQuery)
		fmt.Fprintf(&b, "  aux sibling slots        %d, of which NON-NIL %d\n", s.AuxSlots, s.AuxNonNil)
		if s.AuxNonNil > 0 {
			fmt.Fprintf(&b, "    WARNING: the Zig merkle.Branch has no AuxSiblings field, so these\n")
			fmt.Fprintf(&b, "    %d values would be dropped by the projection. If the Go verifier\n", s.AuxNonNil)
			fmt.Fprintf(&b, "    folds them into the running-layer root and the Zig one does not, the\n")
			fmt.Fprintf(&b, "    two reconstruct different roots. Resolve before encoding anything.\n")
		} else {
			fmt.Fprintf(&b, "    all nil, so dropping the field loses nothing\n")
		}
	}

	fmt.Fprintf(&b, "\nimage size by section:\n\n")
	sections := make([]Section, len(s.Sections))
	copy(sections, s.Sections)
	sort.SliceStable(sections, func(i, j int) bool { return sections[i].Bytes > sections[j].Bytes })
	for _, sec := range sections {
		pct := 0.0
		if s.Total > 0 {
			pct = 100 * float64(sec.Bytes) / float64(s.Total)
		}
		fmt.Fprintf(&b, "  %-26s %10d B  %5.1f%%  %s\n", sec.Name, sec.Bytes, pct, sec.Note)
	}
	fmt.Fprintf(&b, "  %-26s %10d B  (%.2f MiB)\n", "TOTAL", s.Total, float64(s.Total)/(1024*1024))

	if s.Payload > 0 {
		fmt.Fprintf(&b, "\n  field-element payload      %10d B  (%.2f MiB)\n",
			s.Payload, float64(s.Payload)/(1024*1024))
		fmt.Fprintf(&b, "  structural overhead        %10d B  (%.1f%% of image, %.2fx payload)\n",
			s.Overhead(), 100*float64(s.Overhead())/float64(s.Total),
			float64(s.Total)/float64(s.Payload))
	}

	fmt.Fprintf(&b, "\nnot counted (needs the projection):\n")
	fmt.Fprintf(&b, "  - entry_claims: derived by the PCS codegen, absent from wiop.Proof\n")
	fmt.Fprintf(&b, "  - public columns: whether this system exposes any, and their sizes\n")

	return b.String()
}

// histogram renders a count-by-value map in ascending value order.
func histogram(m map[int]int) string {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "%d x%d", k, m[k])
	}
	return b.String()
}
