package fft_test

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/fft"
)

// Aimed at being run only during go-race
func TestGetCosetConcurrent(t *testing.T) {
	for i := 0; i < 100000; i++ {
		go func() { _ = fft.NewDomain(1<<12).WithCustomCoset(4, 3) }()
	}
}
