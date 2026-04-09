package execution

import (
	"math/big"
	"reflect"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/test"
	"github.com/consensys/linea-monorepo/prover/config"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/consensys/linea-monorepo/prover/utils"
	zkcommon "github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	"github.com/sirupsen/logrus"
)

// ExtractedPISnark holds the public input values extracted from the inner wizard
// proof in a lightweight form suitable for gnark constraint solving. Unlike
// [wizard.VerifierCircuit], this struct stores only the extracted public input
// values (a few hundred field elements) rather than the full proof data.
type ExtractedPISnark struct {
	InitialStateRootHash         [zkcommon.NbElemPerHash]frontend.Variable
	FinalStateRootHash           [zkcommon.NbElemPerHash]frontend.Variable
	InitialBlockNumber           [zkcommon.NbLimbU48]frontend.Variable
	FinalBlockNumber             [zkcommon.NbLimbU48]frontend.Variable
	InitialBlockTimestamp        [zkcommon.NbLimbU128]frontend.Variable
	FinalBlockTimestamp          [zkcommon.NbLimbU128]frontend.Variable
	FirstRollingHashUpdate       [zkcommon.NbLimbU256]frontend.Variable
	LastRollingHashUpdate        [zkcommon.NbLimbU256]frontend.Variable
	FirstRollingHashUpdateNumber [zkcommon.NbLimbU128]frontend.Variable
	LastRollingHashUpdateNumber  [zkcommon.NbLimbU128]frontend.Variable
	ChainID                      [zkcommon.NbLimbU256]frontend.Variable
	BaseFee                      [zkcommon.NbLimbU128]frontend.Variable
	CoinBase                     [zkcommon.NbLimbEthAddress]frontend.Variable
	L2MessageServiceAddr         [zkcommon.NbLimbEthAddress]frontend.Variable
	DataNbBytes                  frontend.Variable
	// DataChecksum holds the koalabear Poseidon hash of the execution data (8 limbs).
	DataChecksum [zkcommon.NbLimbU128]frontend.Variable
	// DataSZX and DataSZY hold the Schwarz-Zipfel commitment X and Y coordinates
	// as 4 koalabear field elements each (one extension field element).
	DataSZX [4]frontend.Variable
	DataSZY [4]frontend.Variable
	// L2Messages holds the extracted L2-to-L1 message hashes, one per slot,
	// each as NbLimbU256 koalabear limbs.
	L2Messages [][zkcommon.NbLimbU256]frontend.Variable
}

// PICheckCircuit is a lightweight gnark circuit that checks the public input
// consistency constraints ([checkPublicInputs]) using pre-extracted PI values
// rather than a full [wizard.VerifierCircuit]. This lets us run
// [test.IsSolved] without any proving setup and without copying the entire
// inner proof into gnark's assignment.
type PICheckCircuit struct {
	ExtractedPIs  ExtractedPISnark           `gnark:",secret"`
	FuncInputs    FunctionalPublicInputSnark `gnark:",secret"`
	ExecDataBytes [1 << 17]frontend.Variable `gnark:",secret"`
}

// Define runs only the [checkPublicInputs] constraints.
func (c *PICheckCircuit) Define(api frontend.API) error {
	checkPublicInputs(api, &c.ExtractedPIs, c.FuncInputs, c.ExecDataBytes)
	return nil
}

// AllocatePICheck allocates a [PICheckCircuit] for use with [test.IsSolved].
func AllocatePICheck(maxL2L1Logs int) PICheckCircuit {
	return PICheckCircuit{
		ExtractedPIs: ExtractedPISnark{
			L2Messages: make([][zkcommon.NbLimbU256]frontend.Variable, maxL2L1Logs),
		},
		FuncInputs: FunctionalPublicInputSnark{
			FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
				L2MessageHashes: L2MessageHashes{
					Values: make([][32]frontend.Variable, maxL2L1Logs),
				},
			},
		},
	}
}

