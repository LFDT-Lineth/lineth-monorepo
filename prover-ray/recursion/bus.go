package recursion

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
)

// access is one access of a chip row to a bus: a tuple with multiplicity
// mult — the number of reads for a write, −1 times a selector for a read, 0
// on rows that do not access.
type access struct {
	mod   *module
	tuple []wiop.Expression
	mult  wiop.Expression
}

// logSum is the accesses of one log-derivative sum, balanced with its own
// coins α and γ:
//
//	Σ_writes mult/(γ + fold_α(tuple)) − Σ_reads 1/(γ + fold_α(tuple)) = 0,
//
// fold_α(t) = t_0 + α·t_1 + α^2·t_2 + …, α and γ drawn after the chips'
// columns are committed.
type logSum struct {
	name     string
	accesses []access
}

func (s *logSum) add(m *module, tuple []wiop.Expression, mult wiop.Expression) {
	s.accesses = append(s.accesses, access{m, tuple, mult})
}

// build registers the sum on sys: the coins α and γ in coinRound (after the
// round of the chips' columns), one fraction mult/(γ + fold_α(tuple)) per
// access, their log-derivative sum, and a verifier action asserting that it
// is zero (lookuptologderivsum's, which the verifier-ray codegen recognizes;
// that pass registers it only for the sum it builds from inclusion queries).
// The logderivativesum pass then packs the fractions of each module into its
// running-sum columns.
func (s *logSum) build(sys *wiop.System, coinRound *wiop.Round) *wiop.LogDerivativeSum {
	if len(s.accesses) == 0 {
		return nil
	}
	ctx := sys.Context.Childf("%s", s.name)
	alpha := coinRound.NewCoinField(ctx.Childf("alpha"))
	gamma := coinRound.NewCoinField(ctx.Childf("gamma"))
	fractions := make([]wiop.Fraction, len(s.accesses))
	for i, a := range s.accesses {
		fractions[i] = wiop.Fraction{
			Numerator:   a.mult,
			Denominator: wiop.Add(gamma, wiop.RLCExpression(alpha, a.tuple)),
		}
	}
	ld := sys.NewLogDerivativeSum(ctx.Childf("sum"), fractions)
	ld.Result.Round().RegisterVerifierAction(&lookuptologderivsum.ResultIsZeroVerifierAction{LogDerivativeSum: ld})
	return ld
}

// Bus holds the chips' accesses to the memory and to the tables, two
// log-derivative sums with their own coins.
//
// The memory is write-once: every address is written by exactly one chip
// row, with its read count as multiplicity, and each read is a −1 term, of
// the tuple (addr, v0..v7). The program assigns each address to a single
// write and counts its reads, and both are setup columns, so a read can only
// be balanced by the write of its address, with the same value.
//
// The tables are read at indices computed during execution (see Table): each
// entry is written once, with the tuple (tag, i, v0..v7) for the i-th entry of
// the table tag, and a witness multiplicity, its number of reads. Every entry
// of a table has an index below its length, so a read at another index, or
// with another table's tag, balances against no entry.
//
// The permutations are looked up in the P2 core: each of its rows writes the
// tuple (in0..15, c0..7) of a compression, c = P(in)[8..15] + in[8..15],
// with a witness multiplicity, its number of reads; a chip hashing reads the
// tuple of each compression it needs.
//
// The three sums are kept apart because each is checked against zero on its
// own: a fraction of one sum cannot balance a fraction of another, whatever
// their coins (they need not differ). In a single sum, a table entry
// (tag, i, v0..v7) could balance a memory tuple of the same elements.
type Bus struct {
	mem    logSum
	tables logSum
	p2     logSum
}

func newBus() *Bus {
	return &Bus{mem: logSum{name: "bus"}, tables: logSum{name: "tables"}, p2: logSum{name: "p2"}}
}

// WriteP2 adds the write of the compression in → c, with multiplicity mult.
func (b *Bus) WriteP2(m *module, in [P2Width]wiop.Expression, c [p2Half]wiop.Expression, mult wiop.Expression) {
	b.p2.add(m, append(append([]wiop.Expression(nil), in[:]...), c[:]...), mult)
}

// ReadP2 adds a read of the compression in → c, active when sel is 1.
func (b *Bus) ReadP2(m *module, in [P2Width]wiop.Expression, c [p2Half]wiop.Expression, sel wiop.Expression) {
	b.p2.add(m, append(append([]wiop.Expression(nil), in[:]...), c[:]...), wiop.Negate(sel))
}

// Write adds a write of vals at addr with multiplicity mult.
func (b *Bus) Write(m *module, addr wiop.Expression, vals [CellWidth]wiop.Expression, mult wiop.Expression) {
	b.mem.add(m, append([]wiop.Expression{addr}, vals[:]...), mult)
}

// Read adds a read of vals at addr, active when sel is 1.
func (b *Bus) Read(m *module, addr wiop.Expression, vals [CellWidth]wiop.Expression, sel wiop.Expression) {
	b.mem.add(m, append([]wiop.Expression{addr}, vals[:]...), wiop.Negate(sel))
}

// WriteEntry adds the write of vals as entry idx of the table tag, with
// multiplicity mult.
func (b *Bus) WriteEntry(m *module, tag, idx wiop.Expression, vals [CellWidth]wiop.Expression, mult wiop.Expression) {
	b.tables.add(m, append([]wiop.Expression{tag, idx}, vals[:]...), mult)
}

// ReadEntry adds a read of vals as entry idx of the table tag, active when sel
// is 1.
func (b *Bus) ReadEntry(m *module, tag, idx wiop.Expression, vals [CellWidth]wiop.Expression, sel wiop.Expression) {
	b.tables.add(m, append([]wiop.Expression{tag, idx}, vals[:]...), wiop.Negate(sel))
}
