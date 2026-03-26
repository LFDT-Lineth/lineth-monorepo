package merkle

import (
	"fmt"
	"strings"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/variables"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/parallel"
)

// ComputeMod defines by the modules whose responsibility is
// to recompute the Merkle root from the Merkle proofs.
type ComputeMod struct {

	// The compiled IOP
	Comp *wizard.CompiledIOP

	// Number of rows in the module
	NumRows int
	// Number of proofs
	NumProofs int
	// Depth of the proof
	Depth int
	// Name if the name of parent context joined with a specifier.
	Name string
	// Round of the module
	Round int
	// WithOptProofReuseCheck is true if we want to check reuse of Merkle proofs
	WithOptProofReuseCheck bool

	// Columns of the modules
	Cols struct {
		// IsInactive is a flags used to detect when a column
		// is not used
		IsInactive ifaces.Column
		// NewProof is a flag indicating that a new proof is
		// being verified
		NewProof *dedicated.HeartBeatColumn
		// IsEndOfProof is a flag indicating that this is the
		// last row to verify of a proof and that the NodeHash
		// field contains the computed root.
		IsEndOfProof *dedicated.HeartBeatColumn
		// Root that contains the leaf of the current proof
		Root [blockSize]ifaces.Column
		// Curr contains the current node to be hashes
		Curr [blockSize]ifaces.Column
		// Columns containing the Merkle proof
		Proof [blockSize]ifaces.Column
		// PosBit, indicates whether the current nodes is left
		// or right
		PosBit ifaces.Column
		// PosAcc recomputes the leaf position from the pos-bits
		PosAcc ifaces.Column
		// Zero is a dummy column containing the constant zero
		Zero [blockSize]ifaces.Column
		// Left contains the leftmost node of the current level
		Left [blockSize]ifaces.Column
		// Right contains the rightmost node of the current level
		Right [blockSize]ifaces.Column
		// Interm contains the intermediate hasher state after
		// hashing Left and before hashing Right.
		Interm [blockSize]ifaces.Column
		// NodeHash contains the hash of the parent node.
		NodeHash [blockSize]ifaces.Column
		// UseNextMerkleProof has a special structure when we want to reuse the next Merkle proof
		// and check if two contiguous Merkle proofs are from the same Merkle tree. It is alternatively 1 or 0
		// in a particular segment strating from 1.
		UseNextMerkleProof ifaces.Column
		// The depth expanded version of UseNextMerkleProof is used in the query
		UseNextMerkleProofExpanded ifaces.Column
		// Denote the active part of the accumultor
		IsActiveAccumulator ifaces.Column
		// The depth expanded version of IsActiveAccumulator is used in the query
		IsActiveExpanded ifaces.Column
		// SegmentCounter is constant in a particular proof segment and
		// increases by 1 in the next segment. It is an optonal column when reuse of Merkle proof
		// is verified by the accumulator module
		SegmentCounter ifaces.Column
	}
	// Expressions required to write the queries of the module
	SugarVar struct {
		// NotNewProof = 1 - NewProof
		NotNewProof *symbolic.Expression
		// IsActive = 1 - IsInactive
		IsActive *symbolic.Expression
		// Variables needed for normal Merkle proof verification
		EndOfProof, NewProof, IsInactive, NotEndOfProof *symbolic.Expression
	}
}

