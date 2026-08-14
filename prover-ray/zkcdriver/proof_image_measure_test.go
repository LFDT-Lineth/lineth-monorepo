package zkcdriver_test

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// measureEnvVar gates these measurements. Producing a real proof costs minutes,
// so they must not run as part of the ordinary suite; they exist to be invoked
// deliberately when the proof image's size needs re-measuring.
const measureEnvVar = "MEASURE_PROOF_IMAGE"

// measureProgramEnvVar overrides the measured program, so the R5 arithmetization
// can be measured once it compiles again (see measureTargets).
const measureProgramEnvVar = "MEASURE_PROOF_PROGRAM"

// measureTargets are the programs measured by default, smallest first.
//
// The program we actually care about is the R5 arithmetization
// (../../arithmetization/src/main/riscv/main.zkc), but it does not currently
// compile against the pinned zkc version — `zkc compile error: unexpected token`
// / `unknown symbol`, which fails every pre-existing R5 benchmark in
// r5_benchmark_test.go identically, not just this measurement. These testdata
// programs are real proofs through the same pipeline and stand in until that is
// fixed; point measureProgramEnvVar at the R5 program to measure it directly.
var measureTargets = []string{
	"testdata/modexp_u64",
	"testdata/modexp_u256",
	"testdata/secp256k1_add_u256",
	"testdata/secp256k1_scalarmul_u256",
}

// Byte sizes of the verifier-ray types the proof image is built from, as pinned
// by verifier-ray/src/proof_abi.zig. Kept here so the size arithmetic below is
// readable rather than a wall of magic numbers.
const (
	szSlice         = 16 // {ptr, len}, no capacity field
	szElement       = 4  // koalabear.Element, one Montgomery u32
	szExt           = 24 // ext.Ext (E6)
	szDigest        = 32 // poseidon2.Digest / Commitment
	szUsize         = 8
	szScalar        = 28 // value.Scalar: 24B payload + tag + pad
	szColumnMessage = 40 // protocol.ColumnMessage: 32B payload + tag + pad
	szRoundMessage  = 32
	szRowOpening    = 32
	szOptRowPair    = 72 // ?merkle.RowPair: 64B payload + flag + pad
	szBranch        = 48
	szInputTreeOpen = 32
	szFriProof      = 48
	szOpeningProof  = 64
	szPcsOpening    = 80
	szProof         = 112
)

// TestMeasureProofImage is Phase 0 of docs/proof-serialization.md: build real
// proofs and report the cardinalities the image size depends on, so the format
// can be committed to knowing how big its artifact is and where the bytes go.
func TestMeasureProofImage(t *testing.T) {
	if os.Getenv(measureEnvVar) == "" {
		t.Skipf("set %s=1 to run; each program takes minutes to prove", measureEnvVar)
	}

	targets := measureTargets
	if p := os.Getenv(measureProgramEnvVar); p != "" {
		targets = []string{p}
	}

	for _, path := range targets {
		t.Run(path, func(t *testing.T) {
			sys, proof, pub := proveForMeasurement(t, path)
			t.Log(report(path, sys, proof, pub))
		})
	}
}

// proveForMeasurement mirrors runProveVerify but hands back the proof so it can
// be measured. It verifies too: measuring an unsound proof would be measuring
// the wrong thing.
func proveForMeasurement(t *testing.T, zkcPath string) (*wiop.System, wiop.Proof, wiop.PublicInput) {
	t.Helper()

	binf, err := compileBinaryConstraints(zkcPath + ".zkc")
	if err != nil {
		t.Fatalf("compiling %s.zkc: %v", zkcPath, err)
	}
	inputBytes, err := os.ReadFile(zkcPath + ".json")
	if err != nil {
		t.Fatalf("reading %s.json: %v", zkcPath, err)
	}
	inputs, _, err := parseTestCase(zkcTestCase{
		ZkcFilePath: zkcPath + ".json",
		InputStr:    string(inputBytes),
	}, binf)
	if err != nil {
		t.Fatalf("parsing test case: %v", err)
	}

	compiled, err := binf.MarshalBinary()
	if err != nil {
		t.Fatalf("marshalling constraints: %v", err)
	}

	sys := wiop.NewSystemf("proof-image-measure")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiled))
	proverCompilePipeline(sys)

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignWithPreRead(rt, inputs)
	})
	if err := sys.Verify(proof, pub); err != nil {
		t.Fatalf("verifying the measured proof: %v", err)
	}

	return sys, proof, pub
}

// section accumulates a named group of image bytes so the report can show where
// the size actually goes rather than just a total.
type section struct {
	name  string
	bytes int
	note  string
}

