package config

import (
	"bytes"
	"encoding/json"
	"slices"
	"strings"

	"github.com/consensys/linea-monorepo/prover/utils"
)

// These are modules that are internal to the prover and for which we have a
// hard requirement that they are defined.
var (
	modulePrecompileEcrecoverEffectiveCalls    = "PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS"
	modulePrecompileSha2Blocks                 = "PRECOMPILE_SHA2_BLOCKS"
	modulePrecompileRipemdBlocks               = "PRECOMPILE_RIPEMD_BLOCKS"
	modulePrecompileModexpEffectiveCalls       = "PRECOMPILE_MODEXP_EFFECTIVE_CALLS"
	modulePrecompileModexpEffectiveCallsLarge  = "PRECOMPILE_MODEXP_EFFECTIVE_CALLS_4096"
	modulePrecompileEcaddEffectiveCalls        = "PRECOMPILE_ECADD_EFFECTIVE_CALLS"
	modulePrecompileEcmulEffectiveCalls        = "PRECOMPILE_ECMUL_EFFECTIVE_CALLS"
	modulePrecompileEcpairingEffectiveCalls    = "PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS"
	modulePrecompileEcpairingMillerLoops       = "PRECOMPILE_ECPAIRING_MILLER_LOOPS"
	modulePrecompileEcpairingG2MembershipCalls = "PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS"
	modulePrecompileBlakeEffectiveCalls        = "PRECOMPILE_BLAKE_EFFECTIVE_CALLS"
	modulePrecompileBlakeRounds                = "PRECOMPILE_BLAKE_ROUNDS"
	moduleBlockKeccak                          = "BLOCK_KECCAK"
	moduleBlockL1Size                          = "BLOCK_L1_SIZE"
	moduleBlockL2L1Logs                        = "BLOCK_L2_L1_LOGS"
	moduleBlockTransactions                    = "BLOCK_TRANSACTIONS"
	moduleShomeiMerkleProofs                   = "SHOMEI_MERKLE_PROOFS"

	modulePrecompileBlsG1AddEffectiveCalls               = "PRECOMPILE_BLS_G1_ADD_EFFECTIVE_CALLS"
	modulePrecompileBlsG2AddEffectiveCalls               = "PRECOMPILE_BLS_G2_ADD_EFFECTIVE_CALLS"
	modulePrecompileBlsG1MsmEffectiveCalls               = "PRECOMPILE_BLS_G1_MSM_EFFECTIVE_CALLS"
	modulePrecompileBlsG2MsmEffectiveCalls               = "PRECOMPILE_BLS_G2_MSM_EFFECTIVE_CALLS"
	modulePrecompileBlsPairingCheckMillerLoops           = "PRECOMPILE_BLS_PAIRING_CHECK_MILLER_LOOPS"
	modulePrecompileBlsFinalExponentiations              = "PRECOMPILE_BLS_FINAL_EXPONENTIATIONS"
	modulePrecompileBlsG1MembershipCalls                 = "PRECOMPILE_BLS_G1_MEMBERSHIP_CALLS"
	modulePrecompileBlsG2MembershipCalls                 = "PRECOMPILE_BLS_G2_MEMBERSHIP_CALLS"
	modulePrecompileBlsMapFpToG1EffectiveCalls           = "PRECOMPILE_BLS_MAP_FP_TO_G1_EFFECTIVE_CALLS"
	modulePrecompileBlsMapFp2ToG2EffectiveCalls          = "PRECOMPILE_BLS_MAP_FP2_TO_G2_EFFECTIVE_CALLS"
	modulePrecompileBlsC1MembershipCalls                 = "PRECOMPILE_BLS_C1_MEMBERSHIP_CALLS"
	modulePrecompileBlsC2MembershipCalls                 = "PRECOMPILE_BLS_C2_MEMBERSHIP_CALLS"
	modulePrecompileBlsPointEvaluationEffectiveCalls     = "PRECOMPILE_BLS_POINT_EVALUATION_EFFECTIVE_CALLS"
	modulePrecompilePointEvaluationFailureEffectiveCalls = "PRECOMPILE_POINT_EVALUATION_FAILURE_EFFECTIVE_CALLS"
	modulePrecompileP256VerifyEffectiveCalls             = "PRECOMPILE_P256_VERIFY_EFFECTIVE_CALLS"
)

