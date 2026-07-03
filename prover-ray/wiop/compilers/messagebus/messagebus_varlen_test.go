package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The message bus imposes no cross-participant LENGTH (row-count) constraint on
// a handle: each participant's table lives on its own module, and both the
// logderivativesum and grandproduct compilers bucket per module, committing a
// separate running-accumulator column of that module's own length. Only WIDTH
// (column count) must agree across a handle, because the α-fold
// β + α^{w-1}·c₀ + … + c_{w-1} is only comparable at equal width. The tests
// below exercise handles whose participants have genuinely different lengths.

// TestCompile_VariableLength_LogUp_Balanced: a logup handle whose Send lives on
// a size-4 module and whose Receive lives on a size-8 module. A selector (and
// matching multiplicity) masks the receiver's extra rows, so the active
// multisets coincide and the shard residual is zero.
func TestCompile_VariableLength_LogUp_Balanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
		modR := sys.NewSizedModule(sys.Context.Childf("modR"), 8, wiop.PaddingDirectionNone)
		colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
		colR := modR.NewColumn(sys.Context.Childf("R"), wiop.VisibilityOracle, r0)
		selR := modR.NewColumn(sys.Context.Childf("selR"), wiop.VisibilityOracle, r0)
		mulR := modR.NewColumn(sys.Context.Childf("mR"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-S"), "shard", "vl",
			wiop.NewTable(colS.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-R"), "shard", "vl",
			wiop.NewFilteredTable(selR.View(), colR.View()),
			mulR.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colS, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colR, makeVec(10, 20, 30, 40, 99, 98, 97, 96))
		rt.AssignColumn(selR, makeVec(1, 1, 1, 1, 0, 0, 0, 0))
		rt.AssignColumn(mulR, makeVec(1, 1, 1, 1, 0, 0, 0, 0))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt),
			"a balanced logup bus with different-length participants must be accepted")
	})
}

// TestCompile_VariableLength_LogUp_Unbalanced: same different-length shape, but
// the receiver's selector admits a row (value 99) absent from the sender's
// multiset, so the residual is non-zero and the in-shard check rejects.
func TestCompile_VariableLength_LogUp_Unbalanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
		modR := sys.NewSizedModule(sys.Context.Childf("modR"), 8, wiop.PaddingDirectionNone)
		colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
		colR := modR.NewColumn(sys.Context.Childf("R"), wiop.VisibilityOracle, r0)
		selR := modR.NewColumn(sys.Context.Childf("selR"), wiop.VisibilityOracle, r0)
		mulR := modR.NewColumn(sys.Context.Childf("mR"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-S"), "shard", "vl",
			wiop.NewTable(colS.View()),
		)
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-R"), "shard", "vl",
			wiop.NewFilteredTable(selR.View(), colR.View()),
			mulR.View(),
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colS, makeVec(10, 20, 30, 40))
		rt.AssignColumn(colR, makeVec(10, 20, 30, 40, 99, 98, 97, 96))
		rt.AssignColumn(selR, makeVec(1, 1, 1, 1, 1, 0, 0, 0)) // admits row 4 (value 99)
		rt.AssignColumn(mulR, makeVec(1, 1, 1, 1, 1, 0, 0, 0))

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"a logup bus whose receiver admits a row absent from the sender must be rejected")
	})
}

// TestCompile_VariableLength_Permutation_Balanced: a permutation handle whose
// Send lives on a size-4 module and whose Receive lives on a size-8 module. The
// receiver's selector leaves exactly four active rows that reorder the sender's
// multiset; the masked rows contribute the neutral grand-product factor 1.
func TestCompile_VariableLength_Permutation_Balanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
		modR := sys.NewSizedModule(sys.Context.Childf("modR"), 8, wiop.PaddingDirectionNone)
		colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
		colR := modR.NewColumn(sys.Context.Childf("R"), wiop.VisibilityOracle, r0)
		selR := modR.NewColumn(sys.Context.Childf("selR"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusPermutationSend(
			sys.Context.Childf("send-S"), "shard", "vl",
			wiop.NewTable(colS.View()),
		)
		sys.NewMessageBusPermutationReceive(
			sys.Context.Childf("recv-R"), "shard", "vl",
			wiop.NewFilteredTable(selR.View(), colR.View()),
		)

		messagebus.Compile(sys)
		grandproduct.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colS, makeVec(10, 20, 30, 40))
		// Selected R rows = {40,10,30,20}, a reordering of S; the rest are junk.
		rt.AssignColumn(colR, makeVec(40, 10, 77, 30, 66, 20, 55, 44))
		rt.AssignColumn(selR, makeVec(1, 1, 0, 1, 0, 1, 0, 0))

		drive(&rt)
		require.NoError(t, checkAllVerifierActions(&rt),
			"a balanced permutation bus with different-length participants must be accepted")
	})
}