func report(name string, sys *wiop.System, proof wiop.Proof, pub wiop.PublicInput) string {
	var b bytes.Buffer
	var sections []section
	payload := 0 // bytes of actual field elements, the irreducible part
	add := func(name string, n int, note string) {
		sections = append(sections, section{name, n, note})
	}

	fmt.Fprintf(&b, "\n=== proof image measurement: %s ===\n\n", name)

	// ---- rounds, cells, commitments -----------------------------------------
	numRounds := len(sys.Rounds)
	piIdx := map[wiop.ObjectID]bool{}
	for _, c := range sys.PublicInputs {
		piIdx[c.Context.ID] = true
	}

	baseCells, extCells := 0, 0
	for _, g := range proof.Cells {
		if g.IsBase() {
			baseCells++
		} else {
			extCells++
		}
	}
	totalCells := len(proof.Cells)

	fmt.Fprintf(&b, "rounds                     %d\n", numRounds)
	fmt.Fprintf(&b, "cells (in proof)           %d  (base %d, ext %d)\n", totalCells, baseCells, extCells)
	fmt.Fprintf(&b, "public inputs              %d  (carried separately, not in the image today)\n", len(pub))
	fmt.Fprintf(&b, "oracle commitments         %d  (one per committed round)\n", len(proof.Commitments))
	fmt.Fprintf(&b, "dynamic modules            %d\n", len(proof.DynamicSizes))

	fmt.Fprintf(&b, "\nper-round cell counts (excluding public inputs):\n")
	for _, r := range sys.Rounds {
		n := 0
		for _, c := range r.Cells {
			if !piIdx[c.Context.ID] {
				n++
			}
		}
		if n > 0 {
			fmt.Fprintf(&b, "  round %-3d %d\n", r.ID, n)
		}
	}

	add("root", szProof, "verifier.Proof")
	add("rounds array", numRounds*szRoundMessage, fmt.Sprintf("%d x RoundMessage", numRounds))
	add("cells", totalCells*szScalar, fmt.Sprintf("%d x Scalar(28)", totalCells))
	payload += totalCells * szExt // a cell's 24-byte value is payload; its tag+pad is not
	add("oracle commitments", len(proof.Commitments)*szColumnMessage,
		fmt.Sprintf("%d x ColumnMessage(40)", len(proof.Commitments)))
	payload += len(proof.Commitments) * szDigest
	add("module_sizes", len(proof.DynamicSizes)*szUsize, "")
	add("pcs_opening header", szPcsOpening+szOpeningProof+szFriProof, "PcsOpening + OpeningProof + fri.Proof")

	// ---- FRI opening proof ---------------------------------------------------
	op := proof.PCSOpeningProof
	if op == nil {
		fmt.Fprintf(&b, "\nNO PCS OPENING PROOF — the system was not PCS-compiled, so this\n")
		fmt.Fprintf(&b, "measurement covers only the round messages.\n")
	} else {
		add("input_queries outer", len(op.InputQueries)*szSlice, "")

		numQueries := len(op.InputQueries)
		treesPerQuery, siblings, leaves, presentLeaves := 0, 0, 0, 0
		rowBaseElems, rowExtElems := 0, 0
		depths := map[int]int{}
		for _, iq := range op.InputQueries {
			treesPerQuery = len(iq)
			for _, open := range iq {
				siblings += len(open.Siblings)
				leaves += len(open.Leaves)
				depths[len(open.Leaves)]++
				for _, l := range open.Leaves {
					if l == nil {
						continue
					}
					presentLeaves++
					for _, row := range l {
						rowBaseElems += len(row.Base)
						rowExtElems += len(row.Ext)
					}
				}
			}
		}

		fmt.Fprintf(&b, "\nFRI / PCS opening:\n")
		fmt.Fprintf(&b, "  queries                  %d\n", numQueries)
		fmt.Fprintf(&b, "  input trees per query    %d\n", treesPerQuery)
		fmt.Fprintf(&b, "  input-tree openings      %d\n", numQueries*treesPerQuery)
		fmt.Fprintf(&b, "  sibling digests          %d\n", siblings)
		fmt.Fprintf(&b, "  leaf slots               %d  (present %d, null %d)\n",
			leaves, presentLeaves, leaves-presentLeaves)
		fmt.Fprintf(&b, "  row elements             base %d, ext %d\n", rowBaseElems, rowExtElems)
		fmt.Fprintf(&b, "  opening depths           %s\n", histogram(depths))

		add("input_queries inner", numQueries*treesPerQuery*szInputTreeOpen, "InputTreeOpening structs")
		add("input-tree siblings", siblings*szDigest, "Merkle path digests")
		payload += siblings * szDigest
		// A ?RowPair is 72 B: a 64 B payload -- which IS the two RowOpenings, i.e.
		// four slice headers -- plus the presence flag and its padding. Counting
		// RowOpening separately here would double-count those 64 bytes.
		add("input-tree leaf slots", leaves*szOptRowPair,
			fmt.Sprintf("%d x ?RowPair(72) = 2 x RowOpening(32) + flag; %d null", leaves, leaves-presentLeaves))
		rowData := rowBaseElems*szElement + rowExtElems*szExt
		add("row data", rowData, "opened witness rows -- actual field elements")
		payload += rowData

		// ---- running-layer FRI proof ----------------------------------------
		fp := op.FRIProof
		branches, branchSiblings, auxSiblings, auxNonNil := 0, 0, 0, 0
		layersPerQuery := 0
		for _, rq := range fp.RunningQueries {
			layersPerQuery = len(rq)
			for _, layer := range rq {
				// The Zig verifier consumes one Branch per fold round (layer[0]);
				// count what it reads, not what Go carries.
				if len(layer) == 0 {
					continue
				}
				branches++
				branchSiblings += len(layer[0].Siblings)
				// AuxSiblings is contractually the same length as Siblings, with
				// nil where a level has no aux node. Only NON-NIL entries are real
				// data, and the Zig merkle.Branch has no field for them at all --
				// so a non-zero count here is a genuine prover/verifier
				// disagreement, not just a projection saving.
				for _, aux := range layer[0].AuxSiblings {
					auxSiblings++
					if aux != nil {
						auxNonNil++
					}
				}
			}
		}

		fmt.Fprintf(&b, "  round roots              %d  (fri rounds = %d)\n",
			len(fp.RoundRoots), len(fp.RoundRoots)+1)
		fmt.Fprintf(&b, "  final poly coeffs        %d\n", len(fp.FinalPoly))
		fmt.Fprintf(&b, "  running queries          %d, %d layers each\n",
			len(fp.RunningQueries), layersPerQuery)
		fmt.Fprintf(&b, "  branches                 %d, %d sibling digests total\n", branches, branchSiblings)
		fmt.Fprintf(&b, "  aux sibling slots        %d, of which NON-NIL %d\n", auxSiblings, auxNonNil)
		if auxNonNil > 0 {
			fmt.Fprintf(&b, "    WARNING: the Zig merkle.Branch has no AuxSiblings field, so these\n")
			fmt.Fprintf(&b, "    %d values would be dropped by the projection. If the Go verifier\n", auxNonNil)
			fmt.Fprintf(&b, "    folds them into the running-layer root and the Zig one does not, the\n")
			fmt.Fprintf(&b, "    two reconstruct different roots. Resolve before encoding anything.\n")
		} else {
			fmt.Fprintf(&b, "    all nil, so dropping the field loses nothing (%d B of Go-side\n", auxSiblings*8)
			fmt.Fprintf(&b, "    pointers that never reach the image)\n")
		}

		add("round_roots", len(fp.RoundRoots)*szDigest, "")
		payload += len(fp.RoundRoots) * szDigest
		add("final_poly", len(fp.FinalPoly)*szExt, "")
		payload += len(fp.FinalPoly) * szExt
		add("running_queries outer", len(fp.RunningQueries)*szSlice, "")
		add("running_queries branches", branches*szBranch, "")
		add("branch siblings", branchSiblings*szDigest, "Merkle path digests")
		payload += branchSiblings * szDigest
	}

	// ---- totals --------------------------------------------------------------
	total := 0
	for _, s := range sections {
		total += s.bytes
	}

	fmt.Fprintf(&b, "\n=== image size by section ===\n\n")
	sort.SliceStable(sections, func(i, j int) bool { return sections[i].bytes > sections[j].bytes })
	for _, s := range sections {
		pct := 0.0
		if total > 0 {
			pct = 100 * float64(s.bytes) / float64(total)
		}
		fmt.Fprintf(&b, "  %-26s %10d B  %5.1f%%  %s\n", s.name, s.bytes, pct, s.note)
	}
	fmt.Fprintf(&b, "  %-26s %10d B  (%.2f MiB)\n", "TOTAL (measured parts)", total, float64(total)/(1024*1024))

	// The only irreducible bytes are the field elements themselves; everything
	// else is structure the format pays for. Reporting the ratio makes the
	// format's real cost visible rather than leaving it implied by percentages.
	if payload > 0 {
		fmt.Fprintf(&b, "\n  field-element payload      %10d B  (%.2f MiB)\n", payload, float64(payload)/(1024*1024))
		fmt.Fprintf(&b, "  structural overhead        %10d B  (%.1f%% of image, %.2fx payload)\n",
			total-payload, 100*float64(total-payload)/float64(total), float64(total)/float64(payload))
	}

	fmt.Fprintf(&b, "\nNOT counted (needs the Phase 1 projection):\n")
	fmt.Fprintf(&b, "  - entry_claims: derived by the PCS codegen, not present in wiop.Proof\n")
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
	var b bytes.Buffer
	for i, k := range keys {
		if i > 0 {
			fmt.Fprint(&b, ", ")
		}
		fmt.Fprintf(&b, "%d x%d", k, m[k])
	}
	return b.String()
}