// TracesLimits defines the limits for each module. The way limit work is by
// letting the user specifies prefixes of modules and the size to map to the
// module. The modules are sorted in reverse alphabetical order. When the user
// provides a module name (string), the string is checked against the prefixes
// in reverse alphabetical order.
//
// For instance, the limit map the followings:
//
// log2			= 1000, 2000
// log2_u256 	= 100, 	200
// log2_u160	= 5, 	10
// log2_u32		= 12, 	24
//
// If the user provides the string "log2_u32", the limit will be 12. And if the
// user provides log2_u16 (for which u160 is NOT a prefix) the limit will be
// 1000 because it will match "log2".
//
// Passing a module whose name is the empty string will be used as the default
// module.
//
// At runtime, the user may switch the trace limit file in "Large" mode by
// calling [SetLargeMode].
type TracesLimits struct {
	// ToLargeMode is a flag telling the current struct to return LimitLarge
	// values instead of Limit when a module is requested.
	ToLargeMode bool
	// ScalingFactor is the factor used to scale the limit when a module is
	// requested. This is useful for the limitless prover when it needs to retry
	// with a larger limit. If the value is 0, this is equivalent to 1.
	ScalingFactor int
	// Modules is the list of modules and their limits
	Modules []ModuleLimit `mapstructure:"modules" validate:"required"`
}

// ModuleLimit defines the limit for each module.
type ModuleLimit struct {
	Module     string `mapstructure:"module" validate:"required"`
	Limit      int    `mapstructure:"limit" validate:"required,power_of_2"`
	LimitLarge int    `mapstructure:"limit_large" validate:"required,power_of_2"`
	// IsNotScalable indicates that the module limit should never be scaled up.
	// This can be used for "MAX_L2_BLOCK_SIZE" or "L2L1Logs" whose size is
	// fixed for the proof system.
	IsNotScalable bool `mapstructure:"is_not_scalable"`
}

// normalizeToLowercase normalizes the modules to lowercase
// normalizeToLowercase returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) normalizeToLowercase() {
	for i := range tl.Modules {
		tl.Modules[i].Module = strings.ToLower(tl.Modules[i].Module)
	}
}

// sortReverseAlphabetical sorts the modules in reverse alphabetical order
// sortReverseAlphabetical returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) sortReverseAlphabetical() {
	slices.SortStableFunc(tl.Modules, func(a, b ModuleLimit) int {
		switch {
		case a.Module < b.Module:
			return 1
		case a.Module > b.Module:
			return -1
		default:
			utils.Panic("module %++v is not unique, also have module %++v", a, b)
		}
		return 0 // unreachable
	})
}

// SetLargeMode the TracesLimits to use the large mode
// SetLargeMode returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) SetLargeMode() {
	tl.ToLargeMode = true
}

// CheckSum returns the checksum of the TraceLimits. Checksum returns the limits
// corresponding to the name of the method. (auto-generated)
func (tl *TracesLimits) Checksum() string {

	// encode the struct to json, then hash it
	encoded, err := json.Marshal(tl)
	if err != nil {
		panic(err) // should never happen
	}

	digest, err := utils.Digest(bytes.NewReader(encoded))
	if err != nil {
		panic(err) // should never happen
	}

	return digest
}

// findModuleLimits returns the limits for a module or panics.
// mustFindModuleLimits returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) mustFindModuleLimits(module string) ModuleLimit {
	moduleLower := strings.ToLower(module)
	for _, m := range tl.Modules {
		if strings.HasPrefix(moduleLower, m.Module) {
			return m
		}
	}
	utils.Panic("found no module limits for module %q", module)
	return ModuleLimit{}
}

// ScaleUp increases the scaling factor
func (tl *TracesLimits) ScaleUp(by int) {
	if tl.ScalingFactor == 0 {
		tl.ScalingFactor = 1
	}
	tl.ScalingFactor *= by
}

