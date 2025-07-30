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

	U128 int `mapstructure:"U128" validate:"power_of_2" corset:":u128"`
	U20  int `mapstructure:"U20" validate:"power_of_2" corset:":u20"`
	U32  int `mapstructure:"U32" validate:"power_of_2" corset:":u32"`
	U36  int `mapstructure:"U36" validate:"power_of_2" corset:":u36"`
	U64  int `mapstructure:"U64" validate:"power_of_2" corset:":u64"`

	Binreftable int `mapstructure:"BIN_REFERENCE_TABLE" validate:"power_of_2" corset:"binreftable"`
	Shfreftable int `mapstructure:"SHF_REFERENCE_TABLE" validate:"power_of_2" corset:"shfreftable"`
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
	tl.U128 *= by
	tl.U20 *= by
	tl.U32 *= by
	tl.U36 *= by
	tl.U64 *= by
}

func GetTestTracesLimits() *TracesLimits {

	// This are the config trace-limits from sepolia. All multiplied by 16.
	traceLimits := &TracesLimits{
		Add:                                  1 << 19,
		Bin:                                  1 << 18,
		Blake2Fmodexpdata:                    1 << 14,
		Blockdata:                            1 << 12,
		Blockhash:                            1 << 12,
		Ecdata:                               1 << 18,
		Euc:                                  1 << 16,
		Exp:                                  1 << 14,
		Ext:                                  1 << 20,
		Gas:                                  1 << 16,
		Hub:                                  1 << 21,
		Logdata:                              1 << 16,
		Loginfo:                              1 << 12,
		Mmio:                                 1 << 21,
		Mmu:                                  1 << 21,
		Mod:                                  1 << 17,
		Mul:                                  1 << 16,
		Mxp:                                  1 << 19,
		Oob:                                  1 << 18,
		Rlpaddr:                              1 << 12,
		Rlptxn:                               1 << 17,
		Rlptxrcpt:                            1 << 17,
		Rom:                                  1 << 22,
		Romlex:                               1 << 12,
		Shakiradata:                          1 << 15,
		Shf:                                  1 << 16,
		Stp:                                  1 << 14,
		Trm:                                  1 << 15,
		Txndata:                              1 << 14,
		Wcp:                                  1 << 18,
		Binreftable:                          1 << 20,
		Shfreftable:                          1 << 12,
		Instdecoder:                          1 << 9,
		PrecompileEcrecoverEffectiveCalls:    1 << 9,
		PrecompileSha2Blocks:                 1 << 9,
		PrecompileRipemdBlocks:               0,
		PrecompileModexpEffectiveCalls:       1 << 10,
		PrecompileModexpEffectiveCalls4096:   1 << 4,
		PrecompileEcaddEffectiveCalls:        1 << 6,
		PrecompileEcmulEffectiveCalls:        1 << 6,
		PrecompileEcpairingEffectiveCalls:    1 << 4,
		PrecompileEcpairingMillerLoops:       1 << 4,
		PrecompileEcpairingG2MembershipCalls: 1 << 4,
		PrecompileBlakeEffectiveCalls:        0,
		PrecompileBlakeRounds:                0,
		BlockKeccak:                          1 << 13,
		BlockL1Size:                          100_000,
		BlockL2L1Logs:                        16,
		BlockTransactions:                    1 << 8,
		ShomeiMerkleProofs:                   1 << 14,
		U128:                                 1 << 17,
		U20:                                  1 << 17,
		U32:                                  1 << 17,
		U36:                                  1 << 17,
		U64:                                  1 << 17,
	}

	return traceLimits
}
