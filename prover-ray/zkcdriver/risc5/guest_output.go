package risc5

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
)

// RegisterGuestPublicOutputs registers the first [NumGuestPublicOutputs] bytes
// of the guest output as the [GuestPublicOutputsPI] public inputs of sys, in
// address order.
//
// The guest output memory holds one byte per row, in address order, on the last
// rows of its module. Its length is up to the guest, so the rows holding its
// first bytes are not at a fixed position and cannot be opened directly. They
// are copied instead into a module of [NumGuestPublicOutputs] rows, whose row k
// holds the address k and is constrained, by an inclusion into the rows of the
// guest output memory carrying an element, to hold the byte the guest wrote at
// that address. Addresses being unique in the memory, that byte is the only one
// the row can hold. The public inputs are then openings of that module, at
// fixed positions.
//
// A guest writing fewer than [NumGuestPublicOutputs] bytes fails the inclusion.
//
// Panics if the arithmetization exposes no output memory this package can bind.
func RegisterGuestPublicOutputs(sys *wiop.System) {

	var (
		output = guestOutput(sys)
		data   = sys.LookupColumn(output.Data)
		round  = data.Round()
		ctx    = sys.Context.Childf("guest-public-outputs")
		module = sys.NewSizedModule(ctx.Childf("module"), NumGuestPublicOutputs, wiop.PaddingDirectionNone)
	)

	// The address limbs of row k: k in the least significant limb and zero in
	// the others. k is below any limb's bound, so it needs no carry.
	var (
		// One extra slot each for the data column appended below.
		addresses       = make([]*wiop.ColumnView, len(output.Address), len(output.Address)+1)
		targetAddresses = make([]*wiop.ColumnView, len(output.Address), len(output.Address)+1)
	)
	for i, id := range output.Address {
		limb := make([]field.Element, NumGuestPublicOutputs)
		if i == len(output.Address)-1 {
			for k := range limb {
				limb[k].SetUint64(uint64(k))
			}
		}
		addresses[i] = module.NewPrecomputedColumn(
			ctx.Childf("address-%d", i),
			&wiop.ConcreteVector{Plain: field.VecFromBase(limb)},
		).View()
		targetAddresses[i] = sys.LookupColumn(id).View()
	}

	bytes := module.NewColumn(ctx.Childf("bytes"), round)

	sys.NewInclusion(
		ctx.Childf("inclusion"),
		[]wiop.Table{wiop.NewTable(append(addresses, bytes.View())...)},
		[]wiop.Table{wiop.NewFilteredTable(
			sys.LookupColumn(output.AccessBit).View(),
			append(targetAddresses, data.View())...,
		)},
	)

	for k := range NumGuestPublicOutputs {
		cell := bytes.At(k).Open(ctx.Childf("output-%d", k))
		sys.RegisterPublicInputs(GuestPublicOutputsPI, cell, k)
	}

	// Registered after the action assigning the trace, if any, so that the guest
	// output memory is assigned by the time it runs.
	round.RegisterAction(&assignGuestOutputBytes{
		Bytes:     bytes,
		Data:      data,
		AccessBit: sys.LookupColumn(output.AccessBit),
	})
}

// assignGuestOutputBytes assigns the column of [NumGuestPublicOutputs] bytes
// that [RegisterGuestPublicOutputs] binds to the guest output memory.
type assignGuestOutputBytes struct {
	Bytes, Data, AccessBit *wiop.Column
}

// Run implements [wiop.ProverAction]. It panics if the guest wrote fewer than
// [NumGuestPublicOutputs] bytes, as no assignment could then satisfy the
// inclusion.
func (a *assignGuestOutputBytes) Run(rt *wiop.Runtime) {

	var (
		data      = rt.GetColumnAssignment(a.Data).Plain.AsBase()
		accessBit = rt.GetColumnAssignment(a.AccessBit).Plain.AsBase()
		// The rows carrying an element are the last ones, as the access bit
		// is zero on the first row and never goes back to zero once set.
		first = len(accessBit)
	)

	for first > 0 && accessBit[first-1].IsOne() {
		first--
	}

	if written := len(accessBit) - first; written < NumGuestPublicOutputs {
		panic(fmt.Sprintf(
			"risc5: assignGuestOutputBytes: the guest wrote %d output bytes but the first %d are bound as public inputs",
			written, NumGuestPublicOutputs,
		))
	}

	bytes := make([]field.Element, NumGuestPublicOutputs)
	copy(bytes, data[first:])
	rt.AssignColumn(a.Bytes, &wiop.ConcreteVector{Plain: field.VecFromBase(bytes)})
}

// GetGuestPublicOutputs returns the first [NumGuestPublicOutputs] bytes of the
// guest output in address order, one per element, read from the public inputs
// pub of a proof over sys.
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
