package risc5

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// RegisterGuestPublicOutputs registers the guest program's output as the
// [GuestPublicOutputsPI] public inputs of sys, in address order: public input k
// is the value the guest wrote to guest_output address k.
//
// The output length is [NumGuestPublicOutputs] rather than a runtime value
// because [wiop.System.RegisterPublicInputs] fixes the public-input vector once,
// during definition. The output module is dynamic and left-padded, so the value at
// address k sits at row k-NumGuestPublicOutputs only once the memory is pinned to
// exactly that many elements; that is what the length constraint below does, using
// the last address as the element count (see [zkcdriver.PublicOutput]). Without
// it a prover could grow the memory and pick which of its rows become public
// inputs.
//
// Panics if the arithmetization exposes no output memory this package can bind.
func RegisterGuestPublicOutputs(sys *wiop.System) {

	numOutputs := NumGuestPublicOutputs

	var (
		dataCol, addressCol = guestOutputColumns(sys)
		module              = dataCol.Module
		ctx                 = sys.Context.Childf("guest-public-outputs")
		lastAddress         field.Element
	)

	// add a size constraint: since the module is dynamic (data and size) but here we need its size to be fixed.
	lastAddress.SetUint64(uint64(numOutputs - 1))
	module.NewVanishing(
		ctx.Childf("length"),
		wiop.Sub(addressCol.At(-1), wiop.NewConstantField(lastAddress)),
	)

	for k := range numOutputs {
		cell := dataCol.At(k - numOutputs).Open(ctx.Childf("output-%d", k))
		sys.RegisterPublicInputs(GuestPublicOutputsPI, cell, k)
	}
}

// GetGuestPublicOutputs returns the guest program's output in address order, one
// field element per address, read through the public-input cells that
// [RegisterGuestPublicOutputs] bound to the guest_output columns. The values
// therefore come from the constrained trace rather than from the tracer's own
// output map: they are the ones the length and opening constraints pin down and
// the verifier checks, so they cannot drift from what the proof attests.
//
// Panics if the output length disagrees with [NumGuestPublicOutputs] or if a
// public input is missing.
func GetGuestPublicOutputs(rt *wiop.Runtime) []field.Element {

	numOutputs := NumGuestPublicOutputs
	_, addressCol := guestOutputColumns(rt.System)

	// Checking the length here reports a wrong-sized guest instead of letting it
	// surface as an opaque constraint failure. The last address gives the count in
	// a single read; the row count would not, as both the tracer and wiop pad the
	// module up to a power of two.
	lastAddress := addressCol.At(-1).EvaluateSingle(rt).Value.AsBase()
	if written := lastAddress.Uint64() + 1; written != uint64(numOutputs) {
		panic(fmt.Sprintf(
			"risc5: GetGuestPublicOutputs: the guest wrote %d outputs but the expected output size is %d",
			written, numOutputs,
		))
	}

	out := make([]field.Element, numOutputs)
	for k := range numOutputs {
		cell, pos := rt.System.LookupPublicInputByTag(GuestPublicOutputsPI, k)
		if pos < 0 {
			panic(fmt.Sprintf("risc5: GetGuestPublicOutputs: no public input registered for output %d", k))
		}

		out[k] = rt.GetCellValue(cell).AsBase()
	}

	return out
}

// guestOutputColumns returns the data and address columns of the memory carrying
// the guest program's output. The memory is found through the schema's
// public-output flag rather than by name (see [zkcdriver.PublicOutputs]).
//
// It panics rather than returning an error: a system with no such memory was built
// from an arithmetization that cannot express a guest output in the shape this
// package binds, which no caller can recover from.
func guestOutputColumns(sys *wiop.System) (data, address *wiop.Column) {

	output := zkcdriver.PublicOutputs(sys)

	if output.Name == "" {
		panic("risc5: guestOutputColumns: the arithmetization exposes no public output to bind the guest output from")
	}

	return sys.LookupColumn(output.Data), sys.LookupColumn(output.Address)
}