// extractPIsFromWVC extracts the public input values from the wizard verifier
// circuit in the gnark constraint context. It is called inside
// [CircuitExecution.Define] so that [checkPublicInputs] can operate on the
// lightweight [ExtractedPISnark] instead of the full [wizard.VerifierCircuit].
func extractPIsFromWVC(api frontend.API, wvc *wizard.VerifierCircuit) ExtractedPISnark {
	pie := getPublicInputExtractor(wvc)

	toVar := func(pi wizard.PublicInput) frontend.Variable {
		return getPublicInput(api, wvc, pi)
	}

	toVarArr8 := func(pis [zkcommon.NbElemPerHash]wizard.PublicInput) [zkcommon.NbElemPerHash]frontend.Variable {
		return [zkcommon.NbElemPerHash]frontend.Variable(getPublicInputArr(api, wvc, pis[:]))
	}

	toVarArr3 := func(pis [zkcommon.NbLimbU48]wizard.PublicInput) [zkcommon.NbLimbU48]frontend.Variable {
		return [zkcommon.NbLimbU48]frontend.Variable(getPublicInputArr(api, wvc, pis[:]))
	}

	toVarArr10 := func(pis [zkcommon.NbLimbEthAddress]wizard.PublicInput) [zkcommon.NbLimbEthAddress]frontend.Variable {
		return [zkcommon.NbLimbEthAddress]frontend.Variable(getPublicInputArr(api, wvc, pis[:]))
	}

	toVarArr16 := func(pis [zkcommon.NbLimbU256]wizard.PublicInput) [zkcommon.NbLimbU256]frontend.Variable {
		return [zkcommon.NbLimbU256]frontend.Variable(getPublicInputArr(api, wvc, pis[:]))
	}

	extr := ExtractedPISnark{
		InitialStateRootHash:         toVarArr8(pie.InitialStateRootHash),
		FinalStateRootHash:           toVarArr8(pie.FinalStateRootHash),
		InitialBlockNumber:           toVarArr3(pie.InitialBlockNumber),
		FinalBlockNumber:             toVarArr3(pie.FinalBlockNumber),
		InitialBlockTimestamp:        toVarArr8(pie.InitialBlockTimestamp),
		FinalBlockTimestamp:          toVarArr8(pie.FinalBlockTimestamp),
		FirstRollingHashUpdate:       toVarArr16(pie.FirstRollingHashUpdate),
		LastRollingHashUpdate:        toVarArr16(pie.LastRollingHashUpdate),
		FirstRollingHashUpdateNumber: toVarArr8(pie.FirstRollingHashUpdateNumber),
		LastRollingHashUpdateNumber:  toVarArr8(pie.LastRollingHashUpdateNumber),
		ChainID:                      toVarArr16(pie.ChainID),
		BaseFee:                      toVarArr8(pie.BaseFee),
		CoinBase:                     toVarArr10(pie.CoinBase),
		L2MessageServiceAddr:         toVarArr10(pie.L2MessageServiceAddr),
		DataNbBytes:                  toVar(pie.DataNbBytes),
		DataChecksum:                 toVarArr8(pie.DataChecksum),
		DataSZX:                      getPublicInputExt(api, wvc, pie.DataSZX),
		DataSZY:                      getPublicInputExt(api, wvc, pie.DataSZY),
	}

	extr.L2Messages = make([][zkcommon.NbLimbU256]frontend.Variable, len(pie.L2Messages))
	for i, msgs := range pie.L2Messages {
		extr.L2Messages[i] = [zkcommon.NbLimbU256]frontend.Variable(getPublicInputArr(api, wvc, msgs[:]))
	}

	return extr
}

