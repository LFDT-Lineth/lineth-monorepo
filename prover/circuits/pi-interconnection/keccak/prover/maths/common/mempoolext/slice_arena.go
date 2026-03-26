package mempoolext

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field/fext"
)

// SliceArena is a simple not-threadsafe arena implementation that uses a
// mempool to carry its allocation. It will only put back free memory in the
// the parent pool when TearDown is called.
type SliceArena struct {
	frees  []*[]fext.Element
	parent MemPool
}

func WrapsWithMemCache(pool MemPool) *SliceArena {
	return &SliceArena{
		frees:  make([]*[]fext.Element, 0, 1<<7),
		parent: pool,
	}
}

func (m *SliceArena) Prewarm(nbPrewarm int) MemPool {
	m.parent.Prewarm(nbPrewarm)
	return m
}

func (m *SliceArena) Alloc() *[]fext.Element {

	if len(m.frees) == 0 {
		return m.parent.Alloc()
	}

	last := m.frees[len(m.frees)-1]
	m.frees = m.frees[:len(m.frees)-1]
	return last
}

func (m *SliceArena) Free(v *[]fext.Element) error {
	m.frees = append(m.frees, v)
	return nil
}

func (m *SliceArena) Size() int {
	return m.parent.Size()
}

func (m *SliceArena) TearDown() {
	for i := range m.frees {
		m.parent.Free(m.frees[i])
	}
}
