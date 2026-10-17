package zkcdriver_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildRiscvSystem defines the real RISC-V arithmetization into a fresh
// [wiop.System], without running any compiler pass. It is the measurement
// point "after Define" of issue #4034.
func buildRiscvSystem(t testing.TB, serialized []byte) *wiop.System {
	t.Helper()
	sys := wiop.NewSystemf("zkc-riscv-interning")
	sys.NewRound()
	zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(serialized))
	return sys
}

// riscvConstraints compiles the embedded RISC-V arithmetization once and
// returns its serialised constraints, or skips the test when the toolchain
// cannot produce them.
func riscvConstraints(t testing.TB) []byte {
	t.Helper()
	binF, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Skipf("skipping: cannot compile the embedded R5 arithmetization: %v", err)
	}
	serialized, err := binF.MarshalBinary()
	require.NoError(t, err)
	return serialized
}

// nodeCensus counts, over the vanishings of one module, how many compound
// [wiop.ArithmeticOperation] nodes the expressions contain as tree occurrences
// and how many of them are distinct pointers. The ratio of the two is the
// sharing the interning cache achieves.
type nodeCensus struct {
	occurrences int
	distinct    int
}

// removedFraction is the share of compound-node occurrences that interning
// collapsed away.
func (c nodeCensus) removedFraction() float64 {
	if c.occurrences == 0 {
		return 0
	}
	return 1 - float64(c.distinct)/float64(c.occurrences)
}

// censusOf walks exprs as trees, counting every [wiop.ArithmeticOperation]
// occurrence and the set of distinct node pointers among them. The walk does
// not memoise on the way down: re-descending into a shared node is exactly
// what makes `occurrences` the pre-interning tree size.
func censusOf(exprs []wiop.Expression) nodeCensus {
	distinct := map[wiop.Expression]struct{}{}
	occurrences := 0

	var walk func(e wiop.Expression)
	walk = func(e wiop.Expression) {
		op, ok := e.(*wiop.ArithmeticOperation)
		if !ok {
			return
		}
		occurrences++
		distinct[op] = struct{}{}
		for _, operand := range op.Operands {
			walk(operand)
		}
	}
	for _, e := range exprs {
		walk(e)
	}
	return nodeCensus{occurrences: occurrences, distinct: len(distinct)}
}

// TestInterningSharesCompoundSubexpressions measures what the zkcdriver
// interning cache removes on the real RISC-V system, at the two points the
// acceptance criteria of #4034 name: straight after [zkcdriver.Define], and
// over the non-reduced vanishings left by localvanishing+global, which is what
// the quotient pass binds.
//
// It asserts only that sharing happens at all. The absolute numbers are
// reported via t.Log so that a regression shows up as a readable number rather
// than a brittle threshold: they move with every arithmetization change.
func TestInterningSharesCompoundSubexpressions(t *testing.T) {

	serialized := riscvConstraints(t)
	sys := buildRiscvSystem(t, serialized)

	var total nodeCensus
	for _, module := range sys.Modules {
		exprs := make([]wiop.Expression, 0, len(module.Vanishings))
		for _, v := range module.Vanishings {
			exprs = append(exprs, v.Expression)
		}
		c := censusOf(exprs)
		if c.occurrences == 0 {
			continue
		}
		total.occurrences += c.occurrences
		total.distinct += c.distinct
		t.Logf("after Define: module %-40s %9d occurrences %9d distinct (%.1f%% removed)",
			module.Context.Path(), c.occurrences, c.distinct, 100*c.removedFraction())
	}

	t.Logf("after Define: TOTAL %d occurrences, %d distinct (%.1f%% removed)",
		total.occurrences, total.distinct, 100*total.removedFraction())

	require.NotZero(t, total.occurrences, "expected the RISC-V system to carry compound expression nodes")
	assert.Less(t, total.distinct, total.occurrences,
		"interning removed nothing: every compound node is still a distinct pointer")

	// Second measurement point: over the non-reduced vanishings left by
	// localvanishing+global, which is what the quotient pass binds.
	//
	// Only the two passes the acceptance criteria name are run, on a system
	// that was only Defined: the full prover pipeline needs a shard set up
	// around it (messagebus' shared randomness requires every bus column on
	// the coin round), which has nothing to do with what is being measured.
	localvanishing.Compile(sys)
	global.Compile(sys)

	var afterTotal nodeCensus
	for _, module := range sys.Modules {
		exprs := make([]wiop.Expression, 0, len(module.Vanishings))
		for _, v := range module.Vanishings {
			if v.IsReduced() {
				continue
			}
			exprs = append(exprs, v.Expression)
		}
		c := censusOf(exprs)
		if c.occurrences == 0 {
			continue
		}
		afterTotal.occurrences += c.occurrences
		afterTotal.distinct += c.distinct
		t.Logf("non-reduced vanishings: module %-40s %9d occurrences %9d distinct (%.1f%% removed)",
			module.Context.Path(), c.occurrences, c.distinct, 100*c.removedFraction())
	}

	t.Logf("non-reduced vanishings: TOTAL %d occurrences, %d distinct (%.1f%% removed)",
		afterTotal.occurrences, afterTotal.distinct, 100*afterTotal.removedFraction())

	require.NotZero(t, afterTotal.occurrences)
	assert.Less(t, afterTotal.distinct, afterTotal.occurrences,
		"sharing did not survive localvanishing+global: the quotient pass binds a pure tree")
}

