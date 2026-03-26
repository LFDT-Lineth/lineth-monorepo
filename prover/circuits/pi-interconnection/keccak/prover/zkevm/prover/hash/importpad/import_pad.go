package importpad

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	cs "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common/common_constraints"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/hash/generic"
)

// ImportAndPadInputs collect the inputs of the [ImportAndPad] function.
type ImportAndPadInputs struct {
	// Name is a string identifier used to derive a unique name of each generated
	// column/constraint.
	Name string
	// Src is the list of [generic.GenericByteModule] to import
	Src generic.GenericByteModule
	// PaddingStrategy allows the caller specifying for which use-case import
	// and pad is used. This tells the [ImportAndPad] function which padding
	// strategy to use.
	PaddingStrategy generic.HashingUsecase
}

// Importation stores the wizard compilation context to instantiate the
// functionality of [ImportAndPad]: e.g. it stores all the intermediate columns
// and constraints and implements the [wizard.ProverAction] interface. The
// [Importation.Run] function is responsible for assigning all the generated
// columns.
type Importation struct {

	// Inputs tracks the input structure used for instantiating this [Importation]
	Inputs         ImportAndPadInputs
	HashNum        ifaces.Column // identifier for the hash the current limb belongs to
	Limbs          ifaces.Column // limbs declared by the current row
	NBytes         ifaces.Column // number of bytes in the current limbs
	Index          ifaces.Column // identifier for the limbs within the current hash
	IsInserted     ifaces.Column // indicates whether the current limbs was imported
	IsPadded       ifaces.Column // indicates whether the current limbs is from padding.
	AccPaddedBytes ifaces.Column // counts the number of padded bytes. This is then looked up to ensure that we don't pad more than 1 block.
	IsNewHash      ifaces.Column // indicates that a new hash stars at the current row

	// indicates whether the current row is active (or if it's just a filling row)
	IsActive ifaces.Column

	// Padder stores the padding-strategy-specific
	Padder padder

	// helper column
	IndexIsZero ifaces.Column
	PaIsZero    wizard.ProverAction
}

// importationAssignmentBuilder is a utility struct used to build an assignment
// for the Importation module. It is an internal of the package and is called
// in the [importation.Run] function.
type importationAssignmentBuilder struct {
	HashNum        *common.VectorBuilder
	Limbs          *common.VectorBuilder
	NBytes         *common.VectorBuilder
	Index          *common.VectorBuilder
	IsInserted     *common.VectorBuilder
	IsPadded       *common.VectorBuilder
	AccPaddedBytes *common.VectorBuilder
	IsActive       *common.VectorBuilder
	IsNewHash      *common.VectorBuilder
	Padder         padderAssignmentBuilder
}

type padder interface {
	pushPaddingRows(byteStringSize int, ab *importationAssignmentBuilder)
}

type padderAssignmentBuilder interface {
	pushInsertingRow(nbBytes int, isNewHash bool)
	padAndAssign(run *wizard.ProverRuntime)
}

