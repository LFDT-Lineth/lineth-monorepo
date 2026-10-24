package circuits

import (
	"context"

	"github.com/consensys/gnark-crypto/kzg"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/test/unsafekzg"
)

type SRSProvider interface {
	GetSRS(ctx context.Context, ccs constraint.ConstraintSystem) (kzg.SRS, kzg.SRS, error)
}

// LagrangePersister is implemented by SRS providers that can pre-compute the
// Lagrange basis and leave it on disk for later runs.
//
// It is deliberately separate from SRSProvider: obtaining an SRS is something
// every prove path does, whereas writing into the SRS directory is a
// provisioning action. Keeping the write off the interface every caller holds
// is what stops it from happening implicitly.
//
// Without force, a dump of the right size already on disk counts as done and
// the call is cheap; force re-validates an existing dump in full and repairs
// it if it does not load, mirroring what --force means to `prover setup`.
type LagrangePersister interface {
	DeriveAndPersistLagrange(ctx context.Context, ccs constraint.ConstraintSystem, force bool) error
}

type UnsafeSRSProvider struct {
}

// NewUnsafeSRSProvider returns a new UnsafeSRSProvider
// if tau is provided, it will be used as the tau value for the SRS (slow path)
// otherwise, a random tau will be generated (fast path)
func NewUnsafeSRSProvider() UnsafeSRSProvider {
	return UnsafeSRSProvider{}
}

func (u UnsafeSRSProvider) GetSRS(ctx context.Context, ccs constraint.ConstraintSystem) (kzg.SRS, kzg.SRS, error) {
	return unsafekzg.NewSRS(ccs, unsafekzg.WithFSCache())
}
