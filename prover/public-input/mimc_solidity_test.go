package public_input

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"testing"

	mimc "github.com/consensys/linea-monorepo/prover/crypto/mimc_bls12377"
)

func TestMimcTestCases(t *testing.T) {
	f, err := os.Open("../../contracts/test/hardhat/_testData/mimc-test-data.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	dec := json.NewDecoder(f)
	var cases testCases
	if err := dec.Decode(&cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		inb := make([][]byte, len(tc.In))
		for i, in := range tc.In {
			inb[i], err = hex.DecodeString(in[2:])
			if err != nil {
				t.Fatalf("failed to decode input %s: %v", in, err)
			}
		}
		outb, err := hex.DecodeString(tc.Out[2:])
		if err != nil {
			t.Fatalf("failed to decode output %s: %v", tc.Out, err)
		}
		h := mimc.NewMiMC()
		for _, in := range inb {
			h.Write(in)
		}
		dgst := h.Sum(nil)
		if !bytes.Equal(dgst, outb) {
			t.Fatalf("hash mismatch for input %v: expected %x, got %x", tc.In, outb, dgst)
		}
		// t.Logf("input: %v, expected out: %v, computed output: %x", tc.In, tc.Out, dgst)
	}
}

type testCase struct {
	In  []string `json:"in"`  // inputs as hex strings (with "0x" prefix)
	Out string   `json:"out"` // output as hex string (with "0x" prefix)
}

type testCases []testCase