// Declare all the columns of the module. Assumes that Proof as
// been assigned to the module. Also registers all the constraints
func (cm *ComputeMod) Define(comp *wizard.CompiledIOP, round int, name string, numProofs, depth int) {

	// Sanity-check that proof has been assigned
	for i := 0; i < blockSize; i++ {
		if cm.Cols.Proof[i] == nil {
			panic("proof must be assigned")
		}
	}

	// Optional sanity-check that UseNextMerkleProof column has been assigned
	if cm.WithOptProofReuseCheck && cm.Cols.UseNextMerkleProof == nil {
		panic("UseNextMerkleProof column must be assigned")
	}

	// Optional sanity-check that IsActiveAccumulator column has been assigned
	if cm.WithOptProofReuseCheck && cm.Cols.IsActiveAccumulator == nil {
		panic("IsActiveAccumulator column must be assigned")
	}

	// Sanity-check the value of NumRows
	numRows := cm.Cols.Proof[0].Size()
	if numRows != utils.NextPowerOfTwo(numProofs*depth) {
		utils.Panic("numRows %v but numProofs %v and depth %v", numRows, numProofs, depth)
	}

	// Assigns the depth and the number of proofs
	cm.Comp = comp
	cm.NumRows = numRows
	cm.NumProofs = numProofs
	cm.Depth = depth
	cm.Round = round
	cm.Name = strings.Join([]string{"MERKLE", "COMPUTEMOD", name}, "_")

	// Declare all the columns
	cm.defineIsInactive()
	cm.defineZero()

	cm.Cols.PosBit = comp.InsertCommit(cm.Round, cm.colname("POSBIT"), cm.NumRows, true)
	cm.Cols.PosAcc = comp.InsertCommit(cm.Round, cm.colname("POSACC"), cm.NumRows, true)
	for i := 0; i < blockSize; i++ {
		cm.Cols.Root[i] = comp.InsertCommit(cm.Round, cm.colname("ROOT_%v", i), cm.NumRows, true)
		cm.Cols.Curr[i] = comp.InsertCommit(cm.Round, cm.colname("CURR_%v", i), cm.NumRows, true)
		cm.Cols.Left[i] = comp.InsertCommit(cm.Round, cm.colname("LEFT_%v", i), cm.NumRows, true)
		cm.Cols.Interm[i] = comp.InsertCommit(cm.Round, cm.colname("INTERM_STATE_%v", i), cm.NumRows, true)
		cm.Cols.Right[i] = comp.InsertCommit(cm.Round, cm.colname("RIGHT_%v", i), cm.NumRows, true)
		cm.Cols.NodeHash[i] = comp.InsertCommit(cm.Round, cm.colname("NODE_HASH_%v", i), cm.NumRows, true)
	}
	if cm.WithOptProofReuseCheck {
		cm.Cols.UseNextMerkleProofExpanded = comp.InsertCommit(cm.Round, cm.colname("USE_NEXT_MERKLE_PROOF_EXPANDED"), cm.NumRows, true)
		cm.Cols.IsActiveExpanded = comp.InsertCommit(cm.Round, cm.colname("IS_ACTIVE_ACCUMULATOR_EXPANDED"), cm.NumRows, true)
		cm.Cols.SegmentCounter = comp.InsertCommit(cm.Round, cm.colname("SEGMENT_COUNTER"), cm.NumRows, true)
	}

	// Initializes all the sugar, we will use for the
	// module constraining
	cm.defineNewProofAndProofEnd()
	cm.createSugarVar()

	// And registers all the queries
	cm.rootConsistency()
	cm.rootIsLastNodeHash()
	cm.posbitBoolean()
	cm.posAccConstraint()
	cm.selectLeft()
	cm.selectRight()
	cm.currIsNextNodeHash()
	cm.checkPoseidon2Compressions()

	// The below means the column size is not a power of two
	// and there is zero padding at the right of all columns
	if !cm.isFullyActive() {
		cm.colZeroWhenInactive()
	}
	if cm.WithOptProofReuseCheck {
		cm.reuseMerkleColZeroWhenInactive()
		cm.checkReuseMerkleProofs()
	}

}

// Declare and assigns the IsInactive Column
func (cm *ComputeMod) defineIsInactive() {

	// check for potential optimization
	if cm.isFullyActive() {
		// we can just skip this column because all rows are used
		// no need to cancel anything.
		return
	}

	if cm.WithOptProofReuseCheck {
		// In this case, the IsInactive column is computed very
		// differently from the IsActiveExpanded column.
		return
	}

	activeSize := cm.Depth * cm.NumProofs

	cm.Cols.IsInactive = cm.Comp.InsertPrecomputed(
		cm.colname("IS_INACTIVE"),
		smartvectors.RightPadded(
			vector.Repeat(field.Zero(), activeSize),
			field.One(),
			cm.NumRows,
		),
	)
}