// GetLimit returns the limits of a module
func (tl *TracesLimits) GetLimit(module string) int {

	var (
		ml  = tl.mustFindModuleLimits(module)
		res = ml.Limit
	)

	if tl.ToLargeMode {
		res = ml.LimitLarge
	}

	scalingFactor := tl.ScalingFactor
	if scalingFactor == 0 {
		scalingFactor = 1
	}

	if !ml.IsNotScalable {
		res *= scalingFactor
	}

	return res
}

// PrecompileEcrecoverEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileEcrecoverEffectiveCalls() int {
	name := modulePrecompileEcrecoverEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileSha2Blocks returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileSha2Blocks() int {
	name := modulePrecompileSha2Blocks
	return tl.GetLimit(name)
}

// PrecompileRipemdBlocks returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileRipemdBlocks() int {
	name := modulePrecompileRipemdBlocks
	return tl.GetLimit(name)
}

// PrecompileModexpEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileModexpEffectiveCalls() int {
	name := modulePrecompileModexpEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileModexpEffectiveCalls8192 returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileModexpEffectiveCallsLarge() int {
	name := modulePrecompileModexpEffectiveCallsLarge
	return tl.GetLimit(name)
}

// PrecompileEcaddEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileEcaddEffectiveCalls() int {
	name := modulePrecompileEcaddEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileEcmulEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileEcmulEffectiveCalls() int {
	name := modulePrecompileEcmulEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileEcpairingEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileEcpairingEffectiveCalls() int {
	name := modulePrecompileEcpairingEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileEcpairingMillerLoops returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileEcpairingMillerLoops() int {
	name := modulePrecompileEcpairingMillerLoops
	return tl.GetLimit(name)
}

// PrecompileEcpairingG2MembershipCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileEcpairingG2MembershipCalls() int {
	name := modulePrecompileEcpairingG2MembershipCalls
	return tl.GetLimit(name)
}

// PrecompileBlakeEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlakeEffectiveCalls() int {
	name := modulePrecompileBlakeEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileBlakeRounds returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlakeRounds() int {
	name := modulePrecompileBlakeRounds
	return tl.GetLimit(name)
}

// BlockKeccak returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) BlockKeccak() int {
	name := moduleBlockKeccak
	return tl.GetLimit(name)
}

// BlockL1Size returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) BlockL1Size() int {
	name := moduleBlockL1Size
	return tl.GetLimit(name)
}

// BlockL2L1Logs returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) BlockL2L1Logs() int {
	name := moduleBlockL2L1Logs
	return tl.GetLimit(name)
}

// BlockTransactions returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) BlockTransactions() int {
	name := moduleBlockTransactions
	return tl.GetLimit(name)
}

// ShomeiMerkleProofs returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) ShomeiMerkleProofs() int {
	name := moduleShomeiMerkleProofs
	return tl.GetLimit(name)
}

// PrecompileBlsG1AddEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsG1AddEffectiveCalls() int {
	name := modulePrecompileBlsG1AddEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileBlsG2AddEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsG2AddEffectiveCalls() int {
	name := modulePrecompileBlsG2AddEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileBlsG1MsmEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsG1MsmEffectiveCalls() int {
	name := modulePrecompileBlsG1MsmEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileBlsG2MsmEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsG2MsmEffectiveCalls() int {
	name := modulePrecompileBlsG2MsmEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileBlsPairingCheckMillerLoops returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsPairingCheckMillerLoops() int {
	name := modulePrecompileBlsPairingCheckMillerLoops
	return tl.GetLimit(name)
}

// PrecompileBlsFinalExponentiations returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsFinalExponentiations() int {
	name := modulePrecompileBlsFinalExponentiations
	return tl.GetLimit(name)
}

// PrecompileBlsG1MembershipCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsG1MembershipCalls() int {
	name := modulePrecompileBlsG1MembershipCalls
	return tl.GetLimit(name)
}

// PrecompileBlsG2MembershipCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsG2MembershipCalls() int {
	name := modulePrecompileBlsG2MembershipCalls
	return tl.GetLimit(name)
}

