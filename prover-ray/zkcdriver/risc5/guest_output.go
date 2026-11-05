package risc5

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// RegisterGuestPublicOutputs registers the [NumGuestPublicOutputs] bytes of the
// guest output as the [GuestPublicOutputsPI] public inputs of sys, in address
// order.
//
// The guest output memory holds one byte per row, in address order, on the last
// rows of its module. The guest writes exactly [NumGuestPublicOutputs] bytes to
// it, so byte k is on row k-NumGuestPublicOutputs, counted from the end (-1
// being the last row), and is opened there. A local constraint on the address pins the length: the last row
// holds the address NumGuestPublicOutputs-1, which no padding row does, and the
// addresses start at zero and increment by one, so that row is the last of
// exactly NumGuestPublicOutputs element rows.
//
// Panics if the arithmetization exposes no output memory this package can bind.
func RegisterGuestPublicOutputs(sys *wiop.System) {

	var (
		output = guestOutput(sys)
		data   = sys.LookupColumn(output.Data)
		module = data.Module
		ctx    = sys.Context.Childf("guest-public-outputs")
	)

	lastAddress := sys.LookupColumn(output.Address).At(-1)
	module.NewVanishing(ctx.Childf("last-address"),
		wiop.Sub(lastAddress, wiop.NewConstantField(field.NewElement(NumGuestPublicOutputs-1))))

	for k := range NumGuestPublicOutputs {
		cell := data.At(k - NumGuestPublicOutputs).Open(ctx.Childf("output-%d", k))
		sys.RegisterPublicInputs(GuestPublicOutputsPI, cell, k)
	}
}

// GetGuestPublicOutputs returns the [NumGuestPublicOutputs] bytes of the guest
// output in address order, one per element, read from the public inputs pub of
// a proof over sys.
//
// Panics if a public input is missing.
func GetGuestPublicOutputs(sys *wiop.System, pub wiop.PublicInput) []field.Element {

	out := make([]field.Element, NumGuestPublicOutputs)
	for k := range out {
		_, pos := sys.LookupPublicInputByTag(GuestPublicOutputsPI, k)
		if pos < 0 {
			panic(fmt.Sprintf("risc5: GetGuestPublicOutputs: no public input registered for output byte %d", k))
		}

		out[k] = pub[pos].AsBase()
	}

	return out
}

// guestOutput returns the description of the memory carrying the guest output
// (see [zkcdriver.PublicOutputs]). It panics if no guest output is declared.
func guestOutput(sys *wiop.System) zkcdriver.PublicOutput {

	output := zkcdriver.PublicOutputs(sys)

	if output.Name == "" {
		panic("risc5: guestOutput: the arithmetization exposes no public output to bind the guest output from")
	}

	return output
}