// Declare the NewProof column
func (cm *ComputeMod) defineNewProofAndProofEnd() {

	var isActive *symbolic.Expression

	if cm.WithOptProofReuseCheck {
		isActive = symbolic.NewVariable(cm.Cols.IsActiveExpanded)
	} else if cm.isFullyActive() {
		isActive = symbolic.NewVariable(verifiercol.NewConstantCol(field.One(), cm.NumRows, ""))
	} else if cm.Cols.IsInactive != nil {
		isActive = symbolic.Sub(1, cm.Cols.IsInactive)
	} else {
		panic("none of the three above cases was matched")
	}

	cm.Cols.NewProof = dedicated.CreateHeartBeat(cm.Comp, cm.Round, cm.Depth, cm.Depth-1, isActive)
	cm.Cols.IsEndOfProof = dedicated.CreateHeartBeat(cm.Comp, cm.Round, cm.Depth, 0, isActive)
}

// Defines the precomputed column ZERO (always zero)
func (cm *ComputeMod) defineZero() {
	for i := 0; i < blockSize; i++ {
		cm.Cols.Zero[i] = verifiercol.NewConstantCol(field.Zero(), cm.NumRows, "")
	}
}

// Defines all the variables that we will need for the constraints
// of the module.
func (cm *ComputeMod) createSugarVar() {
	sug := &cm.SugarVar
	cols := cm.Cols
	sug.NewProof = ifaces.ColumnAsVariable(cols.NewProof.Natural)

	if cm.WithOptProofReuseCheck {
		// new definition of IsActive and IsInactive
		sug.IsActive = ifaces.ColumnAsVariable(cols.IsActiveExpanded)
		sug.IsInactive = symbolic.Sub(1, sug.IsActive)
	} else if cm.isFullyActive() {
		sug.IsInactive = symbolic.NewConstant(0)
		sug.IsActive = symbolic.NewConstant(1)
	} else {
		sug.IsInactive = ifaces.ColumnAsVariable(cols.IsInactive)
		sug.IsActive = symbolic.Sub(1, sug.IsInactive)
	}

	if cm.isFullyActive() {
		sug.EndOfProof = variables.NewPeriodicSample(cm.Depth, 0)
	} else {
		sug.EndOfProof = ifaces.ColumnAsVariable(cols.IsEndOfProof.Natural)
	}

	sug.NotNewProof = symbolic.Sub(1, sug.NewProof)
	sug.NotEndOfProof = symbolic.Sub(1, sug.EndOfProof)
}

// Define the query responsible for ensuring that the roots
// are consistent between themselves and with current. It does not
// require a bound cancellation because the inactive flag prevents
// boundary effects.
// NotNewProof[i]*(IsActive[i]*Root[i+1]-Root[i]) = 0
func (cm *ComputeMod) rootConsistency() {
	sug := cm.SugarVar
	cols := cm.Cols
	for i := 0; i < blockSize; i++ {
		expr := symbolic.Mul(sug.NotNewProof,
			symbolic.Sub(symbolic.Mul(sug.IsActive, ifaces.ColumnAsVariable(column.Shift(cols.Root[i], 1))),
				cols.Root[i]))
		cm.Comp.InsertGlobal(cm.Round, cm.qname("ROOT_CONSISTENCY_%v", i), expr, true)
	}
}

// Ensures that the roots column equals the last nodehash of a segment
// EndOfProof[i] * IsActive[i] * (Root[i] - NodeHash[i]) = 0
func (cm *ComputeMod) rootIsLastNodeHash() {
	sug := cm.SugarVar
	cols := cm.Cols
	for i := 0; i < blockSize; i++ {
		expr := symbolic.Mul(sug.EndOfProof,
			sug.IsActive,
			symbolic.Sub(cols.Root[i], cols.NodeHash[i]))
		cm.Comp.InsertGlobal(cm.Round, cm.qname("ROOT_IS_LAST_NODEHASH_%v", i), expr, true)
	}
}