// TestInterningIsExhaustiveWithinEachModule asserts the interning invariant
// itself rather than the saving it produces: within one module, two nodes that
// denote the same value are the same pointer. Anything left over is a key that
// failed to match — a leaf allocated past the cache, or a compound keyed on
// operands that were not yet canonical.
func TestInterningIsExhaustiveWithinEachModule(t *testing.T) {

	serialized := riscvConstraints(t)
	sys := buildRiscvSystem(t, serialized)

	for _, module := range sys.Modules {

		byKey := map[string]wiop.Expression{}
		leaks := 0

		var walk func(e wiop.Expression)
		walk = func(e wiop.Expression) {
			key := structuralKey(e)
			if prev, ok := byKey[key]; ok {
				if prev != e {
					leaks++
				}
			} else {
				byKey[key] = e
			}
			if op, ok := e.(*wiop.ArithmeticOperation); ok {
				for _, operand := range op.Operands {
					walk(operand)
				}
			}
		}
		for _, v := range module.Vanishings {
			walk(v.Expression)
		}

		assert.Zerof(t, leaks,
			"module %s: %d nodes denote a value that already has a different canonical pointer",
			module.Context.Path(), leaks)
	}
}

// TestLiftedLocalConstraintsAreInterned covers the local-constraint half of
// #4034's acceptance criteria. The rewrite that lifts a local vanishing to its
// anchor position allocates fresh nodes throughout — [wiop.Column.At] for the
// leaves, [wiop.DefaultConstruct] for the compounds — so it is re-interned, to
// let local vanishings pinned at the same anchor share their lifted subtrees.
//
// The criterion as written ("distinct lifted nodes shared by >=2 local
// vanishings, per module") cannot be observed on the RISC-V arithmetization:
// it declares exactly one local constraint per module, so no two of them can
// share anything by construction. What is asserted here instead is the
// property that produces the sharing when a module does carry several — that
// within one lifted tree, every structurally identical node is one pointer.
//
// This is a real check rather than a tautology: the lift maps each distinct
// (column, shift) onto a distinct (column, position), so a leaf repeated in
// the source expression stays repeated in the lifted one. Without re-interning
// those repeats are distinct pointers.
func TestLiftedLocalConstraintsAreInterned(t *testing.T) {

	serialized := riscvConstraints(t)
	sys := buildRiscvSystem(t, serialized)

	locals := 0
	for _, module := range sys.Modules {
		for _, v := range module.Vanishings {
			// A lifted local vanishing is the scalar one: its ColumnViews were
			// replaced by ColumnPositions, which are FieldPromises.
			if v.Expression.IsMultiValued() {
				continue
			}
			locals++

			// Every structurally identical node in the lifted tree must be the
			// same pointer, leaves included.
			byKey := map[string]wiop.Expression{}
			var walk func(e wiop.Expression)
			walk = func(e wiop.Expression) {
				key := structuralKey(e)
				if prev, ok := byKey[key]; ok {
					assert.Samef(t, prev, e,
						"lifted local vanishing %s: node %s occurs as two distinct pointers",
						v.Context().Path(), key)
				} else {
					byKey[key] = e
				}
				if op, ok := e.(*wiop.ArithmeticOperation); ok {
					for _, operand := range op.Operands {
						walk(operand)
					}
				}
			}
			walk(v.Expression)
		}
	}

	t.Logf("local constraints: checked %d lifted local vanishings", locals)
	require.NotZero(t, locals, "expected the RISC-V system to declare local constraints")
}

// structuralKey renders an expression by structure alone, so that two nodes
// denoting the same value share a key regardless of their pointers. Compound
// nodes recurse, which makes the key independent of whether interning already
// collapsed the operands.
func structuralKey(e wiop.Expression) string {
	switch e := e.(type) {
	case *wiop.ArithmeticOperation:
		operands := make([]string, len(e.Operands))
		for i, operand := range e.Operands {
			operands[i] = structuralKey(operand)
		}
		return fmt.Sprintf("%v(%v)", e.Operator, operands)
	case *wiop.ColumnView:
		return fmt.Sprintf("view(%d,%d)", e.Column.Context.ID, e.ShiftingOffset)
	case *wiop.ColumnPosition:
		return fmt.Sprintf("at(%d,%d)", e.Column.Context.ID, e.Position)
	case *wiop.Constant:
		return fmt.Sprintf("const(%v)", e.Value)
	default:
		return fmt.Sprintf("%T", e)
	}
}