// TestCompile_VariableLength_Permutation_Unbalanced: same different-length
// shape, but one selected receiver value (88) is absent from the sender's
// multiset, so the product accumulator is not one and the in-shard check
// rejects.
func TestCompile_VariableLength_Permutation_Unbalanced(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modS := sys.NewSizedModule(sys.Context.Childf("modS"), 4, wiop.PaddingDirectionNone)
		modR := sys.NewSizedModule(sys.Context.Childf("modR"), 8, wiop.PaddingDirectionNone)
		colS := modS.NewColumn(sys.Context.Childf("S"), wiop.VisibilityOracle, r0)
		colR := modR.NewColumn(sys.Context.Childf("R"), wiop.VisibilityOracle, r0)
		selR := modR.NewColumn(sys.Context.Childf("selR"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusPermutationSend(
			sys.Context.Childf("send-S"), "shard", "vl",
			wiop.NewTable(colS.View()),
		)
		sys.NewMessageBusPermutationReceive(
			sys.Context.Childf("recv-R"), "shard", "vl",
			wiop.NewFilteredTable(selR.View(), colR.View()),
		)

		messagebus.Compile(sys)
		grandproduct.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colS, makeVec(10, 20, 30, 40))
		// Selected R rows = {40,10,30,88}: 88 does not appear in S.
		rt.AssignColumn(colR, makeVec(40, 10, 77, 30, 66, 88, 55, 44))
		rt.AssignColumn(selR, makeVec(1, 1, 0, 1, 0, 1, 0, 0))

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"a permutation bus whose selected receiver multiset differs from the sender must be rejected")
	})
}

// TestCompile_MixedWidth_LeadingOneDoesNotAlias directly stresses the "leading
// coordinate is 1" concern. A width-2 receive (1, v) folds to α² + α + v, whose
// α¹ term coincides with the α¹ *sentinel* of a width-1 send (v) = α + v, and
// whose constant term also matches. Only the width-2 row's own α² sentinel
// distinguishes them — so the residual is non-zero and the bus is rejected,
// confirming a data value of 1 cannot masquerade as a lower-width sentinel.
func TestCompile_MixedWidth_LeadingOneDoesNotAlias(t *testing.T) {
	runWithAndWithoutHook(t, func(t *testing.T, sys *wiop.System, r0 *wiop.Round) {
		t.Helper()
		modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
		modR2 := sys.NewSizedModule(sys.Context.Childf("modR2"), 2, wiop.PaddingDirectionNone)
		colS1 := modS1.NewColumn(sys.Context.Childf("S1"), wiop.VisibilityOracle, r0)
		hiR := modR2.NewColumn(sys.Context.Childf("hiR"), wiop.VisibilityOracle, r0)
		loR := modR2.NewColumn(sys.Context.Childf("loR"), wiop.VisibilityOracle, r0)

		sys.NewMessageBusSend(
			sys.Context.Childf("send-w1"), "shard", "one",
			wiop.NewTable(colS1.View()),
		)
		// Width-2 receive with the leading (sentinel-adjacent) column pinned to 1,
		// trying to consume the width-1 send as (1, v).
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-w2"), "shard", "one",
			wiop.NewTable(hiR.View(), loR.View()), nil,
		)

		messagebus.Compile(sys)
		logderivativesum.Compile(sys)

		rt := wiop.NewRuntime(sys)
		rt.AssignColumn(colS1, makeVec(5, 6))
		rt.AssignColumn(hiR, makeVec(1, 1)) // leading coordinate = 1
		rt.AssignColumn(loR, makeVec(5, 6))

		drive(&rt)
		assert.Error(t, checkAllVerifierActions(&rt),
			"a data value of 1 in the sentinel-adjacent column must not alias a lower-width sentinel")
	})
}
