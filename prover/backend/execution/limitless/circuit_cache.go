package limitless

import (
	"sync"

	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/zkevm"
)

// circuitCache loads each module's compiled GL circuit once and shares it across
// that module's segments instead of re-deserializing it per segment. Safe: proving
// only reads the CompiledIOP. With segments dispatched grouped by module, a circuit
// is released once its module's last segment is done, bounding resident memory.
type circuitCache struct {
	mu      sync.Mutex
	entries map[distributed.ModuleName]*circuitEntry
	pending map[distributed.ModuleName]int
}

type circuitEntry struct {
	once sync.Once
	comp *distributed.RecursedSegmentCompilation
	buf  *serde.MmapBackedBuffer
	err  error
}

// newCircuitCache seeds per-module release counts (moduleNames[i] is segment i's module).
func newCircuitCache(moduleNames []string) *circuitCache {
	pending := make(map[distributed.ModuleName]int, len(moduleNames))
	for _, name := range moduleNames {
		pending[distributed.ModuleName(name)]++
	}
	return &circuitCache{
		entries: make(map[distributed.ModuleName]*circuitEntry),
		pending: pending,
	}
}

func (c *circuitCache) entry(module distributed.ModuleName) *circuitEntry {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[module]
	if !ok {
		e = &circuitEntry{}
		c.entries[module] = e
	}
	return e
}

// getGL returns the module's compiled GL circuit, loading it once.
func (c *circuitCache) getGL(cfg *config.Config, module distributed.ModuleName) (*distributed.RecursedSegmentCompilation, error) {
	e := c.entry(module)
	e.once.Do(func() {
		e.comp, e.buf, e.err = zkevm.LoadCompiledGLMmap(cfg, module)
	})
	return e.comp, e.err
}

// done marks a segment finished, releasing the module's circuit after its last one.
func (c *circuitCache) done(module distributed.ModuleName) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pending[module]--
	if c.pending[module] > 0 {
		return
	}
	if e, ok := c.entries[module]; ok && e.buf != nil {
		e.buf.Release()
		e.buf = nil
		e.comp = nil
	}
}

// release frees any circuits still cached (error-path safety net).
func (c *circuitCache) release() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, e := range c.entries {
		if e.buf != nil {
			e.buf.Release()
			e.buf = nil
			e.comp = nil
		}
	}
}