// Define the query responsible for ensuring that posbits are boolean
// zero when the inactive flag is set.
// (IsActive[i]*PosBit[i]^2)-PosBit[i] = 0
func (cm *ComputeMod) posbitBoolean() {
	sug := cm.SugarVar
	cols := cm.Cols
	expr := symbolic.Sub(symbolic.Mul(sug.IsActive, symbolic.Square(cols.PosBit)),
		cols.PosBit)
	cm.Comp.InsertGlobal(cm.Round, cm.qname("POSBIT_BOOLEAN"), expr)
}

// Defines the global constraint ensuring that posacc is well constructed
// and consistent with posbit. It does not need bound cancellation (and
// it should not be because we want the inactive flag to work on the last
// column.
// (2*NotEndOfProof[i]*PosAcc[i-1]+PosBit[i])*IsActive[i]-PosAcc[i] = 0
func (cm *ComputeMod) posAccConstraint() {
	sug := cm.SugarVar
	cols := cm.Cols
	expr := symbolic.Mul(2,
		sug.NotEndOfProof,
		ifaces.ColumnAsVariable(column.Shift(cols.PosAcc, -1)))
	expr = symbolic.Mul(sug.IsActive,
		symbolic.Add(cols.PosBit, expr))
	expr = symbolic.Sub(expr, cols.PosAcc)
	cm.Comp.InsertGlobal(cm.Round, cm.qname("POSACC_CMPT"), expr, true)
}

// Defines the global constraint responsible for ensuring that left was
// correctly constructed.
// Left[i] - (PosBit[i]*Proof[i]) - (1 - PosBit[i])*Curr[i] = 0
func (cm *ComputeMod) selectLeft() {
	cols := cm.Cols
	for i := 0; i < blockSize; i++ {
		expr := symbolic.Sub(cols.Left[i],
			symbolic.Mul(cols.PosBit, cols.Proof[i]),
			symbolic.Mul(symbolic.Sub(1, cols.PosBit), cols.Curr[i]))
		cm.Comp.InsertGlobal(cm.Round, cm.qname("SELECT_LEFT_%v", i), expr)
	}
}

// Defines the global constraint responsible for ensuring that right was
// correctly constructed.
// Right[i] - (PosBit[i]*Curr[i]) - (1 - PosBit[i])*Proof[i] = 0
func (cm *ComputeMod) selectRight() {
	cols := cm.Cols
	for i := 0; i < blockSize; i++ {
		expr := symbolic.Sub(cols.Right[i],
			symbolic.Mul(cols.PosBit, cols.Curr[i]),
			symbolic.Mul(symbolic.Sub(1, cols.PosBit), cols.Proof[i]))
		cm.Comp.InsertGlobal(cm.Round, cm.qname("SELECT_RIGHT_%v", i), expr)
	}

}

// The columns should cancel in the inactive area
func (cm *ComputeMod) colZeroWhenInactive() {
	cols := cm.Cols
	// Skipping NewProof and IsEndOfProof, as they are precomputed columns e.g.,
	// they are computed before the active area is known
	// Skipping Zero as it is a verifier defined column

	for i := 0; i < blockSize; i++ {
		cm.colZeroAtInactive(cols.Root[i], fmt.Sprintf("ROOT_%v_ZERO_AT_INACTIVE", i))
		cm.colZeroAtInactive(cols.Curr[i], fmt.Sprintf("CURR_%v_ZERO_AT_INACTIVE", i))
		cm.colZeroAtInactive(cols.Proof[i], fmt.Sprintf("PROOF_%v_ZERO_AT_INACTIVE", i))
		cm.colZeroAtInactive(cols.Left[i], fmt.Sprintf("LEFT_%v_ZERO_AT_INACTIVE", i))
		cm.colZeroAtInactive(cols.Right[i], fmt.Sprintf("RIGHT_%v_ZERO_AT_INACTIVE", i))
	}
	cm.colZeroAtInactive(cols.PosBit, "POS_BIT_ZERO_AT_INACTIVE")
	cm.colZeroAtInactive(cols.PosAcc, "POS_ACC_ZERO_AT_INACTIVE")

	// Skipping Interm and NodeHash as they contain zero hashes.
	// Also optional columns for reuse of Merkle proof are treated
	// separately
}

