package utilities

import (
	"os"
	"testing"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/csvtraces"
	"github.com/ethereum/go-ethereum/core/vm"
)

// CreateCol is a utility function to quickly register columns
func CreateCol(name, subName string, size int, comp *wizard.CompiledIOP) ifaces.Column {
	return comp.InsertCommit(
		0,
		ifaces.ColIDf("%s_%s", name, subName),
		size,
		true,
	)
}

// Ternary is a small utility to construct ternaries is constraints
func Ternary(cond, if1, if0 any) *sym.Expression {
	return sym.Add(
		sym.Mul(sym.Sub(1, cond), if0),
		sym.Mul(cond, if1),
	)
}

// GetTimestampField returns a field element that contains the hardcoded INST
// value for a timestamp
func GetTimestampField() field.Element {
	timestampCode := uint64(vm.TIMESTAMP)
	return field.NewElement(timestampCode)
}

// GetCoinBaseField returns a field element containing the EVM opcode value for
// the instruction COINBASE
func GetCoinBaseField() field.Element {
	coinBaseCode := uint64(vm.COINBASE)
	return field.NewElement(coinBaseCode)
}

// GetBaseFeeField returns a field element containing the EVM opcode value for
// the instruction BASEFEE
func GetBaseFeeField() field.Element {
	baseFeeCode := uint64(vm.BASEFEE)
	return field.NewElement(baseFeeCode)
}

// InitializeCsv is used to initialize a CsvTrace based on a path
func InitializeCsv(csvPath string, t *testing.T) *csvtraces.CsvTrace {
	f, err := os.Open(csvPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ct, err := csvtraces.NewCsvTrace(f)
	if err != nil {
		t.Fatal(err)
	}
	return ct
}

// MustBeBinary constrains the current column to be binary.
func MustBeBinary(comp *wizard.CompiledIOP, c ifaces.Column) {
	comp.InsertGlobal(
		0,
		ifaces.QueryIDf("%v_MUST_BE_BINARY", c.GetColID()),
		sym.Mul(c, sym.Sub(c, 1)),
	)
}

// CheckLastELemConsistency checks that the last element of the active part of parentCol is present in the field element of acc
func CheckLastELemConsistency(comp *wizard.CompiledIOP, isActive ifaces.Column, parentCol ifaces.Column, acc ifaces.Column, name string) {
	// active is already constrained in the fetcher, no need to constrain it again
	// two cases: Case 1: isActive is not completely filled with 1s, then parentCol[i] is equal to acc at the last row i where isActive[i] is 1
	comp.InsertGlobal(0, ifaces.QueryIDf("%s_%s_%s", name, "IS_ACTIVE_BORDER_CONSTRAINT", parentCol.GetColID()),
		sym.Mul(
			isActive,
			sym.Sub(1,
				column.Shift(isActive, 1),
			),
			sym.Sub(
				parentCol,
				acc,
			),
		),
	)

	// Case 2: isActive is completely filled with 1s, in which case we ask that isActive[size]*(parentCol[size]-acc) = 0
	// i.e. at the last row, parentCol contains the same element as acc
	comp.InsertLocal(0, ifaces.QueryIDf("%s_%s_%s", name, "IS_ACTIVE_FULL_CONSTRAINT", parentCol.GetColID()),
		sym.Mul(
			column.Shift(isActive, -1),
			sym.Sub(
				column.Shift(parentCol, -1),
				column.Shift(acc, -1),
			),
		),
	)
}

// Multi16bitLimbAdd adds a uint64 to a multi-limb number represented as a slice of 16-bit field.Element.
func Multi16bitLimbAdd(a []field.Element, carry uint64) []field.Element {
	if len(a) == 0 {
		utils.Panic("Multi16bitLimbAdd: zero limbs")
	}

	const (
		bits = 16
		mask = (1 << bits) - 1
	)

	res := make([]field.Element, len(a))
	for i := len(a) - 1; i >= 0; i-- {
		v := a[i].Uint64()
		if v > mask {
			utils.Panic("Multi16bitLimbAdd: a[%d]=%d exceeds %d bits", i, v, bits)
		}

		sum := v + carry
		res[i].SetUint64(sum & mask)
		carry = sum >> bits
	}

	if carry != 0 {
		utils.Panic("Multi16bitLimbAdd: overflow adding %d to %v", carry, a)
	}

	return res
}