// ImportAndPad defines and constrains the Importation and the padding of a
// group of generic byte module following a prespecified padding strategy.
func ImportAndPad(comp *wizard.CompiledIOP, inp ImportAndPadInputs, numRows int) *Importation {

	var (
		res = &Importation{
			Inputs:         inp,
			HashNum:        comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_HASH_NUM", inp.Name), numRows),
			Limbs:          comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_LIMBS", inp.Name), numRows),
			NBytes:         comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_NBYTES", inp.Name), numRows),
			Index:          comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_INDEX", inp.Name), numRows),
			IsInserted:     comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_IS_INSERTED", inp.Name), numRows),
			IsPadded:       comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_IS_PADDED", inp.Name), numRows),
			AccPaddedBytes: comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_ACC_PADDED_BYTES", inp.Name), numRows),
			IsActive:       comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_IS_ACTIVE", inp.Name), numRows),
			IsNewHash:      comp.InsertCommit(0, ifaces.ColIDf("%v_IMPORT_PAD_IS_NEW_HASH", inp.Name), numRows),
		}
	)

	pragmas.MarkRightPadded(res.HashNum)

	switch {
	case inp.PaddingStrategy == generic.KeccakUsecase:
		res.Padder = res.newKeccakPadder(comp)
	case inp.PaddingStrategy == generic.Sha2Usecase:
		res.Padder = res.newSha2Padder(comp)
	case inp.PaddingStrategy == generic.MiMCUsecase:
		res.Padder = res.newMimcPadder(comp)
	default:
		panic("unknown strategy")
	}

	cs.MustBeBinary(comp, res.IsNewHash)
	cs.MustBeActivationColumns(comp, res.IsActive)
	cs.MustBeMutuallyExclusiveBinaryFlags(comp, res.IsActive, []ifaces.Column{
		res.IsInserted, res.IsPadded,
	})

	cs.MustZeroWhenInactive(comp, res.IsActive,
		res.HashNum,
		res.Limbs,
		res.NBytes,
		res.Index,
		res.AccPaddedBytes,
		res.IsNewHash,
	)

	// When the index flag must keep increasing during the padding zone
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_INDEX_INCREASES", inp.Name),
		sym.Mul(
			res.IsPadded,
			sym.Sub(res.Index, column.Shift(res.Index, -1), 1),
		),
	)

	// When Index = 0, IsNewHash = 1
	res.IndexIsZero, res.PaIsZero = dedicated.IsZero(comp, res.Index).GetColumnAndProverAction()
	comp.InsertGlobal(0, ifaces.QueryIDf("%v_IS_NEW_HASH_WELL_SET", inp.Name),
		sym.Mul(res.IsActive, res.IndexIsZero,
			sym.Sub(1, res.IsNewHash),
		),
	)

	//  IsPadded is correctly set before each newHash.
	// IsInserted[i] * IsPadded[i-1] == IsNewHash
	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_IS_PADDED_WELL_SET", inp.Name),
		sym.Sub(res.IsNewHash, sym.Mul(res.IsInserted, column.Shift(res.IsPadded, -1))),
	)

	if inp.PaddingStrategy != generic.MiMCUsecase {
		// before IsActive transits to 0, there should be a padding zone.
		// IsActive[i] * (1-IsActive[i+1]) * (1-IsPadded[i]) =0
		comp.InsertGlobal(0, ifaces.QueryIDf("%v_LAST_HASH_HAS_PADDING", inp.Name),
			sym.Mul(res.IsActive,
				sym.Sub(1, column.Shift(res.IsActive, 1)),
				sym.Sub(1, res.IsPadded),
			),
		)
	}

	// to handle the above constraint for the case where isActive[i] = 1  for all i .
	// IsPadded[last-row]= isActive[last-row]
	comp.InsertLocal(0, ifaces.QueryIDf("%v_LAST_PADDING_LAST_HASH_WELL_SET", inp.Name),
		sym.Sub(column.Shift(res.IsActive, -1), column.Shift(res.IsPadded, -1)),
	)

	// Ensures that AccPaddedBytes is well-set
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%v_ACC_PADDED_BYTES_CORRECTNESS", inp.Name),
		sym.Sub(
			res.AccPaddedBytes,
			sym.Mul(res.IsPadded, sym.Add(column.Shift(res.AccPaddedBytes, -1), res.NBytes)),
		),
	)

	// Ensures the padding bytes are capped at 16 and cannot be zero.
	comp.InsertInclusionConditionalOnIncluded(0,
		ifaces.QueryIDf("%v_PADDING_AT_MOST_16_BYTES", inp.Name),
		[]ifaces.Column{getLookupForSize(comp, 16)},
		[]ifaces.Column{res.NBytes},
		res.IsPadded,
	)

	comp.InsertProjection(
		ifaces.QueryIDf("%v_IMPORT_PAD_PROJECTION", inp.Name),
		query.ProjectionInput{ColumnA: []ifaces.Column{inp.Src.Data.HashNum, inp.Src.Data.Limb, inp.Src.Data.NBytes, inp.Src.Data.Index},
			ColumnB: []ifaces.Column{res.HashNum, res.Limbs, res.NBytes, res.Index},
			FilterA: inp.Src.Data.ToHash,
			FilterB: res.IsInserted})

	return res
}

