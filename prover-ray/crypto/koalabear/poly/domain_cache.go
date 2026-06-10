package poly

import (
	"sync"

	"github.com/consensys/gnark-crypto/field/koalabear/fft"
)

// DomainCache memoizes FFT domains by cardinality. Safe for concurrent use.
type DomainCache struct {
	mu      sync.Mutex
	domains map[uint64]*fft.Domain
}

// Get returns the FFT domain of cardinality n, creating it on first use. Calling
// Get on a nil cache is valid and behaves like fft.NewDomain(n).
func (c *DomainCache) Get(n uint64) *fft.Domain {
	if c == nil {
		return fft.NewDomain(n)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.domains == nil {
		c.domains = make(map[uint64]*fft.Domain)
	}
	if d, ok := c.domains[n]; ok {
		return d
	}
	d := fft.NewDomain(n)
	c.domains[n] = d
	return d
}
