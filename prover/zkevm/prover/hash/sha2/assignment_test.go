package sha2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
)

// inputRow represents a single row in the SHA2 block input CSV.
type inputRow struct {
	data     uint16
	selector int
	isFirst  int
}

// generateInputCSV builds a SHA2 block input CSV string from the given rows.
func generateInputCSV(rows []inputRow) string {
	var sb strings.Builder
	sb.WriteString("PACKED_DATA,SELECTOR,IS_FIRST_LANE_OF_NEW_HASH\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "0x%04x,%d,%d\n", r.data, r.selector, r.isFirst)
	}
	return sb.String()
}

// makeHashBlock creates 32 active input rows representing a single SHA2 block.
// The first row has IS_FIRST_LANE_OF_NEW_HASH set based on isFirst.
func makeHashBlock(startVal uint16, isFirst bool) []inputRow {
	rows := make([]inputRow, numLimbsPerBlock)
	for i := range rows {
		rows[i] = inputRow{
			data:     startVal + uint16(i),
			selector: 1,
			isFirst:  0,
		}
	}
	if isFirst {
		rows[0].isFirst = 1
	}
	return rows
}

// makeInactiveRows creates n inactive (selector=0) padding rows.
func makeInactiveRows(n int) []inputRow {
	rows := make([]inputRow, n)
	return rows
}

// runAssignmentTest compiles the SHA2 block module with the given input,
// runs the prover, and verifies the proof. It fails the test if any step
// panics or if proof verification fails.
func runAssignmentTest(t *testing.T, rows []inputRow, nbBlockLimit int) {
	t.Helper()

	csvData := generateInputCSV(rows)
	inpCt, err := csvtraces.NewCsvTrace(strings.NewReader(csvData))
	if err != nil {
		t.Fatalf("failed to parse CSV: %v", err)
	}

	var (
		inp sha2BlocksInputs
		mod *sha2BlockModule
	)

	comp := wizard.Compile(func(build *wizard.Builder) {
		inp = sha2BlocksInputs{
			Name:                 "TESTING",
			PackedUint16:         inpCt.GetCommit(build, "PACKED_DATA"),
			Selector:             inpCt.GetCommit(build, "SELECTOR"),
			IsFirstLaneOfNewHash: inpCt.GetCommit(build, "IS_FIRST_LANE_OF_NEW_HASH"),
			MaxNbBlockPerCirc:    nbBlockLimit,
			MaxNbCircuit:         1,
		}
		mod = newSha2BlockModule(build.CompiledIOP, &inp)
	}, dummy.Compile)

	proof := wizard.Prove(comp, func(run *wizard.ProverRuntime) {
		inpCt.Assign(run, inp.PackedUint16, inp.Selector, inp.IsFirstLaneOfNewHash)
		mod.Run(run)
	})

	if err := wizard.Verify(comp, proof); err != nil {
		t.Fatalf("proof verification failed: %v", err)
	}
}

// TestScanCurrHashBoundary exercises the scanCurrHash closure for edge cases
// that previously caused an index-out-of-bounds panic or incorrect cursor
// advancement (see fix for cursorInp+1 bounds check and post-return increment).
func TestScanCurrHashBoundary(t *testing.T) {

	t.Run("single block hash fills entire column", func(t *testing.T) {
		// 32 active rows, NO trailing inactive rows.
		// Triggers the old out-of-bounds bug: cursorInp+1 == numRowInp.
		rows := makeHashBlock(0x1000, true)
		runAssignmentTest(t, rows, 1)
	})

	t.Run("two hashes fill entire column", func(t *testing.T) {
		// 64 active rows across two hashes, no trailing padding.
		// Tests both hash-boundary cursor advancement and end-of-column boundary.
		rows := append(makeHashBlock(0x1000, true), makeHashBlock(0x2000, true)...)
		runAssignmentTest(t, rows, 2)
	})

	t.Run("three consecutive hashes no padding", func(t *testing.T) {
		// 96 rows (padded to 128 by the framework), three hashes back-to-back.
		rows := append(
			append(makeHashBlock(0x1000, true), makeHashBlock(0x2000, true)...),
			makeHashBlock(0x3000, true)...,
		)
		runAssignmentTest(t, rows, 3)
	})

	t.Run("single hash with trailing inactive rows", func(t *testing.T) {
		// 32 active + 32 inactive = 64 rows.
		// The baseline case that already worked before the fix.
		rows := append(makeHashBlock(0x1000, true), makeInactiveRows(32)...)
		runAssignmentTest(t, rows, 1)
	})

	t.Run("two hashes with trailing padding between", func(t *testing.T) {
		// hash1(32) + hash2(32) + pad(64) = 128 rows.
		// Active data for both hashes is contiguous; padding only at the end.
		rows := append(
			append(makeHashBlock(0x1000, true), makeHashBlock(0x2000, true)...),
			makeInactiveRows(64)...,
		)
		runAssignmentTest(t, rows, 2)
	})

	t.Run("hash ending exactly at last active row before padding", func(t *testing.T) {
		// hash1(32) + hash2(32) + pad(64) = 128 rows.
		// Hash boundary lands right before the inactive zone.
		rows := append(
			append(makeHashBlock(0x1000, true), makeHashBlock(0x2000, true)...),
			makeInactiveRows(64)...,
		)
		runAssignmentTest(t, rows, 2)
	})

	t.Run("multi-block single hash fills column", func(t *testing.T) {
		// A single hash requiring two blocks (64 active limbs), filling 64 rows.
		// First block: isFirst=true, second block: isFirst=false (same hash).
		rows := append(makeHashBlock(0x1000, true), makeHashBlock(0x2000, false)...)
		runAssignmentTest(t, rows, 2)
	})

	t.Run("multi-block hash followed by single-block hash no padding", func(t *testing.T) {
		// 2-block hash (64 limbs) + 1-block hash (32 limbs) = 96 rows.
		// Tests cursor advancement when multi-block hash ends and next hash starts.
		rows := append(
			append(makeHashBlock(0x1000, true), makeHashBlock(0x2000, false)...),
			makeHashBlock(0x3000, true)...,
		)
		runAssignmentTest(t, rows, 3)
	})

	t.Run("all inactive rows", func(t *testing.T) {
		// No active data at all — scanCurrHash should return empty and the
		// outer loop should terminate immediately.
		rows := makeInactiveRows(32)
		runAssignmentTest(t, rows, 1)
	})
}
