package zkevm

import (
	"fmt"
	"math/rand/v2"
	"os"
	"path"
	"strings"

	"github.com/consensys/linea-monorepo/prover/config"
	multisethashing "github.com/consensys/linea-monorepo/prover/crypto/multisethashing_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed"
	"github.com/consensys/linea-monorepo/prover/protocol/serde"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/publicInput"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	TinyStuffsModuleName  = "TINY-STUFFS"
	ArithOpsModuleName    = "ARITH-OPS"
	HubAModuleName        = "HUB-A"
	HubBModuleName        = "HUB-B"
	KeccakModuleName      = "KECCAK"
	StaticModuleName      = "STATIC"
	Modexp256ModuleName   = "MODEXP-256"
	ModexpLargeModuleName = "MODEXP_LARGE"
	Sha2ModuleName        = "SHA2"
	EcdsaModuleName       = "ECDSA"
	P256ModuleName        = "P256"
	BlsG1ModuleName       = "BLS-G1"
	BlsG2ModuleName       = "BLS-G2"
	BlsPairingModuleName  = "BLS-PAIRING"
	BlsKzgModuleName      = "BLS-KZG"
	BnEcOpsModuleName     = "BN-EC-OPS"
	BnPairingModuleName   = "BN-PAIRING"
	BnG2CheckModuleName   = "BN-G2-CHECK"
)

var (
	bootstrapperFile              = "dw-bootstrapper.bin"
	discFile                      = "disc.bin"
	zkevmFile                     = "zkevm-wiop.bin"
	blueprintGLPrefix             = "dw-blueprint-gl"
	blueprintLppPrefix            = "dw-blueprint-lpp"
	blueprintGLTemplate           = blueprintGLPrefix + "-%d.bin"
	blueprintLppTemplate          = blueprintLppPrefix + "-%d.bin"
	compileLppTemplate            = "dw-compiled-lpp-%v.bin"
	compileGlTemplate             = "dw-compiled-gl-%v.bin"
	debugLppTemplate              = "dw-debug-lpp-%v.bin"
	debugGlTemplate               = "dw-debug-gl-%v.bin"
	conglomerationFile            = "dw-compiled-conglomeration.bin"
	executionLimitlessPath        = "execution-limitless"
	verificationKeyMerkleTreeFile = "verification-key-merkle-tree.bin"
)

var LimitlessCompilationParams = distributed.CompilationParams{
	// Increased from 1<<24 to 1<<25 because HUB-A-GL produces ~22.8M
	// constraints in its recursion circuit, which exceeds 2^24 = 16.7M.
	// DomainSizePlonk = nextPowerOf2(22877653 + 12346) = 2^25, so 2^25 rows
	// are the minimum that can accommodate HUB-A. This value is global because
	// all segments must share the same Plonk verifier structure for the
	// conglomeration circuit.
	FixedNbRowPlonkCircuit:   1 << 25,
	FixedNbRowExternalHasher: 1 << 19, // Increased from 1<<22 to handle hash claims
	FixedNbPublicInput:       1 << 10,
	InitialCompilerSize:      1 << 18,
	InitialCompilerSizeOverride: map[string]int{
		HubAModuleName + "-GL":  1 << 17,
		HubBModuleName + "-GL":  1 << 17,
		HubAModuleName + "-LPP": 1 << 17,
		HubBModuleName + "-LPP": 1 << 17,
	},
	InitialCompilerSizeConglo:    1 << 18,
	ColumnProfileMPTS:            []int{264, 1400, 256, 24, 12, 28, 8, 8},
	ColumnProfileMPTSPrecomputed: 45,
	FullDebugMode:                false,
}

// GetTestZkEVM returns a ZkEVM object configured for testing.
func GetTestZkEVM() *ZkEvm {
	return FullZKEVMWithSuite(
		config.GetTestTracesLimits(),
		&config.Config{
			Execution: config.Execution{
				IgnoreCompatibilityCheck: true,
			},
		},
		CompilationSuite{},
		nil,
	)
}

// LimitlessZkEVM defines the wizard responsible for proving execution of the EVM
// and the associated wizard circuits for the limitless prover protocol.
type LimitlessZkEVM struct {
	Zkevm      *ZkEvm
	DistWizard *distributed.DistributedWizard
}

