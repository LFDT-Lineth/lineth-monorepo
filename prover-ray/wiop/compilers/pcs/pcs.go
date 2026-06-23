// pcs implements the polynomial commitment part of the proof system as protocol
// compilation step.
package pcs

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

const (
	pcsProverStateKey = "pcsProverState"
)

func Compile(sys *wiop.System) {

	// Scan all the columns:
	//		- map them to a round or precomputed round
	//		- mark them as internal
	// 	- register an object to hold the commitment
	//  - register an object to hold the FRI commitment
	//	- register an object to hold the Merkle proofs

}

type ProverAction struct {
	IsCommitProverAction bool
	Round                int
}

type Commitment struct {
	// Layout maps column k to its size and position in the batch of size k
	Layout []ColumnLocation
	// Is the commitment Merkle root
	Root field.Octuplet
}

type ColumnLocation struct {
	Size     int
	Position int
}

func computeLayout(rt *wiop.Runtime) {

}

func (pa *ProverAction) runCommit(rt *wiop.Runtime) {

	// find the list of columns to commit
	currRoundColumns := rt.System.Rounds[pa.Round].Columns
	commitment := Commitment{}

	// all the columns in that round that are not precomputed count are
	// mapped. We use a bucket of counter whose entry k counts the number of
	// polynomials of size "k" we have encountered so far.
	sizeBucketCounter := make([]int, 64)

	for i := range currRoundColumns {

		size := currRoundColumns[i].Module.RuntimeSize(rt)
		sizeIndex := utils.Log2Ceil(size)

		if size != 1<<sizeIndex {
			panic("wiop: only powers of 2 are supported")
		}

		commitment.Layout = append(commitment.Layout, struct {
			Size     int
			Position int
		}{
			Size:     size,
			Position: sizeBucketCounter[sizeIndex],
		})

		sizeBucketCounter[sizeIndex]++
	}

}
