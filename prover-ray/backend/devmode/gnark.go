package devmode

import (
	"fmt"
	"math/big"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/backend/plonk"
	plonkbn254 "github.com/consensys/gnark/backend/plonk/bn254"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/test/unsafekzg"
)

// gnarkToxicSeed seeds the unsafe KZG SRS of [GnarkSetup]. PLONK setup has no
// other randomness, so a fixed seed gives the same keys on every run.
var gnarkToxicSeed = []byte("lineth-devmode-gnark")

// GnarkSetup compiles [GnarkCircuit] and runs the PLONK setup over BN254 on an
// unsafe KZG SRS from a fixed seed: the keys are the same on every run, and
// anyone can forge proofs under them (test/dev builds only).
func GnarkSetup() (constraint.ConstraintSystem, plonk.ProvingKey, plonk.VerifyingKey, error) {
	ccs, err := frontend.Compile(ecc.BN254.ScalarField(), scs.NewBuilder, &GnarkCircuit{})
	if err != nil {
		return nil, nil, nil, fmt.Errorf("devmode: compiling the gnark circuit: %w", err)
	}
	srs, srsLagrange, err := unsafekzg.NewSRS(ccs, unsafekzg.WithToxicSeed(gnarkToxicSeed))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("devmode: building the KZG SRS: %w", err)
	}
	pk, vk, err := plonk.Setup(ccs, srs, srsLagrange)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("devmode: PLONK setup: %w", err)
	}
	return ccs, pk, vk, nil
}

// GnarkProve proves [GnarkCircuit] with PLONK over BN254 for the devmode public
// inputs pub of a proof over sys, under the keys of [GnarkSetup].
func GnarkProve(sys *wiop.System, pub wiop.PublicInput) (plonk.Proof, error) {
	if len(pub) != NumPublicInputs {
		return nil, fmt.Errorf("devmode: GnarkProve: got %d public inputs, want %d", len(pub), NumPublicInputs)
	}
	ccs, pk, _, err := GnarkSetup()
	if err != nil {
		return nil, err
	}
	w, err := frontend.NewWitness(NewGnarkAssignment(sys, pub), ecc.BN254.ScalarField())
	if err != nil {
		return nil, fmt.Errorf("devmode: gnark witness: %w", err)
	}
	proof, err := plonk.Prove(ccs, pk, w)
	if err != nil {
		return nil, fmt.Errorf("devmode: PLONK prove: %w", err)
	}
	return proof, nil
}

// GnarkEncode writes a proof of [GnarkProve] in the layout the gnark Solidity
// verifier reads.
func GnarkEncode(proof plonk.Proof) ([]byte, error) {
	p, ok := proof.(*plonkbn254.Proof)
	if !ok {
		return nil, fmt.Errorf("devmode: GnarkEncode: got a %T, want a BN254 PLONK proof", proof)
	}
	return p.MarshalSolidity(), nil
}

// GnarkCircuit is the gnark (BN254 PLONK) statement of dev mode. It exposes the
// same two public inputs as the wiop wrapper circuit: the packed guest output
// and the packed program VK, in that order. It does not verify a wiop proof: it
// only binds the 40 devmode public-input cells to their packed values, so its
// proof is as unsound as a devmode wiop proof (test/dev builds only).
type GnarkCircuit struct {
	// GuestOutput is the 32 guest-output bytes read as one big-endian number,
	// reduced mod r.
	GuestOutput frontend.Variable `gnark:",public"`
	// ProgramVK is the 8 program-VK limbs written big-endian, 4 bytes each,
	// read as one big-endian number, reduced mod r.
	ProgramVK frontend.Variable `gnark:",public"`

	// GuestOutputCells are the guest-output cells, one byte each.
	GuestOutputCells [NumGuestOutputCells]frontend.Variable
	// ProgramVKCells are the program-VK limbs, one per cell.
	ProgramVKCells [NumProgramVKCells]frontend.Variable
}

// Define implements [frontend.Circuit].
func (c *GnarkCircuit) Define(api frontend.API) error {
	api.AssertIsEqual(c.GuestOutput, PackGuestOutputCircuit(api, c.GuestOutputCells[:]))
	api.AssertIsEqual(c.ProgramVK, PackProgramVKCircuit(api, c.ProgramVKCells[:]))
	return nil
}

// NewGnarkAssignment returns the full assignment of [GnarkCircuit] for the
// devmode public inputs pub of a proof over sys. The cells are located by
// their wiop tags.
func NewGnarkAssignment(sys *wiop.System, pub wiop.PublicInput) *GnarkCircuit {
	out := risc5.GetGuestPublicOutputs(sys, pub)
	vk := risc5.GetProgramVK(sys, pub)

	a := &GnarkCircuit{
		GuestOutput: PackGuestOutput(out),
		ProgramVK:   PackProgramVK(vk[:]),
	}
	for i := range a.GuestOutputCells {
		a.GuestOutputCells[i] = out[i].Uint64()
	}
	for i := range a.ProgramVKCells {
		a.ProgramVKCells[i] = vk[i].Uint64()
	}
	return a
}

// PackGuestOutput reads the guest-output cells, one byte each, as one
// big-endian number (cell 0 is the most significant byte) and reduces it mod
// the BN254 scalar field order r, as the contract does with
// mod(keccak256(...), MODULO_R).
func PackGuestOutput(cells []field.Element) *big.Int {
	if len(cells) != NumGuestOutputCells {
		panic(fmt.Sprintf("devmode: PackGuestOutput: got %d cells, want %d", len(cells), NumGuestOutputCells))
	}
	return wiop.PackBigEndian(cells, wiop.GuestOutputBitsPerCell)
}

// PackProgramVK writes the program-VK limbs big-endian, 4 bytes each (limb i
// in bytes 4i to 4i+4), reads the 32 bytes as one big-endian number and
// reduces it mod the BN254 scalar field order r.
func PackProgramVK(limbs []field.Element) *big.Int {
	if len(limbs) != NumProgramVKCells {
		panic(fmt.Sprintf("devmode: PackProgramVK: got %d limbs, want %d", len(limbs), NumProgramVKCells))
	}
	return wiop.PackBigEndian(limbs, wiop.ProgramVKBitsPerCell)
}

// PackGuestOutputCircuit is the in-circuit counterpart of [PackGuestOutput].
// The cells are not range-checked.
func PackGuestOutputCircuit(api frontend.API, cells []frontend.Variable) frontend.Variable {
	return wiop.PackBigEndianCircuit(api, cells, wiop.GuestOutputBitsPerCell)
}

// PackProgramVKCircuit is the in-circuit counterpart of [PackProgramVK].
// The limbs are not range-checked.
func PackProgramVKCircuit(api frontend.API, limbs []frontend.Variable) frontend.Variable {
	return wiop.PackBigEndianCircuit(api, limbs, wiop.ProgramVKBitsPerCell)
}
