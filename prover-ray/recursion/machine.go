package recursion

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
)

// Module names of the chips.
const (
	witnessMod = "witness"
	constMod   = "const"
	windowMod  = "window"
	bitsMod    = "bits"
	e6Mod      = "e6"
	selectMod  = "select"
	tableMod   = "table"
	lookupMod  = "lookup"
	p2Mod      = "p2"
	mdMod      = "md"
	merkleMod  = "merkle"
)

// Machine builds a program for the chips: it allocates addresses, counts the
// reads of every cell and records the instructions, without values. A cell
// holds an input (supplied per execution), a constant (known when building),
// or the output of an instruction. Compile turns the program into a wiop
// system and its setup; Program.Execute runs it on inputs and traces the
// witness.
type Machine struct {
	nCells int
	reads  []int // read count of every address

	inputs    []typedCell // input cells, in input order
	hints     []typedCell // cells the witness chip writes with a computed value, checked by their readers
	consts    map[Cell]int
	constList []int        // addresses written by the const chip
	constVal  map[int]Cell // value of every constant

	packs   []packInst
	lanes   []lanesInst
	windows []windowInst // the window chip's rows
	bits    []bitsInst
	e6Insts []e6Inst
	selects []selectInst
	tables  []tableInst
	lookups []lookupInst
	mds     []mdInst
	paths   []pathInst

	instrs []instr // every instruction, in execution order

	// extra are chips added by tests (a chip of their own, with its
	// instructions executed by funcs).
	extra []chip
	funcs []func(r *Run) error
}

// instr is one instruction in execution order: its kind and its index in the
// kind's list.
type instr struct {
	kind, idx int
}

const (
	instrFunc   = iota // a function of the run (tests' chips)
	instrPack          // Pack: the stream cells
	instrLanes         // Lanes: the output cell
	instrBits          // Bits: the shifted values and the bits
	instrE6            // an E6 operation: its result
	instrSelect        // Select: the selected cell
	instrLookup        // Lookup: the entry read
	instrMD            // MDHash: the digest (and the compressions, for the P2 core)
	instrPath          // MerklePath: the compressions, and MerklePathX's [x⁻¹, b0]
)

func (m *Machine) alloc() int {
	m.reads = append(m.reads, 0)
	m.nCells++
	return m.nCells - 1
}

func (m *Machine) read(addr int) { m.reads[addr]++ }

func (m *Machine) record(kind, idx int) { m.instrs = append(m.instrs, instr{kind, idx}) }

// InputScalar, InputE6 and InputDigest allocate a cell of their kind whose
// value is the next input of every execution (proof values, indices) and
// return its address. The witness chip constrains its lanes beyond the kind to
// zero.
func (m *Machine) InputScalar() int { return m.input(KindScalar) }
func (m *Machine) InputE6() int     { return m.input(KindE6) }
func (m *Machine) InputDigest() int { return m.input(KindDigest) }

func (m *Machine) input(kind int) int {
	checkCellKind(kind)
	a := m.alloc()
	m.inputs = append(m.inputs, typedCell{a, kind})
	return a
}

// hint allocates a cell of the given kind that the witness chip writes with a
// value computed during execution, which its readers check.
func (m *Machine) hint(kind int) int {
	checkCellKind(kind)
	a := m.alloc()
	m.hints = append(m.hints, typedCell{a, kind})
	return a
}

// NumInputs returns the number of input cells.
func (m *Machine) NumInputs() int { return len(m.inputs) }

// recordFunc records an instruction executed by f (used by tests' chips).
func (m *Machine) recordFunc(f func(r *Run) error) {
	m.funcs = append(m.funcs, f)
	m.record(instrFunc, len(m.funcs)-1)
}

// chip is one module of the machine: setup fills its setup columns (from the
// program), define its constraints and bus accesses (setup columns are
// precomputed columns, declared before define), trace its witness columns
// (from an execution). Chips are traced in the order of Machine.chips, so a
// chip may use what an earlier one recorded in the Run.
type chip interface {
	name() string
	rows() int
	setup(c *cols)
	define(m *module, bus *Bus) error
	trace(c *cols, r *Run) error
}

func (m *Machine) chips() []chip {
	return append([]chip{
		witnessChip{m}, constChip{m}, windowChip{m}, bitsChip{m},
		e6Chip{m}, selectChip{m}, tableChip{m}, lookupChip{m},
		p2Chip{m}, mdChip{m}, merkleChip{m},
	}, m.extra...)
}

// height returns the number of rows of a chip's module: a power of two, at
// least 4.
func height(n int) int {
	h := 4
	for h < n {
		h *= 2
	}
	return h
}

// Program is a compiled machine: its wiop system (with the setup columns as
// precomputed columns, compiled down to the PCS), built once, and executed
// on any number of inputs.
type Program struct {
	Sys     *wiop.System
	Bus     *wiop.LogDerivativeSum // nil if no chip accesses the memory
	Tables  *wiop.LogDerivativeSum // nil if the program has no table
	P2      *wiop.LogDerivativeSum // nil if the program hashes nothing
	m       *Machine
	modules []*module // the chips with rows, in Machine.chips order
	chips   []chip
}

