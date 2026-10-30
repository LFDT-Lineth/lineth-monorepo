package lineth.coordinator.clients.prover

import com.fasterxml.jackson.annotation.JsonInclude
import linea.clients.ChunkWitness
import linea.clients.ConflationWitness
import linea.clients.FinalizationPublicInputs
import linea.clients.ForcedTransaction
import linea.clients.L2ExecutionProofPublicInputs
import linea.clients.RollupProofPublicInputs
import linea.domain.BlockInterval
import linea.domain.ExecutionPayload
import linea.ethapi.ExecutionWitness
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.kotlin.toHexString
import kotlin.time.Instant

/**
 * Request/response DTOs for the RISC-V provers.
 *
 * The wire contract is the set of V1 JSON Schemas under `rollup_spec/src/rollup_spec/prover_io/schemas/`, with golden
 * fixtures under `rollup_spec/src/rollup_spec/prover_io/testdata/`. Field names are kept identical to the JSON keys so
 * they serialize without custom naming.
 *
 * The DTO <-> domain mapping helpers live at the bottom of this file.
 */

/** The PI tuple emitted by an l2-execution proof (rollup_spec §2.1, `L2ExecutionProofPublicInput`). */
data class L2ExecutionProofPublicInputsDto(
  val parentBlockHash: String,
  val endBlockHash: String,
  val endBlockNumber: Long,
  val endBlockTimestamp: Long,
  val l2L1Messages: List<String>,
  val endL1L2BridgeRollingHash: String,
  val endL1L2BridgeRollingHashMessageNumber: Long,
  val dynamicChainConfigHash: String,
  val parentFtxRollingHash: String,
  val parentFtxNumber: Long,
  val endFtxRollingHash: String,
  val endProcessedFtxNumber: Long,
  /** Distinct, sorted ascending by byte value. */
  val filteredAddresses: List<String>,
  val txFromsHash: String,
  val blockCount: Long,
  val l2MessagingBlocksOffsets: List<Long>,
)

/** The PI tuple emitted by a rollup proof (rollup_spec §2.4, `RollupPublicInput`). */
data class RollupProofPublicInputsDto(
  val endBlockNumber: Long,
  val endBlockTimestamp: Long,
  val endL1L2BridgeRollingHash: String,
  val endL1L2BridgeRollingHashMessageNumber: Long,
  val dynamicChainConfigHash: String,
  val parentFtxRollingHash: String,
  val parentFtxNumber: Long,
  val endFtxRollingHash: String,
  val endProcessedFtxNumber: Long,
  val parentDataRollingHash: String,
  val endDataRollingHash: String,
  val parentBlockHash: String,
  val endBlockHash: String,
  val parentDataTailTakeBytes: Int,
  val finalDataTailDiscardBytes: Int,
  val l2L1Messages: List<String>,
  val filteredAddresses: List<String>,
  /** Distinct, sorted ascending by byte value. */
  val programVks: List<String>,
  val blockCount: Long,
  val l2MessagingBlocksOffsets: List<Long>,
)

data class MetaDataDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
  val startBlockTimestamp: Long,
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val endBlockTimestamp: Long? = null,
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val transactionsCount: Long? = null,
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val totalGasUsed: Long? = null,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkL2ExecutionProof.request.json (§2.1)
// ---------------------------------------------------------------------------------------------------------------------

/** One of the five `ForcedTransactionAcceptance` variants (rollup_spec §6.5). */
enum class ForcedTransactionAcceptance {
  INCLUDED,
  BAD_NONCE,
  BAD_BALANCE,
  FILTERED_ADDRESS_FROM,
  FILTERED_ADDRESS_TO,
}

/** Per-FTX metadata (rollup_spec §6.5). */
data class ForcedTransactionDto(
  val number: Long,
  val deadline: Long,
  val signedTxRlp: String,
  val acceptance: ForcedTransactionAcceptance,
)

data class WithdrawalDto(
  val index: Long,
  val validatorIndex: Long,
  val address: String,
  val amount: Long,
)

