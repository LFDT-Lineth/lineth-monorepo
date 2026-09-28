package wioptest

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// TestingCompileOptions selects two FRI queries and an unsharded message bus
// for full-pipeline test scenarios with round-0 witness columns. Do not use
// these reduced-security options in production or seeded round-1 bus fixtures.
func TestingCompileOptions() []compilers.Option {
	return []compilers.Option{
		compilers.WithPCSOption(pcs.WithFRINumQueries(2)),
		compilers.WithMessageBusOption(messagebus.WithoutSharedRandomness()),
	}
}
