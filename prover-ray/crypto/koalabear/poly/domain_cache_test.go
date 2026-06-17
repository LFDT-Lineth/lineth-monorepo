package poly

import "testing"

func TestDomainCacheReusesDomainsBySize(t *testing.T) {
	var cache DomainCache
	d8 := cache.Get(8)
	if got := cache.Get(8); got != d8 {
		t.Fatal("DomainCache did not reuse domain for same size")
	}
	if got := cache.Get(16); got == d8 {
		t.Fatal("DomainCache reused domain for different size")
	}
}

func TestNilDomainCacheGet(t *testing.T) {
	var cache *DomainCache
	if got := cache.Get(8); got == nil {
		t.Fatal("nil DomainCache returned nil domain")
	}
}