// Run performs the assignment of the Importation module.
func (imp *Importation) Run(run *wizard.ProverRuntime) {

	var (
		sha2Count = 0
		srcData   = imp.Inputs.Src.Data
		hashNum   = srcData.HashNum.GetColAssignment(run).IntoRegVecSaveAlloc()
		limbs     = srcData.Limb.GetColAssignment(run).IntoRegVecSaveAlloc()
		nBytes    = srcData.NBytes.GetColAssignment(run).IntoRegVecSaveAlloc()
		index     = srcData.Index.GetColAssignment(run).IntoRegVecSaveAlloc()
		toHash    = srcData.ToHash.GetColAssignment(run).IntoRegVecSaveAlloc()

		iab = importationAssignmentBuilder{
			HashNum:        common.NewVectorBuilder(imp.HashNum),
			Limbs:          common.NewVectorBuilder(imp.Limbs),
			NBytes:         common.NewVectorBuilder(imp.NBytes),
			Index:          common.NewVectorBuilder(imp.Index),
			IsInserted:     common.NewVectorBuilder(imp.IsInserted),
			IsPadded:       common.NewVectorBuilder(imp.IsPadded),
			AccPaddedBytes: common.NewVectorBuilder(imp.AccPaddedBytes),
			IsActive:       common.NewVectorBuilder(imp.IsActive),
			IsNewHash:      common.NewVectorBuilder(imp.IsNewHash),
		}

		currByteSize = 0
		currHashNum  field.Element
	)

	switch {
	case imp.Inputs.PaddingStrategy == generic.KeccakUsecase:
		iab.Padder = keccakPadderAssignmentBuilder{}
	case imp.Inputs.PaddingStrategy == generic.Sha2Usecase:
		iab.Padder = &sha2PaddingAssignmentBuilder{
			AccInsertedBytes: common.NewVectorBuilder(imp.Padder.(*Sha2Padder).AccInsertedBytes),
		}
	case imp.Inputs.PaddingStrategy == generic.MiMCUsecase:
		iab.Padder = &mimcPadderAssignmentBuilder{}
	default:
		panic("unknown strategy")
	}

	for i := range hashNum {

		if toHash[i].IsZero() {
			// The condition of sha2Count addresses the case were sha2 is never
			// called.
			if sha2Count > 0 && i == len(hashNum)-1 {
				imp.Padder.pushPaddingRows(currByteSize, &iab)
			}

			continue
		}

		sha2Count++

		if index[i].IsZero() && !currHashNum.IsZero() {
			imp.Padder.pushPaddingRows(currByteSize, &iab)
		}

		if index[i].IsZero() {
			currHashNum = hashNum[i]
			currByteSize = 0
			iab.IsNewHash.PushOne()
		} else {
			iab.IsNewHash.PushZero()
		}

		indexInt := field.ToInt(&index[i])
		nBytesInt := field.ToInt(&nBytes[i])

		currByteSize += nBytesInt

		iab.pushInsertion(hashNum[i], limbs[i], nBytesInt, indexInt)

		if i == len(hashNum)-1 {
			imp.Padder.pushPaddingRows(currByteSize, &iab)
		}
	}

	iab.HashNum.PadAndAssign(run, field.Zero())
	iab.Limbs.PadAndAssign(run, field.Zero())
	iab.NBytes.PadAndAssign(run, field.Zero())
	iab.Index.PadAndAssign(run, field.Zero())
	iab.IsInserted.PadAndAssign(run, field.Zero())
	iab.IsPadded.PadAndAssign(run, field.Zero())
	iab.AccPaddedBytes.PadAndAssign(run, field.Zero())
	iab.IsActive.PadAndAssign(run, field.Zero())
	iab.IsNewHash.PadAndAssign(run, field.Zero())
	iab.Padder.padAndAssign(run)

	imp.PaIsZero.Run(run)
}

// pushPaddingCommonColumns push an insertion row corresponding to the first
// row of a new hash.
func (iab *importationAssignmentBuilder) pushInsertion(hashNum field.Element, limb field.Element, nbBytes int, index int) {

	iab.HashNum.PushField(hashNum)
	iab.Limbs.PushField(limb)
	iab.NBytes.PushInt(nbBytes)
	iab.Index.PushInt(index)
	iab.AccPaddedBytes.PushZero()
	iab.IsActive.PushOne()
	iab.IsInserted.PushOne()
	iab.IsPadded.PushZero()
	iab.Padder.pushInsertingRow(nbBytes, index == 0)
}

// pushPaddingCommonColumns adds pushes a padding rows for the columns that are
// independant of the padding strategy
func (ipad *importationAssignmentBuilder) pushPaddingCommonColumns() {

	ipad.HashNum.RepushLast()
	ipad.Index.PushInc()
	ipad.IsActive.PushOne()
	ipad.IsInserted.PushZero()
	ipad.IsPadded.PushOne()
	ipad.IsNewHash.PushZero()
}
