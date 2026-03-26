package accumulator

import (
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"

	//lint:ignore ST1001 -- the package contains a list of standard types for this repo

	. "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/pkg/errors"
)

// ReadNonZeroTrace contains all the information needed to audit a read-only
// access to an existing key in the map.
type ReadNonZeroTrace[K, V io.WriterTo] struct {
	// Identifier for the tree this trace belongs to
	Type         int                 `json:"type"`
	Location     string              `json:"location"`
	NextFreeNode int                 `json:"nextFreeNode"`
	Key          K                   `json:"key"`
	Value        V                   `json:"value"`
	SubRoot      KoalaOctuplet       `json:"subRoot"`
	LeafOpening  LeafOpening         `json:"leaf"`
	Proof        smt_koalabear.Proof `json:"proof"`
}

// ReadNonZeroAndProve perform a read on the accumulator and returns a trace.
// Panics if the the associated key is missing.
func (p *ProverState[K, V]) ReadNonZeroAndProve(key K) ReadNonZeroTrace[K, V] {

	// Find the position of the leaf containing our value
	i, found := p.FindKey(key)
	if !found {
		utils.Panic("called read-non-zero, but the key was not present")
	}

	tuple := p.Data.MustGet(i)

	if hash(key) != hash(tuple.Key) {
		utils.Panic("sanity-check : the key mismatched")
	}

	return ReadNonZeroTrace[K, V]{
		Location:     p.Location,
		Key:          tuple.Key,
		Value:        tuple.Value,
		LeafOpening:  tuple.LeafOpening,
		SubRoot:      p.SubTreeRoot(),
		Proof:        p.Tree.MustProve(int(i)),
		NextFreeNode: int(p.NextFreeNode),
	}
}

// Verify a read on the accumulator. Returns an error if the verification fails.
func (v *VerifierState[K, V]) ReadNonZeroVerify(trace ReadNonZeroTrace[K, V]) error {

	// If the location does not match the we return an error
	if v.Location != trace.Location {
		return fmt.Errorf("inconsistent location : %v != %v", v.Location, trace.Location)
	}

	// Check that verifier's root is the same as the one in the traces
	if v.SubTreeRoot != trace.SubRoot {
		return fmt.Errorf("inconsistent root %v != %v", v.SubTreeRoot.Hex(), trace.SubRoot.Hex())
	}

	tuple := KVOpeningTuple[K, V]{
		Key:         trace.Key,
		Value:       trace.Value,
		LeafOpening: trace.LeafOpening,
	}

	leaf, err := tuple.CheckAndLeaf()
	if err != nil {
		return errors.WithMessage(err, "read non zero verifier failed")
	}

	if smt_koalabear.Verify(&trace.Proof, field.Octuplet(leaf), field.Octuplet(trace.SubRoot)) != nil {
		return fmt.Errorf("merkle proof verification failed")
	}

	// Check that NextFreeNode for the Prover and the Verifier are the same
	if v.NextFreeNode != int64(trace.NextFreeNode) {
		return fmt.Errorf("inconsistent NextFreeNode %v != %v", v.NextFreeNode, trace.NextFreeNode)
	}

	return nil
}

// DeferMerkleChecks implements [Trace]
func (trace ReadNonZeroTrace[K, V]) DeferMerkleChecks(
	appendTo []smt_koalabear.ProvedClaim,
) []smt_koalabear.ProvedClaim {
	tuple := KVOpeningTuple[K, V]{
		Key:         trace.Key,
		Value:       trace.Value,
		LeafOpening: trace.LeafOpening,
	}

	leaf, _ := tuple.CheckAndLeaf()
	appendTo = append(appendTo, smt_koalabear.ProvedClaim{Proof: trace.Proof, Root: trace.SubRoot, Leaf: leaf})
	return appendTo
}

func (trace ReadNonZeroTrace[K, V]) HKey() KoalaOctuplet {
	return hash(trace.Key)
}

func (trace ReadNonZeroTrace[K, V]) RWInt() int {
	return 0
}