// The columns used in the check of reuse of Merkle proofs,
// should be zero in the inactive area
func (cm *ComputeMod) reuseMerkleColZeroWhenInactive() {
	cols := cm.Cols
	// We verify the expanded version of the optional columns
	// because they are the ones used in the queries
	cm.colZeroAtInactive(cols.UseNextMerkleProofExpanded, "USE_NEXT_MERKLE_PROOF_EXPANDED_ZERO_AT_INACTIVE")
	cm.colZeroAtInactive(cols.IsActiveExpanded, "IS_ACTIVE_EXPANDED_ZERO_AT_INACTIVE")
	cm.colZeroAtInactive(cols.SegmentCounter, "SEGMENT_COUNTER_ZERO_AT_INACTIVE")
}

// Defines the global constraint responsible for ensuring that NodeHash
// is correctly reported into Curr during the computation. No bound cancel
// to ensure that curr is zero at the last row.
// (IsActive[i]*NodeHash[i+1] - Curr[i])*NotNewProof[i] = 0
func (cm *ComputeMod) currIsNextNodeHash() {
	sug := cm.SugarVar
	cols := cm.Cols
	for i := 0; i < blockSize; i++ {
		expr := symbolic.Mul(sug.NotNewProof,
			symbolic.Sub(symbolic.Mul(sug.IsActive, ifaces.ColumnAsVariable(column.Shift(cols.NodeHash[i], 1))),
				cols.Curr[i]))
		cm.Comp.InsertGlobal(cm.Round, cm.qname("CURR_IS_NEXT_NODE_HASH_%v", i), expr, true)
	}
}

// Ensures that the triplets (LEFT, ZERO, INTERM) and (RIGHT, INTERM, NODEHASH)
// are valid Poseidon2 triplets.
func (cm *ComputeMod) checkPoseidon2Compressions() {
	cols := cm.Cols
	cm.Comp.InsertPoseidon2(cm.Round, cm.qname("Poseidon2_LEFT"), cols.Left, cols.Zero, cols.Interm, nil)
	cm.Comp.InsertPoseidon2(cm.Round, cm.qname("Poseidon2_RIGHT"), cols.Right, cols.Interm, cols.NodeHash, nil)
}

