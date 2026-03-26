package dedicated

import (
	"slices"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common"
	commonconstraints "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/zkevm/prover/common/common_constraints"
)

// LcInputs stores the inputs for [LengthConsistency] function.
type LcInputs struct {
	// input tables
	Table []ifaces.Column
	// the claimed lengths for the elements embedded inside Table.
	TableLen []ifaces.Column
	// max length in bytes.
	MaxLen int
	// name of the table
	Name string
}

// LengthConsistencyCtx stores the intermediate columns for [LengthConsistency] function.
type LengthConsistencyCtx struct {
	Inp LcInputs
	// decomposition of Table into Bytes
	Bytes []byte32cmp.LimbColumns
	// the ProverAction for decomposition of Tables into bytes.
	PA []wizard.ProverAction
	// a binary column indicating if the byte is non-empty
	BytesLen [][]ifaces.Column
	// Size of the columns
	Size int
}

// LengthConsistency receives a table and the associated lengths,
//
//	and assert that the elements in the table of are the given lengths.
func LengthConsistency(comp *wizard.CompiledIOP, inp LcInputs) *LengthConsistencyCtx {

	var (
		name      = inp.Name
		numCol    = len(inp.Table)
		size      = inp.Table[0].Size()
		numBytes  = inp.MaxLen
		createCol = common.CreateColFn(comp, "LENGTH_CONSISTENCY_"+name, size, pragmas.RightPadded)
	)

	res := &LengthConsistencyCtx{
		BytesLen: make([][]ifaces.Column, numCol),
		Inp:      inp,
		Size:     size,
	}

	for j := 0; j < numCol; j++ {
		res.BytesLen[j] = make([]ifaces.Column, numBytes)
		for k := range res.BytesLen[0] {
			res.BytesLen[j][k] = createCol("BYTE_LEN_%v_%v", j, k)
		}
	}

	// 1. decompose each column of the table to bytes
	// 2. check that number of bytes == tableLen
	res.Bytes = make([]byte32cmp.LimbColumns, numCol)
	res.PA = make([]wizard.ProverAction, numCol)
	for j := range inp.Table {
		// 	// constraint asserting to the correct decomposition of table to bytes
		res.Bytes[j], res.PA[j] = byte32cmp.Decompose(comp, inp.Table[j], numBytes, 8, res.BytesLen[j]...)
	}

	// claimed lengths for the table are correct;
	//   - bytesLen is binary
	//   - bytesLen over a row adds up to tableLen
	for j := range inp.Table {
		sum := sym.NewConstant(0)
		for k := 0; k < numBytes; k++ {
			sum = sym.Add(sum, res.BytesLen[j][k])

			// bytesLen is binary
			commonconstraints.MustBeBinary(comp, res.BytesLen[j][k])
		}
		comp.InsertGlobal(0, ifaces.QueryIDf("%v_CLDLen_%v", name, j), sym.Sub(sum, inp.TableLen[j]))
	}
	return res
}

func (lc *LengthConsistencyCtx) Run(run *wizard.ProverRuntime) {
	var (
		numCol   = len(lc.Inp.Table)
		numBytes = lc.Inp.MaxLen
	)

	for j := range lc.Inp.Table {
		lc.PA[j].Run(run)
	}

	tableLen := make([]smartvectors.SmartVector, numCol)
	bytesLen := make([][]*common.VectorBuilder, numCol)

	// allocate bytesLen
	for j := 0; j < numCol; j++ {
		bytesLen[j] = make([]*common.VectorBuilder, numBytes)
		for k := range bytesLen[0] {
			bytesLen[j][k] = common.NewVectorBuilder(lc.BytesLen[j][k])
		}
	}

	// populate bytesLen
	for j := 0; j < numCol; j++ {

		tableLen[j] = lc.Inp.TableLen[j].GetColAssignment(run)
		startPlainRange, stopPlainRange := smartvectors.CoWindowRange(tableLen[j])

		if startPlainRange != 0 {
			utils.Panic(
				"tableLen were expected to be padded on the right, not on the left, start: %v, stop: %v len: %v",
				startPlainRange, stopPlainRange, tableLen[j].Len(),
			)
		}

		for tl := range tableLen[j].IterateSkipPadding() {
			dec := getZeroOnes(tl, numBytes)
			//  this is used in bytes32cmp.Decompose() which needs little-endian
			slices.Reverse(dec)

			for k := range dec {
				bytesLen[j][k].PushField(dec[k])
			}
		}
	}

	for j := range tableLen {
		for k := range bytesLen[0] {
			bytesLen[j][k].PadAndAssign(run)
		}
	}
}

// getZeroOnes receives n  and outputs the pattern  (0,..0,1,..,1) such that there are n elements 1.
func getZeroOnes(n field.Element, max int) (a []field.Element) {
	_n := field.ToInt(&n)
	if _n > max {
		utils.Panic("%v should be smaller than %v", _n, max)
	}
	for j := 0; j < max-_n; j++ {
		a = append(a, field.Zero())

	}
	for i := max - _n; i < max; i++ {
		a = append(a, field.One())

	}

	return a

}
