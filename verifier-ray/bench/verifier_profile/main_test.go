package main

import (
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
