package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
)

// newSinglePermutation builds a size-4 single-column permutation between two
// modules, compiled through grandproduct.Compile — which registers exactly one
// FinalProductCheck and one CheckResultIsOne, both on the same GrandProduct.
func newSinglePermutation(t *testing.T) *wiop.System {
	t.Helper()
	sys := wiop.NewSystemf("gp-codegen")
	r0 := sys.NewRound()
	sys.NewRound() // coin round for beta, following the column round
	sys.NewRound() // result round, following the coin round
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	sys.NewPermutation(
		sys.Context.Childf("perm"),
		[]wiop.Table{wiop.NewTable(colA.View())},
		[]wiop.Table{wiop.NewTable(colB.View())},
	)
	grandproduct.Compile(sys)
	global.Compile(sys) // appends quotient+eval rounds so no cell lands in the last round
	return sys
}

// newSingleMessageBusHandle builds a size-4 Send/Receive pair on one handle,
// compiled through messagebus.Compile then grandproduct.Compile — which
// together register one FinalProductCheck and (unless skipInShard) one
// CheckHandleSumInShard, both on the same GrandProduct.
func newSingleMessageBusHandle(t *testing.T, skipInShard bool) *wiop.System {
	t.Helper()
	sys := wiop.NewSystemf("mb-codegen")
	r0 := sys.NewRound()
	sys.NewRound() // coin round for alpha/beta, following the column round
	sys.NewRound() // result round, following the coin round
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)

	send := sys.NewMessageBusSend(sys.Context.Childf("send"), "shard", "route", wiop.NewTable(colA.View()))
	recv := sys.NewMessageBusReceive(sys.Context.Childf("recv"), "shard", "route", wiop.NewTable(colB.View()))
	send.SkipInShardCheck = skipInShard
	recv.SkipInShardCheck = skipInShard

	messagebus.Compile(sys)
	grandproduct.Compile(sys)
	global.Compile(sys)
	return sys
}

func TestBuildGrandProductSystemExtractsPermutation(t *testing.T) {
	sys := newSinglePermutation(t)

	gp, err := BuildGrandProductSystem(sys)
	if err != nil {
		t.Fatalf("BuildGrandProductSystem() error = %v", err)
	}
	if len(gp.Queries) != 1 {
		t.Fatalf("expected exactly one query, got %d", len(gp.Queries))
	}
	q := gp.Queries[0]
	if len(q.ZFinalRefs) == 0 {
		t.Fatalf("expected at least one z-final ref")
	}
	if !q.HasExpected || q.Expected != 1 {
		t.Fatalf("a permutation-reduced GrandProduct must expect Result == 1, got HasExpected=%v Expected=%d", q.HasExpected, q.Expected)
	}
}

func TestBuildGrandProductSystemExtractsMessageBusHandle(t *testing.T) {
	sys := newSingleMessageBusHandle(t, false)

	gp, err := BuildGrandProductSystem(sys)
	if err != nil {
		t.Fatalf("BuildGrandProductSystem() error = %v", err)
	}
	if len(gp.Queries) != 1 {
		t.Fatalf("expected exactly one query, got %d", len(gp.Queries))
	}
	q := gp.Queries[0]
	if !q.HasExpected || q.Expected != 1 {
		t.Fatalf("an in-shard-checked message-bus handle must expect Result == 1, got HasExpected=%v Expected=%d", q.HasExpected, q.Expected)
	}
}

func TestBuildGrandProductSystemSkipsInShardCheck(t *testing.T) {
	sys := newSingleMessageBusHandle(t, true)

	gp, err := BuildGrandProductSystem(sys)
	if err != nil {
		t.Fatalf("BuildGrandProductSystem() error = %v", err)
	}
	if len(gp.Queries) != 1 {
		t.Fatalf("expected exactly one query, got %d", len(gp.Queries))
	}
	q := gp.Queries[0]
	if q.HasExpected {
		t.Fatalf("a handle with SkipInShardCheck must carry no expected-result check, got Expected=%d", q.Expected)
	}
}

func TestBuildGrandProductSystemNoQueries(t *testing.T) {
	sys := wiop.NewSystemf("gp-none")
	sys.NewRound()
	sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	gp, err := BuildGrandProductSystem(sys)
	if err != nil {
		t.Fatalf("BuildGrandProductSystem() error = %v", err)
	}
	if len(gp.Queries) != 0 {
		t.Fatalf("a system without GrandProduct queries must yield no queries, got %d", len(gp.Queries))
	}
}

func TestWriteGrandProductSystemZigRendersPermutation(t *testing.T) {
	sys := newSinglePermutation(t)
	gp, err := BuildGrandProductSystem(sys)
	if err != nil {
		t.Fatalf("BuildGrandProductSystem() error = %v", err)
	}

	var out bytes.Buffer
	if err := WriteGrandProductSystemZig(&out, 0, gp); err != nil {
		t.Fatalf("WriteGrandProductSystemZig() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"const grandproduct = @import",
		"system_0_grandproduct_query_0_zfinal_refs = [_]grandproduct.ScalarRef{",
		"system_0_grandproduct_queries = [_]grandproduct.Query{",
		".result_ref = .{",
		".expected = 1",
		"const system_0_grandproduct = grandproduct.System{ .queries = &system_0_grandproduct_queries };",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Zig missing %q:\n%s", want, got)
		}
	}
}

func TestWriteGrandProductSystemZigRendersSkippedCheck(t *testing.T) {
	sys := newSingleMessageBusHandle(t, true)
	gp, err := BuildGrandProductSystem(sys)
	if err != nil {
		t.Fatalf("BuildGrandProductSystem() error = %v", err)
	}

	var out bytes.Buffer
	if err := WriteGrandProductSystemZig(&out, 0, gp); err != nil {
		t.Fatalf("WriteGrandProductSystemZig() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, ".expected = null") {
		t.Fatalf("expected a skipped in-shard check to render `.expected = null`:\n%s", got)
	}
}
