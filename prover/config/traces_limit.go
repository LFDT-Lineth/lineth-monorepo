package config

import (
	"bytes"
	"encoding/json"

	"github.com/consensys/linea-monorepo/prover/utils"
)

// The trace limits define the maximum trace size per module that the prover can handle.
// Raising these limits increases the prover's memory needs and simultaneously decreases the number
// of transactions it can prove in a single go. These traces are vital for the setup generator, so
// any changes in trace limits mean we'll need to run a new setup and update the verifier contracts
// before deploying.
type TracesLimits struct {
	Add               int `mapstructure:"ADD" validate:"power_of_2" corset:"add"`
	Bin               int `mapstructure:"BIN" validate:"power_of_2" corset:"bin"`
	Blake2Fmodexpdata int `mapstructure:"BLAKE_MODEXP_DATA" validate:"power_of_2" corset:"blake2fmodexpdata"`
	Blockdata         int `mapstructure:"BLOCK_DATA" corset:"blockdata"`
	Blockhash         int `mapstructure:"BLOCK_HASH" validate:"power_of_2" corset:"blockhash"`
	Ecdata            int `mapstructure:"EC_DATA" validate:"power_of_2" corset:"ecdata"`
	Euc               int `mapstructure:"EUC" validate:"power_of_2" corset:"euc"`
	Exp               int `mapstructure:"EXP" validate:"power_of_2" corset:"exp"`
	Ext               int `mapstructure:"EXT" validate:"power_of_2" corset:"ext"`
	Gas               int `mapstructure:"GAS" validate:"power_of_2" corset:"gas"`
	Hub               int `mapstructure:"HUB" validate:"power_of_2" corset:"hub"`
	Logdata           int `mapstructure:"LOG_DATA" validate:"power_of_2" corset:"logdata"`
	Loginfo           int `mapstructure:"LOG_INFO" validate:"power_of_2" corset:"loginfo"`
	Mmio              int `mapstructure:"MMIO" validate:"power_of_2" corset:"mmio"`
	Mmu               int `mapstructure:"MMU" validate:"power_of_2" corset:"mmu"`
	Mod               int `mapstructure:"MOD" validate:"power_of_2" corset:"mod"`
	Mul               int `mapstructure:"MUL" validate:"power_of_2" corset:"mul"`
	Mxp               int `mapstructure:"MXP" validate:"power_of_2" corset:"mxp"`
	Oob               int `mapstructure:"OOB" validate:"power_of_2" corset:"oob"`
	Rlpaddr           int `mapstructure:"RLP_ADDR" validate:"power_of_2" corset:"rlpaddr"`
	Rlptxn            int `mapstructure:"RLP_TXN" validate:"power_of_2" corset:"rlptxn"`
	Rlptxrcpt         int `mapstructure:"RLP_TXN_RCPT" validate:"power_of_2" corset:"rlptxrcpt"`
	Rom               int `mapstructure:"ROM" validate:"power_of_2" corset:"rom"`
	Romlex            int `mapstructure:"ROM_LEX" validate:"power_of_2" corset:"romlex"`
	Shakiradata       int `mapstructure:"SHAKIRA_DATA" validate:"power_of_2" corset:"shakiradata"`
	Shf               int `mapstructure:"SHF" validate:"power_of_2" corset:"shf"`
	Stp               int `mapstructure:"STP" validate:"power_of_2" corset:"stp"`
	Trm               int `mapstructure:"TRM" validate:"power_of_2" corset:"trm"`
	Txndata           int `mapstructure:"TXN_DATA" validate:"power_of_2" corset:"txndata"`
	Wcp               int `mapstructure:"WCP" validate:"power_of_2" corset:"wcp"`
	// internal modules
	BitShl256        int `mapstructure:"BIT_SHL256" validate:"power_of_2" corset:"bit_shl256"`
	BitShl256U7      int `mapstructure:"BIT_SHL256_U7" validate:"power_of_2" corset:"bit_shl256_u7"`
	BitShl256U6      int `mapstructure:"BIT_SHL256_U6" validate:"power_of_2" corset:"bit_shl256_u6"`
	BitShl256U5      int `mapstructure:"BIT_SHL256_U5" validate:"power_of_2" corset:"bit_shl256_u5"`
	BitShl256U4      int `mapstructure:"BIT_SHL256_U4" validate:"power_of_2" corset:"bit_shl256_u4"`
	BitShl256U3      int `mapstructure:"BIT_SHL256_U3" validate:"power_of_2" corset:"bit_shl256_u3"`
	BitShl256U2      int `mapstructure:"BIT_SHL256_U2" validate:"power_of_2" corset:"bit_shl256_u2"`
	BitShl256U1      int `mapstructure:"BIT_SHL256_U1" validate:"power_of_2" corset:"bit_shl256_u1"`
	BitShr256        int `mapstructure:"BIT_SHR256" validate:"power_of_2" corset:"bit_shr256"`
	BitShr256U7      int `mapstructure:"BIT_SHR256_U7" validate:"power_of_2" corset:"bit_shr256_u7"`
	BitShr256U6      int `mapstructure:"BIT_SHR256_U6" validate:"power_of_2" corset:"bit_shr256_u6"`
	BitShr256U5      int `mapstructure:"BIT_SHR256_U5" validate:"power_of_2" corset:"bit_shr256_u5"`
	BitShr256U4      int `mapstructure:"BIT_SHR256_U4" validate:"power_of_2" corset:"bit_shr256_u4"`
	BitShr256U3      int `mapstructure:"BIT_SHR256_U3" validate:"power_of_2" corset:"bit_shr256_u3"`
	BitShr256U2      int `mapstructure:"BIT_SHR256_U2" validate:"power_of_2" corset:"bit_shr256_u2"`
	BitShr256U1      int `mapstructure:"BIT_SHR256_U1" validate:"power_of_2" corset:"bit_shr256_u1"`
	BitSar256        int `mapstructure:"BIT_SAR256" validate:"power_of_2" corset:"bit_sar256"`
	BitSar256U7      int `mapstructure:"BIT_SAR256_U7" validate:"power_of_2" corset:"bit_sar256_u7"`
	BitSar256U6      int `mapstructure:"BIT_SAR256_U6" validate:"power_of_2" corset:"bit_sar256_u6"`
	BitSar256U5      int `mapstructure:"BIT_SAR256_U5" validate:"power_of_2" corset:"bit_sar256_u5"`
	BitSar256U4      int `mapstructure:"BIT_SAR256_U4" validate:"power_of_2" corset:"bit_sar256_u4"`
	BitSar256U3      int `mapstructure:"BIT_SAR256_U3" validate:"power_of_2" corset:"bit_sar256_u3"`
	BitSar256U2      int `mapstructure:"BIT_SAR256_U2" validate:"power_of_2" corset:"bit_sar256_u2"`
	BitSar256U1      int `mapstructure:"BIT_SAR256_U1" validate:"power_of_2" corset:"bit_sar256_u1"`
	CallGasExtra     int `mapstructure:"CALL_GAS_EXTRA" validate:"power_of_2" corset:"call_gas_extra"`
	FillBytesBetween int `mapstructure:"FILL_BYTES_BETWEEN" validate:"power_of_2" corset:"fill_bytes_between"`
	GasOutOfPocket   int `mapstructure:"GAS_OUT_OF_POCKET" validate:"power_of_2" corset:"gas_out_of_pocket"`
	Log2             int `mapstructure:"LOG2" validate:"power_of_2" corset:"log2"`
	Log2U128         int `mapstructure:"LOG2_U128" validate:"power_of_2" corset:"log2_u128"`
	Log2U64          int `mapstructure:"LOG2_U64" validate:"power_of_2" corset:"log2_u64"`
	Log2U32          int `mapstructure:"LOG2_U32" validate:"power_of_2" corset:"log2_u32"`
	Log2U16          int `mapstructure:"LOG2_U16" validate:"power_of_2" corset:"log2_u16"`
	Log2U8           int `mapstructure:"LOG2_U8" validate:"power_of_2" corset:"log2_u8"`
	Log2U4           int `mapstructure:"LOG2_U4" validate:"power_of_2" corset:"log2_u4"`
	Log2U2           int `mapstructure:"LOG2_U2" validate:"power_of_2" corset:"log2_u2"`
	Log256           int `mapstructure:"LOG256" validate:"power_of_2" corset:"log256"`
	Log256U128       int `mapstructure:"LOG256_U128" validate:"power_of_2" corset:"log256_u128"`
	Log256U64        int `mapstructure:"LOG256_U64" validate:"power_of_2" corset:"log256_u64"`
	Log256U32        int `mapstructure:"LOG256_U32" validate:"power_of_2" corset:"log256_u32"`
	Log256U16        int `mapstructure:"LOG256_U16" validate:"power_of_2" corset:"log256_u16"`
	Min25664         int `mapstructure:"MIN256_64" validate:"power_of_2" corset:"min256_64"`
	SetByte256       int `mapstructure:"SET_BYTE256" validate:"power_of_2" corset:"set_byte256"`
	SetByte128       int `mapstructure:"SET_BYTE128" validate:"power_of_2" corset:"set_byte128"`
	SetByte64        int `mapstructure:"SET_BYTE64" validate:"power_of_2" corset:"set_byte64"`
	SetByte32        int `mapstructure:"SET_BYTE32" validate:"power_of_2" corset:"set_byte32"`
	SetByte16        int `mapstructure:"SET_BYTE16" validate:"power_of_2" corset:"set_byte16"`
	// limitless typing modules
	U128 int `mapstructure:"U128" validate:"power_of_2" corset:"u128"`
	U127 int `mapstructure:"U127" validate:"power_of_2" corset:"u127"`
	U126 int `mapstructure:"U126" validate:"power_of_2" corset:"u126"`
	U125 int `mapstructure:"U125" validate:"power_of_2" corset:"u125"`
	U124 int `mapstructure:"U124" validate:"power_of_2" corset:"u124"`
	U123 int `mapstructure:"U123" validate:"power_of_2" corset:"u123"`
	U120 int `mapstructure:"U120" validate:"power_of_2" corset:"u120"`
	U119 int `mapstructure:"U119" validate:"power_of_2" corset:"u119"`
	U112 int `mapstructure:"U112" validate:"power_of_2" corset:"u112"`
	U111 int `mapstructure:"U111" validate:"power_of_2" corset:"u111"`
	U96  int `mapstructure:"U96" validate:"power_of_2" corset:"u96"`
	U95  int `mapstructure:"U95" validate:"power_of_2" corset:"u95"`
	U64  int `mapstructure:"U64" validate:"power_of_2" corset:"u64"`
	U63  int `mapstructure:"U63" validate:"power_of_2" corset:"u63"`
	U62  int `mapstructure:"U62" validate:"power_of_2" corset:"u62"`
	U61  int `mapstructure:"U61" validate:"power_of_2" corset:"u61"`
	U60  int `mapstructure:"U60" validate:"power_of_2" corset:"u60"`
	U59  int `mapstructure:"U59" validate:"power_of_2" corset:"u59"`
	U58  int `mapstructure:"U58" validate:"power_of_2" corset:"u58"`
	U56  int `mapstructure:"U56" validate:"power_of_2" corset:"u56"`
	U55  int `mapstructure:"U55" validate:"power_of_2" corset:"u55"`
	U48  int `mapstructure:"U48" validate:"power_of_2" corset:"u48"`
	U47  int `mapstructure:"U47" validate:"power_of_2" corset:"u47"`
	U36  int `mapstructure:"U36" validate:"power_of_2" corset:"u36"`
	U32  int `mapstructure:"U32" validate:"power_of_2" corset:"u32"`
	U31  int `mapstructure:"U31" validate:"power_of_2" corset:"u31"`
	U30  int `mapstructure:"U30" validate:"power_of_2" corset:"u30"`
	U29  int `mapstructure:"U29" validate:"power_of_2" corset:"u29"`
	U28  int `mapstructure:"U28" validate:"power_of_2" corset:"u28"`
	U27  int `mapstructure:"U27" validate:"power_of_2" corset:"u27"`
	U26  int `mapstructure:"U26" validate:"power_of_2" corset:"u26"`
	U24  int `mapstructure:"U24" validate:"power_of_2" corset:"u24"`
	U23  int `mapstructure:"U23" validate:"power_of_2" corset:"u23"`
	U20  int `mapstructure:"U20" validate:"power_of_2" corset:"u20"`
	// reference tables
	Binreftable int `mapstructure:"BIN_REFERENCE_TABLE" validate:"power_of_2" corset:"binreftable"`
	Instdecoder int `mapstructure:"INSTRUCTION_DECODER" validate:"power_of_2" corset:"instdecoder"`

	PrecompileEcrecoverEffectiveCalls    int `mapstructure:"PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS"`
	PrecompileSha2Blocks                 int `mapstructure:"PRECOMPILE_SHA2_BLOCKS"`
	PrecompileRipemdBlocks               int `mapstructure:"PRECOMPILE_RIPEMD_BLOCKS"`
	PrecompileModexpEffectiveCalls       int `mapstructure:"PRECOMPILE_MODEXP_EFFECTIVE_CALLS"`
	PrecompileModexpEffectiveCalls4096   int `mapstructure:"PRECOMPILE_MODEXP_EFFECTIVE_CALLS_4096"`
	PrecompileEcaddEffectiveCalls        int `mapstructure:"PRECOMPILE_ECADD_EFFECTIVE_CALLS"`
	PrecompileEcmulEffectiveCalls        int `mapstructure:"PRECOMPILE_ECMUL_EFFECTIVE_CALLS"`
	PrecompileEcpairingEffectiveCalls    int `mapstructure:"PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS"`
	PrecompileEcpairingMillerLoops       int `mapstructure:"PRECOMPILE_ECPAIRING_MILLER_LOOPS"`
	PrecompileEcpairingG2MembershipCalls int `mapstructure:"PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS"`
	PrecompileBlakeEffectiveCalls        int `mapstructure:"PRECOMPILE_BLAKE_EFFECTIVE_CALLS"`
	PrecompileBlakeRounds                int `mapstructure:"PRECOMPILE_BLAKE_ROUNDS"`

	BlockKeccak       int `mapstructure:"BLOCK_KECCAK"`
	BlockL1Size       int `mapstructure:"BLOCK_L1_SIZE"`
	BlockL2L1Logs     int `mapstructure:"BLOCK_L2_L1_LOGS"`
	BlockTransactions int `mapstructure:"BLOCK_TRANSACTIONS"`

	ShomeiMerkleProofs int `mapstructure:"SHOMEI_MERKLE_PROOFS"`

	// beta v4.0
	PrecompileBlsPointEvaluationEffectiveCalls     int `mapstructure:"PRECOMPILE_BLS_POINT_EVALUATION_EFFECTIVE_CALLS"`
	PrecompilePointEvaluationFailureEffectiveCalls int `mapstructure:"PRECOMPILE_POINT_EVALUATION_FAILURE_EFFECTIVE_CALLS"`
	PrecompileBlsG1AddEffectiveCalls               int `mapstructure:"PRECOMPILE_BLS_G1_ADD_EFFECTIVE_CALLS"`
	PrecompileBlsG1MsmEffectiveCalls               int `mapstructure:"PRECOMPILE_BLS_G1_MSM_EFFECTIVE_CALLS"`
	PrecompileBlsG2AddEffectiveCalls               int `mapstructure:"PRECOMPILE_BLS_G2_ADD_EFFECTIVE_CALLS"`
	PrecompileBlsG2MsmEffectiveCalls               int `mapstructure:"PRECOMPILE_BLS_G2_MSM_EFFECTIVE_CALLS"`
	PrecompileBlsPairingCheckMillerLoops           int `mapstructure:"PRECOMPILE_BLS_PAIRING_CHECK_MILLER_LOOPS"`
	PrecompileBlsFinalExponentiations              int `mapstructure:"PRECOMPILE_BLS_FINAL_EXPONENTIATIONS"`
	PrecompileBlsMapFpToG1EffectiveCalls           int `mapstructure:"PRECOMPILE_BLS_MAP_FP_TO_G1_EFFECTIVE_CALLS"`
	PrecompileBlsMapFp2ToG2EffectiveCalls          int `mapstructure:"PRECOMPILE_BLS_MAP_FP2_TO_G2_EFFECTIVE_CALLS"`
	PrecompileBlsC1MembershipChecks                int `mapstructure:"PRECOMPILE_BLS_C1_MEMBERSHIP_CHECKS"`
	PrecompileBlsC2MembershipCalls                 int `mapstructure:"PRECOMPILE_BLS_C2_MEMBERSHIP_CALLS"`
	PrecompileBlsG1MembershipCalls                 int `mapstructure:"PRECOMPILE_BLS_G1_MEMBERSHIP_CALLS"`
	PrecompileBlsG2MembershipCalls                 int `mapstructure:"PRECOMPILE_BLS_G2_MEMBERSHIP_CALLS"`
	Bls                                            int `mapstructure:"BLS" corset:"bls"`
	PointEval                                      int `mapstructure:"POINT_EVAL" corset:"pointeval"`
	BlsData                                        int `mapstructure:"BLS_DATA" validate:"power_of_2" corset:"blsdata"`
	RlpUtils                                       int `mapstructure:"RLP_UTILS" validate:"power_of_2" corset:"rlputils"`
	PowerReferenceTable                            int `mapstructure:"POWER_REFERENCE_TABLE" validate:"power_of_2" corset:"power"`
	BlsReferenceTable                              int `mapstructure:"BLS_REFERENCE_TABLE" validate:"power_of_2" corset:"blsreftable"`
}

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