// Optional constraints checking reuse of Merkle proofs e.g., all the position
// bits and proofs are the same for the contiguous Merkle proofs
func (cm *ComputeMod) checkReuseMerkleProofs() {
	cols := cm.Cols
	// UseNextMerkleProofExpanded[i] * IsActiveExpanded[i] * (Proof[i] * (SegmentCounter[i] + 1) - Proof[i+depth] * SegmentCounter[i+depth]) = 0, two consecutive proofs are equal when UseNextMerkleProofExpanded is 1, it is in the active area. It also verify that SegmentCounter is consistent with the Proof column
	for i := 0; i < blockSize; i++ {
		expr1 := symbolic.Mul(cols.UseNextMerkleProofExpanded,
			cols.IsActiveExpanded,
			symbolic.Sub(symbolic.Mul(cols.Proof[i], symbolic.Add(cols.SegmentCounter, 1)),
				symbolic.Mul(ifaces.ColumnAsVariable(column.Shift(cols.Proof[i], cm.Depth)),
					ifaces.ColumnAsVariable(column.Shift(cols.SegmentCounter, cm.Depth)))))
		cm.Comp.InsertGlobal(cm.Round, cm.qname("CONSECUTIVE_PROOFS_EQUAL_%v", i), expr1)
	}
	// UseNextMerkleProofExpanded[i] * IsActiveExpanded[i] * (PosBit[i] * (SegmentCounter[i] + 1) - PosBit[i+depth] * SegmentCounter[i+depth]) = 0, two consecutive PosBits are equal when UseNextMerkleProofExpanded is 1 and it is in the active area. It also verify that SegmentCounter is consistent with the PosBit column
	expr2 := symbolic.Mul(cols.UseNextMerkleProofExpanded,
		cols.IsActiveExpanded,
		symbolic.Sub(symbolic.Mul(cols.PosBit, symbolic.Add(cols.SegmentCounter, 1)),
			symbolic.Mul(ifaces.ColumnAsVariable(column.Shift(cols.PosBit, cm.Depth)),
				ifaces.ColumnAsVariable(column.Shift(cols.SegmentCounter, cm.Depth)))))
	cm.Comp.InsertGlobal(cm.Round, cm.qname("CONSECUTIVE_POSBIT_EQUAL"), expr2)

	// UseNextMerkleProofExpanded is segment wise constant i.e.,
	// IsActiveExpanded[i] * (UseNextMerkleProofExpanded[i+1] - UseNextMerkleProofExpanded[i]) * (1 - Proof[i])
	expr3 := symbolic.Mul(cols.IsActiveExpanded,
		symbolic.Sub(ifaces.ColumnAsVariable(column.Shift(cols.UseNextMerkleProofExpanded, 1)), cols.UseNextMerkleProofExpanded),
		symbolic.Sub(1, cols.NewProof.Natural))
	cm.Comp.InsertGlobal(cm.Round, cm.qname("USE_NEXT_MERKLE_PROOF_EXPANDED_CONSISTENCY"), expr3)

	// SegmentCounter is segment wise constant i.e.,
	// IsActiveExpanded[i] * (SegmentCounter[i+1] - SegmentCounter[i]) * (1 - Proof[i]),
	// It does not require a bound cancellation because the inactive flag prevents
	// boundary effects.
	expr4 := symbolic.Mul(cols.IsActiveExpanded,
		symbolic.Sub(ifaces.ColumnAsVariable(column.Shift(cols.SegmentCounter, 1)), cols.SegmentCounter),
		symbolic.Sub(1, cols.NewProof.Natural))
	cm.Comp.InsertGlobal(cm.Round, cm.qname("SEGMENT_COUNTER_CONSISTENCY_1"), expr4)

	// SegmentCounter is incremented by 1 in the next segment in the active area
	// (this is false when SegmentCounter[i+depth] = 0) i.e.,
	// IsActiveExpanded[i+depth] * (SegmentCounter[i+depth] - SegmentCounter[i] - 1)
	expr5 := symbolic.Mul(ifaces.ColumnAsVariable(column.Shift(cols.IsActiveExpanded, cm.Depth)),
		symbolic.Sub(ifaces.ColumnAsVariable(column.Shift(cols.SegmentCounter, cm.Depth)),
			cols.SegmentCounter, 1))
	cm.Comp.InsertGlobal(cm.Round, cm.qname("SEGMENT_COUNTER_CONSISTENCY_2"), expr5)

	// Booleanity check on IsActive
	expr6 := symbolic.Sub(symbolic.Square(cols.IsActiveExpanded),
		cols.IsActiveExpanded)
	cm.Comp.InsertGlobal(cm.Round, cm.qname("ISACTIVE_EXPANDED_BOOLEANITY"), expr6)
}

func (cm *ComputeMod) colname(name string, args ...any) ifaces.ColID {
	return ifaces.ColIDf("%v_%v", cm.Name, cm.Comp.SelfRecursionCount) + "_" + ifaces.ColIDf(name, args...)
}

func (cm *ComputeMod) qname(name string, args ...any) ifaces.QueryID {
	return ifaces.QueryIDf("%v_%v", cm.Name, cm.Comp.SelfRecursionCount) + "_" + ifaces.QueryIDf(name, args...)
}

// Function inserting a query that col is zero when IsActive is zero
func (cm *ComputeMod) colZeroAtInactive(col ifaces.Column, name string) {
	// col zero at inactive area, e.g., IsInactive[i]) * col[i] = 0
	sug := cm.SugarVar
	cm.Comp.InsertGlobal(cm.Round, cm.qname("%s", name),
		symbolic.Mul(sug.IsInactive, col))
}

