package main

import (
	"strings"
	"testing"
)

func TestParseCaseSelector(t *testing.T) {
	cases, err := parseCaseSelector("0,2-4,3", 6)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 2, 3, 4}
	if len(cases) != len(want) {
		t.Fatalf("case count mismatch: got %v, want %v", cases, want)
	}
	for i := range want {
		if cases[i] != want[i] {
			t.Fatalf("case selector mismatch: got %v, want %v", cases, want)
		}
	}
}

func TestParseTrace(t *testing.T) {
	trace := strings.Join([]string{
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

	stats, err := parseTrace(strings.NewReader(trace))
	if err != nil {
		t.Fatal(err)
	}
	if stats.totalCycles != 4 {
		t.Fatalf("total cycles: got %d, want 4", stats.totalCycles)
	}
	if stats.instructions["ADDI"] != 1 || stats.instructions["ECALL"] != 2 || stats.instructions["LD"] != 1 {
		t.Fatalf("unexpected instruction counts: %#v", stats.instructions)
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