func (tl *TracesLimits) ScaleUp(by int) {

	tl.Add *= by
	tl.Bin *= by
	tl.Blake2Fmodexpdata *= by
	tl.Blockdata *= by
	tl.Blockhash *= by
	tl.Ecdata *= by
	tl.Euc *= by
	tl.Exp *= by
	tl.Ext *= by
	tl.Gas *= by
	tl.Hub *= by
	tl.Logdata *= by
	tl.Loginfo *= by
	tl.Mmio *= by
	tl.Mmu *= by
	tl.Mod *= by
	tl.Mul *= by
	tl.Mxp *= by
	tl.Oob *= by
	tl.Rlpaddr *= by
	tl.Rlptxn *= by
	tl.Rlptxrcpt *= by
	tl.Rom *= by
	tl.Romlex *= by
	tl.Shakiradata *= by
	tl.Shf *= by
	tl.Stp *= by
	tl.Trm *= by
	tl.Txndata *= by
	tl.Wcp *= by
	tl.PrecompileEcrecoverEffectiveCalls *= by
	tl.PrecompileSha2Blocks *= by
	tl.PrecompileModexpEffectiveCalls *= by
	tl.PrecompileModexpEffectiveCalls4096 *= by
	tl.PrecompileEcaddEffectiveCalls *= by
	tl.PrecompileEcmulEffectiveCalls *= by
	tl.PrecompileEcpairingEffectiveCalls *= by
	tl.PrecompileEcpairingMillerLoops *= by
	tl.PrecompileEcpairingG2MembershipCalls *= by
	tl.BlockKeccak *= by
	tl.BlockTransactions *= by
	tl.ShomeiMerkleProofs *= by

	// Beta 4.0 internal modules
	tl.BitShl256 *= by
	tl.BitShl256U7 *= by
	tl.BitShl256U6 *= by
	tl.BitShl256U5 *= by
	tl.BitShl256U4 *= by
	tl.BitShl256U3 *= by
	tl.BitShl256U2 *= by
	tl.BitShl256U1 *= by
	tl.BitShr256 *= by
	tl.BitShr256U7 *= by
	tl.BitShr256U6 *= by
	tl.BitShr256U5 *= by
	tl.BitShr256U4 *= by
	tl.BitShr256U3 *= by
	tl.BitShr256U2 *= by
	tl.BitShr256U1 *= by
	tl.BitSar256 *= by
	tl.BitSar256U7 *= by
	tl.BitSar256U6 *= by
	tl.BitSar256U5 *= by
	tl.BitSar256U4 *= by
	tl.BitSar256U3 *= by
	tl.BitSar256U2 *= by
	tl.BitSar256U1 *= by
	tl.CallGasExtra *= by
	tl.FillBytesBetween *= by
	tl.GasOutOfPocket *= by
	tl.Log2 *= by
	tl.Log2U128 *= by
	tl.Log2U64 *= by
	tl.Log2U32 *= by
	tl.Log2U16 *= by
	tl.Log2U8 *= by
	tl.Log2U4 *= by
	tl.Log2U2 *= by
	tl.Log256 *= by
	tl.Log256U128 *= by
	tl.Log256U64 *= by
	tl.Log256U32 *= by
	tl.Log256U16 *= by
	tl.Min25664 *= by
	tl.SetByte256 *= by
	tl.SetByte128 *= by
	tl.SetByte64 *= by
	tl.SetByte32 *= by
	tl.SetByte16 *= by
	tl.U128 *= by
	tl.U127 *= by
	tl.U126 *= by
	tl.U125 *= by
	tl.U124 *= by
	tl.U123 *= by
	tl.U120 *= by
	tl.U119 *= by
	tl.U112 *= by
	tl.U111 *= by
	tl.U96 *= by
	tl.U95 *= by
	tl.U64 *= by
	tl.U63 *= by
	tl.U62 *= by
	tl.U61 *= by
	tl.U60 *= by
	tl.U59 *= by
	tl.U58 *= by
	tl.U56 *= by
	tl.U55 *= by
	tl.U48 *= by
	tl.U47 *= by
	tl.U36 *= by
	tl.U32 *= by
	tl.U31 *= by
	tl.U30 *= by
	tl.U29 *= by
	tl.U28 *= by
	tl.U27 *= by
	tl.U26 *= by
	tl.U24 *= by
	tl.U23 *= by
	tl.U20 *= by
	// beta v4.0
	tl.PrecompileBlsPointEvaluationEffectiveCalls *= by
	tl.PrecompilePointEvaluationFailureEffectiveCalls *= by
	tl.PrecompileBlsG1AddEffectiveCalls *= by
	tl.PrecompileBlsG1MsmEffectiveCalls *= by
	tl.PrecompileBlsG2AddEffectiveCalls *= by
	tl.PrecompileBlsG2MsmEffectiveCalls *= by
	tl.PrecompileBlsPairingCheckMillerLoops *= by
	tl.PrecompileBlsFinalExponentiations *= by
	tl.PrecompileBlsMapFpToG1EffectiveCalls *= by
	tl.PrecompileBlsMapFp2ToG2EffectiveCalls *= by
	tl.PrecompileBlsC1MembershipChecks *= by
	tl.PrecompileBlsC2MembershipCalls *= by
	tl.PrecompileBlsG1MembershipCalls *= by
	tl.PrecompileBlsG2MembershipCalls *= by
	tl.Bls *= by
	tl.PointEval *= by
	tl.BlsData *= by
	tl.RlpUtils *= by
}

