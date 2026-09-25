package risc5

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// RegisterGuestPublicOutputs registers the guest output hash as the
// [GuestPublicOutputsPI] public inputs of sys, in address order.
//
// A hash word might be wider than one field element, so the schema splits it across
// several limb columns and every limb has to be opened for the word to be
// recoverable. The public inputs therefore hold [GuestPublicOutputCells] cells where
// GuestPublicOutputCells = NumGuestOutputs * NumGuestOutputLimbs
//
// Panics if the arithmetization exposes no output memory this package can bind.
func RegisterGuestPublicOutputs(sys *wiop.System) {

	numOutputs := NumGuestPublicOutputs

	var (
		dataCols, addressCol = guestOutputColumns(sys)
		module               = dataCols[0].Module
		ctx                  = sys.Context.Childf("guest-public-outputs")
		lastAddress          field.Element
	)

	// add a size constraint: since the module is dynamic (data and size) but here we need its size to be fixed.
	lastAddress.SetUint64(uint64(numOutputs - 1))
	module.NewVanishing(
		ctx.Childf("length"),
		wiop.Sub(addressCol.At(-1), wiop.NewConstantField(lastAddress)),
	)

	for k := range numOutputs {
		for l, dataCol := range dataCols {
			cell := dataCol.At(k - numOutputs).Open(ctx.Childf("output-%d-limb-%d", k, l))
			sys.RegisterPublicInputs(GuestPublicOutputsPI, cell, k*len(dataCols)+l)
		}
	}
}

// GetGuestPublicOutputs returns the guest output hash in address order, read
// through the public-input cells that [RegisterGuestPublicOutputs] bound to the
// guest_output_hash columns.
//
// Panics if the output length disagrees with [NumGuestPublicOutputs] or if a
// public input is missing.
func GetGuestPublicOutputs(rt *wiop.Runtime) []field.Element {

	numOutputs := NumGuestPublicOutputs
	dataCols, addressCol := guestOutputColumns(rt.System)

	// check that the hardcoded value for [NumGuestPublicOutputs] is consistent with the interpreter choice.
	lastAddress := addressCol.At(-1).EvaluateSingle(rt).Value.AsBase()

	if written := lastAddress.Uint64() + 1; written != uint64(numOutputs) {
		panic(fmt.Sprintf(
			"risc5: GetGuestPublicOutputs: the guest wrote %d outputs but the expected output size is %d",
			written, numOutputs,
		))
	}

	out := make([]field.Element, numOutputs*len(dataCols))
	for k := range out {
		cell, pos := rt.System.LookupPublicInputByTag(GuestPublicOutputsPI, k)
		if pos < 0 {
			panic(fmt.Sprintf("risc5: GetGuestPublicOutputs: no public input registered for output cell %d", k))
		}

		out[k] = rt.GetCellValue(cell).AsBase()
	}

	return out
}

// GuestPublicOutputLimbs returns the number of columns the schema splits one hash
// word across.
func GuestPublicOutputLimbs(sys *wiop.System) int {
	data, _ := guestOutputColumns(sys)
	return len(data)
}

// GuestPublicOutputCells returns how many public inputs
// [RegisterGuestPublicOutputs] registers: one per limb of each of the
// [NumGuestPublicOutputs] hash words.
func GuestPublicOutputCells(sys *wiop.System) int {
	return NumGuestPublicOutputs * GuestPublicOutputLimbs(sys)
}

// guestOutput returns the description of the memory carrying the guest output
// hash together with its resolved columns (see [zkcdriver.PublicOutputs]).
// It panics if no guest output hash is declared.
func guestOutputColumns(sys *wiop.System) (data []*wiop.Column, address *wiop.Column) {

	output := zkcdriver.PublicOutputs(sys)

	if output.Name == "" {
		panic("risc5: guestOutputColumns: the arithmetization exposes no public output to bind the guest output from")
	}

	data = make([]*wiop.Column, len(output.Data))
	for i, id := range output.Data {
		data[i] = sys.LookupColumn(id)
	}

	return data, sys.LookupColumn(output.Address)
}