// Compile builds the chips' modules, their setup (precomputed) columns,
// constraints and bus accesses, the bus, and runs prover-ray's full compiler
// pipeline down to the PCS. The machine must not be modified afterwards.
//
// Rounds: the chips' witness columns are committed in round 0; the coins α
// and γ of the memory, the tables and the permutations are drawn in round 1;
// their sums are claimed in round 2.
func (m *Machine) Compile(opts ...compilers.Option) (*Program, error) {
	sys := wiop.NewSystemf("recursion")
	witnessRound := sys.NewRound()
	coinRound := witnessRound.EnsureNext()
	coinRound.EnsureNext()
	p := &Program{Sys: sys, m: m}
	bus := newBus()
	for _, c := range m.chips() {
		if c.rows() == 0 {
			continue
		}
		n := height(c.rows())
		setup := newCols(n)
		c.setup(setup)
		mod := newModule(sys, witnessRound, c.name(), n, setup)
		if err := c.define(mod, bus); err != nil {
			return nil, fmt.Errorf("chip %s: %w", c.name(), err)
		}
		p.modules = append(p.modules, mod)
		p.chips = append(p.chips, c)
	}
	p.Bus = bus.mem.build(sys, coinRound)
	p.Tables = bus.tables.build(sys, coinRound)
	p.P2 = bus.p2.build(sys, coinRound)

	if err := compilers.CompileFull(sys, opts...); err != nil {
		return nil, fmt.Errorf("recursion: %w", err)
	}
	return p, nil
}

// Run is one execution of a program: every cell's value and the chips'
// witness columns.
type Run struct {
	values  []Cell
	witness []*cols                  // the witness columns of Program.modules[i]
	p2      [][P2Width]field.Element // the permutations' inputs, in P2 core row order
}

// Value returns the value of the cell at addr.
func (r *Run) Value(addr int) Cell { return r.values[addr] }

// Witness returns the values of a chip's witness column, for tests to tamper
// with (nil if the chip has no such column).
func (r *Run) Witness(p *Program, chip, name string) []field.Element {
	for i, mod := range p.modules {
		if mod.name == chip {
			return r.witness[i].vals[name]
		}
	}
	return nil
}

// Execute runs the program on inputs (one cell per Input, in order) and
// traces the witness.
func (p *Program) Execute(inputs []Cell) (*Run, error) {
	m := p.m
	if len(inputs) != len(m.inputs) {
		return nil, fmt.Errorf("Execute: %d inputs, want %d", len(inputs), len(m.inputs))
	}
	r := &Run{values: make([]Cell, m.nCells)}
	for i, in := range m.inputs {
		for k := in.kind; k < CellWidth; k++ {
			if !inputs[i][k].IsZero() {
				return nil, fmt.Errorf("Execute: input %d has kind %d but a nonzero lane %d", i, in.kind, k)
			}
		}
		r.values[in.addr] = inputs[i]
	}
	for a, v := range m.constVal {
		r.values[a] = v
	}
	for _, in := range m.instrs {
		if err := m.exec(in, r); err != nil {
			return nil, err
		}
	}
	if err := p.trace(r); err != nil {
		return nil, err
	}
	return r, nil
}

// trace computes the chips' witness columns from the cells' values.
func (p *Program) trace(r *Run) error {
	r.witness = r.witness[:0]
	for i, c := range p.chips {
		w := newCols(p.modules[i].mod.Size())
		if err := c.trace(w, r); err != nil {
			return fmt.Errorf("chip %s: %w", c.name(), err)
		}
		r.witness = append(r.witness, w)
	}
	return nil
}

// exec computes the cells an instruction writes.
func (m *Machine) exec(in instr, r *Run) error {
	switch in.kind {
	case instrFunc:
		return m.funcs[in.idx](r)
	case instrPack:
		m.execPack(m.packs[in.idx], r)
		return nil
	case instrLanes:
		m.execLanes(m.lanes[in.idx], r)
		return nil
	case instrBits:
		m.execBits(m.bits[in.idx], r)
		return nil
	case instrE6:
		return m.execE6(m.e6Insts[in.idx], r)
	case instrSelect:
		return m.execSelect(m.selects[in.idx], r)
	case instrLookup:
		return m.execLookup(m.lookups[in.idx], r)
	case instrMD:
		m.execMD(m.mds[in.idx], r)
		return nil
	case instrPath:
		m.execPath(m.paths[in.idx], r)
		return nil
	}
	return fmt.Errorf("unknown instruction kind %d", in.kind)
}

// Prove proves an execution: it assigns every chip's witness columns (zero
// where the trace left a column unset) and runs the compiled system's prover.
func (p *Program) Prove(r *Run) (wiop.Proof, wiop.PublicInput) {
	return p.Sys.Prove(func(rt *wiop.Runtime) {
		for i, mod := range p.modules {
			for _, name := range mod.witnessNames {
				col := mod.witness[name]
				v, ok := r.witness[i].vals[name]
				if !ok {
					v = make([]field.Element, mod.mod.Size())
				}
				rt.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(v)})
			}
		}
	})
}

// Verify verifies a proof of the program.
func (p *Program) Verify(proof wiop.Proof, pub wiop.PublicInput) error {
	return p.Sys.Verify(proof, pub)
}
