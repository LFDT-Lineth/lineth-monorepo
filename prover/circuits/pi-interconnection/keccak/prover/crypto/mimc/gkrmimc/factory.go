package gkrmimc

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash"
	gkr_mimc "github.com/consensys/gnark/std/hash/mimc/gkr-mimc"
)

// HasherFactory wraps the gnark gkr-mimc package to provide a compatible interface
type HasherFactory struct {
	api frontend.API
}

// NewHasherFactory initializes a new [HasherFactory] object.
func NewHasherFactory(api frontend.API) *HasherFactory {
	return &HasherFactory{
		api: api,
	}
}

// NewHasher creates a new hash.StateStorer using the gnark gkr-mimc implementation
func (f *HasherFactory) NewHasher() hash.StateStorer {
	hasher, err := gkr_mimc.New(f.api)
	if err != nil {
		panic(err)
	}
	return hasher
}
