package risc5

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// RegisterGuestPublicOutputs registers the guest program's output as the
// [GuestPublicOutputsPI] public inputs of sys, in address order: public input k
// is the byte the guest wrote to guest_output address k.
//
// The output length is [NumGuestPublicOutputs], a constant of the guest program
// rather than a setting: it has to be known here rather than at proving time
// because [wiop.System.RegisterPublicInputs] fixes the public-input vector once,
// during definition.
//
// The output memory's wiop module is dynamic and left-padded, so its rows have a
// stable index only when counted from the end of the domain: the byte at address
// k sits at row k-NumGuestPublicOutputs. That holds only once the memory is known
// to hold exactly that many elements, which is what the length constraint below
// pins, leaning on the constraints the arithmetization already emits for the
// address column: it vanishes on padding rows, is zero on the first row carrying
// an element and increments from there, so the address on the last row is one less
// than the number of elements.
//
// Panics if the arithmetization exposes no output memory this package can bind.
func RegisterGuestPublicOutputs(sys *wiop.System) {

	numBytes := NumGuestPublicOutputs

	var (
		dataCol, addressCol = guestOutputColumns(sys)
		module              = dataCol.Module
		ctx                 = sys.Context.Childf("guest-public-outputs")
		lastAddress         field.Element
	)

	lastAddress.SetUint64(uint64(numBytes - 1))
	module.NewVanishing(
		ctx.Childf("length"),
		wiop.Sub(addressCol.At(-1), wiop.NewConstantField(lastAddress)),
	)

	for k := range numBytes {
		cell := dataCol.At(k - numBytes).Open(ctx.Childf("byte-%d", k))
		sys.RegisterPublicInputs(GuestPublicOutputsPI, cell, k)
	}
}

// GetGuestPublicOutputs returns the guest program's output in address order,
// read through the public-input cells that [RegisterGuestPublicOutputs] bound to
// the guest_output columns.
//
// The bytes come from the constrained trace rather than from the tracer's own
// output map: these are the values the length and opening constraints pin down
// and the verifier checks, so they cannot drift from what the proof attests.
//
// It first checks that the memory really holds [NumGuestPublicOutputs] bytes, so
// a guest that wrote a different number than that is reported here instead of
// surfacing as an opaque constraint failure.
//
// Panics if the output length disagrees with [NumGuestPublicOutputs], if a public
// input is missing, or if a value does not fit in a byte.
func GetGuestPublicOutputs(rt *wiop.Runtime) []byte {

	numBytes := NumGuestPublicOutputs
	_, addressCol := guestOutputColumns(rt.System)

	// The address on the last row is one less than the number of elements the
	// memory holds, so it gives the count in a single read. The row count would
	// not: both the tracer and wiop pad the module up to a power of two.
	lastAddress := addressCol.At(-1).EvaluateSingle(rt).Value.AsBase()
	if written := lastAddress.Uint64() + 1; written != uint64(numBytes) {
		panic(fmt.Sprintf(
			"risc5: GetGuestPublicOutputs: the guest wrote %d output bytes but the expected output size is %d",
			written, numBytes,
		))
	}

	out := make([]byte, numBytes)
	for k := range numBytes {
		cell, pos := rt.System.LookupPublicInputByTag(GuestPublicOutputsPI, k)
		if pos < 0 {
			panic(fmt.Sprintf("risc5: GetGuestPublicOutputs: no public input registered for output byte %d", k))
		}

		value := rt.GetCellValue(cell).AsBase()
		if v := value.Uint64(); v > 0xff {
			panic(fmt.Sprintf("risc5: GetGuestPublicOutputs: output byte %d has value %d", k, v))
		} else {
			out[k] = byte(v)
		}
	}

	return out
}

// guestOutputColumns returns the data and address columns of the memory carrying
// the guest program's output. The memory is not identified by name: it is the
// arithmetization's public output, which the schema flags for us (see
// [zkcdriver.PublicOutputs]).
//
// It panics rather than returning an error, because every case it rejects means
// the system was built from an arithmetization that cannot express a guest output
// in the shape this package binds, which no caller can recover from.
func guestOutputColumns(sys *wiop.System) (data, address *wiop.Column) {

	output := zkcdriver.PublicOutputs(sys)

	if output.Name == "" {
		panic("risc5: the arithmetization exposes no byte-addressed public output to bind the guest output from")
	}

	return sys.LookupColumn(output.Data), sys.LookupColumn(output.Address)
}
