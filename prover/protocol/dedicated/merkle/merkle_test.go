package merkle_test

import (
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/merkle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"
)

const (
	blockSize = 8
)

// merkleTestBuilder is used to build the assignment of merkle proofs
// and is implemented like a writer.
type merkleTestBuilder struct {
	proofs             []smt_koalabear.Proof
	pos                []field.Element
	roots              []field.Octuplet
	leaves             []field.Octuplet
	useNextMerkleProof []field.Element
	isActive           []field.Element
	counter            []field.Element
}

// newMerkleTestBuilder returns an empty builder
func newMerkleTestBuilder(numProofs int) *merkleTestBuilder {
	return &merkleTestBuilder{
		proofs:             make([]smt_koalabear.Proof, 0, numProofs),
		pos:                make([]field.Element, 0, numProofs),
		roots:              make([]field.Octuplet, 0, numProofs),
		leaves:             make([]field.Octuplet, 0, numProofs),
		useNextMerkleProof: make([]field.Element, 0, numProofs),
		isActive:           make([]field.Element, 0, numProofs),
		counter:            make([]field.Element, 0, numProofs),
	}
}

// assignProofs is a low level function to be used by each test to assign values to
// the various columns for testing
func (b *merkleTestBuilder) assignProofs(numProofs, depth int, isReuse bool, reusePos, numNonReUseProofs int) {
	leaves := make([]field.Octuplet, 1<<depth)
	for i := range leaves {
		// #nosec G404 -- no need for a cryptographically strong PRNG for testing purposes
		var x vortex.Hash
		for j := 0; j < 8; j++ {
			x[j].SetRandom()
		}
		leaves[i] = x
	}
	tree := smt_koalabear.NewTree(leaves)
	root := tree.Root
	if !isReuse {

		for i := 0; i < numProofs; i++ {
			proof := tree.MustProve(i)
			b.proofs = append(b.proofs, proof)
			var po field.Element
			le := leaves[i]
			ro := root
			po.SetUint64(uint64(i))
			b.leaves = append(b.leaves, le)
			b.roots = append(b.roots, ro)
			b.pos = append(b.pos, po)

		}
	} else {
		// We first assign the counter column
		for i := 0; i < numProofs; i++ {
			b.counter = append(b.counter, field.NewElement(uint64(i)))
		}
		switch reusePos {
		// We disable useNextMerkleProof at the last
		case 0:
			for i := 0; i < (numProofs-numNonReUseProofs)/2; i++ {
				j := 2 * i
				proof_old := tree.MustProve(j)
				root_old := tree.Root
				b.proofs = append(b.proofs, proof_old)

				var po field.Element
				le := leaves[j]
				ro := root_old
				po.SetUint64(uint64(j))
				b.leaves = append(b.leaves, le)
				b.roots = append(b.roots, ro)
				b.pos = append(b.pos, po)
				// At the starting row for Update useNextMerkleProof is one
				b.useNextMerkleProof = append(b.useNextMerkleProof, field.One())
				b.isActive = append(b.isActive, field.One())

				// Update the tree by changing the leaf value at position j
				var newVal [8]field.Element
				for i := 0; i < 8; i++ {
					newVal[i].SetRandom()
				}

				tree.Update(j, newVal)
				leaves[j] = newVal
				proof_new := tree.MustProve(j)
				root_new := tree.Root
				b.proofs = append(b.proofs, proof_new)

				var po_2 field.Element
				le_2 := leaves[j]
				ro_2 := root_new
				po_2.SetUint64(uint64(j))
				b.leaves = append(b.leaves, le_2)
				b.pos = append(b.pos, po_2)
				b.roots = append(b.roots, ro_2)

				// At the updation row useNextMerkleProof is zero
				b.useNextMerkleProof = append(b.useNextMerkleProof, field.Zero())
				b.isActive = append(b.isActive, field.One())
			}
			// In the below code, there is no more update, so the root will
			// not change anymore
			root := tree.Root
			for i := (numProofs - numNonReUseProofs); i < numProofs; i++ {
				proof := tree.MustProve(i)
				b.proofs = append(b.proofs, proof)

				var po field.Element
				le := leaves[i]
				ro := root
				po.SetUint64(uint64(i))
				b.leaves = append(b.leaves, le)
				b.roots = append(b.roots, ro)
				b.pos = append(b.pos, po)

				b.useNextMerkleProof = append(b.useNextMerkleProof, field.Zero())
				b.isActive = append(b.isActive, field.One())
			}
		// We disable useNextMerkleProof at the begining
		case 1:
			// In the below code, there is no update, so the root will
			// not change anymore
			root := tree.Root
			for i := 0; i < numNonReUseProofs; i++ {
				proof := tree.MustProve(i)
				b.proofs = append(b.proofs, proof)

				var po field.Element
				le := leaves[i]
				ro := root
				po.SetUint64(uint64(i))
				b.leaves = append(b.leaves, le)
				b.roots = append(b.roots, ro)
				b.pos = append(b.pos, po)

				b.useNextMerkleProof = append(b.useNextMerkleProof, field.Zero())
				b.isActive = append(b.isActive, field.One())
			}
			// In the below code there is reuse of Merkle proofs
			for i := (numNonReUseProofs / 2); i < (numProofs / 2); i++ {
				j := 2 * i
				proof_old := tree.MustProve(j)
				root_old := tree.Root
				b.proofs = append(b.proofs, proof_old)

				var po field.Element
				le := leaves[j]
				ro := root_old
				po.SetUint64(uint64(j))
				b.leaves = append(b.leaves, le)
				b.roots = append(b.roots, ro)
				b.pos = append(b.pos, po)

				// At the starting row for Update useNextMerkleProof is one
				b.useNextMerkleProof = append(b.useNextMerkleProof, field.One())
				b.isActive = append(b.isActive, field.One())

				// Update the tree by changing the leaf value at position j
				var newVal [8]field.Element
				for i := 0; i < 8; i++ {
					newVal[i].SetRandom()
				}

				tree.Update(j, newVal)
				leaves[j] = newVal
				proof_new := tree.MustProve(j)
				root_new := tree.Root
				b.proofs = append(b.proofs, proof_new)

				var po_2 field.Element
				le_2 := leaves[j]
				ro_2 := root_new
				po_2.SetUint64(uint64(j))
				b.leaves = append(b.leaves, le_2)
				b.roots = append(b.roots, ro_2)
				b.pos = append(b.pos, po_2)

				// At the updation row useNextMerkleProof is zero
				b.useNextMerkleProof = append(b.useNextMerkleProof, field.Zero())
				b.isActive = append(b.isActive, field.One())
			}
		// We disable useNextMerkleProof in between (in the third row onwards)
		case 2:
			// In the below code there is reuse of Merkle proofs
			for i := 0; i < 1; i++ {
				j := 2 * i
				proof_old := tree.MustProve(j)
				root_old := tree.Root
				b.proofs = append(b.proofs, proof_old)

				var po field.Element
				le := leaves[j]
				ro := root_old
				po.SetUint64(uint64(j))
				b.leaves = append(b.leaves, le)
				b.roots = append(b.roots, ro)
				b.pos = append(b.pos, po)

				// At the starting row for Update useNextMerkleProof is one
				b.useNextMerkleProof = append(b.useNextMerkleProof, field.One())
				b.isActive = append(b.isActive, field.One())

				// Update the tree by changing the leaf value at position j
				var newVal [8]field.Element
				for i := 0; i < 8; i++ {
					newVal[i].SetRandom()
				}

				tree.Update(j, newVal)
				leaves[j] = newVal
				proof_new := tree.MustProve(j)
				root_new := tree.Root
				b.proofs = append(b.proofs, proof_new)

				var po_2 field.Element
				le_2 := leaves[j]
				ro_2 := root_new
				po_2.SetUint64(uint64(j))
				b.leaves = append(b.leaves, le_2)
				b.roots = append(b.roots, ro_2)
				b.pos = append(b.pos, po_2)

				// At the updation row useNextMerkleProof is zero
				b.useNextMerkleProof = append(b.useNextMerkleProof, field.Zero())
				b.isActive = append(b.isActive, field.One())
			}
			// In the below code, there is no update, so the root will
			// not change anymore
			root := tree.Root
			for i := 2; i < 2+numNonReUseProofs; i++ {
				proof := tree.MustProve(i)
				b.proofs = append(b.proofs, proof)

				var po field.Element
				le := leaves[i]
				ro := root
				po.SetUint64(uint64(i))
				b.leaves = append(b.leaves, le)
				b.roots = append(b.roots, ro)
				b.pos = append(b.pos, po)

				b.useNextMerkleProof = append(b.useNextMerkleProof, field.Zero())
				b.isActive = append(b.isActive, field.One())
			}
			// In the below code there is reuse of Merkle proofs
			for i := (2 + numNonReUseProofs) / 2; i < (numProofs / 2); i++ {
				j := 2 * i
				proof_old := tree.MustProve(j)
				root_old := tree.Root
				b.proofs = append(b.proofs, proof_old)

				var po field.Element
				le := leaves[j]
				ro := root_old
				po.SetUint64(uint64(j))
				b.leaves = append(b.leaves, le)
				b.roots = append(b.roots, ro)
				b.pos = append(b.pos, po)

				// At the starting row for Update useNextMerkleProof is one
				b.useNextMerkleProof = append(b.useNextMerkleProof, field.One())
				b.isActive = append(b.isActive, field.One())

				// Update the tree by changing the leaf value at position j
				var newVal [8]field.Element
				for i := 0; i < 8; i++ {
					newVal[i].SetRandom()
				}

				tree.Update(j, newVal)
				leaves[j] = newVal
				proof_new := tree.MustProve(j)
				root_new := tree.Root
				b.proofs = append(b.proofs, proof_new)

				var po_2 field.Element
				le_2 := leaves[j]
				ro_2 := root_new
				po_2.SetUint64(uint64(j))
				b.leaves = append(b.leaves, le_2)
				b.roots = append(b.roots, ro_2)
				b.pos = append(b.pos, po_2)

				// At the updation row useNextMerkleProof is zero
				b.useNextMerkleProof = append(b.useNextMerkleProof, field.Zero())
				b.isActive = append(b.isActive, field.One())
			}
		default:
			utils.Panic("Not a valid disable position")
		}
	}
}

