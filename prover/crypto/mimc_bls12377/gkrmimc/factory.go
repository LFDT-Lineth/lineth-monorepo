package gkrmimc

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash"
	gkr_mimc "github.com/consensys/gnark/std/hash/mimc/gkr-mimc"
)

// NewHasherFactory initializes a new [HasherFactory] object. Ideally, it should
// be called only once per circuit.
func NewHasherFactory(api frontend.API) *HasherFactory {
	return &HasherFactory{api}
}

// HasherFactory is an object that can construct hashers satisfying the
// [hash.FieldHasher] interface and which can be used to perform MiMC hashing
// in a gnark circuit. All hashing operations performed by these hashers are
// bare claims whose truthfulness is backed by the verification of a GKR proof
// in the same circuit. This deferred GKR verification is hidden from the user.
type HasherFactory struct {
	api frontend.API
}

// NewHasher spawns a hasher that will defer the hash verification to the
// factory. It is safe to be called multiple times and the returned Hasher can
// be used exactly in the same way as [github.com/consensys/gnark/std/hash/mimc.NewMiMC]
// and will provide the same results for the same usage.
//
// However, the hasher should not be used in deferred gnark circuit execution.
func (f *HasherFactory) NewHasher() hash.StateStorer {
	h, err := gkr_mimc.New(f.api)
	if err != nil {
		panic(err)
	}
	return h
}