// Assigns the module from an assignment to the inputs of the
// roots and leaves
func (cm *ComputeMod) assign(
	run *wizard.ProverRuntime,
	pos smartvectors.SmartVector,
	leaves [blockSize]smartvectors.SmartVector,
) {

	// Function responsible for post-padding with zeroes
	pad := func(vec []field.Element, padVal_ ...field.Element) smartvectors.SmartVector {
		// padding value
		padVal := field.Zero()
		if len(padVal_) > 0 {
			padVal = padVal_[0]
		}
		return smartvectors.RightPadded(vec, padVal, cm.NumRows)
	}

	// Number of active rows
	numActiveRows := cm.Depth * cm.NumProofs

	// List of columns to assign
	var (
		roots    = make([]field.Octuplet, numActiveRows)
		curr     = make([]field.Octuplet, numActiveRows)
		proof    [8]smartvectors.SmartVector
		posbit   = make([]field.Element, numActiveRows)
		posacc   = make([]field.Element, numActiveRows)
		left     = make([]field.Octuplet, numActiveRows)
		right    = make([]field.Octuplet, numActiveRows)
		interm   = make([]field.Octuplet, numActiveRows)
		nodehash = make([]field.Octuplet, numActiveRows)
		// optional columns coming from the Accumulator module
		useNextMerkleProof = func() smartvectors.SmartVector {
			if cm.WithOptProofReuseCheck {
				return cm.Cols.UseNextMerkleProof.GetColAssignment(run)
			} else {
				return smartvectors.AllocateRegular(cm.NumProofs)
			}
		}()
		isActiveAccumulator = func() smartvectors.SmartVector {
			if cm.WithOptProofReuseCheck {
				return cm.Cols.IsActiveAccumulator.GetColAssignment(run)
			} else {
				return smartvectors.AllocateRegular(cm.NumProofs)
			}
		}()
		useNextMerkleProofExpanded = make([]field.Element, numActiveRows)
		isActiveExpanded           = make([]field.Element, numActiveRows)
		// The counter slice is used to populate the segmentCounter column
		counter        = make([]field.Element, 0, cm.NumProofs)
		segmentCounter = make([]field.Element, numActiveRows)
	)

	for i := 0; i < blockSize; i++ {
		proof[i] = cm.Cols.Proof[i].GetColAssignment(run)
	}
	bitAt := func(x field.Element, i int) uint64 {
		xint := x.Uint64()
		return (xint >> i) & 1
	}

	// For the Accumulator module, cm.NumProofs is the maximum number of merkle proofs
	// the module can verify rather than the actual number of proofs that is assigned.
	// Hence we compute the actual number of proofs below to know the assignment range.
	numProofs := cm.NumProofs
	if cm.WithOptProofReuseCheck {
		proofCounter := 0
		isActiveAccumulatorReg := smartvectors.IntoRegVec(isActiveAccumulator)
		for _, elem := range isActiveAccumulatorReg {
			if elem == field.One() {
				counter = append(counter, field.NewElement(uint64(proofCounter)))
				proofCounter += 1
			}
			// If we encounter a zero, that denotes inactive area. We don't need to continue
			if elem == field.Zero() {
				break
			}
		}
		numProofs = proofCounter
	}

	var zeroBlock field.Octuplet
	// Assigns everything in parallel proof per proof
	parallel.Execute(numProofs, func(start, stop int) {
		for proofNo := start; proofNo < stop; proofNo++ {

			// placeholder for the root of the current proof
			var root field.Octuplet

			// recall that we fill the trace bottom up for every proof
			for level := 0; level < cm.Depth; level++ {
				row := (proofNo+1)*cm.Depth - level - 1

				// assign curr
				if level == 0 {
					for i := 0; i < blockSize; i++ {
						curr[row][i] = leaves[i].Get(proofNo)
					}
				} else {
					curr[row] = nodehash[row+1]
				}

				// Assign posbit
				posbitUint := bitAt(pos.Get(proofNo), level)
				posbit[row].SetUint64(posbitUint)

				// Assign left, right
				switch posbitUint {
				case 0:
					left[row] = curr[row]
					for i := 0; i < blockSize; i++ {
						right[row][i] = proof[i].Get(row)
					}
				case 1:
					for i := 0; i < blockSize; i++ {
						left[row][i] = proof[i].Get(row)
					}
					right[row] = curr[row]
				default:
					utils.Panic("not a bit")
				}

				// And run the poseidon2 compression function
				interm[row] = vortex.CompressPoseidon2(zeroBlock, left[row])
				nodehash[row] = vortex.CompressPoseidon2(interm[row], right[row])
			}

			root = nodehash[proofNo*cm.Depth]

			// Then computes the posacc and root, topdown
			for i := 0; i < cm.Depth; i++ {
				row := proofNo*cm.Depth + i
				if i == 0 {
					posacc[row] = posbit[row]
				} else {
					posacc[row].Double(&posacc[row-1]).Add(&posacc[row], &posbit[row])
				}
				roots[row] = root
			}
			// Assign useNextMerkleProofExpanded, isActiveAccumulatorExpanded, and segmentCounter
			if cm.WithOptProofReuseCheck {
				for i := 0; i < cm.Depth; i++ {
					row := proofNo*cm.Depth + i
					useNextMerkleProofExpanded[row] = useNextMerkleProof.Get(proofNo)
					isActiveExpanded[row] = isActiveAccumulator.Get(proofNo)
					segmentCounter[row] = counter[proofNo]
				}
			}
		}
	})

	intermPadding := vortex.CompressPoseidon2(zeroBlock, zeroBlock)
	// Assign zero blocks in the inactive area when the actual number of proofs and maximum number of proofs
	// are different
	if cm.WithOptProofReuseCheck {
		for i := numProofs * cm.Depth; i < numActiveRows; i++ {
			interm[i] = intermPadding
			nodehash[i] = vortex.CompressPoseidon2(intermPadding, zeroBlock)
		}
	}

	nodeHashPadding := vortex.CompressPoseidon2(intermPadding, zeroBlock)

	// and assign the freshly computed columns
	cols := cm.Cols
	run.AssignColumn(cols.PosBit.GetColID(), pad(posbit))
	run.AssignColumn(cols.PosAcc.GetColID(), pad(posacc))

	trRoot := Transpose(roots)
	trCurr := Transpose(curr)
	trLeft := Transpose(left)
	trRight := Transpose(right)
	trInterm := Transpose(interm)
	trNodehash := Transpose(nodehash)

	for i := 0; i < blockSize; i++ {
		run.AssignColumn(cols.Root[i].GetColID(), pad(trRoot[i]))
		run.AssignColumn(cols.Curr[i].GetColID(), pad(trCurr[i]))
		run.AssignColumn(cols.Left[i].GetColID(), pad(trLeft[i]))
		run.AssignColumn(cols.Right[i].GetColID(), pad(trRight[i]))
		run.AssignColumn(cols.Interm[i].GetColID(), pad(trInterm[i], intermPadding[i]))
		run.AssignColumn(cols.NodeHash[i].GetColID(), pad(trNodehash[i], nodeHashPadding[i]))
	}
	if cm.WithOptProofReuseCheck {
		run.AssignColumn(cols.UseNextMerkleProofExpanded.GetColID(), pad(useNextMerkleProofExpanded))
		run.AssignColumn(cols.IsActiveExpanded.GetColID(), pad(isActiveExpanded))
		run.AssignColumn(cols.SegmentCounter.GetColID(), pad(segmentCounter))
	}
	cm.Cols.IsEndOfProof.Assign(run)
	cm.Cols.NewProof.Assign(run)
}

func (cm *ComputeMod) isFullyActive() bool {
	return cm.NumRows == cm.Depth*cm.NumProofs
}
