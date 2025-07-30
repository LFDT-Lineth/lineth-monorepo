package logderivativesum

import (
	"fmt"
	"sort"
	"strings"

	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
)

const (
	// LogDerivativePrefix is a prefix that we commonly use to derive query,
	// coin or column names that are introduced by the compiler.
	LogDerivativePrefix = "LOGDERIVATIVE"
)

// GetTableCanonicalOrder extracts the lookup table and the queried tables
// from `q` and rearrange them conjointly so that the names of T are returned
// in alphabetical order.
//
// This reordering of the column ensures that if two lookup queries share the
// same table scrambling the order of the columns. They still get both
// recognized as being the same table.
//
// In case the table is fragmented, the function is a no-op. It is a low-priority
// to do to implement as this optimization is likely unnecessary and the user
// can always make sure to specify the table in the same order all the time.
//
// Importantly, the function allocates its own result.
func GetTableCanonicalOrder(q query.Inclusion) ([]ifaces.Column, [][]ifaces.Column) {

	if len(q.Including) > 1 {
		// The append here are performing a deep-copy of the slice within the
		// query. This prevents side-effects from appearing later when appending
		// conditional filter to theses.
		return append([]ifaces.Column{}, q.Included...),
			append([][]ifaces.Column{}, q.Including...)
	}

	var (
		checked    = make([]ifaces.Column, len(q.Included))
		table      = make([]ifaces.Column, len(q.Including[0]))
		colNamesT  = make([]ifaces.ColID, len(checked))
		sortingMap = make([]int, len(table))
	)

	for col := range colNamesT {
		colNamesT[col] = q.Including[0][col].GetColID()
		sortingMap[col] = col
	}

	sort.Slice(colNamesT, func(i, j int) bool {
		return strings.Compare(string(colNamesT[i]), string(colNamesT[j])) < 0
	})

	for i, colName := range colNamesT {
		for oldPos, includedCol := range q.Including[0] {
			if includedCol.GetColID() == colName {
				checked[i] = q.Included[oldPos]
				table[i] = includedCol
				break
			}
		}
	}

	return checked, [][]ifaces.Column{table}
}

// DeriveName constructs a generic name
func DeriveName[R ~string](args ...any) R {
	argStr := []string{"LOOKUP_LOGDERIVATIVE"}
	for _, arg := range args {
		argStr = append(argStr, fmt.Sprintf("%v", arg))
	}
	return R(strings.Join(argStr, "_"))
}

// DeriveTableName constructs a name for the table `t`. The caller may provide
// a context and a suffix to the name. If `t` is empty, the name is the
// concatenation of `context` and `name` separated by an underscore.
func DeriveTableName[R ~string](context string, t [][]ifaces.Column, name string) R {
	res := fmt.Sprintf("%v_%v_%v", NameTable(t), context, name)
	return R(res)
}

// DeriveTableNameWithIndex is as [deriveTableName] but additionally allows
// appending an integer index in the name.
func DeriveTableNameWithIndex[R ~string](context string, t [][]ifaces.Column, index int, name string) R {
	res := fmt.Sprintf("%v_%v_%v_%v", NameTable(t), index, context, name)
	return R(res)
}

// NameTable returns a unique name corresponding to the provided
// sequence of columns `t`. The unique name is constructed by appending the
// name of all the column separated by an underscore.
func NameTable(t []table) string {
	// This single fragment case is managed as a special case although it is
	// not really one. This is for backwards compatibility.
	if len(t) == 1 {
		colNames := make([]string, len(t[0]))
		for col := range t[0] {
			colNames[col] = string(t[0][col].GetColID())
		}
		return fmt.Sprintf("TABLE_%v", strings.Join(colNames, ","))
	}

	fragNames := make([]string, len(t))
	for frag := range t {
		colNames := make([]string, len(t[frag]))
		for col := range t[frag] {
			colNames[col] = string(t[frag][col].GetColID())
		}
		fragNames[frag] = fmt.Sprintf("(%v)", strings.Join(colNames, ", "))
	}

	return fmt.Sprintf("TABLE_%v", strings.Join(fragNames, "_"))
}
