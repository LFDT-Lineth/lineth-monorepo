package messagebus_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
)

// Temporary scaffold: prints the Verify error of representative negative cases
// so the rejection reason can be eyeballed. Deleted after inspection.
func TestZZTmpReasons(t *testing.T) {
	t.Run("tampered-value-no-pcs", func(t *testing.T) {
		sys := wiop.NewSystemf("zz-tampered-value")
		r0 := sys.NewRound()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		colA := modA.NewColumn(sys.Context.Childf("A"), r0)
		colB := modB.NewColumn(sys.Context.Childf("B"), r0)
		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "ping", wiop.NewTable(colA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "ping", wiop.NewTable(colB.View()))
		compilePermutationBus(sys)
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(colA, makeVec(10, 20, 30, 40))
			rt.AssignColumn(colB, makeVec(10, 21, 30, 40))
		})
		t.Log("ERR:", sys.Verify(proof, pub))
	})

	t.Run("sentinel-aliasing-no-pcs", func(t *testing.T) {
		sys := wiop.NewSystemf("zz-sentinel")
		r0 := sys.NewRound()
		modS1 := sys.NewSizedModule(sys.Context.Childf("modS1"), 2, wiop.PaddingDirectionNone)
		modR2 := sys.NewSizedModule(sys.Context.Childf("modR2"), 2, wiop.PaddingDirectionNone)
		colS1 := modS1.NewColumn(sys.Context.Childf("S1"), r0)
		hiR := modR2.NewColumn(sys.Context.Childf("hiR"), r0)
		loR := modR2.NewColumn(sys.Context.Childf("loR"), r0)
		sys.NewMessageBusSend(
			sys.Context.Childf("send-w1"), "shard", "alias", wiop.NewTable(colS1.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-w2"), "shard", "alias",
			wiop.NewTable(hiR.View(), loR.View()))
		compilePermutationBus(sys)
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(colS1, makeVec(5, 6))
			rt.AssignColumn(hiR, makeVec(0, 0))
			rt.AssignColumn(loR, makeVec(5, 6))
		})
		t.Log("ERR:", sys.Verify(proof, pub))
	})

	t.Run("kv-unbalanced-with-pcs", func(t *testing.T) {
		sys := wiop.NewSystemf("zz-kv-unbalanced")
		r0 := sys.NewRound()
		modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
		modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
		keyA := modA.NewColumn(sys.Context.Childf("kA"), r0)
		valA := modA.NewColumn(sys.Context.Childf("vA"), r0)
		keyB := modB.NewColumn(sys.Context.Childf("kB"), r0)
		valB := modB.NewColumn(sys.Context.Childf("vB"), r0)
		sys.NewMessageBusSend(
			sys.Context.Childf("send-A"), "shard", "kv",
			wiop.NewTable(keyA.View(), valA.View()))
		sys.NewMessageBusReceive(
			sys.Context.Childf("recv-B"), "shard", "kv",
			wiop.NewTable(keyB.View(), valB.View()))
		compilePermutationBusWithPCS(sys)
		proof, pub := sys.Prove(func(rt *wiop.Runtime) {
			rt.AssignColumn(keyA, makeVec(1, 2, 3, 4))
			rt.AssignColumn(valA, makeVec(10, 20, 30, 40))
			rt.AssignColumn(keyB, makeVec(1, 2, 3, 4))
			rt.AssignColumn(valB, makeVec(10, 21, 30, 40))
		})
		t.Log("ERR:", sys.Verify(proof, pub))
	})
}
