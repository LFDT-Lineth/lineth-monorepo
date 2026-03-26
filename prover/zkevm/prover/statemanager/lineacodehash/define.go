package lineacodehash

import (
	"fmt"
	"math/big"

	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

const (
	// Column names
	POSEIDON2_CODE_HASH_IS_ACTIVE_NAME       ifaces.ColID = "POSEIDON2_CODE_HASH_IS_ACTIVE"
	POSEIDON2_CODE_HASH_CFI_NAME             ifaces.ColID = "POSEIDON2_CODE_HASH_CFI"
	POSEIDON2_CODE_HASH_LIMB_NAME            ifaces.ColID = "POSEIDON2_CODE_HASH_LIMB"
	POSEIDON2_CODE_HASH_IS_NEW_HASH_NAME     ifaces.ColID = "POSEIDON2_CODE_HASH_IS_NEW_HASH"
	POSEIDON2_CODE_HASH_IS_HASH_END_NAME     ifaces.ColID = "POSEIDON2_CODE_HASH_IS_HASH_END"
	POSEIDON2_CODE_HASH_PREV_STATE_NAME      ifaces.ColID = "POSEIDON2_CODE_HASH_PREV_STATE"
	POSEIDON2_CODE_HASH_NEW_STATE_NAME       ifaces.ColID = "POSEIDON2_CODE_HASH_NEW_STATE"
	POSEIDON2_CODE_HASH_CODE_SIZE_NAME       ifaces.ColID = "POSEIDON2_CODE_HASH_CODE_SIZE"
	POSEIDON2_CODE_HASH_KECCAK_CODEHASH_NAME ifaces.ColID = "POSEIDON2_CODE_HASH_KECCAK_CODEHASH"
	POSEIDON2_CODE_HASH_IS_FOR_CONSISTENCY   ifaces.ColID = "POSEIDON2_CODE_HASH_IS_NON_EMPTY_CODEHASH"
)

// initEmptyKeccak initialises emptyKeccak variable from emptyKeccakString.
//
// Returns a representation of empty keccak value in limbs with size defined
// by common.LimbBytes.
func initEmptyKeccak() (res [common.NbLimbU256]field.Element) {
	var emptyKeccakBig big.Int
	_, isErr := emptyKeccakBig.SetString(emptyKeccakString, 16)
	if !isErr {
		panic("empty keccak string is not correct")
	}

	emptyKeccakByteLimbs := common.SplitBytes(emptyKeccakBig.Bytes())
	for i, limbByte := range emptyKeccakByteLimbs {
		res[i] = *new(field.Element).SetBytes(limbByte)
	}

	return res
}

var (
	emptyKeccakString = "c5d2460186f7233c927e7db2dcc703c0e500b653ca82273b7bfad8045d85a470"
	emptyKeccak       = initEmptyKeccak()
)

// Inputs stores the caller's parameters to NewPOSEIDON2CodeHash
type Inputs struct {
	Round int
	Name  string
	Size  int
}

// Module stores all the columns responsible for computing the POSEIDON2
// codehash of every contract occuring during the EVM computation.
type Module struct {
	// Inputs are the parameteress provided by the user of the struct
	Inputs Inputs

	// All the columns characterizing the module
	IsActive  ifaces.Column                    // lights-up when we are in the POSEIDON2 codehash computation area
	CFI       [common.NbLimbU32]ifaces.Column  // an index column for the limbs, indicating to which contract they belong. It increments when a new codehash import starts.
	Limb      [common.NbLimbU128]ifaces.Column // Contains the input to POSEIDON2, i.e., the code bytes of the contract
	CodeHash  [common.NbLimbU256]ifaces.Column // Contains the imported keccak code hash
	CodeSize  [common.NbLimbU32]ifaces.Column  //  size of the contract code in bytes, constant in a CFI segment.
	IsNewHash ifaces.Column
	IsHashEnd ifaces.Column
	PrevState [poseidon2.BlockSize]ifaces.Column // to hash the limbs; we have 8 columns of 2 bytes for limbs and 8 columns of 4-bytes for the state.

	// Contains the POSEIDON2 code hash when IsHashEnd is 1
	NewState [poseidon2.BlockSize]ifaces.Column

	// inputModule stores the modules connected the present Module (e.g. Rom/RomLex)
	// when they are not omitted.
	InputModules *inputModules

	// IsForConsistency lights-up when the imported keccak code-hash is not the empty
	// codehash. This is used as an import filter for the consistency module with the
	// state summary.
	IsForConsistency ifaces.Column
	IsEmptyKeccak    [common.NbLimbU256]ifaces.Column
	CptIsEmptyKeccak [common.NbLimbU256]wizard.ProverAction
}

// NewModule registers and committing all the columns and queries in the POSEIDON2_code_hash module
func NewModule(comp *wizard.CompiledIOP, inputs Inputs) (mh Module) {

	mh = Module{
		Inputs:    inputs,
		IsActive:  comp.InsertCommit(inputs.Round, POSEIDON2_CODE_HASH_IS_ACTIVE_NAME, inputs.Size, true),
		IsNewHash: comp.InsertCommit(inputs.Round, POSEIDON2_CODE_HASH_IS_NEW_HASH_NAME, inputs.Size, true),
		IsHashEnd: comp.InsertCommit(inputs.Round, POSEIDON2_CODE_HASH_IS_HASH_END_NAME, inputs.Size, true),
	}

	copy(mh.Limb[:], common.CreateMultiColumn(comp, "POSEIDON2_CODE_HASH_LIMB_NAME", inputs.Size, common.NbLimbU128, pragmas.None))

	copy(mh.CodeSize[:], common.CreateMultiColumn(comp, "POSEIDON2_CODE_HASH_CODE_SIZE_NAME", inputs.Size, common.NbLimbU32, pragmas.None))

	copy(mh.CFI[:], common.CreateMultiColumn(comp, "POSEIDON2_CODE_HASH_CFI_NAME", inputs.Size, common.NbLimbU32, pragmas.None))

	copy(mh.CodeHash[:], common.CreateMultiColumn(comp, "POSEIDON2_CODE_HASH_KECCAK_CODEHASH_NAME", inputs.Size, common.NbLimbU256, pragmas.None))

	copy(mh.PrevState[:], common.CreateMultiColumn(comp, "POSEIDON2_CODE_HASH_PREV_STATE_NAME", inputs.Size, poseidon2.BlockSize, pragmas.None))

	copy(mh.NewState[:], common.CreateMultiColumn(comp, "POSEIDON2_CODE_HASH_NEW_STATE_NAME", inputs.Size, poseidon2.BlockSize, pragmas.None))

	mh.IsForConsistency = comp.InsertCommit(inputs.Round, POSEIDON2_CODE_HASH_IS_FOR_CONSISTENCY, inputs.Size, true)

	for i := range common.NbLimbU256 {

		// IsEmptyKeccak[i] = 1 if CodeHash[i] = emptyKeccak[i]
		mh.IsEmptyKeccak[i], mh.CptIsEmptyKeccak[i] = dedicated.IsZero(comp,
			sym.Sub(mh.CodeHash[i], emptyKeccak[i])).GetColumnAndProverAction()

		comp.InsertGlobal(
			0,
			ifaces.QueryIDf("POSEIDON2_CODE_HASH_CPT_IF_FOR_CONSISTENCY_%d", i),
			sym.Sub(
				mh.IsForConsistency,
				sym.Mul(
					sym.Sub(1, mh.IsEmptyKeccak[i]),
					mh.IsHashEnd,
				),
			),
		)
	}

	mh.checkConsistency(comp)

	return mh
}

// checkConsistency adds the constraints securing the POSEIDON2CodeHash module.
//
//	We have the following constraints:
//
//	1. NewState = POSEIDON2(PrevState, Limb)
//	2. If IsNewHash = 0, PrevState[i] = NextState[i-1] (in the active area)
//	3. If IsNewHash = 1, PrevState = 0 (in the active area)
//	4. If CFI incremented, IsNewHash = 1
//	5. Local constraint IsNewHash starts with 1
//	6. if CFI[i+1] - CFI[i] != 0, IsHashEnd[i] = 1
//	7. Booleanity of IsNewHash, IsHashEnd (in the active area)
//	8. Booeanity of IsActive
//	9. IsActive[i] = 0 IMPLIES IsActive[i+1] = 0
//	10. in a particular CFI segment, CodeHashHi and CodeHashLo remain constant
//	11. in a particular CFI segment, CodeSize remains constant
//	11. All columns are zero in the inactive area
func (mh *Module) checkConsistency(comp *wizard.CompiledIOP) {

	// NewState = POSEIDON2(PrevState, Limb)
	comp.InsertPoseidon2(mh.Inputs.Round, mh.qname("POSEIDON2_CODE_HASH"), mh.Limb, mh.PrevState, mh.NewState, nil)

	// Local constraint IsNewHash starts with 1
	comp.InsertLocal(mh.Inputs.Round, mh.qname("IS_NEW_HASH_LOCAL"), sym.Sub(mh.IsNewHash, mh.IsActive))

	// Booleanity of IsNewHash, IsHashEnd (in the active area)
	comp.InsertGlobal(mh.Inputs.Round, mh.qname("IS_NEW_HASH_BOOLEAN"),
		sym.Sub(sym.Mul(sym.Square(mh.IsNewHash), mh.IsActive),
			mh.IsNewHash))

	comp.InsertGlobal(mh.Inputs.Round, mh.qname("IS_HASH_END_BOOLEAN"),
		sym.Sub(sym.Mul(sym.Square(mh.IsHashEnd), mh.IsActive),
			mh.IsHashEnd))

	// Booeanity of IsActive
	comp.InsertGlobal(mh.Inputs.Round, mh.qname("IS_ACTIVE_BOOLEAN"),
		sym.Sub(
			sym.Square(mh.IsActive),
			mh.IsActive))

	// IsActive[i] = 0 IMPLIES IsActive[i+1] = 0 e.g. IsActive[i] = IsActive[i-1] * IsActive[i]
	comp.InsertGlobal(mh.Inputs.Round, mh.qname("IS_ACTIVE_ZERO_FOLLOWED_BY_ZERO"),
		sym.Sub(mh.IsActive,
			sym.Mul(ifaces.ColumnAsVariable(column.Shift(mh.IsActive, -1)),
				mh.IsActive)))

	for i := range common.NbLimbU256 {
		// In a particular CFI segment, CodeHash limbs remain constant,
		// e.g., IsActive[i] * (1 - IsEndHash[i]) * (CodeHash[i+1] - CodeHash[i]) = 0
		comp.InsertGlobal(mh.Inputs.Round, mh.qname("CODE_HASH_HI_SEGMENT_WISE_CONSTANT_%d", i),
			sym.Mul(mh.IsActive,
				sym.Sub(1, mh.IsHashEnd),
				sym.Sub(ifaces.ColumnAsVariable(column.Shift(mh.CodeHash[i], 1)), mh.CodeHash[i]),
			),
		)

		// All columns of CodeHash are zero in the inactive area
		mh.colZeroAtInactive(comp, mh.CodeHash[i], fmt.Sprintf("CODE_HASH_HI_ZERO_IN_INACTIVE_%d", i))
	}

	for i := range poseidon2.BlockSize {
		// If IsNewHash = 0, PrevState[i] = NewState[i-1] (in the active area), e.g.,
		// IsActive[i] * (1 - IsNewHash[i]) * (PrevState[i] - NextState[i-1]) = 0
		comp.InsertGlobal(mh.Inputs.Round, mh.qname("PREV_STATE_CONSISTENCY_2_%d", i),
			sym.Mul(mh.IsActive,
				sym.Sub(1, mh.IsNewHash),
				sym.Sub(mh.PrevState[i], column.Shift(mh.NewState[i], -1))))

		// If IsNewHash = 1, PrevState = 0 (in the active area) e.g., IsActive[i] * IsNewHash[i] * PrevState[i] = 0
		comp.InsertGlobal(mh.Inputs.Round, mh.qname("PREV_STATE_ZERO_AT_BEGINNING_%d", i),
			sym.Mul(mh.IsActive, mh.IsNewHash, mh.PrevState[i]))

		mh.colZeroAtInactive(comp, mh.PrevState[i], fmt.Sprintf("PREV_STATE_ZERO_IN_INACTIVE_%d", i))
	}

	for i := range common.NbLimbU128 {
		mh.colZeroAtInactive(comp, mh.Limb[i], fmt.Sprintf("LIMB_ZERO_IN_INACTIVE_%d", i))
	}

	for i := range common.NbLimbU32 {
		// if CFI[i+1] - CFI[i] != 0, IsHashEnd[i] = 1, e.g., IsActive[i] * (CFI[i+1] - CFI[i]) * (1 - IsHashEnd[i]) = 0
		comp.InsertGlobal(mh.Inputs.Round, mh.qname("IS_HASH_END_CONSISTENCY_1_%d", i),
			sym.Mul(mh.IsActive,
				sym.Sub(ifaces.ColumnAsVariable(column.Shift(mh.CFI[i], 1)), mh.CFI[i]),
				sym.Sub(1, mh.IsHashEnd)))

		// In a particular CFI segment, CodeSize remains constant,
		// e.g., IsActive[i] * (1 - IsEndHash[i]) * (CodeSize[i+1] - CodeSize[i]) = 0
		comp.InsertGlobal(mh.Inputs.Round, mh.qname("CODE_SIZE_SEGMENT_WISE_CONSTANT_%d", i),
			sym.Mul(mh.IsActive,
				sym.Sub(1, mh.IsHashEnd),
				sym.Sub(ifaces.ColumnAsVariable(column.Shift(mh.CodeSize[i], 1)), mh.CodeSize[i])))

		mh.colZeroAtInactive(comp, mh.CodeSize[i], fmt.Sprintf("CODE_SIZE_ZERO_IN_INACTIVE_%d", i))
		mh.colZeroAtInactive(comp, mh.CFI[i], fmt.Sprintf("CFI_ZERO_IN_INACTIVE_%d", i))
	}

	// All columns are zero in the inactive area, except newState
	mh.colZeroAtInactive(comp, mh.IsNewHash, "IS_NEW_HASH_ZERO_IN_INACTIVE")
	mh.colZeroAtInactive(comp, mh.IsHashEnd, "IS_HASH_END_ZERO_IN_INACTIVE")
}

// Function returning a query name
func (mh *Module) qname(name string, n ...int) ifaces.QueryID {
	if len(n) > 0 {
		return ifaces.QueryIDf("%v_%v_%v", mh.Inputs.Name, name, n[0])
	} else {
		return ifaces.QueryIDf("%v_%v", mh.Inputs.Name, name)
	}
}

// Function inserting a query that col is zero when IsActive is zero
func (mh *Module) colZeroAtInactive(comp *wizard.CompiledIOP, col ifaces.Column, name string) {
	// col zero at inactive area, e.g., (1-IsActive[i]) * col[i] = 0
	comp.InsertGlobal(mh.Inputs.Round, mh.qname(name),
		sym.Mul(sym.Sub(1, mh.IsActive), col))
}
