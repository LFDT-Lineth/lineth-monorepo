package wiop_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/stretchr/testify/require"
)

// TestTableStructLiteralRejected checks that a Table built as a struct literal
// is rejected up front by every constructor that consumes tables.
func TestTableStructLiteralRejected(t *testing.T) {
	sys := wiop.NewSystemf("literal")
	m := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := m.NewColumn(m.Context.Childf("c"), sys.NewRound())
	bad := wiop.Table{Columns: []wiop.TableEntry{col.View()}}
	good := wiop.NewTable(col.View())

	require.PanicsWithValue(t,
		"wiop: System.NewPermutation: a fragment 0: Table must be built with NewTable/NewFilteredTable/NewTableEntries",
		func() { sys.NewPermutation(sys.Context.Childf("p"), []wiop.Table{bad}, []wiop.Table{good}) })
	require.Panics(t, func() {
		sys.NewInclusion(sys.Context.Childf("i"), []wiop.Table{good}, []wiop.Table{bad})
	})
	require.Panics(t, func() {
		sys.NewMessageBusSend(sys.Context.Childf("b"), "shard", "h", bad)
	})
}

// TestNewTableEntriesNilTypedSelector checks that a typed-nil selector hits a
// clear panic instead of a nil dereference or a silent "unfiltered" table.
func TestNewTableEntriesNilTypedSelector(t *testing.T) {
	sys := wiop.NewSystemf("nilsel")
	m := sys.NewSizedModule(sys.Context.Childf("m"), 4, wiop.PaddingDirectionNone)
	col := m.NewColumn(m.Context.Childf("c"), sys.NewRound())

	require.PanicsWithValue(t, "wiop: Table selector is a nil *Constant", func() {
		wiop.NewTableEntries(m, (*wiop.Constant)(nil), col.View())
	})
	require.Panics(t, func() {
		wiop.NewTableEntries(m, (*wiop.ColumnView)(nil), col.View())
	})
}