// extractPIsNatively extracts the public input values from the inner wizard
// proof at the native (non-gnark) level, by running [wizard.VerifyWithRuntime].
// The result is an [ExtractedPISnark] whose fields are populated with
// *big.Int values suitable for use as gnark assignment values.
func extractPIsNatively(comp *wizard.CompiledIOP, proof wizard.Proof) ExtractedPISnark {
	pie, ok := comp.ExtraData[publicInput.PublicInputExtractorMetadata].(*publicInput.FunctionalInputExtractor)
	if !ok {
		utils.Panic("public input extractor not found or wrong type in ExtraData")
	}

	runtime, err := wizard.VerifyWithRuntime(comp, proof, true)
	if err != nil {
		utils.Panic("extractPIsNatively: VerifyWithRuntime failed: %v", err)
	}

	toVar := func(pi wizard.PublicInput) frontend.Variable {
		g := runtime.GetPublicInput(pi.Name)
		var bi big.Int
		g.Base.BigInt(&bi)
		return new(big.Int).Set(&bi)
	}

	toVarExt := func(pi wizard.PublicInput) [4]frontend.Variable {
		g := runtime.GetPublicInput(pi.Name)
		ext := g.GetExt()
		var b00, b01, b10, b11 big.Int
		ext.B0.A0.BigInt(&b00)
		ext.B0.A1.BigInt(&b01)
		ext.B1.A0.BigInt(&b10)
		ext.B1.A1.BigInt(&b11)
		return [4]frontend.Variable{
			new(big.Int).Set(&b00),
			new(big.Int).Set(&b01),
			new(big.Int).Set(&b10),
			new(big.Int).Set(&b11),
		}
	}

	toVarArr8 := func(pis [zkcommon.NbElemPerHash]wizard.PublicInput) [zkcommon.NbElemPerHash]frontend.Variable {
		var res [zkcommon.NbElemPerHash]frontend.Variable
		for i, pi := range pis {
			res[i] = toVar(pi)
		}
		return res
	}

	toVarArr3 := func(pis [zkcommon.NbLimbU48]wizard.PublicInput) [zkcommon.NbLimbU48]frontend.Variable {
		var res [zkcommon.NbLimbU48]frontend.Variable
		for i, pi := range pis {
			res[i] = toVar(pi)
		}
		return res
	}

	toVarArr10 := func(pis [zkcommon.NbLimbEthAddress]wizard.PublicInput) [zkcommon.NbLimbEthAddress]frontend.Variable {
		var res [zkcommon.NbLimbEthAddress]frontend.Variable
		for i, pi := range pis {
			res[i] = toVar(pi)
		}
		return res
	}

	toVarArr16 := func(pis [zkcommon.NbLimbU256]wizard.PublicInput) [zkcommon.NbLimbU256]frontend.Variable {
		var res [zkcommon.NbLimbU256]frontend.Variable
		for i, pi := range pis {
			res[i] = toVar(pi)
		}
		return res
	}

	extr := ExtractedPISnark{
		InitialStateRootHash:         toVarArr8(pie.InitialStateRootHash),
		FinalStateRootHash:           toVarArr8(pie.FinalStateRootHash),
		InitialBlockNumber:           toVarArr3(pie.InitialBlockNumber),
		FinalBlockNumber:             toVarArr3(pie.FinalBlockNumber),
		InitialBlockTimestamp:        toVarArr8(pie.InitialBlockTimestamp),
		FinalBlockTimestamp:          toVarArr8(pie.FinalBlockTimestamp),
		FirstRollingHashUpdate:       toVarArr16(pie.FirstRollingHashUpdate),
		LastRollingHashUpdate:        toVarArr16(pie.LastRollingHashUpdate),
		FirstRollingHashUpdateNumber: toVarArr8(pie.FirstRollingHashUpdateNumber),
		LastRollingHashUpdateNumber:  toVarArr8(pie.LastRollingHashUpdateNumber),
		ChainID:                      toVarArr16(pie.ChainID),
		BaseFee:                      toVarArr8(pie.BaseFee),
		CoinBase:                     toVarArr10(pie.CoinBase),
		L2MessageServiceAddr:         toVarArr10(pie.L2MessageServiceAddr),
		DataNbBytes:                  toVar(pie.DataNbBytes),
		DataChecksum:                 toVarArr8(pie.DataChecksum),
		DataSZX:                      toVarExt(pie.DataSZX),
		DataSZY:                      toVarExt(pie.DataSZY),
	}

	extr.L2Messages = make([][zkcommon.NbLimbU256]frontend.Variable, len(pie.L2Messages))
	for i, msgs := range pie.L2Messages {
		var row [zkcommon.NbLimbU256]frontend.Variable
		for j, pi := range msgs {
			row[j] = toVar(pi)
		}
		extr.L2Messages[i] = row
	}

	return extr
}

