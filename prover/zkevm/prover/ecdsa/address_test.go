package ecdsa

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/generic/testdata"
	keccak "github.com/consensys/linea-monorepo/prover/zkevm/prover/hash/keccak/glue"
	"github.com/stretchr/testify/assert"
)

func TestAddress(t *testing.T) {
	c := testCases
	limits := &Settings{
		MaxNbEcRecover: 1,
		MaxNbTx:        4,
	}
	ac := &Antichamber{Inputs: &antichamberInput{Settings: limits}}
	var addr *Addresses
	var uaGnark *UnalignedGnarkData
	var ecRec *EcRecover
	var td *txnData
	gbmGnark := generic.GenDataModule{}
	m := &keccak.KeccakSingleProvider{}

	size := limits.sizeAntichamber()

	nbKeccakF := ac.Inputs.Settings.nbKeccakF(8) // if each txn has 8 blocks
	nbRowsPerTxInTxnData := 9

	sizeTxnData := utils.NextPowerOfTwo(limits.MaxNbTx * nbRowsPerTxInTxnData)

	compiled := wizard.Compile(func(b *wizard.Builder) {
		comp := b.CompiledIOP

		// generate a gbm and use it to represent gnark-columns
		gbmGnark = testdata.CreateGenDataModule(comp, "UnGNARK", size, common.NbLimbU128)
		ac = &Antichamber{
			Inputs: &antichamberInput{Settings: limits},
			ID:     gbmGnark.HashNum,
		}

		uaGnark = &UnalignedGnarkData{
			GnarkPublicKeyIndex: gbmGnark.Index,
			IsPublicKey:         gbmGnark.ToHash,
			IsPkHashFilter:      gbmGnark.ToHash,
		}

		uaGnark.GnarkData = gbmGnark.Limbs.ToLittleEndianUint()

		ac.UnalignedGnarkData = uaGnark

		// commit to txnData and ecRecover
		td, ecRec = commitEcRecTxnData(comp, sizeTxnData, size, ac)

		// native columns and  constraints
		addr = newAddress(comp, size, ecRec, ac, td)

		// define keccak (columns and constraints)
		keccakInp := keccak.KeccakSingleProviderInput{
			Provider:      addr.Provider,
			MaxNumKeccakF: nbKeccakF,
		}

		m = keccak.NewKeccakSingleProvider(comp, keccakInp)

	}, dummy.Compile)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {

		testdata.GenerateAndAssignGenDataModule(run, &gbmGnark, c.HashNum, c.ToHash, false)

		// it assign mock data to EcRec and txn_data
		AssignEcRecTxnData(run, gbmGnark, limits.MaxNbEcRecover, limits.MaxNbTx, sizeTxnData, size, td, ecRec, ac)

		// assign address columns
		addr.assignAddress(run, limits.MaxNbEcRecover, size, ac, ecRec, uaGnark, td)

		// assign keccak columns via provider that is embedded in the receiver
		m.Run(run)
	})
	assert.NoError(t, wizard.Verify(compiled, proof))
}

type makeTestCase struct {
	HashNum []int
	ToHash  []int
}

var testCases = makeTestCase{
	HashNum: []int{1, 1, 1, 1, 1, 1, 2, 2, 2, 2, 2, 2, 3, 3, 3, 3, 3, 3, 4, 4, 4, 4, 4, 4, 5, 5, 5, 5, 5, 5},
	ToHash:  []int{1, 1, 1, 1, 1, 0, 1, 1, 1, 1, 0, 0, 1, 1, 1, 1, 0, 0, 1, 1, 1, 1, 0, 0, 1, 1, 1, 1, 0, 0},
}

