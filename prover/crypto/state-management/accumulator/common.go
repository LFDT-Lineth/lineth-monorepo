package accumulator

import (
	"io"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"

	//lint:ignore ST1001 -- the package contains a list of standard types for this repo
	. "github.com/consensys/linea-monorepo/prover/utils/types"
)

// Generic hashing for object satisfying the io.WriterTo interface
func hash[T io.WriterTo](m T) KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	m.WriteTo(hasher)
	digest := hasher.Sum(nil)
	var d KoalaOctuplet
	if err := d.SetBytes(digest); err != nil {
		panic(err)
	}
	return d
}

// Trace is an interface shared by all the "traces" types. Used to
// collect MerkleProof verifications claims.
type Trace interface {
	// DeferMerkleChecks appends all the merkle-proofs checks happening in a trace
	// verification into a slice of smt.ProvedClaim
	DeferMerkleChecks(appendTo []smt_koalabear.ProvedClaim) []smt_koalabear.ProvedClaim
	// HKey returns the HKey of the trace
	HKey() KoalaOctuplet
	// RWInt returns 0 is the trace is a read-only operation and 1 if it is a
	// read-write operation.
	RWInt() int
}