// Execution payload including Amsterdam blockAccessList and slotNumber.
data class ExecutionPayloadDto(
  val parentHash: String,
  val feeRecipient: String,
  val stateRoot: String,
  val receiptsRoot: String,
  val logsBloom: String,
  val prevRandao: String,
  val blockNumber: Long,
  val gasLimit: Long,
  val gasUsed: Long,
  val timestamp: Long,
  val extraData: String,
  val baseFeePerGas: String,
  val blockHash: String,
  val transactions: List<String>,
  val withdrawals: List<WithdrawalDto>,
  val blobGasUsed: Long,
  val excessBlobGas: Long,
  val blockAccessList: String,
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val slotNumber: Long? = null,
)

data class NewPayloadRequestDto(
  val executionPayload: ExecutionPayloadDto,
  val versionedHashes: List<String>,
  val parentBeaconBlockRoot: String,
  val executionRequests: List<String>,
)

/** System-contract addresses handed to the execution engine (not part of the `dynamicChainConfigHash` preimage). */
data class SystemContractsDto(
  val beaconRoots: String,
  val historyStorage: String,
  val deposit: String,
  val withdrawalRequest: String,
  val consolidationRequest: String,
  val builderDepositRequest: String,
  val builderExitRequest: String,
)

/** Static chain configuration (`systemContracts` aside, preimage inputs of `dynamicChainConfigHash`). */
data class ChainConfigDto(
  val l2MessageServiceAddress: String,
  val coinbase: String,
  val chainId: Long,
  val forkName: String,
  val systemContracts: SystemContractsDto,
)

/** A single `debug_executionWitness` entry, one per block in canonical order. */
data class ExecutionWitnessDto(
  val state: List<String>,
  val codes: List<String>,
  val headers: List<String>,
)

data class RollupExtensionDto(
  val forcedTransactions: List<ForcedTransactionDto>,
)

data class StatelessInputDto(
  val newPayloadRequest: NewPayloadRequestDto,
  val executionWitness: ExecutionWitnessDto,
)

data class PayloadInputDto(
  val statelessInput: StatelessInputDto,
  val rollupExtension: RollupExtensionDto,
)

data class L2ExecutionProofRequestParamsDto(
  val parentFtxRollingHash: String,
  val parentFtxNumber: Long,
  val payloads: List<PayloadInputDto>,
  val chainConfig: ChainConfigDto,
)

data class L2ExecutionProofRequestDto(
  val guestProgramId: String,
  val provingSystem: String,
  val proofRequest: L2ExecutionProofRequestParamsDto,
  val metadata: MetaDataDto,
)

data class L2ExecutionProofResponseDto(
  /**
   * Required on the `getZkL2ExecutionProofV1` response, but not allowed on the l2-execution proof embedded in a
   * `getZkRollupProofV1` request (`additionalProperties: false`). Nullable so the same DTO serves both:
   * [RollupProofRequestDtoMapper] clears it when embedding, and `NON_NULL` keeps it off the wire.
   */
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val proverVersion: String? = null,
  val startBlockNumber: Long,
  val proof: String,
  val publicInputs: L2ExecutionProofPublicInputsDto,
  val programVk: String,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkRollupProof.request.json (§2.2)
// ---------------------------------------------------------------------------------------------------------------------

data class ConflationWitnessDto(
  val blockRlps: List<String>,
)

data class BlockIntervalDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
) {
  companion object {
    fun BlockInterval.toDto() = BlockIntervalDto(
      startBlockNumber = this.startBlockNumber.toLong(),
      endBlockNumber = this.endBlockNumber.toLong(),
    )
  }
}

/**
 * One touched DA chunk (rollup_spec §3.1). Blob chunks carry `calldataBytes = "0x"`; calldata chunks carry
 * `blobBytes = "0x"`, `kzgCommitment = "0x"` and `kzgProof = "0x"`.
 */
data class ChunkWitnessDto(
  val chunkHash: String,
  val isCalldata: Boolean,
  val blobBytes: String,
  val calldataBytes: String,
  val kzgCommitment: String,
  val kzgProof: String,
)

/**
 * Exactly one of [l2ExecutionProofs] and [l2ExecutionProofIndexes] is set:
 * - **Spec mode** ([l2ExecutionProofs]): the l2-execution proofs are inlined, as `getZkRollupProofV1.request` requires.
 * - **Gateway mode** ([l2ExecutionProofIndexes]): only the proofs' block ranges are sent, for a prover gateway that
 *   resolves the proofs itself and forwards a spec request to the prover. This shape is a coordinator <-> gateway
 *   extension: it is NOT valid against the V1 schema, and the prover rejects it.
 */
