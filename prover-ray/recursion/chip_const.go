package recursion

// Setup columns of the const chip.
const (
	constAddr = "addr"
	constMult = "mult"
	constV    = "v" // lanes
)

// Const returns the address of a cell holding v, allocating it on first use
// (constants are deduplicated by value).
func (m *Machine) Const(v Cell) int {
	if a, ok := m.consts[v]; ok {
		return a
	}
	if m.consts == nil {
		m.consts, m.constVal = map[Cell]int{}, map[int]Cell{}
	}
	a := m.alloc()
	m.consts[v] = a
	m.constVal[a] = v
	m.constList = append(m.constList, a)
	return a
}

// constChip: one row per constant cell, all setup: addr, mult, v0..v7. Bus:
// writes [addr ‖ v] with multiplicity mult.
type constChip struct{ m *Machine }

func (c constChip) name() string { return constMod }

func (c constChip) rows() int { return len(c.m.constList) }

func (c constChip) setup(cs *cols) {
	cs.declare(constAddr, constMult)
	cs.declareLanes(constV, CellWidth)
	for row, a := range c.m.constList {
		cs.set(constAddr, row, uint64(a))
		cs.set(constMult, row, uint64(c.m.reads[a]))
		cs.setCell(constV, row, c.m.constVal[a])
	}
}

func (c constChip) define(m *module, bus *Bus) error {
	bus.Write(m, m.s(constAddr), cellLanes(m.setupLanes(constV, CellWidth)), m.s(constMult))
	return nil
}

func (c constChip) trace(*cols, *Run) error { return nil }
