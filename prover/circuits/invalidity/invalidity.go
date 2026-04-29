package invalidity

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	emPlonk "github.com/consensys/gnark/std/recursion/plonk"
	"github.com/consensys/linea-monorepo/prover/circuits"
	wizardk "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/wizard"
	wizard "github.com/consensys/linea-monorepo/prover/protocol/wizard"
	public_input "github.com/consensys/linea-monorepo/prover/public-input"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/sirupsen/logrus"
)

type CircuitInvalidity struct {
	// The sub circuits for the invalidity cases:
	// - bad transaction nonce
	// - bad transaction value
	// ...
	SubCircuit SubCircuit `gnark:",secret"`
	// the functional public inputs of the circuit.
	FuncInputs FunctionalPublicInputsGnark `gnark:",secret"`
	// the hash of the functional public inputs
	PublicInput frontend.Variable `gnark:",public"`
}

// SubCircuit is the circuit for the invalidity case.
// It embeds frontend.Circuit (which provides Define) and adds FunctionalPIQGnark.
// Allocation and assignment are handled via concrete type methods.
type SubCircuit interface {
	frontend.Circuit
	FunctionalPIQGnark() FunctionalPIQGnark
}

// AssigningInputs collects the inputs used for the circuit assignment
type AssigningInputs struct {
	AccountTrieInputs AccountTrieInputs
	Transaction       *types.Transaction
	FuncInputs        public_input.Invalidity
	InvalidityType    InvalidityType
	FromAddress       common.Address
	RlpEncodedTx      []byte // the RLP encoded of the unsigned transaction
	KeccakCompiledIOP *wizardk.CompiledIOP
	KeccakProof       wizardk.Proof
	MaxRlpByteSize    int

	// inputs related to zkevm-wizard
	ZkEvmComp        *wizard.CompiledIOP
	ZkEvmWizardProof wizard.Proof

	// CachedProofPath, if set, enables proof caching: on first run the proof
	// is written to this path; on subsequent runs with the same witness the
	// cached proof is loaded and verified instead of re-proving.
	CachedProofPath string
}

// Define the constraints
func (c *CircuitInvalidity) Define(api frontend.API) error {
	// subCircuit constraints
	c.SubCircuit.Define(api)

	// Use the subcircuit's functional public inputs directly.
	// TxHash, FromAddress, StateRootHash, ToAddress come from the subcircuit via gnark:"-" fields;
	// TxNumber, ExpectedBlockNumber, FtxRollingHash are outer-circuit witness fields.
	c.FuncInputs.FunctionalPIQGnark = c.SubCircuit.FunctionalPIQGnark()

	// Range-check the functional public inputs to their expected bit widths.
	c.FuncInputs.RangeCheck(api)

	//  constraint on the hashing of functional public inputs
	api.AssertIsEqual(c.PublicInput, c.FuncInputs.Sum(api))

	return nil
}

// Allocate the circuit
func (c *CircuitInvalidity) Allocate(config Config) {
	switch sub := c.SubCircuit.(type) {
	case *BadNonceBalanceCircuit:
		sub.Allocate(config)
	case *FilteredAddressCircuit:
		sub.Allocate(config)
	case *BadPrecompileCircuit:
		sub.Allocate(config)
	default:
		panic(fmt.Sprintf("unsupported subcircuit type: %T", c.SubCircuit))
	}
}

// Assign the circuit
func (c *CircuitInvalidity) Assign(assi AssigningInputs) {
	switch sub := c.SubCircuit.(type) {
	case *BadNonceBalanceCircuit:
		sub.Assign(assi)
	case *FilteredAddressCircuit:
		sub.Assign(assi)
	case *BadPrecompileCircuit:
		sub.Assign(assi)
	default:
		panic(fmt.Sprintf("unsupported subcircuit type: %T", c.SubCircuit))
	}
	// assign the Functional Public Inputs
	c.FuncInputs.Assign(assi.FuncInputs)
	// assign the public input
	c.PublicInput = assi.FuncInputs.Sum(nil)
}

// MakeProof and solve the circuit.
func (c *CircuitInvalidity) MakeProof(
	setup circuits.Setup,
	assi AssigningInputs,
) string {

	switch assi.InvalidityType {
	case BadNonce, BadBalance:
		c.SubCircuit = &BadNonceBalanceCircuit{}
	case BadPrecompile, TooManyLogs:
		c.SubCircuit = &BadPrecompileCircuit{}
	case FilteredAddressFrom, FilteredAddressTo:
		c.SubCircuit = &FilteredAddressCircuit{}
	default:
		panic("unsupported invalidity type")
	}

	c.Assign(assi)

	opts := []any{
		emPlonk.GetNativeProverOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
		emPlonk.GetNativeVerifierOptions(ecc.BW6_761.ScalarField(), ecc.BLS12_377.ScalarField()),
	}
	if assi.CachedProofPath != "" {
		opts = append(opts, circuits.WithCachedProof(assi.CachedProofPath))
	}

	proof, err := circuits.ProveCheck(
		&setup,
		c,
		opts...,
	)

	if err != nil {
		panic(err)
	}

	// Do not read c.PublicInput after ProveCheck: gnark may replace it with []uint8 in the assignment.
	serialized := circuits.SerializeProofRaw(proof)
	proofByteLen := (len(serialized) - 2) / 2 // hexutil.Encode: "0x" + 2 hex chars per byte
	if proofByteLen < 0 {
		proofByteLen = 0
	}
	logrus.Infof("generated invalidity circuit proof (%d bytes) for public input 0x%x", proofByteLen, assi.FuncInputs.Sum(nil))

	return serialized
}

