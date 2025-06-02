package net.consensys.linea.traces

sealed interface TracingModule {
  val name: String
}

enum class TracingModuleV2 : TracingModule {
  // EMV Module limits
  ADD,
  BIN,
  BLAKE_MODEXP_DATA,
  BLOCK_DATA,
  BLOCK_HASH,
  EC_DATA,
  EUC,
  EXP,
  EXT,
  GAS,
  HUB,
  LOG_DATA,
  LOG_INFO,
  MMIO,
  MMU,
  MOD,
  MUL,
  MXP,
  OOB,
  RLP_ADDR,
  RLP_TXN,
  RLP_TXN_RCPT,
  ROM,
  ROM_LEX,
  SHAKIRA_DATA,
  SHF,
  STP,
  TRM,
  TXN_DATA,
  WCP,

  // Reference table limits
  BIN_REFERENCE_TABLE,
  SHF_REFERENCE_TABLE,
  INSTRUCTION_DECODER,

  // Precompiles call limits
  PRECOMPILE_ECRECOVER_EFFECTIVE_CALLS,
  PRECOMPILE_SHA2_BLOCKS,
  PRECOMPILE_RIPEMD_BLOCKS,
  PRECOMPILE_MODEXP_EFFECTIVE_CALLS,
  PRECOMPILE_ECADD_EFFECTIVE_CALLS,
  PRECOMPILE_ECMUL_EFFECTIVE_CALLS,
  PRECOMPILE_ECPAIRING_FINAL_EXPONENTIATIONS,
  PRECOMPILE_ECPAIRING_G2_MEMBERSHIP_CALLS,
  PRECOMPILE_ECPAIRING_MILLER_LOOPS,
  PRECOMPILE_BLAKE_EFFECTIVE_CALLS,
  PRECOMPILE_BLAKE_ROUNDS,

  // Block-specific limits
  BLOCK_KECCAK,
  BLOCK_L1_SIZE,
  BLOCK_L2_L1_LOGS,
  BLOCK_TRANSACTIONS,
  ;

  companion object {
    val evmModules: Set<TracingModuleV2> = setOf(
      ADD,
      BIN,
      BLAKE_MODEXP_DATA,
      BLOCK_DATA,
      BLOCK_HASH,
      EC_DATA,
      EUC,
      EXP,
      EXT,
      GAS,
      HUB,
      LOG_DATA,
      LOG_INFO,
      MMIO,
      MMU,
      MOD,
      MUL,
      MXP,
      OOB,
      RLP_ADDR,
      RLP_TXN,
      RLP_TXN_RCPT,
      ROM,
      ROM_LEX,
      SHAKIRA_DATA,
      SHF,
      STP,
      TRM,
      TXN_DATA,
      WCP,
    )
  }
}