// assignPICheck builds the gnark assignment for a [PICheckCircuit].
func assignPICheck(
	comp *wizard.CompiledIOP,
	proof wizard.Proof,
	funcInputs public_input.Execution,
	execData []byte,
	maxL2L1Logs int,
) PICheckCircuit {
	extr := extractPIsNatively(comp, proof)

	res := PICheckCircuit{
		ExtractedPIs: extr,
		FuncInputs: FunctionalPublicInputSnark{
			FunctionalPublicInputQSnark: FunctionalPublicInputQSnark{
				L2MessageHashes: L2MessageHashes{
					Values: make([][32]frontend.Variable, maxL2L1Logs),
				},
			},
		},
	}

	if len(execData) > len(res.ExecDataBytes) {
		utils.Panic("execData is too long: conflation contains too much data: %v > %v", len(execData), len(res.ExecDataBytes))
	}

	for i, b := range execData {
		res.ExecDataBytes[i] = b
	}

	for i := len(execData); i < len(res.ExecDataBytes); i++ {
		res.ExecDataBytes[i] = 0
	}

	if err := res.FuncInputs.Assign(&funcInputs); err != nil {
		panic(err)
	}

	return res
}

// CheckPublicInputConsistency verifies that the [checkPublicInputs] gnark
// constraints are satisfied by the given inner wizard proof and functional
// inputs. It uses gnark's test.IsSolved — no proving setup is required.
//
// comp must be the compiled IOP that was used to generate proof (i.e.
// [zkevm.ZkEvm.InitialCompiledIOP] when recursion is not set up, or
// [zkevm.ZkEvm.RecursionCompiledIOP] when it is).
func CheckPublicInputConsistency(
	limits *config.TracesLimits,
	comp *wizard.CompiledIOP,
	proof wizard.Proof,
	funcInputs public_input.Execution,
	execData []byte,
) {
	circuit := AllocatePICheck(limits.BlockL2L1Logs())
	assignment := assignPICheck(comp, proof, funcInputs, execData, limits.BlockL2L1Logs())
	if err := test.IsSolved(&circuit, &assignment, fr.Modulus()); err != nil {
		utils.Panic("public input consistency check failed: %v", err)
	}
	logrus.Infof("Public input consistency check passed")
}

// getPublicInputExtractor extracts the [publicInput.FunctionalInputExtractor]
// from the wizard verifier circuit's compiled IOP ExtraData.
func getPublicInputExtractor(wvc *wizard.VerifierCircuit) *publicInput.FunctionalInputExtractor {
	extraData, extraDataFound := wvc.Spec.ExtraData[publicInput.PublicInputExtractorMetadata]
	if !extraDataFound {
		panic("public input extractor not found")
	}
	pie, ok := extraData.(*publicInput.FunctionalInputExtractor)
	if !ok {
		panic("public input extractor not of the right type: " + reflect.TypeOf(extraData).String())
	}
	return pie
}

// getPublicInputExt returns the extension-field public input coordinates as a
// [4]frontend.Variable (4 koalabear field elements).
func getPublicInputExt(api frontend.API, wvc *wizard.VerifierCircuit, pi wizard.PublicInput) [4]frontend.Variable {
	name := pi.Name
	if !wvc.HasPublicInput(name) {
		name = "functional." + name
	}
	res := wvc.GetPublicInputExt(api, name)
	return [4]frontend.Variable{
		res.B0.A0.Native(),
		res.B0.A1.Native(),
		res.B1.A0.Native(),
		res.B1.A1.Native(),
	}
}

// getPublicInputArr returns a slice of frontend.Variable values from a slice
// of [wizard.PublicInput] references.
func getPublicInputArr(api frontend.API, wvc *wizard.VerifierCircuit, pis []wizard.PublicInput) []frontend.Variable {
	res := make([]frontend.Variable, len(pis))
	for i := range pis {
		// When the outer-proof runs on top of the limitless prover, the names of
		// the "functional" public inputs are prefixed with "functional.".
		name := pis[i].Name
		if !wvc.HasPublicInput(name) {
			name = "functional." + name
		}
		r := wvc.GetPublicInput(api, name)
		res[i] = r.Native()
	}
	return res
}

// getPublicInput returns a single frontend.Variable from a [wizard.PublicInput].
func getPublicInput(api frontend.API, wvc *wizard.VerifierCircuit, pi wizard.PublicInput) frontend.Variable {
	name := pi.Name
	if !wvc.HasPublicInput(name) {
		name = "functional." + name
	}
	r := wvc.GetPublicInput(api, name)
	return r.Native()
}
