package main

import (
	"encoding/csv"
	"strings"
	"testing"
)

func TestParseTrace(t *testing.T) {
	output := strings.Join([]string{
		"----------------------------------------------------------------- PC=1, clock cycle: 1",
		"ADDI sp, sp, 0xff",
		"----------------------------------------------------------------- PC=5, clock cycle: 2",
		"ECALL",
		"ECALL for write",
		"VERIFIER-MARK\t2\t5",
		"----------------------------------------------------------------- PC=9, clock cycle: 3",
		"LD a0, 0x0(sp)",
		"----------------------------------------------------------------- PC=13, clock cycle: 4",
		"ECALL",
		"ECALL for write",
		"VERIFIER-MARK\t4\t8",
	}, "\n")

	stats, err := parseTrace(strings.NewReader(output))
	if err != nil {
		t.Fatal(err)
	}
	if stats.totalCycles != 4 {
		t.Fatalf("total cycles: got %d, want 4", stats.totalCycles)
	}
	markerTranscript := stats.markers[markTranscriptDone]
	if markerTranscript.cycle != 2 || markerTranscript.value != 5 {
		t.Fatalf("marker: got %#v, want cycle=2 value=5", markerTranscript)
	}
	markerVanishing := stats.markers[markVanishingDone]
	if markerVanishing.cycle != 4 || markerVanishing.value != 8 {
		t.Fatalf("marker: got %#v, want cycle=4 value=8", markerVanishing)
	}
}

func TestRenderCSV(t *testing.T) {
	report, err := renderCSV([]result{{
		caseIndex: 7,
		metadata: caseMetadata{
			name:                "Case, With Comma",
			moduleCount:         1,
			dynamicModuleCount:  2,
			roundCount:          3,
			expressionCount:     4,
			bucketCount:         5,
			vanishingCount:      6,
			totalWitnessClaims:  7,
			totalQuotientClaims: 8,
		},
		stats: traceStats{
			totalCycles: 100,
			markers: map[uint64]marker{
				markVerifyStart:    {phase: markVerifyStart, cycle: 10},
				markTranscriptDone: {phase: markTranscriptDone, cycle: 40, value: 3},
				markVanishingStart: {phase: markVanishingStart, cycle: 50, value: 3},
				markVanishingDone:  {phase: markVanishingDone, cycle: 90, value: 5},
				markVerifyDone:     {phase: markVerifyDone, cycle: 95, value: 5},
			},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}

	records, err := csv.NewReader(strings.NewReader(report)).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 2 {
		t.Fatalf("record count: got %d, want 2", len(records))
	}
	row := records[1]
	if row[0] != "7" || row[1] != "Case, With Comma" || row[2] != "100" {
		t.Fatalf("unexpected identity/cycle fields: %#v", row)
	}
	if row[3] != "85" || row[4] != "30" || row[5] != "40" || row[6] != "5" {
		t.Fatalf("unexpected profiling fields: %#v", row)
	}
	if row[7] != "1" || row[14] != "8" {
		t.Fatalf("unexpected metadata fields: %#v", row)
	}
}