func GetTestTracesLimits() *TracesLimits {

	// This are the config trace-limits from sepolia.
	traceLimits := &TracesLimits{
		Add:                                  262144,
		Bin:                                  262144,
		Blake2Fmodexpdata:                    16384,
		Blockdata:                            4096,
		Blockhash:                            2048,
		Ecdata:                               65536,
		Euc:                                  65536,
		Exp:                                  65536,
		Ext:                                  524288,
		Gas:                                  65536,
		Hub:                                  2097152,
		Logdata:                              65536,
		Loginfo:                              4096,
		Mmio:                                 2097152,
		Mmu:                                  1048576,
		Mod:                                  131072,
		Mul:                                  65536,
		Mxp:                                  524288,
		Oob:                                  262144,
		Rlpaddr:                              4096,
		Rlptxn:                               131072,
		Rlptxrcpt:                            65536,
		Rom:                                  8388608,
		Romlex:                               1024,
		Shakiradata:                          65536,
		Shf:                                  262144,
		Stp:                                  16384,
		Trm:                                  32768,
		Txndata:                              8192,
		Wcp:                                  262144,
		Binreftable:                          262144,
		Instdecoder:                          512,
		PrecompileEcrecoverEffectiveCalls:    128,
		PrecompileSha2Blocks:                 200,
		PrecompileRipemdBlocks:               0,
		PrecompileModexpEffectiveCalls:       32,
		PrecompileModexpEffectiveCalls4096:   1,
		PrecompileEcaddEffectiveCalls:        256,
		PrecompileEcmulEffectiveCalls:        40,
		PrecompileEcpairingEffectiveCalls:    16,
		PrecompileEcpairingMillerLoops:       64,
		PrecompileEcpairingG2MembershipCalls: 64,
		PrecompileBlakeEffectiveCalls:        0,
		PrecompileBlakeRounds:                0,
		BlockKeccak:                          8192,
		BlockL1Size:                          1000000,
		BlockL2L1Logs:                        16,
		BlockTransactions:                    300,
		ShomeiMerkleProofs:                   16384,
		// Beta 4.0 internal modules
		BitShl256:        131072,
		BitShl256U7:      131072,
		BitShl256U6:      131072,
		BitShl256U5:      131072,
		BitShl256U4:      131072,
		BitShl256U3:      131072,
		BitShl256U2:      131072,
		BitShl256U1:      131072,
		BitShr256:        131072,
		BitShr256U7:      131072,
		BitShr256U6:      131072,
		BitShr256U5:      131072,
		BitShr256U4:      131072,
		BitShr256U3:      131072,
		BitShr256U2:      131072,
		BitShr256U1:      131072,
		BitSar256:        131072,
		BitSar256U7:      131072,
		BitSar256U6:      131072,
		BitSar256U5:      131072,
		BitSar256U4:      131072,
		BitSar256U3:      131072,
		BitSar256U2:      131072,
		BitSar256U1:      131072,
		CallGasExtra:     131072,
		FillBytesBetween: 131072,
		GasOutOfPocket:   131072,
		Log2:             131072,
		Log2U128:         131072,
		Log2U64:          131072,
		Log2U32:          131072,
		Log2U16:          131072,
		Log2U8:           131072,
		Log2U4:           131072,
		Log2U2:           131072,
		Log256:           131072,
		Log256U128:       131072,
		Log256U64:        131072,
		Log256U32:        131072,
		Log256U16:        131072,
		SetByte256:       131072,
		SetByte128:       131072,
		SetByte64:        131072,
		SetByte32:        131072,
		SetByte16:        131072,
		Min25664:         131072,
		U128:             131072,
		U127:             131072,
		U126:             131072,
		U125:             131072,
		U124:             131072,
		U123:             131072,
		U120:             131072,
		U119:             131072,
		U112:             131072,
		U111:             131072,
		U96:              131072,
		U95:              131072,
		U64:              131072,
		U63:              131072,
		U62:              131072,
		U61:              131072,
		U60:              131072,
		U59:              131072,
		U58:              131072,
		U56:              131072,
		U55:              131072,
		U48:              131072,
		U47:              131072,
		U36:              131072,
		U32:              131072,
		U31:              131072,
		U30:              131072,
		U29:              131072,
		U28:              131072,
		U27:              131072,
		U26:              131072,
		U24:              131072,
		U23:              131072,
		U20:              131072,

		// beta v4.0
		PrecompileBlsPointEvaluationEffectiveCalls:     0,
		PrecompilePointEvaluationFailureEffectiveCalls: 0,
		PrecompileBlsG1AddEffectiveCalls:               0,
		PrecompileBlsG1MsmEffectiveCalls:               0,
		PrecompileBlsG2AddEffectiveCalls:               0,
		PrecompileBlsG2MsmEffectiveCalls:               0,
		PrecompileBlsPairingCheckMillerLoops:           0,
		PrecompileBlsFinalExponentiations:              0,
		PrecompileBlsMapFpToG1EffectiveCalls:           0,
		PrecompileBlsMapFp2ToG2EffectiveCalls:          0,
		PrecompileBlsC1MembershipChecks:                0,
		PrecompileBlsC2MembershipCalls:                 0,
		PrecompileBlsG1MembershipCalls:                 0,
		PrecompileBlsG2MembershipCalls:                 0,
		Bls:                                            0,
		PointEval:                                      0,
		BlsData:                                        4096,
		RlpUtils:                                       131072,
		PowerReferenceTable:                            32,
		BlsReferenceTable:                              256,
	}

	return traceLimits
}