func TestMerklePow2(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	// Generates a list of Merkle proofs for the same tree
	depth := 4
	numProofs := 1 << 2
	builder := newMerkleTestBuilder(numProofs)
	builder.assignProofs(numProofs, depth, false, 0, 0)

	define := func(b *wizard.Builder) {
		var proofcol, rootscol, leavescol [blockSize]ifaces.Column

		for i := 0; i < blockSize; i++ {
			proofcol[i] = b.RegisterCommit(ifaces.ColIDf("PROOF_%v", i), depth*numProofs)
			rootscol[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%v", i), numProofs)
			leavescol[i] = b.RegisterCommit(ifaces.ColIDf("LEAVES_%v", i), numProofs)
		}
		poscol := b.RegisterCommit("POS", numProofs)

		merkle.MerkleProofCheck(b.CompiledIOP, "TEST", depth, numProofs, poscol, proofcol, rootscol, leavescol)
	}

	prove := func(run *wizard.ProverRuntime) {
		proofAssignment := merkle.PackMerkleProofs(builder.proofs)
		trRoot := merkle.Transpose(builder.roots)
		trLeaves := merkle.Transpose(builder.leaves)

		for i := 0; i < blockSize; i++ {
			run.AssignColumn(ifaces.ColIDf("PROOF_%v", i), proofAssignment[i])
			run.AssignColumn(ifaces.ColIDf("ROOTS_%v", i), smartvectors.NewRegular(trRoot[i]))
			run.AssignColumn(ifaces.ColIDf("LEAVES_%v", i), smartvectors.NewRegular(trLeaves[i]))
		}
		run.AssignColumn("POS", smartvectors.NewRegular(builder.pos))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)

	require.NoError(t, err)

}

func TestMerkleNotPow2(t *testing.T) {

	logrus.SetLevel(logrus.DebugLevel)

	// Generates a list of Merkle proofs for the same tree
	depth := 3
	numProofs := 1 << 2
	smallSize := utils.NextPowerOfTwo(numProofs)
	largeSize := utils.NextPowerOfTwo(numProofs * depth)
	builder := newMerkleTestBuilder(numProofs)
	builder.assignProofs(numProofs, depth, false, 0, 0)

	define := func(b *wizard.Builder) {
		var proofcol, rootscol, leavescol [blockSize]ifaces.Column

		for i := 0; i < blockSize; i++ {
			proofcol[i] = b.RegisterCommit(ifaces.ColIDf("PROOF_%v", i), largeSize)
			rootscol[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%v", i), smallSize)
			leavescol[i] = b.RegisterCommit(ifaces.ColIDf("LEAVES_%v", i), smallSize)
		}
		poscol := b.RegisterCommit("POS", smallSize)

		merkle.MerkleProofCheck(b.CompiledIOP, "TEST", depth, numProofs, poscol, proofcol, rootscol, leavescol)
	}

	prove := func(run *wizard.ProverRuntime) {
		proofAssignment := merkle.PackMerkleProofs(builder.proofs)
		trRoot := merkle.Transpose(builder.roots)
		trLeaves := merkle.Transpose(builder.leaves)

		for i := 0; i < blockSize; i++ {
			run.AssignColumn(ifaces.ColIDf("PROOF_%v", i), proofAssignment[i])
			run.AssignColumn(ifaces.ColIDf("ROOTS_%v", i), smartvectors.NewRegular(trRoot[i]))
			run.AssignColumn(ifaces.ColIDf("LEAVES_%v", i), smartvectors.NewRegular(trLeaves[i]))
		}
		run.AssignColumn("POS", padWithLast(builder.pos))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)

	require.NoError(t, err)

}

func TestMerkleManySizes(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)

	dimensions := []map[string]int{
		{"depth": 4, "numProofs": 16},
		{"depth": 8, "numProofs": 16},
		{"depth": 4, "numProofs": 14},
		{"depth": 5, "numProofs": 14},
		{"depth": 6, "numProofs": 14},
		{"depth": 6, "numProofs": 48},
	}

	runTest := func(dims map[string]int) {

		// Generates a list of Merkle proofs for the same tree
		depth := dims["depth"]
		numProofs := dims["numProofs"]

		smallSize := utils.NextPowerOfTwo(numProofs)
		largeSize := utils.NextPowerOfTwo(numProofs * depth)
		builder := newMerkleTestBuilder(numProofs)
		builder.assignProofs(numProofs, depth, false, 0, 0)

		define := func(b *wizard.Builder) {
			var proofcol, rootscol, leavescol [blockSize]ifaces.Column

			for i := 0; i < blockSize; i++ {
				proofcol[i] = b.RegisterCommit(ifaces.ColIDf("PROOF_%v", i), largeSize)
				rootscol[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%v", i), smallSize)
				leavescol[i] = b.RegisterCommit(ifaces.ColIDf("LEAVES_%v", i), smallSize)
			}
			poscol := b.RegisterCommit("POS", smallSize)

			merkle.MerkleProofCheck(b.CompiledIOP, "TEST", depth, numProofs, poscol, proofcol, rootscol, leavescol)
		}

		prove := func(run *wizard.ProverRuntime) {
			proofAssignment := merkle.PackMerkleProofs(builder.proofs)

			trRoot := merkle.Transpose(builder.roots)
			trLeaves := merkle.Transpose(builder.leaves)

			for i := 0; i < blockSize; i++ {
				run.AssignColumn(ifaces.ColIDf("PROOF_%v", i), proofAssignment[i])
				run.AssignColumn(ifaces.ColIDf("ROOTS_%v", i), smartvectors.RightZeroPadded(trRoot[i], smallSize))
				run.AssignColumn(ifaces.ColIDf("LEAVES_%v", i), smartvectors.RightZeroPadded(trLeaves[i], smallSize))
			}

			run.AssignColumn("POS", smartvectors.RightZeroPadded(builder.pos, smallSize))
		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prove)
		err := wizard.Verify(comp, proof)
		require.NoError(t, err)
	}

	for i := range dimensions {
		t.Logf("run test case #%v", i)
		runTest(dimensions[i])
	}

}

// Tests for reuse of Merkle trees
func TestMerklePow2ReuseMerkle(t *testing.T) {
	logrus.SetLevel(logrus.DebugLevel)
	testOps := map[int]string{
		0: "disableAtLast",
		1: "disableAtFirst",
		2: "disableInBetween",
	}
	runTest := func(disablePos string) {
		// Generates a list of Merkle proofs for the same tree
		depth := 4
		numProofs := 1 << 3
		// Must be even and can be atmost (numProofs - 2)
		numNonReUseProofs := 2
		builder := newMerkleTestBuilder(numProofs)
		switch disablePos {
		// We disable useNextMerkleProof at the last
		case "disableAtLast":
			builder.assignProofs(numProofs, depth, true, 0, numNonReUseProofs)
		// We disable useNextMerkleProof at the begining
		case "disableAtFirst":
			builder.assignProofs(numProofs, depth, true, 1, numNonReUseProofs)
		// We disable useNextMerkleProof in between (in the third row onwards)
		case "disableInBetween":
			builder.assignProofs(numProofs, depth, true, 2, numNonReUseProofs)
		}

		define := func(b *wizard.Builder) {
			var proofcol, rootscol, leavescol [blockSize]ifaces.Column

			for i := 0; i < blockSize; i++ {
				proofcol[i] = b.RegisterCommit(ifaces.ColIDf("PROOF_%v", i), depth*numProofs)
				rootscol[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%v", i), numProofs)
				leavescol[i] = b.RegisterCommit(ifaces.ColIDf("LEAVES_%v", i), numProofs)
			}
			poscol := b.RegisterCommit("POS", numProofs)
			useNextMerkleProofCol := b.RegisterCommit("REUSE_NEXT_PROOF", numProofs)
			isActiveCol := b.RegisterCommit("IS_ACTIVE", numProofs)
			counterCol := b.RegisterCommit("COUNTER", numProofs)

			merkle.MerkleProofCheckWithReuse(b.CompiledIOP, "TEST", depth, numProofs, poscol, proofcol, rootscol, leavescol, useNextMerkleProofCol, isActiveCol, counterCol)
		}

		prove := func(run *wizard.ProverRuntime) {
			proofAssignment := merkle.PackMerkleProofs(builder.proofs)

			trRoot := merkle.Transpose(builder.roots)
			trLeaves := merkle.Transpose(builder.leaves)

			for i := 0; i < blockSize; i++ {
				run.AssignColumn(ifaces.ColIDf("PROOF_%v", i), proofAssignment[i])
				run.AssignColumn(ifaces.ColIDf("ROOTS_%v", i), smartvectors.NewRegular(trRoot[i]))
				run.AssignColumn(ifaces.ColIDf("LEAVES_%v", i), smartvectors.NewRegular(trLeaves[i]))
			}
			run.AssignColumn("POS", smartvectors.NewRegular(builder.pos))
			run.AssignColumn("REUSE_NEXT_PROOF", smartvectors.NewRegular(builder.useNextMerkleProof))
			run.AssignColumn("IS_ACTIVE", smartvectors.NewRegular(builder.isActive))
			run.AssignColumn("COUNTER", smartvectors.NewRegular(builder.counter))
		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prove)
		err := wizard.Verify(comp, proof)

		require.NoError(t, err)
	}
	for i := range testOps {
		t.Logf("run test case #%v", i)
		runTest(testOps[i])
	}

}

func TestMerkleNotPow2ReuseMerkle(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)
	testOps := map[int]string{
		0: "disableAtLast",
		1: "disableAtFirst",
		2: "disableInBetween",
	}
	runTest := func(disablePos string) {
		// Generates a list of Merkle proofs for the same tree
		depth := 3
		numProofs := 1 << 2
		// Must be even and can be atmost (numProofs - 2)
		numNonReUseProofs := 2
		smallSize := utils.NextPowerOfTwo(numProofs)
		largeSize := utils.NextPowerOfTwo(numProofs * depth)
		builder := newMerkleTestBuilder(numProofs)
		switch disablePos {
		// We disable useNextMerkleProof at the last
		case "disableAtLast":
			builder.assignProofs(numProofs, depth, true, 0, numNonReUseProofs)
		// We disable useNextMerkleProof at the begining
		case "disableAtFirst":
			builder.assignProofs(numProofs, depth, true, 1, numNonReUseProofs)
		// We disable useNextMerkleProof in between (in the third row onwards)
		case "disableInBetween":
			builder.assignProofs(numProofs, depth, true, 2, numNonReUseProofs)
		}

		define := func(b *wizard.Builder) {
			var proofcol, rootscol, leavescol [blockSize]ifaces.Column

			for i := 0; i < blockSize; i++ {
				proofcol[i] = b.RegisterCommit(ifaces.ColIDf("PROOF_%v", i), largeSize)
				rootscol[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%v", i), smallSize)
				leavescol[i] = b.RegisterCommit(ifaces.ColIDf("LEAVES_%v", i), smallSize)
			}
			poscol := b.RegisterCommit("POS", smallSize)
			useNextMerkleProofCol := b.RegisterCommit("REUSE_NEXT_PROOF", smallSize)
			isActiveCol := b.RegisterCommit("IS_ACTIVE", smallSize)
			counterCol := b.RegisterCommit("COUNTER", smallSize)

			merkle.MerkleProofCheckWithReuse(b.CompiledIOP, "TEST", depth, numProofs, poscol, proofcol, rootscol, leavescol, useNextMerkleProofCol, isActiveCol, counterCol)
		}

		prove := func(run *wizard.ProverRuntime) {
			proofAssignment := merkle.PackMerkleProofs(builder.proofs)
			trRoot := merkle.Transpose(builder.roots)
			trLeaves := merkle.Transpose(builder.leaves)

			for i := 0; i < blockSize; i++ {
				run.AssignColumn(ifaces.ColIDf("PROOF_%v", i), proofAssignment[i])
				run.AssignColumn(ifaces.ColIDf("ROOTS_%v", i), padWithLast(trRoot[i]))
				run.AssignColumn(ifaces.ColIDf("LEAVES_%v", i), padWithLast(trLeaves[i]))
			}
			run.AssignColumn("POS", padWithLast(builder.pos))
			run.AssignColumn("REUSE_NEXT_PROOF", smartvectors.NewRegular(builder.useNextMerkleProof))
			run.AssignColumn("IS_ACTIVE", smartvectors.NewRegular(builder.isActive))
			run.AssignColumn("COUNTER", smartvectors.NewRegular(builder.counter))
		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prove)
		err := wizard.Verify(comp, proof)

		require.NoError(t, err)
	}
	for i := range testOps {
		t.Logf("run test case #%v", i)
		runTest(testOps[i])
	}

}

func TestMerkleManySizesReuseMerkle(t *testing.T) {

	logrus.SetLevel(logrus.FatalLevel)
	// Note: numNonReUseProofs should be even and can be atmost (numProofs - 2)
	dimensions := []map[string]int{
		{"depth": 4, "numProofs": 16, "numNonReUseProofs": 2, "disablePosition": 2},
		{"depth": 8, "numProofs": 16, "numNonReUseProofs": 4, "disablePosition": 1},
		{"depth": 4, "numProofs": 14, "numNonReUseProofs": 8, "disablePosition": 0},
		{"depth": 5, "numProofs": 24, "numNonReUseProofs": 18, "disablePosition": 0},
		{"depth": 4, "numProofs": 12, "numNonReUseProofs": 10, "disablePosition": 2},
		{"depth": 6, "numProofs": 48, "numNonReUseProofs": 20, "disablePosition": 2},
	}

	runTest := func(dims map[string]int) {

		// Generates a list of Merkle proofs for the same tree
		depth := dims["depth"]
		numProofs := dims["numProofs"]
		numNonReUseProofs := dims["numNonReUseProofs"]
		disablePos := dims["disablePosition"]

		smallSize := utils.NextPowerOfTwo(numProofs)
		largeSize := utils.NextPowerOfTwo(numProofs * depth)

		builder := newMerkleTestBuilder(numProofs)
		builder.assignProofs(numProofs, depth, true, disablePos, numNonReUseProofs)
		define := func(b *wizard.Builder) {
			var proofcol, rootscol, leavescol [blockSize]ifaces.Column

			for i := 0; i < blockSize; i++ {
				proofcol[i] = b.RegisterCommit(ifaces.ColIDf("PROOF_%v", i), largeSize)
				rootscol[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%v", i), smallSize)
				leavescol[i] = b.RegisterCommit(ifaces.ColIDf("LEAVES_%v", i), smallSize)
			}
			poscol := b.RegisterCommit("POS", smallSize)
			useNextMerkleProofCol := b.RegisterCommit("REUSE_NEXT_PROOF", smallSize)
			isActiveCol := b.RegisterCommit("IS_ACTIVE", smallSize)
			counterCol := b.RegisterCommit("COUNTER", smallSize)

			merkle.MerkleProofCheckWithReuse(b.CompiledIOP, "TEST", depth, numProofs, poscol, proofcol, rootscol, leavescol, useNextMerkleProofCol, isActiveCol, counterCol)
		}

		prove := func(run *wizard.ProverRuntime) {
			proofAssignment := merkle.PackMerkleProofs(builder.proofs)
			trRoot := merkle.Transpose(builder.roots)
			trLeaves := merkle.Transpose(builder.leaves)

			for i := 0; i < blockSize; i++ {
				run.AssignColumn(ifaces.ColIDf("PROOF_%v", i), proofAssignment[i])
				run.AssignColumn(ifaces.ColIDf("ROOTS_%v", i), smartvectors.RightZeroPadded(trRoot[i], smallSize))
				run.AssignColumn(ifaces.ColIDf("LEAVES_%v", i), smartvectors.RightZeroPadded(trLeaves[i], smallSize))
			}
			run.AssignColumn("POS", smartvectors.RightZeroPadded(builder.pos, smallSize))
			run.AssignColumn("REUSE_NEXT_PROOF", smartvectors.RightZeroPadded(builder.useNextMerkleProof, smallSize))
			run.AssignColumn("IS_ACTIVE", smartvectors.RightZeroPadded(builder.isActive, smallSize))
			run.AssignColumn("COUNTER", smartvectors.RightZeroPadded(builder.counter, smallSize))
		}

		comp := wizard.Compile(define, dummy.Compile)
		proof := wizard.Prove(comp, prove)
		err := wizard.Verify(comp, proof)
		require.NoError(t, err)
	}

	for i := range dimensions {
		t.Logf("run test case #%v", i)
		runTest(dimensions[i])
	}

}

// Test where numProofs and MaxNumProofs are different
// This case is not handled in the non-reuse case
func TestDifferentNumProofMaxProof(t *testing.T) {
	depth := 4
	numProofs := 4
	maxNumProofs := 6
	numNonReUseProofs := 2
	smallSize := utils.NextPowerOfTwo(maxNumProofs)
	largeSize := utils.NextPowerOfTwo(maxNumProofs * depth)
	// We allocate the maximum size for the columns
	builder := newMerkleTestBuilder(maxNumProofs)
	// We assign only numProofs (< maxNumProofs) number of proofs
	builder.assignProofs(numProofs, depth, true, 0, numNonReUseProofs)
	define := func(b *wizard.Builder) {
		var proofcol, rootscol, leavescol [blockSize]ifaces.Column

		for i := 0; i < blockSize; i++ {
			proofcol[i] = b.RegisterCommit(ifaces.ColIDf("PROOF_%v", i), largeSize)
			rootscol[i] = b.RegisterCommit(ifaces.ColIDf("ROOTS_%v", i), smallSize)
			leavescol[i] = b.RegisterCommit(ifaces.ColIDf("LEAVES_%v", i), smallSize)
		}
		poscol := b.RegisterCommit("POS", smallSize)
		useNextMerkleProofCol := b.RegisterCommit("REUSE_NEXT_PROOF", smallSize)
		isActiveCol := b.RegisterCommit("IS_ACTIVE", smallSize)
		counterCol := b.RegisterCommit("COUNTER", smallSize)

		merkle.MerkleProofCheckWithReuse(b.CompiledIOP, "TEST", depth, maxNumProofs, poscol, proofcol, rootscol, leavescol, useNextMerkleProofCol, isActiveCol, counterCol)
	}

	prove := func(run *wizard.ProverRuntime) {
		proofs_ := merkle.PackMerkleProofs(builder.proofs)
		var proofsReg [blockSize][]field.Element
		var proofPadded [blockSize]smartvectors.SmartVector
		trRoot := merkle.Transpose(builder.roots)
		trLeaves := merkle.Transpose(builder.leaves)
		for i := 0; i < blockSize; i++ {
			proofsReg[i] = smartvectors.IntoRegVec(proofs_[i])
			proofPadded[i] = smartvectors.RightZeroPadded(proofsReg[i], largeSize)
			run.AssignColumn(ifaces.ColIDf("PROOF_%v", i), proofPadded[i])
			run.AssignColumn(ifaces.ColIDf("ROOTS_%v", i), smartvectors.RightZeroPadded(trRoot[i], smallSize))
			run.AssignColumn(ifaces.ColIDf("LEAVES_%v", i), smartvectors.RightZeroPadded(trLeaves[i], smallSize))
		}
		run.AssignColumn("POS", smartvectors.RightZeroPadded(builder.pos, smallSize))
		run.AssignColumn("REUSE_NEXT_PROOF", smartvectors.RightZeroPadded(builder.useNextMerkleProof, smallSize))
		run.AssignColumn("IS_ACTIVE", smartvectors.RightZeroPadded(builder.isActive, smallSize))
		run.AssignColumn("COUNTER", smartvectors.RightZeroPadded(builder.counter, smallSize))
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prove)
	err := wizard.Verify(comp, proof)
	require.NoError(t, err)

}

func padWithLast(v []field.Element) smartvectors.SmartVector {
	return smartvectors.RightPadded(v, v[len(v)-1], utils.NextPowerOfTwo(len(v)))
}
