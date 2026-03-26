package accumulator

import (
	"io"

	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"

	//lint:ignore ST1001 -- the package contains a list of standard types for this repo
	. "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Track the informations that are maintained by the verifier
type VerifierState[K, V io.WriterTo] struct {
	// Location, identifier for the tree
	Location string
	// Track the index of the next free node
	NextFreeNode int64
	// Internal tree
	SubTreeRoot KoalaOctuplet
}

// VerifierState create a verifier state from a prover state. The returned
// verifier state corresponds to a snapshot of the prover's state.
func (p *ProverState[K, V]) VerifierState() VerifierState[K, V] {
	return VerifierState[K, V]{
		Location:     p.Location,
		NextFreeNode: p.NextFreeNode,
		SubTreeRoot:  KoalaOctuplet(p.Tree.Root),
	}
}

// updateCheckRoot audit an atomic update of the merkle tree (e.g. one leaf) and
// returns the new root
func updateCheckRoot(proof smt_koalabear.Proof, root, old, new KoalaOctuplet) (newRoot KoalaOctuplet, err error) {

	if ok := smt_koalabear.Verify(&proof, KoalaOctuplet(old), KoalaOctuplet(root)); ok != nil {
		return KoalaOctuplet{}, errors.New("root update audit failed : could not authenticate the old")
	}

	// Note: all possible errors are already convered by `proof.Verify`
	newRootOct, _ := smt_koalabear.RecoverRoot(&proof, new)
	logrus.Tracef("update check root %v leaf: %x->%x root: %x->%x\n", proof.Path, old, new, root, newRootOct)
	return newRootOct, nil
}

// TopRoot returns the top-root hash which includes `NextFreeNode` and the
// `SubTreeRoot`
func (v *VerifierState[K, V]) TopRoot() KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	WriteInt64On64Bytes(hasher, v.NextFreeNode)
	v.SubTreeRoot.WriteTo(hasher)
	digest := hasher.Sum(nil)
	return MustBytesToKoalaOctuplet(digest)
}

// deferCheckUpdateRoot appends to `appendTo` the merkle proof claims that are
// audited when checking the updates of a leaf in the tree. The function panics
// if the proof malformed.
func deferCheckUpdateRoot(
	proof smt_koalabear.Proof,
	root, old, new KoalaOctuplet,
	appendTo []smt_koalabear.ProvedClaim,
) (appended []smt_koalabear.ProvedClaim, newRoot KoalaOctuplet) {
	newRootOct, err := smt_koalabear.RecoverRoot(&proof, new)
	if err != nil {
		panic(err)
	}
	newRoot = KoalaOctuplet(newRootOct)
	appended = append(appendTo,
		smt_koalabear.ProvedClaim{Proof: proof, Leaf: old, Root: root},
		smt_koalabear.ProvedClaim{Proof: proof, Leaf: new, Root: newRoot},
	)
	return appended, newRoot
}