// PrecompileBlsMapFpToG1EffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsMapFpToG1EffectiveCalls() int {
	name := modulePrecompileBlsMapFpToG1EffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileBlsMapFp2ToG2EffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsMapFp2ToG2EffectiveCalls() int {
	name := modulePrecompileBlsMapFp2ToG2EffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileBlsC1MembershipCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsC1MembershipCalls() int {
	name := modulePrecompileBlsC1MembershipCalls
	return tl.GetLimit(name)
}

// PrecompileBlsC2MembershipCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsC2MembershipCalls() int {
	name := modulePrecompileBlsC2MembershipCalls
	return tl.GetLimit(name)
}

// PrecompileBlsPointEvaluationEffectiveCalls returns the limits corresponding
// to the name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileBlsPointEvaluationEffectiveCalls() int {
	name := modulePrecompileBlsPointEvaluationEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompilePointEvaluationFailureEffectiveCalls returns the limits
// corresponding to the name of the method. (auto-generated)
func (tl *TracesLimits) PrecompilePointEvaluationFailureEffectiveCalls() int {
	name := modulePrecompilePointEvaluationFailureEffectiveCalls
	return tl.GetLimit(name)
}

// PrecompileP256VerifyEffectiveCalls returns the limits corresponding to the
// name of the method. (auto-generated)
func (tl *TracesLimits) PrecompileP256VerifyEffectiveCalls() int {
	name := modulePrecompileP256VerifyEffectiveCalls
	return tl.GetLimit(name)
}