// DiscoveryAdvices returns a list of advice for the discovery of the modules. These
// values have been obtained thanks to a statistical analysis of the traces
// assignments involving correlation of the modules and hierarchical clustering.
// The advices are optimized to minimize the number of segments generated when
// producing an EVM proof.
func DiscoveryAdvices(zkevm *ZkEvm) []*distributed.ModuleDiscoveryAdvice {

	return []*distributed.ModuleDiscoveryAdvice{

		// ARITH-OPS
		//
		{BaseSize: 16384, Cluster: ArithOpsModuleName, Regexp: `^exp\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^bit_(sar|shr|ror|shl|xoan)[0-9]+(_u[0-9]+)?\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^byte_(sar|shr|ror|shl|xoan)[0-9]+(_u[0-9]+)?\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^byte_slice_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^byte_size_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^fill_bytes_between\.`},
		// BaseSize set to fit P75 module heights in 1 segment (reduces segment count).
		{BaseSize: 2097152, Cluster: ArithOpsModuleName, Regexp: `^u32\.`},
		{BaseSize: 2097152, Cluster: ArithOpsModuleName, Regexp: `^u36\.`},
		{BaseSize: 2097152, Cluster: ArithOpsModuleName, Regexp: `^u64\.`},
		{BaseSize: 1048576, Cluster: ArithOpsModuleName, Regexp: `^u128\.`},
		{BaseSize: 65536, Cluster: ArithOpsModuleName, Regexp: `^u113\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^log[0-9]+(_u[0-9]+)?\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^set_byte[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^bit_xoan_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^mul\.`},
		{BaseSize: 65536, Cluster: ArithOpsModuleName, Regexp: `^add\.`},
		{BaseSize: 65536, Cluster: ArithOpsModuleName, Regexp: `^mod\.`},
		{BaseSize: 65536, Cluster: ArithOpsModuleName, Regexp: `^min256_64\.`},
		{BaseSize: 131072, Cluster: ArithOpsModuleName, Regexp: `^shf\.`},
		{BaseSize: 131072, Cluster: ArithOpsModuleName, Regexp: `^bin\.`},
		{BaseSize: 1048576, Cluster: ArithOpsModuleName, ModuleRef: "POSEIDON2_COMPILER"},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^byte[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^signextend\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^max3_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^maxlog\.`},
		{BaseSize: 262144, Cluster: ArithOpsModuleName, Regexp: `^wcp\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^counts_nz_[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^divide\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^max_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^negate\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^pow\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^signed_divide\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^xor_on_xor\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^zero_check\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^abs\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^byte_size\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^rpad_[0-9]+_[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^switch_endian_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^switch_endian_8_args\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^cap32\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^ceil_div\.`},
		{BaseSize: 65536, Cluster: ArithOpsModuleName, Regexp: `^euc\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^limb_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^modulus_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^modulus_u[0-9]_u[0-9]+\.`},
		{BaseSize: 32768, Cluster: ArithOpsModuleName, Regexp: `^modulus_u512_u256\.`},

		// Hub
		//
		{BaseSize: 262144, Cluster: HubAModuleName, Regexp: `^hub\.`},
		{BaseSize: 1048576, Cluster: HubAModuleName, Regexp: `^hub×4\.`},
		{BaseSize: 131072, Cluster: HubBModuleName, Regexp: `^mxp\.`},
		{BaseSize: 131072, Cluster: HubBModuleName, Regexp: `^oob\.`},
		{BaseSize: 262144, Cluster: HubBModuleName, Regexp: `^mmio\.`},
		{BaseSize: 1048576, Cluster: HubBModuleName, Regexp: `^mmio×3\.`},
		{BaseSize: 262144, Cluster: HubBModuleName, Regexp: `^mmu\.`},
		{BaseSize: 65536, Cluster: HubBModuleName, Regexp: `^stp\.`},
		{BaseSize: 65536, Cluster: HubBModuleName, Regexp: `^gas\.`},
		{BaseSize: 65536, Cluster: HubBModuleName, Regexp: `^gas_out_of_pocket\.`},
		{BaseSize: 65536, Cluster: HubBModuleName, Regexp: `^call_gas_extra\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^oob_prc_pricing\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^oob_prc\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^jump_target_check\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^oob_gas_cost\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^oob_cds_valid\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^out_of_bounds_check\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^oob_bytecodes\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^oob_check\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^rpad_[0-9]+\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^abort_check\.`},
		{BaseSize: 16384, Cluster: HubBModuleName, Regexp: `^get_ms\.`},

		// Keccak
		//
		{BaseSize: 524288, Cluster: KeccakModuleName, Regexp: `^rom\.`},
		{BaseSize: 32768, Cluster: KeccakModuleName, Regexp: `^rlptxn\.`},
		{BaseSize: 65536, Cluster: KeccakModuleName, Regexp: `^shakiradata\.`},
		{BaseSize: 32768, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.KeccakOverBlocks.Blocks.IsBlock},
		{BaseSize: 16384, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_accInfo.Provider.IsHashHi},
		{BaseSize: 32768, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.KeccakOverBlocks.Outputs.HashBytes[0]},
		{BaseSize: 131072, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_accData.IsActive},
		// BaseSize increased from 262144 to 8388608 to fit ~8.4M rows in 1 segment.
		{BaseSize: 8388608, Cluster: KeccakModuleName, Column: zkevm.StateManager.LineaCodeHash.CodeSize[0]},
		{BaseSize: 262144, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Packing.Repacked.Lanes},
		{BaseSize: 262144, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Packing.Block.AccNumLane},
		{BaseSize: 32768, Cluster: KeccakModuleName, Column: zkevm.StateManager.Accumulator.Cols.IsActiveAccumulator},
		{BaseSize: 32768, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.ImportPad.IsPadded},
		{BaseSize: 131072, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Packing.Repacked.Inputs.Spaghetti.FilterSpaghetti},
		{BaseSize: 131072, Cluster: KeccakModuleName, Column: zkevm.Keccak.Pa_keccak.Packing.Repacked.Inputs.Spaghetti.PA.ContentSpaghetti[0]},
		{BaseSize: 32768, Cluster: KeccakModuleName, Regexp: `^keccak\.`},

		// MODEXP 256
		//
		{BaseSize: 65536, Cluster: Modexp256ModuleName, Regexp: `^blake2fmodexpdata\.`},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Column: zkevm.Modexp.Small.IsActive},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Regexp: `^oob_modexp`},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Regexp: `^oob_prc_blake`},
		{BaseSize: 8192, Cluster: Modexp256ModuleName, Regexp: `^blake2f`},

		// MODEXP 8192
		//
		{BaseSize: 256, Cluster: ModexpLargeModuleName, Column: zkevm.Modexp.Large.IsActive},

		// SHA2
		//
		{BaseSize: 512, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_cSha2.GnarkCircuitConnector.IsActive},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_packing.Repacked.Inputs.Spaghetti.CleanLimbSp},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_packing.Repacked.Inputs.Spaghetti.PA.TagSpaghetti},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_packing.Block.AccNumLane},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_cSha2.Hash[0]},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_importPad.Index},
		{BaseSize: 16384, Cluster: Sha2ModuleName, Column: zkevm.Sha2.Pa_packing.Repacked.IsLaneActive},

		// TINY-STUFFS
		//
		{BaseSize: 1, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.ExecDataSchwarzZipfelX},
		{BaseSize: 512, Cluster: TinyStuffsModuleName, Regexp: `^romlex\.`},
		{BaseSize: 512, Cluster: TinyStuffsModuleName, Column: zkevm.StateManager.CodeHashConsistency.RomKeccak.Hi[0]},
		{BaseSize: 2048, Cluster: TinyStuffsModuleName, Regexp: `^loginfo\.`},
		{BaseSize: 2048, Cluster: TinyStuffsModuleName, Regexp: `^trm\.`},
		// BaseSize increased from 2048 to 4096 to fit ~2.1K rows in 1 segment.
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Regexp: `^blockhash\.`},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Regexp: `^logdata\.`},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Regexp: `^rlpaddr\.`},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Regexp: `^blockdata\.`},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.BlockDataFetcher.LastTimestamp[0]},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.FetchedL2L1.Data[0]},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.FetchedRollingHash.Data[0]},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.FetchedRollingMsg.Data[0]},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.RollingHashFetcher.ExistsMsg},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.BlockTxnMetadata.BlockID},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.TxnDataFetcher.AbsTxNum},
		{BaseSize: 16384, Cluster: TinyStuffsModuleName, Column: zkevm.StateManager.StateSummary.WorldStateRoot[0]},
		{BaseSize: 32768, Cluster: TinyStuffsModuleName, Regexp: `^rlptxrcpt\.`},
		{BaseSize: 16384, Cluster: TinyStuffsModuleName, Regexp: `^rlpauth\.`},
		{BaseSize: 32768, Cluster: TinyStuffsModuleName, Regexp: `^rlputils\.`},
		{BaseSize: 32768, Cluster: TinyStuffsModuleName, Regexp: `^compute_rlp_integer_u256\.`},
		{BaseSize: 32768, Cluster: TinyStuffsModuleName, Regexp: `^compute_rlp\.`},
		{BaseSize: 65536, Cluster: TinyStuffsModuleName, Regexp: `^txndata\.`},
		{BaseSize: 131072, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.RlpTxnFetcher.NBytes},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.ExecDataCollector.AbsTxID},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.PadderPacker.CounterColumnPadded},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.PadderPacker.OneColumn},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.Aux.PadderPacker.SplitOuter[0]},
		{BaseSize: 262144, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.ExecPoseidonHasher.Hash[0]},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.ChainIDFetcher.NBytesChainID},
		{BaseSize: 4096, Cluster: TinyStuffsModuleName, Column: zkevm.PublicInput.L2L1LogCompacter.CompactifiedSelector},
		distributed.SameSizeAdvice(TinyStuffsModuleName, zkevm.PublicInput.ExecDataSchwarzZipfelEval.Pol),

		// ECDSA
		// BaseSize increased from 16384 to 32768 for gnark columns to fit ~25K rows in 1 segment.
		//
		{BaseSize: 65536, Cluster: EcdsaModuleName, Regexp: `^ext\.`},
		{BaseSize: 32768, Cluster: EcdsaModuleName, Column: zkevm.Ecdsa.Ant.AlignedGnarkData.CircuitInput},
		{BaseSize: 32768, Cluster: EcdsaModuleName, Column: zkevm.Ecdsa.Ant.Addresses.IsAddress},
		{BaseSize: 32768, Cluster: EcdsaModuleName, Column: zkevm.Ecdsa.Ant.FlattenLimbs.Limbs},
		// TODO: remove this advice after fixing  [common.CsFlattenProjection]; a dummy module for the orphan column AuxProjectionMask
		// BaseSize must match the precomputed column size (NbLimbsCols * originalSize = 65536) to avoid multi-segment splits.
		{BaseSize: 65536, Cluster: EcdsaModuleName, Column: zkevm.Ecdsa.Ant.FlattenLimbs.AuxProjectionMask},
		{BaseSize: 32768, Cluster: EcdsaModuleName, Regexp: `ecrecover\.`},

		// P256
		//
		{BaseSize: 4096, Cluster: P256ModuleName, Column: zkevm.P256Verify.P256VerifyGnarkData.CircuitInput},

		// ELLIPTIC CURVES
		//
		// The blsdata FLATTEN_LIMBS column (blsdata.LIMB'0_FLATTEN_LIMBS) has
		// ~1M rows (NextPowerOfTwo(blsdata_size * 8)). It shares a QBM with
		// MANUALLY_SHIFTED_FLATTEN_LIMBS columns (via ManuallyShift global
		// constraints). With BaseSize=512 (from the generic ^blsdata\. catch-all),
		// this produced 2046 segments. This specific regex must appear BEFORE
		// the generic ^blsdata\. to override it with a large BaseSize.
		// BaseSize increased from 131072 to 1048576 to fit ~1M rows in 1 segment.
		{BaseSize: 1048576, Cluster: BnEcOpsModuleName, Regexp: `^blsdata\..*FLATTEN`},
		{BaseSize: 512, Cluster: BnEcOpsModuleName, Regexp: `^blsdata\.`},
		// BaseSize must be 1048576 to match the precomputed PROJECTION_MASK column size (1048576 rows).
		{BaseSize: 1048576, Cluster: BnEcOpsModuleName, Regexp: `^ecdata\..*FLATTEN`},
		{BaseSize: 4096, Cluster: BnEcOpsModuleName, Regexp: `^ecdata\.`},
		{BaseSize: 4096, Cluster: BnEcOpsModuleName, Column: zkevm.Ecadd.AlignedGnarkData.IsActive},
		// Ecadd/Ecmul FlattenLimbs: both share the same Limbs column
		// (ecdata.LIMB'0_FLATTEN_LIMBS) because initColumns deduplicates by
		// column ID. This advice covers both. The FlattenLimbs column and its
		// ManuallyShifted derivatives are in a separate QBM from AlignedGnarkData.
		{BaseSize: 4096, Cluster: BnEcOpsModuleName, Column: zkevm.Ecadd.FlattenLimbs.Limbs},
		{BaseSize: 512, Cluster: BnEcOpsModuleName, Column: zkevm.Ecmul.AlignedGnarkData.IsActive},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g1\.`},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g1_discount\.`},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g1g2\.`},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g2\.`},
		{BaseSize: 1024, Cluster: BnEcOpsModuleName, Regexp: `^g2_discount\.`},

		// ECPAIRING
		//
		{BaseSize: 1024, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.IsActive},
		{BaseSize: 1024, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.AlignedMillerLoopCircuit.IsActive},
		{BaseSize: 1024, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.AlignedFinalExpCircuit.IsActive},
		{BaseSize: 1024, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.FlattenLimbsMillerLoop.Limbs},
		// BaseSize must match the precomputed PROJECTION_MASK column size (65536 rows) to avoid 64-segment splits.
		{BaseSize: 65536, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.FlattenLimbsMillerLoop.AuxProjectionMask},
		{BaseSize: 1024, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.FlattenLimbsG2Membership.Limbs},
		// BaseSize must match the precomputed PROJECTION_MASK column size (65536 rows) to avoid 64-segment splits.
		{BaseSize: 65536, Cluster: BnPairingModuleName, Column: zkevm.Ecpair.FlattenLimbsG2Membership.AuxProjectionMask},

		// G2_CHECK
		//
		{BaseSize: 1024, Cluster: BnG2CheckModuleName, Column: zkevm.Ecpair.AlignedG2MembershipData.IsActive},

		// BLS_G1
		//
		{BaseSize: 4096, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Msm.UnalignedMsmData.CurrentAccumulator[0]},
		{BaseSize: 4096, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Msm.GnarkDataMsm},
		{BaseSize: 1024, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Msm.AlignedGnarkMsmData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Map.AlignedGnarkData.CircuitInput},
		{BaseSize: 4096, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Add.AlignedAddGnarkData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Add.AlignedCurveMembershipGnarkData.CircuitInput},
		{BaseSize: 1024, Cluster: BlsG1ModuleName, Column: zkevm.BlsG1Msm.AlignedGnarkGroupMembershipData.CircuitInput},

		// BLS_G2
		// BaseSize increased from 1024 to 2048 for GnarkDataMsm to fit 1200 rows in 1 segment.
		//
		{BaseSize: 4096, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Msm.UnalignedMsmData.CurrentAccumulator[0]},
		{BaseSize: 2048, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Add.AlignedCurveMembershipGnarkData.CircuitInput},
		{BaseSize: 4096, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Msm.AlignedGnarkMsmData.CircuitInput},
		{BaseSize: 2048, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Msm.AlignedGnarkGroupMembershipData.CircuitInput},
		{BaseSize: 2048, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Map.AlignedGnarkData.CircuitInput},
		{BaseSize: 8192, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Add.AlignedAddGnarkData.CircuitInput},
		{BaseSize: 2048, Cluster: BlsG2ModuleName, Column: zkevm.BlsG2Msm.GnarkDataMsm},

		// BLS POINT EVAL
		//
		{BaseSize: 128, Cluster: BlsKzgModuleName, Column: zkevm.PointEval.AlignedGnarkData.CircuitInput},
		{BaseSize: 128, Cluster: BlsKzgModuleName, Column: zkevm.PointEval.AlignedFailureGnarkData.CircuitInput},

		// BLS PAIR
		// BaseSize increased from 1024 to 2048 to fit ~1600 rows in 1 segment.
		//
		{BaseSize: 2048, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.CsG1Membership},
		{BaseSize: 2048, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.AlignedG1MembershipGnarkData.CircuitInput},
		{BaseSize: 2048, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.AlignedG2MembershipGnarkData.CircuitInput},
		{BaseSize: 2048, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.AlignedMillerLoopData.CircuitInput},
		{BaseSize: 2048, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.AlignedFinalExpData.CircuitInput},
		{BaseSize: 4096, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.UnalignedPairData.IsActive},
		{BaseSize: 2048, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.UnalignedPairData.GnarkDataMillerLoop},
		{BaseSize: 2048, Cluster: BlsPairingModuleName, Column: zkevm.BlsPairingCheck.UnalignedPairData.GnarkIsActiveFinalExp},

		// STATIC
		//
		{BaseSize: 16, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_16$`},
		{BaseSize: 32, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_30$`},
		{BaseSize: 128, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_72$`},
		{BaseSize: 256, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_136`},
		{BaseSize: 256, Cluster: StaticModuleName, Regexp: `^LOOKUP_TABLE_RANGE_1_144`},
		{BaseSize: 32, Cluster: StaticModuleName, Regexp: `^power\.`},
		{BaseSize: 512, Cluster: StaticModuleName, Regexp: `^instdecoder\.`},
		{BaseSize: 512, Cluster: StaticModuleName, Regexp: `^blsreftable\.`},
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.Packing.Decomposed.Inputs.Lookup.ColNumber),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.Theta.LookupTable[0]),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.BackToThetaOrOutput.LookupTable.ColBase2),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.BackToThetaOrOutput.LookupTable.ColBaseChi),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.BackToThetaOrOutput.LookupTable.ColBaseTheta),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.Blocks.Bc.Lookup.ColMAXNBYTE),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecdsa.Ant.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[0].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[1].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[2].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[3].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[4].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[5].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[6].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[7].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[8].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[9].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[10].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[11].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[12].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[13].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[14].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.StateManager.Accumulator.OffsetLimbRepeated[15].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.Blocks.ColRound.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.ChiIota.Rc[0].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.ChiIota.Rc[1].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.ChiIota.Rc[2].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.ChiIota.Rc[3].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.ChiIota.Rc[4].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.ChiIota.Rc[5].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.ChiIota.Rc[6].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Keccak.Pa_keccak.KeccakOverBlocks.KeccakF.ChiIota.Rc[7].PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecadd.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecmul.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecpair.AlignedG2MembershipData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecpair.AlignedFinalExpCircuit.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Ecpair.AlignedMillerLoopCircuit.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Sha2.Pa_cSha2.GnarkCircuitConnector.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.Sha2.Pa_cSha2.CanBeBlockOfInstance.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Add.AlignedAddGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Add.AlignedCurveMembershipGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Msm.AlignedGnarkGroupMembershipData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Msm.AlignedGnarkMsmData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG1Map.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Add.AlignedAddGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Add.AlignedCurveMembershipGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Msm.AlignedGnarkGroupMembershipData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Msm.AlignedGnarkMsmData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsG2Map.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsPairingCheck.AlignedG2MembershipGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsPairingCheck.AlignedMillerLoopData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsPairingCheck.AlignedFinalExpData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.BlsPairingCheck.AlignedG1MembershipGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.PointEval.AlignedFailureGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.PointEval.AlignedGnarkData.ActualCircuitInputMask.PatternPrecomp),
		distributed.SameSizeAdvice(StaticModuleName, zkevm.P256Verify.P256VerifyGnarkData.ActualCircuitInputMask.PatternPrecomp),
	}
}

// NewLimitlessZkEVM returns a new LimitlessZkEVM object.
func NewLimitlessZkEVM(cfg *config.Config) *LimitlessZkEVM {
	var (
		traceLimits = cfg.TracesLimits
		zkevm       = FullZKEVMWithSuite(&traceLimits, cfg, CompilationSuite{}, nil)
		disc        = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 28,
			Advices:      DiscoveryAdvices(zkevm),
		}
		dw = distributed.DistributeWizard(zkevm.InitialCompiledIOP, disc)
	)

	// These are the slow and expensive operations.
	dw.CompileSegments(LimitlessCompilationParams).Conglomerate(LimitlessCompilationParams)

	// This is needed because the outer-circuit will expect the conglomeration
	// wizard to provide the public-inputs metadata.
	dw.CompiledConglomeration.RecursionCompBLS.
		ExtraData[publicInput.PublicInputExtractorMetadata] = zkevm.
		InitialCompiledIOP.ExtraData[publicInput.PublicInputExtractorMetadata]

	return &LimitlessZkEVM{
		Zkevm:      zkevm,
		DistWizard: dw,
	}
}

// NewLimitlessRawZkEVM returns a new LimitlessZkEVM object without any
// compilation.
func NewLimitlessRawZkEVM(cfg *config.Config) *LimitlessZkEVM {

	var (
		traceLimits = cfg.TracesLimits
		zkevm       = FullZKEVMWithSuite(&traceLimits, cfg, CompilationSuite{}, nil)
		disc        = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 29,
			Advices:      DiscoveryAdvices(zkevm),
		}
		dw = distributed.DistributeWizard(zkevm.InitialCompiledIOP, disc)
	)

	return &LimitlessZkEVM{
		Zkevm:      zkevm,
		DistWizard: dw,
	}
}

// NewLimitlessDebugZkEVM returns a new LimitlessZkEVM with only the debugging
// components. The resulting object is not meant to be stored on disk and should
// be used right away to debug the prover. The return object can run the
// bootstrapper (with added) sanity-checks, the segmentation and then sanity-
// checking all the segments.
func NewLimitlessDebugZkEVM(cfg *config.Config) *LimitlessZkEVM {

	var (
		traceLimits = cfg.TracesLimits
		zkevm       = FullZKEVMWithSuite(&traceLimits, cfg, CompilationSuite{}, nil)
		disc        = &distributed.StandardModuleDiscoverer{
			TargetWeight: 1 << 29,
			Advices:      DiscoveryAdvices(zkevm),
		}
		dw             = distributed.DistributeWizard(zkevm.InitialCompiledIOP, disc)
		limitlessZkEVM = &LimitlessZkEVM{
			Zkevm:      zkevm,
			DistWizard: dw,
		}
	)

	// This adds debugging to the bootstrapper which are normally not present by
	// default.
	wizard.ContinueCompilation(
		limitlessZkEVM.DistWizard.Bootstrapper,
		dummy.CompileAtProverLvl(dummy.WithMsg("bootstrapper")),
	)

	return limitlessZkEVM
}

// GetScaledUpBootstrapper returns a bootstrapper where all the limits have
// been increased.
func GetScaledUpBootstrapper(cfg *config.Config, disc *distributed.StandardModuleDiscoverer, scalingFactor int) (*wizard.CompiledIOP, *ZkEvm) {

	traceLimits := cfg.TracesLimits
	traceLimits.ScaleUp(scalingFactor)

	zkevm := FullZKEVMWithSuite(&traceLimits, cfg, CompilationSuite{}, nil)
	distributed.CompileManualShifter(zkevm.InitialCompiledIOP)
	return distributed.PrecompileInitialWizard(zkevm.InitialCompiledIOP, disc), zkevm
}

// RunStatRecords runs only the bootstrapper and returns a list of stat records
func (lz *LimitlessZkEVM) RunStatRecords(cfg *config.Config, witness *Witness) []distributed.QueryBasedAssignmentStatsRecord {

	var (
		runtimeBoot = runBootstrapperWithRescaling(
			cfg,
			lz.DistWizard.Bootstrapper,
			lz.Zkevm,
			lz.DistWizard.Disc,
			witness,
			true,
		)

		res  = []distributed.QueryBasedAssignmentStatsRecord{}
		disc = lz.DistWizard.Disc
	)

	for _, mod := range disc.Modules {
		res = append(res, mod.RecordAssignmentStats(runtimeBoot)...)
	}

	return res
}

// RunDebug runs the LimitlessZkEVM on debug mode. It will run the boostrapper,
// the segmentation and then the sanity checks for all the segments. The
// check of the LPP module is done using a deterministic pseudo-random number
// generator and will yield the same result every time.
func (lz *LimitlessZkEVM) RunDebug(cfg *config.Config, witness *Witness) {

	runtimeBoot := runBootstrapperWithRescaling(
		cfg,
		lz.DistWizard.Bootstrapper,
		lz.Zkevm,
		lz.DistWizard.Disc,
		witness,
		true,
	)

	witnessGLs, witnessLPPs := distributed.SegmentRuntime(
		runtimeBoot,
		lz.DistWizard.Disc,
		lz.DistWizard.BlueprintGLs,
		lz.DistWizard.BlueprintLPPs,
		// The verification key merkle tree does not exists in debug mode. So
		// we can get the value here. It is not needed anyway.
		field.Octuplet{},
	)

	logrus.Infof("Segmented %v GL segments and %v LPP segments", len(witnessGLs), len(witnessLPPs))

	type hornerSegInfo struct {
		Module       string
		SegmentIndex int
		N0, N1       int
		Count        int
		DataSize     int
		Contribution fext.Element
	}

	type perSegmentMSetInfo struct {
		ModuleName       distributed.ModuleName
		ModuleIndex      int
		SegmentIndex     int
		ProofType        string // "GL" or "LPP"
		MSetContribution multisethashing.MSetHash
	}

	var (
		allGrandProduct     = fext.One()
		allLogDerivativeSum = fext.Element{}
		allHornerSum        = fext.Element{}
		generalMSet         = multisethashing.MSetHash{}

		perPartSums = map[string]fext.GenericFieldElem{}

		perHornerPartContrib = map[string]fext.Element{}
		perHornerPartSegs    = map[string][]hornerSegInfo{}
		moduleSegmentCounter = map[distributed.ModuleName]int{}

		// Per-segment multiset contributions for detailed diagnostics
		perSegmentMSets []perSegmentMSetInfo
	)

	for i, witness := range witnessGLs {

		logrus.Infof("Checking GL witness %v, module=%v", i, witness.ModuleName)

		var (
			debugGL        = lz.DistWizard.DebugGLs[witness.ModuleIndex]
			mainProverStep = debugGL.GetMainProverStep(witness)
			compiledIOP    = debugGL.Wiop
		)

		// The debugGLs is compiled with the CompileAtProverLevel routine so we
		// don't need the proof to complete the sanity checks: everything is
		// done at the prover level.
		rt := wizard.RunProver(compiledIOP, mainProverStep, false)

		generalMSetFromGLFr := distributed.GetPublicInputList(rt, distributed.GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize)
		generalMSetFromGL := multisethashing.MSetHash(generalMSetFromGLFr)
		generalMSet.Add(generalMSetFromGL)

		// Collect per-segment multiset contribution for diagnostics
		perSegmentMSets = append(perSegmentMSets, perSegmentMSetInfo{
			ModuleName:       witness.ModuleName,
			ModuleIndex:      witness.ModuleIndex,
			SegmentIndex:     witness.SegmentModuleIndex,
			ProofType:        "GL",
			MSetContribution: generalMSetFromGL,
		})
	}

	// Here, we can't we can't just use 0 or a dummy small value because there
	// is a risk of creating false-positives with the grand-products and the
	// horner (as if one of the term of the product cancels, the product is
	// zero and we want to prevent that) or false negative due to inverting
	// zeroes in the log-derivative sums.
	// #nosec G404 --we don't need a cryptographic RNG for debugging purpose
	rng := rand.New(utils.NewRandSource(42))
	sharedRandomness := field.PseudoRandOctuplet(rng)

	for i, witness := range witnessLPPs {

		logrus.Infof("Checking LPP witness %v, module=%v", i, witness.ModuleName)

		var debugLPP *distributed.ModuleLPP

		for _, dl := range lz.DistWizard.DebugLPPs {
			if dl.ModuleName() == witness.ModuleName {
				debugLPP = dl
				break
			}
		}

		if debugLPP == nil {
			utils.Panic("debugLPP not found")
		}

		witness.InitialFiatShamirState = sharedRandomness

		var (
			mainProverStep = debugLPP.GetMainProverStep(witness)
			compiledIOP    = debugLPP.Wiop
		)

		// The debugLPP is compiled with the CompileAtProverLevel routine so we
		// don't need the proof to complete the sanity checks: everything is
		// done at the prover level.
		rt := wizard.RunProver(compiledIOP, mainProverStep, false)

		generalMSetFromLPPFr := distributed.GetPublicInputList(rt, distributed.GeneralMultiSetPublicInputBase, multisethashing.MSetHashSize)
		generalMSetFromLPP := multisethashing.MSetHash(generalMSetFromLPPFr)
		generalMSet.Add(generalMSetFromLPP)

		// Collect per-segment multiset contribution for diagnostics
		perSegmentMSets = append(perSegmentMSets, perSegmentMSetInfo{
			ModuleName:       witness.ModuleName,
			ModuleIndex:      witness.ModuleIndex,
			SegmentIndex:     witness.SegmentModuleIndex,
			ProofType:        "LPP",
			MSetContribution: generalMSetFromLPP,
		})

		logDerivativeSum := rt.GetPublicInput(distributed.LogDerivativeSumPublicInput).Ext
		grandProduct := rt.GetPublicInput(distributed.GrandProductPublicInput).Ext
		hornerSum := rt.GetPublicInput(distributed.HornerPublicInput).Ext

		logrus.Infof("LPP segment %v: log-derivative-sum=%v grand-product=%v horner-sum=%v",
			i, logDerivativeSum.String(), grandProduct.String(), hornerSum.String())

		allGrandProduct.Mul(&allGrandProduct, &grandProduct)
		allHornerSum.Add(&allHornerSum, &hornerSum)
		allLogDerivativeSum.Add(&allLogDerivativeSum, &logDerivativeSum)

		if debugLPP.LogDerivativeSum != nil {
			perParts, err := debugLPP.LogDerivativeSum.ComputePerPart(rt)
			if err != nil {
				logrus.Warnf("LPP segment %v: per-part computation error: %v", i, err)
			} else {
				for _, pp := range perParts {
					acc := perPartSums[pp.Name]
					acc.Add(&pp.Sum)
					perPartSums[pp.Name] = acc
				}
			}
		}

		if debugLPP.Horner != nil {
			hornerParams := rt.GetHornerParams(debugLPP.Horner.ID)
			ppResults := hornerParams.GetPerPartResults(rt, *debugLPP.Horner)
			segIdx := moduleSegmentCounter[witness.ModuleName]
			moduleSegmentCounter[witness.ModuleName] = segIdx + 1

			for _, pp := range ppResults {
				logrus.Infof("  Horner part %q: sign=%v count=%d N0=%d N1=%d dataSize=%d contribution=%v (module=%v seg=%d)",
					pp.Name, pp.SignNegative, pp.Count, pp.N0, pp.N1, pp.DataSize, pp.Contribution.String(),
					witness.ModuleName, segIdx)

				acc := perHornerPartContrib[pp.Name]
				acc.Add(&acc, &pp.Contribution)
				perHornerPartContrib[pp.Name] = acc

				perHornerPartSegs[pp.Name] = append(perHornerPartSegs[pp.Name], hornerSegInfo{
					Module:       string(witness.ModuleName),
					SegmentIndex: segIdx,
					N0:           pp.N0,
					N1:           pp.N1,
					Count:        pp.Count,
					DataSize:     pp.DataSize,
					Contribution: pp.Contribution,
				})
			}
		}
	}

	logrus.Infof("Checking accumulation cancellation invariants")

	if !allGrandProduct.IsOne() {
		utils.Panic("grand-product does not cancel: %v", allGrandProduct.String())
	}

	if !allHornerSum.IsZero() {
		logrus.Errorf("horner does not cancel: %v", allHornerSum.String())

		projSums := map[string]fext.Element{}
		projCounts := map[string][2]int{}
		for name, contrib := range perHornerPartContrib {
			projName := name
			side := "?"
			if strings.HasSuffix(name, "_A") {
				projName = name[:len(name)-2]
				side = "A"
			} else if strings.HasSuffix(name, "_B") {
				projName = name[:len(name)-2]
				side = "B"
			}
			_ = side

			acc := projSums[projName]
			acc.Add(&acc, &contrib)
			projSums[projName] = acc
		}

		logrus.Infof("=== Per-projection Horner breakdown ===")
		nonZero := 0
		for projName, sum := range projSums {
			if sum.IsZero() {
				continue
			}
			nonZero++
			logrus.Errorf("  NON-ZERO projection %q sum=%v", projName, sum.String())

			nameA := projName + "_A"
			nameB := projName + "_B"
			if segs, ok := perHornerPartSegs[nameA]; ok {
				totalCount := 0
				for _, seg := range segs {
					totalCount += seg.Count
					logrus.Infof("    side=A module=%v seg=%d N0=%d N1=%d count=%d dataSize=%d contribution=%v",
						seg.Module, seg.SegmentIndex, seg.N0, seg.N1, seg.Count, seg.DataSize, seg.Contribution.String())
				}
				logrus.Infof("    side=A totalSegments=%d totalCount=%d", len(segs), totalCount)
				projCounts[projName] = [2]int{totalCount, projCounts[projName][1]}
			}
			if segs, ok := perHornerPartSegs[nameB]; ok {
				totalCount := 0
				for _, seg := range segs {
					totalCount += seg.Count
					logrus.Infof("    side=B module=%v seg=%d N0=%d N1=%d count=%d dataSize=%d contribution=%v",
						seg.Module, seg.SegmentIndex, seg.N0, seg.N1, seg.Count, seg.DataSize, seg.Contribution.String())
				}
				logrus.Infof("    side=B totalSegments=%d totalCount=%d", len(segs), totalCount)
				c := projCounts[projName]
				c[1] = totalCount
				projCounts[projName] = c
			}

			c := projCounts[projName]
			if c[0] != c[1] {
				logrus.Errorf("    COUNT MISMATCH: A_total=%d B_total=%d", c[0], c[1])
			}
		}
		logrus.Infof("Found %d non-zero projections out of %d total", nonZero, len(projSums))

		utils.Panic("horner does not cancel: %v", allHornerSum.String())
	}

	if !allLogDerivativeSum.IsZero() {
		logrus.Errorf("log-derivative-sum does not cancel: %v", allLogDerivativeSum.String())

		perTableSums := map[string]fext.GenericFieldElem{}
		for name, sum := range perPartSums {
			tableName := name
			if idx := strings.LastIndex(name, "_T_"); idx >= 0 {
				tableName = name[:idx]
			} else if idx := strings.LastIndex(name, "_S_"); idx >= 0 {
				tableName = name[:idx]
			}
			acc := perTableSums[tableName]
			acc.Add(&sum)
			perTableSums[tableName] = acc
		}

		logrus.Infof("Per-table log-derivative breakdown (%d tables):", len(perTableSums))
		nonZeroCount := 0
		for tableName, sum := range perTableSums {
			if !sum.IsZero() {
				nonZeroCount++
				logrus.Errorf("  NON-ZERO table %q = %v", tableName, sum.String())
			}
		}
		logrus.Infof("Found %d non-zero tables out of %d total", nonZeroCount, len(perTableSums))

		utils.Panic("log-derivative-sum does not cancel: %v", allLogDerivativeSum.String())
	}

	if !generalMSet.IsEmpty() {
		logrus.Errorf("general multiset does not cancel")

		// Per-segment breakdown to identify which segments contribute non-zero residuals
		logrus.Infof("=== Per-segment general multiset breakdown ===")
		for _, info := range perSegmentMSets {
			if !info.MSetContribution.IsEmpty() {
				// Log the first few non-zero elements for diagnostics
				nonZeroIdx := -1
				var nonZeroVal field.Element
				for k := 0; k < multisethashing.MSetHashSize; k++ {
					if !info.MSetContribution[k].IsZero() {
						nonZeroIdx = k
						nonZeroVal = info.MSetContribution[k]
						break
					}
				}
				logrus.Errorf("  NON-ZERO %v module=%v (index=%d) segment=%d first_nonzero_at=%d val=%v",
					info.ProofType, info.ModuleName, info.ModuleIndex, info.SegmentIndex, nonZeroIdx, nonZeroVal.String())
			}
		}

		utils.Panic("general multiset does not cancel")
	}

	logrus.Infof("All accumulation cancellation invariants passed (horner,log-derivative-sum,general-multiset,grand-product)")

	// --- Conglomeration completion checks (mirrors checkLimitlessConglomerationCompletion) ---
	// These checks verify that the segment counts match the expected target for
	// each module, which is what the outer proof circuit enforces via
	// checkLimitlessConglomerationCompletion.
	logrus.Infof("Checking conglomeration completion invariants (segment counts)")

	numModules := len(lz.DistWizard.Disc.Modules)
	glSegmentCounts := make([]int, numModules)
	lppSegmentCounts := make([]int, numModules)

	for _, w := range witnessGLs {
		glSegmentCounts[w.ModuleIndex]++
	}
	for _, w := range witnessLPPs {
		lppSegmentCounts[w.ModuleIndex]++
	}

	// The target segment count is the same across all witnesses; take it from
	// the first GL witness (it always exists because there is at least one
	// module).
	var targetSegmentCounts []int
	if len(witnessGLs) > 0 {
		targetSegmentCounts = witnessGLs[0].TotalSegmentCount
	} else if len(witnessLPPs) > 0 {
		targetSegmentCounts = witnessLPPs[0].TotalSegmentCount
	}

	segCountMismatch := false
	for m := 0; m < numModules; m++ {
		target := 0
		if targetSegmentCounts != nil && m < len(targetSegmentCounts) {
			target = targetSegmentCounts[m]
		}
		moduleName := lz.DistWizard.Disc.Modules[m].ModuleName

		if glSegmentCounts[m] != target {
			logrus.Errorf("segment count mismatch for module %v (index=%d): GL count=%d, target=%d",
				moduleName, m, glSegmentCounts[m], target)
			segCountMismatch = true
		}
		if lppSegmentCounts[m] != target {
			logrus.Errorf("segment count mismatch for module %v (index=%d): LPP count=%d, target=%d",
				moduleName, m, lppSegmentCounts[m], target)
			segCountMismatch = true
		}

		logrus.Infof("module %v (index=%d): target=%d GL=%d LPP=%d",
			moduleName, m, target, glSegmentCounts[m], lppSegmentCounts[m])
	}

	if segCountMismatch {
		utils.Panic("conglomeration completion check failed: segment count mismatch detected (see errors above)")
	}

	logrus.Infof("All conglomeration completion invariants passed (segment-counts)")

	// --- LPP column data consistency check ---
	// In production, GL inserts hash(moduleIndex, segmentIndex, lppMerkleRoot)
	// into the general multiset, and LPP removes hash(moduleIndex,
	// segmentIndex, lppMerkleRoot). The lppMerkleRoot is the Vortex Merkle
	// root of round-0 columns. In both GL and LPP modules, round-0 columns
	// are the LPP columns. For the multiset to cancel, GL and LPP must have
	// identical LPP column data.
	//
	// This check verifies that the LPP column data extracted during
	// segmentation is identical between GL and LPP witnesses for each
	// (module, segment) pair. A mismatch here would cause the Vortex
	// Merkle roots to differ in production, resulting in a general multiset
	// cancellation failure.
	logrus.Infof("Checking LPP column data consistency between GL and LPP witnesses")

	// Build a lookup of LPP witnesses by (moduleIndex, segmentIndex)
	type moduleSegKey struct {
		ModuleIndex  int
		SegmentIndex int
	}
	lppWitnessMap := make(map[moduleSegKey]*distributed.ModuleWitnessLPP)
	for _, w := range witnessLPPs {
		lppWitnessMap[moduleSegKey{w.ModuleIndex, w.SegmentModuleIndex}] = w
	}

	lppColMismatch := false
	for i, glW := range witnessGLs {
		key := moduleSegKey{glW.ModuleIndex, glW.SegmentModuleIndex}
		lppW, ok := lppWitnessMap[key]
		if !ok {
			logrus.Errorf("GL witness %d (module=%v index=%d segment=%d) has no matching LPP witness",
				i, glW.ModuleName, glW.ModuleIndex, glW.SegmentModuleIndex)
			lppColMismatch = true
			continue
		}

		// Get LPP column IDs from the blueprint
		blueprintLPP := lz.DistWizard.BlueprintLPPs[glW.ModuleIndex]
		for _, colID := range blueprintLPP.LPPColumnSets {
			glData, glHas := glW.Columns[colID]
			lppData, lppHas := lppW.Columns[colID]

			if !glHas && !lppHas {
				continue
			}
			if glHas != lppHas {
				logrus.Errorf("LPP column %v: GL has=%v LPP has=%v (module=%v segment=%d)",
					colID, glHas, lppHas, glW.ModuleName, glW.SegmentModuleIndex)
				lppColMismatch = true
				continue
			}

			glVec := smartvectors.IntoRegVec(glData)
			lppVec := smartvectors.IntoRegVec(lppData)

			if len(glVec) != len(lppVec) {
				logrus.Errorf("LPP column %v length mismatch: GL=%d LPP=%d (module=%v segment=%d)",
					colID, len(glVec), len(lppVec), glW.ModuleName, glW.SegmentModuleIndex)
				lppColMismatch = true
				continue
			}

			for j := range glVec {
				if glVec[j] != lppVec[j] {
					logrus.Errorf("LPP column %v data mismatch at row %d: GL=%v LPP=%v (module=%v segment=%d)",
						colID, j, glVec[j].String(), lppVec[j].String(), glW.ModuleName, glW.SegmentModuleIndex)
					lppColMismatch = true
					break // one mismatch per column is enough
				}
			}
		}
	}

	if lppColMismatch {
		utils.Panic("LPP column data consistency check failed: GL and LPP witnesses have different LPP column data (see errors above). This would cause Vortex Merkle root mismatch in production.")
	}

	logrus.Infof("All conglomeration completion checks passed (segment-counts, LPP-column-consistency)")
}

// runBootstrapperWithRescaling runs the bootstrapper and returns the resulting
// prover runtime.
func runBootstrapperWithRescaling(
	cfg *config.Config,
	bootstrapper *wizard.CompiledIOP,
	zkevm *ZkEvm,
	disc *distributed.StandardModuleDiscoverer,
	zkevmWitness *Witness,
	withDebug bool,
) *wizard.ProverRuntime {

	var (
		scalingFactor = 1
		runtimeBoot   *wizard.ProverRuntime
	)

	for runtimeBoot == nil {

		logrus.Infof("Trying to bootstrap with a scaling of %v\n", scalingFactor)

		func() {

			// Since the [exit] package is configured to only send panic messages
			// on overflow. The overflows are catchable.
			defer func() {
				if err := recover(); err != nil {
					oFReport, isOF := err.(exit.LimitOverflowReport)
					if isOF {
						extra := utils.DivCeil(oFReport.RequestedSize, oFReport.Limit)
						scalingFactor *= utils.NextPowerOfTwo(extra)
						return
					}

					panic(err)
				}
			}()

			if scalingFactor == 1 {
				logrus.Infof("Running bootstrapper")
				runtimeBoot = wizard.RunProver(
					bootstrapper,
					zkevm.GetMainProverStep(zkevmWitness),
					true,
				)
				return
			}

			scaledUpBootstrapper, scaledUpZkEVM := GetScaledUpBootstrapper(
				cfg, disc, scalingFactor,
			)

			if withDebug {
				// This adds debugging to the bootstrapper which are normally
				// not present by default.
				wizard.ContinueCompilation(
					scaledUpBootstrapper,
					dummy.CompileAtProverLvl(dummy.WithMsg("bootstrapper")),
				)
			}

			runtimeBoot = wizard.RunProver(
				scaledUpBootstrapper,
				scaledUpZkEVM.GetMainProverStep(zkevmWitness),
				true,
			)
		}()
	}

	return runtimeBoot
}

// Store writes the limitless prover zkevm into disk in the folder given by
// [cfg.PathforLimitlessProverAssets].
func (lz *LimitlessZkEVM) Store(cfg *config.Config) error {

	// asset is a utility struct used to list the object and the file name
	type asset struct {
		Name   string
		Object any
	}

	if cfg == nil {
		utils.Panic("config is nil")
	}

	// Create directory for assets
	assetDir := cfg.PathForSetup(executionLimitlessPath)
	if err := os.MkdirAll(assetDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", assetDir, err)
	}

	assets := []asset{
		{
			Name:   zkevmFile,
			Object: lz.Zkevm,
		},
		{
			Name:   discFile,
			Object: *lz.DistWizard.Disc,
		},
		{
			Name:   bootstrapperFile,
			Object: lz.DistWizard.Bootstrapper,
		},
		{
			Name:   conglomerationFile,
			Object: *lz.DistWizard.CompiledConglomeration,
		},
		{
			Name:   verificationKeyMerkleTreeFile,
			Object: lz.DistWizard.VerificationKeyMerkleTree,
		},
	}

	for _, modGl := range lz.DistWizard.CompiledGLs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(compileGlTemplate, modGl.ModuleGL.DefinitionInput.ModuleName),
			Object: *modGl,
		})
	}

	for i, blueprintGL := range lz.DistWizard.BlueprintGLs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(blueprintGLTemplate, i),
			Object: blueprintGL,
		})
	}

	for _, debugGL := range lz.DistWizard.DebugGLs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(debugGlTemplate, debugGL.DefinitionInput.ModuleName),
			Object: debugGL,
		})
	}

	for _, modLpp := range lz.DistWizard.CompiledLPPs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(compileLppTemplate, modLpp.ModuleLPP.ModuleName()),
			Object: *modLpp,
		})
	}

	for i, blueprintLPP := range lz.DistWizard.BlueprintLPPs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(blueprintLppTemplate, i),
			Object: blueprintLPP,
		})
	}

	for _, debugLPP := range lz.DistWizard.DebugLPPs {
		assets = append(assets, asset{
			Name:   fmt.Sprintf(debugLppTemplate, debugLPP.ModuleName()),
			Object: debugLPP,
		})
	}

	for _, asset := range assets {
		logrus.Infof("writing %s to disk", asset.Name)
		if err := serde.StoreToDisk(assetDir+"/"+asset.Name, asset.Object, true); err != nil {
			return err
		}
	}

	logrus.Info("limitless prover assets written to disk")
	return nil
}

// LoadBootstrapperAsync loads the bootstrapper from disk.
func (lz *LimitlessZkEVM) LoadBootstrapper(cfg *config.Config) error {
	if lz.DistWizard == nil {
		lz.DistWizard = &distributed.DistributedWizard{}
	}
	closer, err := serde.LoadFromDisk(
		cfg.PathForSetup(executionLimitlessPath)+"/"+bootstrapperFile,
		&lz.DistWizard.Bootstrapper,
		true,
	)
	if err != nil {
		return err
	}
	defer closer.Close()
	return nil
}

// LoadZkEVM loads the zkevm from disk
func (lz *LimitlessZkEVM) LoadZkEVM(cfg *config.Config) error {
	closer, err := serde.LoadFromDisk(cfg.PathForSetup(executionLimitlessPath)+"/"+zkevmFile, &lz.Zkevm, true)
	if err != nil {
		return err
	}
	defer closer.Close()
	return nil
}

// LoadDisc loads the discoverer from disk
func (lz *LimitlessZkEVM) LoadDisc(cfg *config.Config) error {
	if lz.DistWizard == nil {
		lz.DistWizard = &distributed.DistributedWizard{}
	}

	// The discoverer is not directly deserialized as an interface object as we
	// figured that it does not work very well and the reason is unclear. This
	// conversion step is a workaround for the problem.
	res := &distributed.StandardModuleDiscoverer{}

	closer, err := serde.LoadFromDisk(cfg.PathForSetup(executionLimitlessPath)+"/"+discFile, res, true)
	if err != nil {
		return err
	}
	defer closer.Close()

	lz.DistWizard.Disc = res
	return nil
}

// LoadBlueprints loads the segmentation blueprints from disk for all the modules
// LPP and GL.
func (lz *LimitlessZkEVM) LoadBlueprints(cfg *config.Config) error {

	var (
		assetDir        = cfg.PathForSetup(executionLimitlessPath)
		cntLpps, cntGLs int
	)

	if lz.DistWizard == nil {
		lz.DistWizard = &distributed.DistributedWizard{}
	}

	files, err := os.ReadDir(assetDir)
	if err != nil {
		return fmt.Errorf("could not read directory %s: %w", assetDir, err)
	}

	for _, file := range files {

		if strings.HasPrefix(file.Name(), blueprintGLPrefix) {
			cntGLs++
		}

		if strings.HasPrefix(file.Name(), blueprintLppPrefix) {
			cntLpps++
		}
	}

	lz.DistWizard.BlueprintGLs = make([]distributed.ModuleSegmentationBlueprint, cntGLs)
	lz.DistWizard.BlueprintLPPs = make([]distributed.ModuleSegmentationBlueprint, cntLpps)

	eg := &errgroup.Group{}

	for i := 0; i < cntGLs; i++ {
		eg.Go(func() error {
			filePath := path.Join(assetDir, fmt.Sprintf(blueprintGLTemplate, i))
			closer, err := serde.LoadFromDisk(filePath, &lz.DistWizard.BlueprintGLs[i], true)
			if err != nil {
				return err
			}
			defer closer.Close()
			return nil
		})
	}

	for i := 0; i < cntLpps; i++ {
		eg.Go(func() error {
			filePath := path.Join(assetDir, fmt.Sprintf(blueprintLppTemplate, i))
			closer, err := serde.LoadFromDisk(filePath, &lz.DistWizard.BlueprintLPPs[i], true)
			if err != nil {
				return err
			}
			defer closer.Close()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// LoadCompiledGL loads the compiled GL from disk
func LoadCompiledGL(cfg *config.Config, moduleName distributed.ModuleName) (*distributed.RecursedSegmentCompilation, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(compileGlTemplate, moduleName))
		res      = &distributed.RecursedSegmentCompilation{}
	)

	closer, err := serde.LoadFromDisk(filePath, res, true)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return res, nil
}

// LoadCompiledLPP loads the compiled LPP from disk
func LoadCompiledLPP(cfg *config.Config, moduleNames distributed.ModuleName) (*distributed.RecursedSegmentCompilation, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(compileLppTemplate, moduleNames))
		res      = &distributed.RecursedSegmentCompilation{}
	)

	closer, err := serde.LoadFromDisk(filePath, res, true)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return res, nil
}

// LoadCompiledGLMmap loads the compiled GL into mmap-backed memory for explicit release.
// The caller must call buf.Release() when done, after nilling all references to res.
func LoadCompiledGLMmap(cfg *config.Config, moduleName distributed.ModuleName) (*distributed.RecursedSegmentCompilation, *serde.MmapBackedBuffer, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(compileGlTemplate, moduleName))
		res      = &distributed.RecursedSegmentCompilation{}
	)

	buf, err := serde.LoadFromDiskMmapBacked(filePath, res)
	if err != nil {
		return nil, nil, err
	}

	return res, buf, nil
}

// LoadCompiledLPPMmap loads the compiled LPP into mmap-backed memory for explicit release.
// The caller must call buf.Release() when done, after nilling all references to res.
func LoadCompiledLPPMmap(cfg *config.Config, moduleNames distributed.ModuleName) (*distributed.RecursedSegmentCompilation, *serde.MmapBackedBuffer, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(compileLppTemplate, moduleNames))
		res      = &distributed.RecursedSegmentCompilation{}
	)

	buf, err := serde.LoadFromDiskMmapBacked(filePath, res)
	if err != nil {
		return nil, nil, err
	}

	return res, buf, nil
}

// LoadDebugGL loads the debug GL from disk
func LoadDebugGL(cfg *config.Config, moduleName distributed.ModuleName) (*distributed.ModuleGL, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(debugGlTemplate, moduleName))
		res      = &distributed.ModuleGL{}
	)

	closer, err := serde.LoadFromDisk(filePath, res, true)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return res, nil
}

// LoadDebugLPP loads the debug LPP from disk
func LoadDebugLPP(cfg *config.Config, moduleName []distributed.ModuleName) (*distributed.ModuleLPP, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, fmt.Sprintf(debugLppTemplate, moduleName))
		res      = &distributed.ModuleLPP{}
	)

	closer, err := serde.LoadFromDisk(filePath, res, true)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return res, nil
}

// LoadCompiledConglomeration loads the conglomeration assets from disk
func LoadCompiledConglomeration(cfg *config.Config) (*distributed.RecursedSegmentCompilation, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, conglomerationFile)
		conglo   = &distributed.RecursedSegmentCompilation{}
	)

	closer, err := serde.LoadFromDisk(filePath, conglo, true)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return conglo, nil
}

// LoadCompiledConglomerationMmap loads the conglomeration assets into mmap-backed memory.
// The caller must call buf.Release() when done, after nilling all references to conglo.
func LoadCompiledConglomerationMmap(cfg *config.Config) (*distributed.RecursedSegmentCompilation, *serde.MmapBackedBuffer, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, conglomerationFile)
		conglo   = &distributed.RecursedSegmentCompilation{}
	)

	buf, err := serde.LoadFromDiskMmapBacked(filePath, conglo)
	if err != nil {
		return nil, nil, err
	}

	return conglo, buf, nil
}

func LoadVerificationKeyMerkleTree(cfg *config.Config) (*distributed.VerificationKeyMerkleTree, error) {

	var (
		assetDir = cfg.PathForSetup(executionLimitlessPath)
		filePath = path.Join(assetDir, verificationKeyMerkleTreeFile)
		mt       = &distributed.VerificationKeyMerkleTree{}
	)

	closer, err := serde.LoadFromDisk(filePath, mt, true)
	if err != nil {
		return nil, err
	}
	defer closer.Close()

	return mt, nil
}