// nodeSequence renders the per-module post-order node sequence of every
// vanishing expression in the system, in declaration order. Two builds of the
// same arithmetization must produce the same sequence: the interning maps are
// only ever read, never iterated, so the canonical node order stays the first
// occurrence in the sorted constraint traversal that scanConstraints performs.
//
// Pointers are deliberately not part of the rendering — they differ between
// builds by construction. What is compared is the *shape*: the same node
// sequence with the same sharing structure, encoded by giving each distinct
// node pointer an index in order of first appearance.
func nodeSequence(sys *wiop.System) []string {

	out := []string{}

	for _, module := range sys.Modules {

		ids := map[wiop.Expression]int{}

		var render func(e wiop.Expression) string
		render = func(e wiop.Expression) string {
			if id, ok := ids[e]; ok {
				return fmt.Sprintf("#%d", id)
			}
			id := len(ids)
			ids[e] = id

			var body string
			switch e := e.(type) {
			case *wiop.ArithmeticOperation:
				operands := make([]string, len(e.Operands))
				for i, operand := range e.Operands {
					operands[i] = render(operand)
				}
				body = fmt.Sprintf("%v(%v)", e.Operator, operands)
			case *wiop.ColumnView:
				body = fmt.Sprintf("view(%d,%d)", e.Column.Context.ID, e.ShiftingOffset)
			case *wiop.ColumnPosition:
				body = fmt.Sprintf("at(%d,%d)", e.Column.Context.ID, e.Position)
			case *wiop.Constant:
				body = fmt.Sprintf("const(%v)", e.Value)
			default:
				body = fmt.Sprintf("%T", e)
			}
			return fmt.Sprintf("#%d=%s", id, body)
		}

		for _, v := range module.Vanishings {
			out = append(out, fmt.Sprintf("%s|%s|%s",
				module.Context.Path(), v.Context().Path(), render(v.Expression)))
		}
	}

	return out
}

// TestInterningIsDeterministicInProcess builds the RISC-V system twice in one
// process and asserts the two builds produce identical per-module node
// sequences — the determinism criterion of #4034, in its in-process half. The
// cross-process half is TestInterningIsDeterministicAcrossProcesses.
func TestInterningIsDeterministicInProcess(t *testing.T) {

	serialized := riscvConstraints(t)

	first := nodeSequence(buildRiscvSystem(t, serialized))
	second := nodeSequence(buildRiscvSystem(t, serialized))

	require.NotEmpty(t, first)
	require.Equal(t, first, second,
		"two builds in one process disagree on the interned node sequence")
}

// interningDeterminismEnvVar makes the test binary print its node sequence
// digest instead of running the suite, so that
// TestInterningIsDeterministicAcrossProcesses can compare two fresh processes.
const interningDeterminismEnvVar = "ZKCDRIVER_PRINT_NODE_SEQUENCE"

// TestInterningIsDeterministicAcrossProcesses re-executes this test binary
// twice as a subprocess and asserts both report the same node sequence. Go
// randomises map iteration order per process, so this is what rules out the
// interning maps leaking into the output order; the in-process test cannot see
// that on its own.
func TestInterningIsDeterministicAcrossProcesses(t *testing.T) {

	if os.Getenv(interningDeterminismEnvVar) != "" {
		// Child mode: emit the sequence and stop.
		serialized := riscvConstraints(t)
		for _, line := range nodeSequence(buildRiscvSystem(t, serialized)) {
			fmt.Println(line)
		}
		return
	}

	first := runNodeSequenceSubprocess(t)
	second := runNodeSequenceSubprocess(t)

	require.NotEmpty(t, first)
	require.Equal(t, first, second,
		"two processes disagree on the interned node sequence")
}

// runNodeSequenceSubprocess re-runs this test binary in child mode and returns
// the node sequence it printed.
func runNodeSequenceSubprocess(t *testing.T) string {
	t.Helper()

	cmd := exec.Command(os.Args[0],
		"-test.run", "^TestInterningIsDeterministicAcrossProcesses$",
		"-test.v")
	cmd.Env = append(os.Environ(), interningDeterminismEnvVar+"=1")

	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "subprocess failed: %s", out)

	if bytes.Contains(out, []byte("--- SKIP")) {
		t.Skipf("subprocess skipped (toolchain unavailable): %s", out)
	}

	// Keep only the node-sequence lines: the child also writes the testing
	// framework's own PASS/RUN chatter, which carries timings and so differs
	// between runs.
	var seq [][]byte
	for _, line := range bytes.Split(out, []byte("\n")) {
		if bytes.Count(line, []byte("|")) >= 2 {
			seq = append(seq, line)
		}
	}
	return string(bytes.Join(seq, []byte("\n")))
}