// GetTestTracesLimits returns a sample of the trace limits.
func GetTestTracesLimits() *TracesLimits {

	// These are the config trace-limits from sepolia updated with latest precompile values.
	traceLimits := &TracesLimits{
		Modules: []ModuleLimit{
			{Module: "add", Limit: 262144, LimitLarge: 524288},
			{Module: "bin", Limit: 262144, LimitLarge: 524288},
			{Module: "blake_modexp_data", Limit: 16384, LimitLarge: 32768},
			{Module: "block_data", Limit: 4096, LimitLarge: 8192},
			{Module: "block_hash", Limit: 2048, LimitLarge: 4096},
			{Module: "ec_data", Limit: 65536, LimitLarge: 131072},
			{Module: "euc", Limit: 65536, LimitLarge: 131072},
			{Module: "exp", Limit: 65536, LimitLarge: 131072},
			{Module: "ext", Limit: 524288, LimitLarge: 1048576},
			{Module: "gas", Limit: 65536, LimitLarge: 131072},
			{Module: "hub", Limit: 2097152, LimitLarge: 4194304},
			{Module: "log_data", Limit: 65536, LimitLarge: 131072},
			{Module: "log_info", Limit: 4096, LimitLarge: 8192},
			{Module: "mmio", Limit: 2097152, LimitLarge: 4194304},
			{Module: "mmu", Limit: 1048576, LimitLarge: 2097152},
			{Module: "mod", Limit: 131072, LimitLarge: 262144},
			{Module: "mul", Limit: 65536, LimitLarge: 131072},
			{Module: "mxp", Limit: 524288, LimitLarge: 1048576},
			{Module: "oob", Limit: 262144, LimitLarge: 524288},
			{Module: "rlp_addr", Limit: 4096, LimitLarge: 8192},
			{Module: "rlp_txn", Limit: 131072, LimitLarge: 262144},
			{Module: "rlp_txn_rcpt", Limit: 65536, LimitLarge: 131072},
			{Module: "rlp_auth", Limit: 16384, LimitLarge: 32768},
			{Module: "rom", Limit: 8388608, LimitLarge: 8388608},
			{Module: "rom_lex", Limit: 1024, LimitLarge: 2048},
			{Module: "shakira_data", Limit: 65536, LimitLarge: 65536},
			{Module: "shf", Limit: 262144, LimitLarge: 524288},
			{Module: "stp", Limit: 16384, LimitLarge: 32768},
			{Module: "trm", Limit: 32768, LimitLarge: 65536},
			{Module: "txn_data", Limit: 8192, LimitLarge: 16384},
			{Module: "wcp", Limit: 262144, LimitLarge: 524288},
			{Module: "bin_reference_table", Limit: 262144, LimitLarge: 262144, IsNotScalable: true},
			{Module: "shf_reference_table", Limit: 4096, LimitLarge: 4096, IsNotScalable: true},
			{Module: "instruction_decoder", Limit: 512, LimitLarge: 512, IsNotScalable: true},
			{Module: "precompile_ecrecover_effective_calls", Limit: 128, LimitLarge: 256},
			{Module: "precompile_sha2_blocks", Limit: 200, LimitLarge: 400},
			{Module: "precompile_ripemd_blocks", Limit: 0, LimitLarge: 0},
			{Module: "precompile_modexp_effective_calls", Limit: 32, LimitLarge: 64},
			{Module: "precompile_modexp_effective_calls_4096", Limit: 1, LimitLarge: 1},
			{Module: "precompile_ecadd_effective_calls", Limit: 256, LimitLarge: 512},
			{Module: "precompile_ecmul_effective_calls", Limit: 40, LimitLarge: 80},
			{Module: "precompile_ecpairing_final_exponentiations", Limit: 16, LimitLarge: 32},
			{Module: "precompile_ecpairing_miller_loops", Limit: 64, LimitLarge: 128},
			{Module: "precompile_ecpairing_g2_membership_calls", Limit: 64, LimitLarge: 128},
			{Module: "precompile_blake_effective_calls", Limit: 0, LimitLarge: 0},
			{Module: "precompile_blake_rounds", Limit: 0, LimitLarge: 0},
			{Module: "block_keccak", Limit: 8192, LimitLarge: 8192},
			{Module: "block_l1_size", Limit: 1000000, LimitLarge: 1000000, IsNotScalable: true},
			{Module: "block_l2_l1_logs", Limit: 16, LimitLarge: 16, IsNotScalable: true},
			{Module: "block_transactions", Limit: 300, LimitLarge: 300},
			{Module: "shomei_merkle_proofs", Limit: 8192, LimitLarge: 16384},
			{Module: "precompile_bls_g1_add_effective_calls", Limit: 256, LimitLarge: 512},
			{Module: "precompile_bls_g2_add_effective_calls", Limit: 16, LimitLarge: 32},
			{Module: "precompile_bls_g1_msm_effective_calls", Limit: 32, LimitLarge: 64},
			{Module: "precompile_bls_g2_msm_effective_calls", Limit: 16, LimitLarge: 32},
			{Module: "precompile_bls_pairing_check_miller_loops", Limit: 64, LimitLarge: 128},
			{Module: "precompile_bls_final_exponentiations", Limit: 16, LimitLarge: 32},
			{Module: "precompile_bls_g1_membership_calls", Limit: 64, LimitLarge: 128},
			{Module: "precompile_bls_g2_membership_calls", Limit: 64, LimitLarge: 128},
			{Module: "precompile_bls_map_fp_to_g1_effective_calls", Limit: 64, LimitLarge: 128},
			{Module: "precompile_bls_map_fp2_to_g2_effective_calls", Limit: 64, LimitLarge: 128},
			{Module: "precompile_bls_c1_membership_calls", Limit: 64, LimitLarge: 128},
			{Module: "precompile_bls_c2_membership_calls", Limit: 64, LimitLarge: 128},
			{Module: "precompile_bls_point_evaluation_effective_calls", Limit: 16, LimitLarge: 32},
			{Module: "precompile_point_evaluation_failure_effective_calls", Limit: 4, LimitLarge: 8},
			{Module: "precompile_p256_verify_effective_calls", Limit: 128, LimitLarge: 256},
			{Module: "u128", Limit: 131072, LimitLarge: 262144},
			{Module: "u20", Limit: 131072, LimitLarge: 262144},
			{Module: "u32", Limit: 131072, LimitLarge: 262144},
			{Module: "u36", Limit: 131072, LimitLarge: 262144},
			{Module: "u64", Limit: 131072, LimitLarge: 262144},
			{Module: "", Limit: 131072, LimitLarge: 262144},
		},
	}

	return traceLimits
}
