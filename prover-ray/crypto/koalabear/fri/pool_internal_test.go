package fri

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// poisonPools fills the field pools with released buffers full of junk, of
// every capacity class up to 2^maxLog, several per class.
func poisonPools(maxLog int) {
	var junk field.Element
	junk.SetUint64(0x1234567)
	for l := 0; l <= maxLog; l++ {
		for range 8 {
			b := field.BasePool.Get(1 << l)
			e := field.ExtPool.Get(1 << l)
			o := field.OctupletPool.Get(1 << l)
			for i := range b {
				b[i] = junk
				e[i] = field.Lift(junk)
				for k := range o[i] {
					o[i][k] = junk
				}
			}
			field.BasePool.Put(b)
			field.ExtPool.Put(e)
			field.OctupletPool.Put(o)
		}
	}
}

// Commitments and FRI layer trees take their buffers from the field pools,
// which hand out stale values: committing into poisoned buffers must give
// the same roots as committing into fresh memory.
func TestCommitIgnoresStalePooledBuffers(t *testing.T) {
	sizes := []int{1, 2, -1, 8, 16}
	witness := func() MultiSizeTable {
		var ctr uint64
		w := make(MultiSizeTable, len(sizes))
		for i, s := range sizes {
			w[i] = tableOfSize(s, &ctr)
		}
		return w
	}
	encoders := makeEncoders(len(sizes), 2)
	layer := field.VecRandomExt(64)

	field.BasePool.Drain()
	field.ExtPool.Drain()
	field.OctupletPool.Drain()
	fresh := Commit(encoders, witness())
	freshLayer := buildTreeExt(layer).Root()

	poisonPools(8)
	pooled := Commit(encoders, witness())
	require.Equal(t, fresh.Tree.Root(), pooled.Tree.Root())
	require.Equal(t, freshLayer, buildTreeExt(layer).Root())
	pooled.Release()
	require.Nil(t, pooled.Tree)
}
