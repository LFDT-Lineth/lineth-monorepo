package wiop

import "fmt"

// ObjectKind is the type tag stored in the top byte of an [ObjectID]. It
// identifies the concrete wiop type the ID refers to.
type ObjectKind uint8

const (
	// KindColumn identifies a [Column].
	KindColumn ObjectKind = 0x01
	// KindCell identifies a [Cell].
	KindCell ObjectKind = 0x02
	// KindCoinField identifies a [CoinField].
	KindCoinField ObjectKind = 0x03
)

// String implements [fmt.Stringer].
func (k ObjectKind) String() string {
	switch k {
	case KindColumn:
		return "Column"
	case KindCell:
		return "Cell"
	case KindCoinField:
		return "CoinField"
	default:
		return fmt.Sprintf("ObjectKind(0x%02x)", uint8(k))
	}
}

// ObjectID is a structured uint64 serialisable identifier for registered
// protocol objects ([Column], [Cell], [CoinField]). Its 64 bits are laid out
// as follows:
//
//	[63:56]  ObjectKind  (8 bits)   — type tag
//	[55:40]  slot        (16 bits)  — module index (Column) or round ID (Cell/CoinField)
//	[39: 0]  position    (40 bits)  — position of the object within its slot
//
// An ObjectID of zero is the unregistered sentinel: no valid registered object
// ever receives ID 0 because position 0 with kind 0 is not a valid kind.
type ObjectID uint64

const (
	idKindShift = 56
	idSlotShift = 40
	idSlotMask  = ObjectID(0xFFFF)
	idPosMask   = ObjectID((1 << 40) - 1)
)

// Kind returns the [ObjectKind] encoded in the top 8 bits.
func (id ObjectID) Kind() ObjectKind { return ObjectKind(id >> idKindShift) }

// Slot returns the 16-bit slot encoded in bits 40–55. For [Column] IDs this is
// the owning module's index in [System.Modules]. For [Cell] and [CoinField]
// IDs this is the owning round's [Round.ID].
func (id ObjectID) Slot() int { return int((id >> idSlotShift) & idSlotMask) }

// Position returns the 40-bit position encoded in bits 0–39. It is the index
// of the object within the slice of its slot ([Module.Columns],
// [Round.Cells], or [Round.Coins]).
func (id ObjectID) Position() int { return int(id & idPosMask) }

// newColumnID constructs an [ObjectID] for a column in module moduleIdx at
// position pos within [Module.Columns].
func newColumnID(moduleIdx, pos int) ObjectID {
	return ObjectID(KindColumn)<<idKindShift | ObjectID(moduleIdx)<<idSlotShift | ObjectID(pos)
}

// newCellID constructs an [ObjectID] for a cell in round roundIdx at position
// pos within [Round.Cells].
func newCellID(roundIdx, pos int) ObjectID {
	return ObjectID(KindCell)<<idKindShift | ObjectID(roundIdx)<<idSlotShift | ObjectID(pos)
}

// newCoinID constructs an [ObjectID] for a coin in round roundIdx at position
// pos within [Round.Coins].
func newCoinID(roundIdx, pos int) ObjectID {
	return ObjectID(KindCoinField)<<idKindShift | ObjectID(roundIdx)<<idSlotShift | ObjectID(pos)
}

// LookupColumn recovers the [Column] identified by id. Panics if id does not
// carry [KindColumn] or if the slot/position is out of range.
func (sys *System) LookupColumn(id ObjectID) *Column {
	if id.Kind() != KindColumn {
		panic(fmt.Sprintf("wiop: LookupColumn: id has kind %s, want Column", id.Kind()))
	}
	return sys.Modules[id.Slot()].Columns[id.Position()]
}

// LookupCell recovers the [Cell] identified by id. Panics if id does not carry
// [KindCell] or if the slot/position is out of range.
func (sys *System) LookupCell(id ObjectID) *Cell {
	if id.Kind() != KindCell {
		panic(fmt.Sprintf("wiop: LookupCell: id has kind %s, want Cell", id.Kind()))
	}
	return sys.Rounds[id.Slot()].Cells[id.Position()]
}

// LookupCoinField recovers the [CoinField] identified by id. Panics if id does
// not carry [KindCoinField] or if the slot/position is out of range.
func (sys *System) LookupCoinField(id ObjectID) *CoinField {
	if id.Kind() != KindCoinField {
		panic(fmt.Sprintf("wiop: LookupCoinField: id has kind %s, want CoinField", id.Kind()))
	}
	return sys.Rounds[id.Slot()].Coins[id.Position()]
}