// CheckOnly verifies the constraint system is satisfied without generating a
// real proof. Used in partial mode.
//
// All types use native Go checks, avoiding the expensive frontend.Compile:
//   - BadPrecompile/TooManyLogs: wizard constraints are validated by the dummy
//     compiler during ProveInner/VerifyInner; outer PI extraction + type
//     assertions are checked natively.
//   - BadNonce/BadBalance: keccak wizard is validated by dummy compiler; RLP
//     parsing, account trie membership, and nonce/balance assertions are
//     checked natively.
//   - FilteredAddressFrom/FilteredAddressTo: keccak wizard is validated by
//     dummy compiler; RLP to-address extraction and filtered flag assertions
//     are checked natively.
func (c *CircuitInvalidity) CheckOnly(assi AssigningInputs) error {
	t0 := time.Now()
	var err error

	switch assi.InvalidityType {
	case BadPrecompile, TooManyLogs:
		if assi.ZkEvmComp == nil {
			return fmt.Errorf("ZkEvmComp is nil for %s", assi.InvalidityType)
		}
		err = CheckOnlyNativeBadPrecompile(assi.ZkEvmComp, assi.ZkEvmWizardProof, assi.FuncInputs, assi.InvalidityType)

	case BadNonce, BadBalance:
		err = CheckOnlyNativeNonceBalance(assi)

	case FilteredAddressFrom, FilteredAddressTo:
		err = CheckOnlyNativeFilteredAddress(assi)

	default:
		return fmt.Errorf("unsupported invalidity type: %d", assi.InvalidityType)
	}

	logrus.Infof("invalidity CheckOnly (type=%s): native check done in %s", assi.InvalidityType, time.Since(t0))
	return err
}

// Config collects the data used for the sub circuits allocation
type Config struct {
	// depth of the merkle tree for the account trie
	Depth             int
	KeccakCompiledIOP *wizardk.CompiledIOP
	MaxRlpByteSize    int
	ZkEvmComp         *wizard.CompiledIOP
}

type builder struct {
	config     Config
	subCircuit SubCircuit
}

func NewBuilder(config Config, subCircuit SubCircuit) *builder {
	return &builder{config: config,
		subCircuit: subCircuit,
	}
}

func (b *builder) Compile() (constraint.ConstraintSystem, error) {
	circuit := &CircuitInvalidity{
		SubCircuit: b.subCircuit,
	}
	return makeCS(b.config, circuit), nil
}

type limitlessBuilder struct {
	congWIOP *wizard.CompiledIOP
}

func NewBuilderLimitless(congWIOP *wizard.CompiledIOP) *limitlessBuilder {
	return &limitlessBuilder{congWIOP: congWIOP}
}

func (b *limitlessBuilder) Compile() (constraint.ConstraintSystem, error) {
	circuit := &CircuitInvalidity{
		SubCircuit: &BadPrecompileCircuit{
			ExecutionCtx: ExecutionCtx{LimitlessMode: true},
		},
	}
	circuit.Allocate(Config{ZkEvmComp: b.congWIOP})

	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit, frontend.WithCapacity(1<<24))
	if err != nil {
		return nil, fmt.Errorf("failed to compile limitless invalidity circuit: %w", err)
	}
	return ccs, nil
}

// compile  the circuit to the constraints
func makeCS(config Config, circuit *CircuitInvalidity) constraint.ConstraintSystem {

	circuit.Allocate(config)

	scs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, circuit, frontend.WithCapacity(1<<24))
	if err != nil {
		panic(err)
	}
	return scs
}

type InvalidityType uint8

const (
	BadNonce            InvalidityType = 0
	BadBalance          InvalidityType = 1
	BadPrecompile       InvalidityType = 2
	TooManyLogs         InvalidityType = 3
	FilteredAddressFrom InvalidityType = 4
	FilteredAddressTo   InvalidityType = 5
)

// String returns the string representation of the InvalidityType
func (t InvalidityType) String() string {
	switch t {
	case BadNonce:
		return "BadNonce"
	case BadBalance:
		return "BadBalance"
	case BadPrecompile:
		return "BadPrecompile"
	case TooManyLogs:
		return "TooManyLogs"
	case FilteredAddressFrom:
		return "FilteredAddressFrom"
	case FilteredAddressTo:
		return "FilteredAddressTo"
	default:
		return "Unknown"
	}
}

// MarshalJSON converts InvalidityType to its string representation for JSON
func (t InvalidityType) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.String())
}

// UnmarshalJSON converts a JSON string to InvalidityType
func (t *InvalidityType) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("InvalidityType must be a string: %w", err)
	}
	switch s {
	case "BadNonce":
		*t = BadNonce
	case "BadBalance":
		*t = BadBalance
	case "BadPrecompile":
		*t = BadPrecompile
	case "TooManyLogs":
		*t = TooManyLogs
	case "FilteredAddressFrom":
		*t = FilteredAddressFrom
	case "FilteredAddressTo":
		*t = FilteredAddressTo
	default:
		return fmt.Errorf("unknown InvalidityType: %s", s)
	}
	return nil
}