data class RollupProofRequestParamsDto(
  val chainId: Long,
  val conflations: List<ConflationWitnessDto>,
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val l2ExecutionProofs: List<L2ExecutionProofResponseDto>? = null,
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val l2ExecutionProofIndexes: List<BlockIntervalDto>? = null,
  val chunks: List<ChunkWitnessDto>,
  val parentDataRollingHash: String,
  val parentDataTailTakeBytes: Int,
  /** Required only when [parentDataTailTakeBytes] > 0 (§3.4). */
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val boundaryPrevDataRollingHash: String? = null,
) {
  init {
    require((l2ExecutionProofs == null) != (l2ExecutionProofIndexes == null)) {
      "exactly one of l2ExecutionProofs (spec mode) or l2ExecutionProofIndexes (gateway mode) must be set"
    }
  }
}

data class FileBasedRollupProofRequestDto(
  val guestProgramId: String,
  val provingSystem: String,
  val proofRequest: RollupProofRequestParamsDto,
  val metadata: MetaDataDto,
)

data class RollupProofResponseDto(
  /**
   * Required on the `getZkRollupProofV1` response, but not allowed on the rollup proof embedded in a
   * `getZkRollupAggregationProofV1` request (`additionalProperties: false`). Nullable so the same DTO serves both:
   * [RollupAggregationProofRequestDtoMapper] clears it when embedding, and `NON_NULL` keeps it off the wire.
   */
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val proverVersion: String? = null,
  val programVk: String,
  val proof: String,
  val startBlockNumber: Long,
  val publicInputs: RollupProofPublicInputsDto,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkRollupAggregationProof.request.json (§2.3)
// ---------------------------------------------------------------------------------------------------------------------

/**
 * Exactly one of [rollupProofs] and [rollupProofsIndexes] is set:
 * - **Spec mode** ([rollupProofs]): the rollup proofs are inlined, as `getZkRollupAggregationProofV1.request` requires.
 * - **Gateway mode** ([rollupProofsIndexes]): only the proofs' block ranges are sent, for a prover gateway that
 *   resolves the proofs itself and forwards a spec request to the prover. This shape is a coordinator <-> gateway
 *   extension: it is NOT valid against the V1 schema, and the prover rejects it.
 */
data class FileBasedRollupAggregationProofRequestParamsDto(
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val rollupProofs: List<RollupProofResponseDto>? = null,
  @get:JsonInclude(JsonInclude.Include.NON_NULL)
  val rollupProofsIndexes: List<BlockIntervalDto>? = null,
) {
  init {
    require((rollupProofs == null) != (rollupProofsIndexes == null)) {
      "exactly one of rollupProofs (spec mode) or rollupProofsIndexes (gateway mode) must be set"
    }
  }
}

data class FileBasedRollupAggregationProofRequestDto(
  val guestProgramId: String,
  val provingSystem: String,
  val proofRequest: FileBasedRollupAggregationProofRequestParamsDto,
  val metadata: MetaDataDto,
)

/**
 * The PI tuple emitted by a rollup-aggregation proof and submitted to L1 finalization (rollup_spec §2.4,
 * `FinalizationPublicInput`). Shares its first 15 scalars with [RollupProofPublicInputsDto].
 */
data class FinalizationPublicInputsDto(
  val endBlockNumber: Long,
  val endBlockTimestamp: Long,
  val endL1L2BridgeRollingHash: String,
  val endL1L2BridgeRollingHashMessageNumber: Long,
  val dynamicChainConfigHash: String,
  val parentFtxRollingHash: String,
  val parentFtxNumber: Long,
  val endFtxRollingHash: String,
  val endProcessedFtxNumber: Long,
  val parentDataRollingHash: String,
  val endDataRollingHash: String,
  val parentBlockHash: String,
  val endBlockHash: String,
  val parentDataTailTakeBytes: Int,
  val finalDataTailDiscardBytes: Int,
  val l2L1Roots: List<String>,
  val l2L1TreeDepth: Long,
  val filteredAddresses: List<String>,
  /** Distinct, sorted ascending by byte value. */
  val programIds: List<String>,
  val l2MessagingBlocksOffsets: List<Long>,
)

/** Response of a rollup-aggregation proof: the final proof bytes plus the finalization PI tuple. */
data class RollupAggregationProofResponseDto(
  val proverVersion: String,
  val proof: String,
  val startBlockNumber: Long,
  val publicInputs: FinalizationPublicInputsDto,
)

// ---------------------------------------------------------------------------------------------------------------------
// to/fromDomainObject helper functions between the RISC-V proof DTOs (RiscvProofDtos.kt) and their domain twins.
// ---------------------------------------------------------------------------------------------------------------------

internal fun L2ExecutionProofPublicInputsDto.toDomainObject(): L2ExecutionProofPublicInputs {
  return L2ExecutionProofPublicInputs(
    parentBlockHash = parentBlockHash.decodeHex(),
    endBlockHash = endBlockHash.decodeHex(),
    endBlockNumber = endBlockNumber.toULong(),
    endBlockTimestamp = Instant.fromEpochSeconds(endBlockTimestamp),
    l2L1Messages = l2L1Messages.map { it.decodeHex() },
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    parentFtxNumber = parentFtxNumber.toULong(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    endFtxNumber = endProcessedFtxNumber.toULong(),
    filteredAddresses = filteredAddresses.map { it.decodeHex() },
    txFromsHash = txFromsHash.decodeHex(),
    blockCount = blockCount.toULong(),
    l2MessagingBlocksOffsets = l2MessagingBlocksOffsets.map { it.toULong() },
  )
}

internal fun L2ExecutionProofPublicInputs.toDto(): L2ExecutionProofPublicInputsDto {
  return L2ExecutionProofPublicInputsDto(
    parentBlockHash = parentBlockHash.encodeHex(),
    endBlockHash = endBlockHash.encodeHex(),
    endBlockNumber = endBlockNumber.toLong(),
    endBlockTimestamp = endBlockTimestamp.epochSeconds,
    l2L1Messages = l2L1Messages.map { it.encodeHex() },
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.encodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toLong(),
    dynamicChainConfigHash = dynamicChainConfigHash.encodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.encodeHex(),
    parentFtxNumber = parentFtxNumber.toLong(),
    endFtxRollingHash = endFtxRollingHash.encodeHex(),
    endProcessedFtxNumber = endFtxNumber.toLong(),
    filteredAddresses = filteredAddresses.map { it.encodeHex() },
    txFromsHash = txFromsHash.encodeHex(),
    blockCount = blockCount.toLong(),
    l2MessagingBlocksOffsets = l2MessagingBlocksOffsets.map { it.toLong() },
  )
}

internal fun ExecutionPayload.toDto(): ExecutionPayloadDto {
  return ExecutionPayloadDto(
    parentHash = parentHash.encodeHex(),
    feeRecipient = feeRecipient.encodeHex(),
    stateRoot = stateRoot.encodeHex(),
    receiptsRoot = receiptsRoot.encodeHex(),
    logsBloom = logsBloom.encodeHex(),
    prevRandao = prevRandao.encodeHex(),
    blockNumber = blockNumber.toLong(),
    gasLimit = gasLimit.toLong(),
    gasUsed = gasUsed.toLong(),
    timestamp = timestamp.toLong(),
    extraData = extraData.encodeHex(),
    baseFeePerGas = baseFeePerGas.toHexString(),
    blockHash = blockHash.encodeHex(),
    transactions = transactions.map { it.encodeHex() },
    withdrawals = withdrawals.map {
      WithdrawalDto(
        index = it.index.toLong(),
        validatorIndex = it.validatorIndex.toLong(),
        address = it.address.encodeHex(),
        amount = it.amount.toLong(),
      )
    },
    blobGasUsed = blobGasUsed.toLong(),
    excessBlobGas = excessBlobGas.toLong(),
    blockAccessList = blockAccessList.encodeHex(),
    slotNumber = slotNumber?.toLong(),
  )
}

internal fun ExecutionWitness.toDto(): ExecutionWitnessDto {
  return ExecutionWitnessDto(
    state = state.map { it.encodeHex() },
    codes = codes.map { it.encodeHex() },
    headers = headers.map { it.encodeHex() },
  )
}

private fun mapFtxInclusionResultToAcceptance(
  inclusionResult: ForcedTransactionInclusionResult,
): ForcedTransactionAcceptance {
  return when (inclusionResult) {
    ForcedTransactionInclusionResult.Included -> ForcedTransactionAcceptance.INCLUDED
    ForcedTransactionInclusionResult.BadNonce -> ForcedTransactionAcceptance.BAD_NONCE
    ForcedTransactionInclusionResult.BadBalance -> ForcedTransactionAcceptance.BAD_BALANCE
    ForcedTransactionInclusionResult.FilteredAddressFrom -> ForcedTransactionAcceptance.FILTERED_ADDRESS_FROM
    ForcedTransactionInclusionResult.FilteredAddressTo -> ForcedTransactionAcceptance.FILTERED_ADDRESS_TO
    else -> throw IllegalArgumentException("Unsupported FTX inclusion result: $inclusionResult")
  }
}

fun ForcedTransaction.toDto(): ForcedTransactionDto {
  return ForcedTransactionDto(
    number = ftxNumber.toLong(),
    deadline = deadlineBlockNumber.toLong(),
    signedTxRlp = signedTxRlp.encodeHex(),
    acceptance = mapFtxInclusionResultToAcceptance(acceptance),
  )
}

/** Maps the rollup PI tuple DTO onto its domain twin (rollup_spec §2.4). */
fun RollupProofPublicInputsDto.toDomainObject(): RollupProofPublicInputs {
  return RollupProofPublicInputs(
    endBlockNumber = endBlockNumber.toULong(),
    endBlockTimestamp = Instant.fromEpochSeconds(endBlockTimestamp),
    endL1L2BridgeMessageRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    parentFtxNumber = parentFtxNumber.toULong(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    endFtxNumber = endProcessedFtxNumber.toULong(),
    parentDataRollingHash = parentDataRollingHash.decodeHex(),
    endDataRollingHash = endDataRollingHash.decodeHex(),
    parentBlockHash = parentBlockHash.decodeHex(),
    endBlockHash = endBlockHash.decodeHex(),
    parentDataTailTakeBytes = parentDataTailTakeBytes,
    finalDataTailDiscardBytes = finalDataTailDiscardBytes,
    l2L1Messages = l2L1Messages.map { it.decodeHex() },
    filteredAddresses = filteredAddresses.map { it.decodeHex() },
    programVks = programVks.map { it.decodeHex() },
    blockCount = blockCount.toULong(),
    l2MessagingBlocksOffsets = l2MessagingBlocksOffsets.map { it.toULong() },
  )
}

/** Maps the finalization PI tuple DTO onto its domain twin (rollup_spec §2.4). */
fun FinalizationPublicInputsDto.toDomainObject(): FinalizationPublicInputs {
  return FinalizationPublicInputs(
    endBlockNumber = endBlockNumber.toULong(),
    endBlockTimestamp = Instant.fromEpochSeconds(endBlockTimestamp),
    endL1L2BridgeMessageRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    parentFtxNumber = parentFtxNumber.toULong(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    endFtxNumber = endProcessedFtxNumber.toULong(),
    parentDataRollingHash = parentDataRollingHash.decodeHex(),
    endDataRollingHash = endDataRollingHash.decodeHex(),
    parentBlockHash = parentBlockHash.decodeHex(),
    endBlockHash = endBlockHash.decodeHex(),
    parentDataTailTakeBytes = parentDataTailTakeBytes,
    finalDataTailDiscardBytes = finalDataTailDiscardBytes,
    l2L1Roots = l2L1Roots.map { it.decodeHex() },
    l2L1TreeDepth = l2L1TreeDepth.toULong(),
    filteredAddresses = filteredAddresses.map { it.decodeHex() },
    programIds = programIds.map { it.decodeHex() },
    l2MessagingBlocksOffsets = l2MessagingBlocksOffsets.map { it.toULong() },
  )
}

fun ConflationWitness.toDto(): ConflationWitnessDto {
  return ConflationWitnessDto(
    blockRlps = blockRlps.map { it.encodeHex() },
  )
}

fun ChunkWitness.toDto(): ChunkWitnessDto {
  return ChunkWitnessDto(
    chunkHash = chunkHash.encodeHex(),
    isCalldata = isCalldata,
    blobBytes = blobBytes.encodeHex(),
    calldataBytes = calldataBytes.encodeHex(),
    kzgCommitment = kzgCommitment.encodeHex(),
    kzgProof = kzgProof.encodeHex(),
  )
}