// TestAddressesFunction tests that the Addresses() method correctly extracts
// a 160-bit address using the actual newAddress() function with all constraints.
func TestAddressesFunction(t *testing.T) {
	c := testCases
	limits := &Settings{
		MaxNbEcRecover: 1,
		MaxNbTx:        4,
	}
	ac := &Antichamber{Inputs: &antichamberInput{Settings: limits}}
	var addr *Addresses
	var uaGnark *UnalignedGnarkData
	var ecRec *EcRecover
	var td *txnData
	gbmGnark := generic.GenDataModule{}
	m := &keccak.KeccakSingleProvider{}

	size := limits.sizeAntichamber()
	nbKeccakF := ac.Inputs.Settings.nbKeccakF(8)
	nbRowsPerTxInTxnData := 9
	sizeTxnData := utils.NextPowerOfTwo(limits.MaxNbTx * nbRowsPerTxInTxnData)

	compiled := wizard.Compile(func(b *wizard.Builder) {
		comp := b.CompiledIOP

		// Generate a gbm and use it to represent gnark-columns
		gbmGnark = testdata.CreateGenDataModule(comp, "UnGNARK", size, common.NbLimbU128)
		ac = &Antichamber{
			Inputs: &antichamberInput{Settings: limits},
			ID:     gbmGnark.HashNum,
		}

		uaGnark = &UnalignedGnarkData{
			GnarkPublicKeyIndex: gbmGnark.Index,
			IsPublicKey:         gbmGnark.ToHash,
			IsPkHashFilter:      gbmGnark.ToHash,
		}
		uaGnark.GnarkData = gbmGnark.Limbs.ToLittleEndianUint()
		ac.UnalignedGnarkData = uaGnark

		// Commit to txnData and ecRecover
		td, ecRec = commitEcRecTxnData(comp, sizeTxnData, size, ac)

		// Create address using newAddress (with all constraints)
		addr = newAddress(comp, size, ecRec, ac, td)

		// Define keccak
		keccakInp := keccak.KeccakSingleProviderInput{
			Provider:      addr.Provider,
			MaxNumKeccakF: nbKeccakF,
		}
		m = keccak.NewKeccakSingleProvider(comp, keccakInp)

	}, dummy.Compile)

	proof := wizard.Prove(compiled, func(run *wizard.ProverRuntime) {
		testdata.GenerateAndAssignGenDataModule(run, &gbmGnark, c.HashNum, c.ToHash, false)

		// Assign mock data to EcRec and txn_data
		AssignEcRecTxnData(run, gbmGnark, limits.MaxNbEcRecover, limits.MaxNbTx, sizeTxnData, size, td, ecRec, ac)

		// Assign address columns
		addr.assignAddress(run, limits.MaxNbEcRecover, size, ac, ecRec, uaGnark, td)

		// Assign keccak columns via provider
		m.Run(run)

		// Verify Addresses() returns correct values at runtime
		// Check that we can retrieve address values for rows where IsAddressFromTxnData or IsAddressFromEcRec is set
		addressResult := addr.Addresses()
		isFromTxnData := addr.IsAddressFromTxnData.GetColAssignment(run)
		isFromEcRec := addr.IsAddressFromEcRec.GetColAssignment(run)

		foundAddress := false
		for i := 0; i < size; i++ {
			valTxnData := isFromTxnData.Get(i)
			valEcRec := isFromEcRec.Get(i)
			if valTxnData.IsOne() || valEcRec.IsOne() {
				foundAddress = true

				// Get the address row
				addressRow := addressResult.GetRow(run, i)

				// Verify the address is consistent with AddressLo and AddressHiUntrimmed
				// AddressLo should match the first 8 limbs of the address
				addressLoRow := addr.AddressLo.GetRow(run, i)
				for j := 0; j < common.NbLimbU128; j++ {
					assert.Equal(t, addressLoRow.T[j], addressRow.T[j],
						"AddressLo limb %d should match address limb %d", j, j)
				}

				// AddressHiUntrimmed.SliceOnBit(96, 128) = limbs 0-1 should match limbs 8-9 of address
				addressHiRow := addr.AddressHiUntrimmed.GetRow(run, i)
				for j := 0; j < 2; j++ {
					assert.Equal(t, addressHiRow.T[j], addressRow.T[8+j],
						"AddressHiUntrimmed limb %d should match address limb %d", j, 8+j)
				}

				break // Only need to verify one address row
			}
		}
		assert.True(t, foundAddress, "Should find at least one address row")
	})

	assert.NoError(t, wizard.Verify(compiled, proof))
}
