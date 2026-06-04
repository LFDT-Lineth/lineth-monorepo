package linea.coordinator.clients.prover.riscv

import com.fasterxml.jackson.annotation.JsonProperty
import linea.clients.L2ExecutionProofPublicInputs
import linea.clients.L2ExecutionProofResponse
import linea.clients.RollupProofPublicInputs
import linea.clients.RollupProofResponse
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import kotlin.time.Instant

/**
 * Request DTOs for the RISC-V provers.
 *
 * These mirror the JSON request fixtures under `rollup_spec/prover_inputs/` one-to-one, EXCLUDING the documentation
 * helper fields whose names start with an underscore (`_comment`, `_comment_*`). Field names are kept identical to
 * the JSON keys so they serialize without custom naming.
 */

/** Inclusive `[startBlockNumber, endBlockNumber]` block range. */
data class BlockRangeDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
)

/** The 15-field PI tuple emitted by an l2-execution proof (rollup_spec §2.1). */
data class L2ExecutionProofPublicInputsDto(
  val parentBlockHash: String,
  val endBlockHash: String,
  val endBlockNumber: Long,
  val endBlockTimestamp: Long,
  @get:JsonProperty("L2L1MessagesHash")
  val L2L1MessagesHash: String,
  val parentL1L2BridgeRollingHash: String,
  val parentL1L2BridgeRollingHashMessageNumber: Long,
  val endL1L2BridgeRollingHash: String,
  val endL1L2BridgeRollingHashMessageNumber: Long,
  val dynamicChainConfigHash: String,
  val parentFtxRollingHash: String,
  val endFtxRollingHash: String,
  val lastProcessedFtxNumber: Long,
  val filteredAddressesHash: String,
  val txFromsHash: String,
)

/** The 14-field PI tuple emitted by a rollup / rollup-aggregation proof (rollup_spec §2.4). */
data class RollupProofPublicInputsDto(
  val endBlockNumber: Long,
  val endBlockTimestamp: Long,
  @get:JsonProperty("L2L1BridgeTransactionTree")
  val L2L1BridgeTransactionTree: String,
  val parentL1L2BridgeRollingHash: String,
  val parentL1L2BridgeRollingHashMessageNumber: Long,
  val endL1L2BridgeRollingHash: String,
  val endL1L2BridgeRollingHashMessageNumber: Long,
  val dynamicChainConfigHash: String,
  val parentFtxRollingHash: String,
  val endFtxRollingHash: String,
  val lastProcessedFtxNumber: Long,
  val filteredAddressesHash: String,
  val parentShnarf: String,
  val endShnarf: String,
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
  val ftxNumber: Long,
  val deadlineBlockNumber: Long,
  val signedTxRlp: String,
  val acceptance: ForcedTransactionAcceptance,
)

/** A canonical RLP-encoded L2 block plus its per-block FTX metadata. */
data class L2BlockDto(
  val blockRlp: String,
  val forcedTransactions: List<ForcedTransactionDto>,
)

/** Static chain configuration (preimage inputs of `dynamicChainConfigHash`). */
data class ChainConfigDto(
  val l2MessageServiceContract: String,
  val coinbase: String,
  val chainId: Long,
)

/** A single `debug_executionWitness` entry, one per block in canonical order. */
data class ExecutionWitnessDto(
  val state: List<String>,
  val keys: List<String>,
  val codes: List<String>,
  val headers: List<String>,
)

data class L2ExecutionProofRequestDto(
  val proverVersion: String,
  val blockRange: BlockRangeDto,
  val publicInputs: L2ExecutionProofPublicInputsDto,
  val chainConfig: ChainConfigDto,
  val blocks: List<L2BlockDto>,
  val executionWitness: List<ExecutionWitnessDto>,
)

data class L2ExecutionProofResponseDto(
  val proof: String,
  val publicInputs: L2ExecutionProofPublicInputsDto,
  @get:JsonProperty("L2L1MsgList")
  val L2L1MsgList: List<String>,
  val froms: List<String>,
  val addrs: List<String>,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkRollupProof.request.json (§2.2)
// ---------------------------------------------------------------------------------------------------------------------

data class BlobInputsDto(
  val blobHash: String,
  val blobKzgProof: String,
)

data class BlobDto(
  val blobInputs: BlobInputsDto,
  val blockRange: BlockRangeDto,
  val blockRlps: List<String>,
)

data class ShnarfTransitionDto(
  val parentShnarf: String,
  val endShnarf: String,
)

/** An inlined l2-execution proof consumed by the rollup guest. */
data class InlinedL2ExecutionProofDto(
  val proof: String,
  val publicInputs: L2ExecutionProofPublicInputsDto,
  @get:JsonProperty("L2L1MsgList")
  val L2L1MsgList: List<String>,
  val froms: List<String>,
  val addrs: List<String>,
)

data class RollupProofRequestDto(
  val proverVersion: String,
  val chainId: Long,
  val blockRange: BlockRangeDto,
  val blobs: List<BlobDto>,
  val shnarfTransition: ShnarfTransitionDto,
  val l2ExecutionProofs: List<InlinedL2ExecutionProofDto>,
  val publicInputs: RollupProofPublicInputsDto,
)

data class RollupProofResponseDto(
  val proof: String,
  val publicInputs: RollupProofPublicInputsDto,
  @get:JsonProperty("L2L1Roots")
  val L2L1Roots: List<String>,
  val filteredAddresses: List<String>,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkRollupAggregationProof.request.json (§2.3)
// ---------------------------------------------------------------------------------------------------------------------

/** An inlined rollup proof consumed by the rollup-aggregation guest. */
data class InlinedRollupProofDto(
  val proof: String,
  val publicInputs: RollupProofPublicInputsDto,
  @get:JsonProperty("L2L1Roots")
  val L2L1Roots: List<String>,
  val filteredAddresses: List<String>,
)

typealias RollupAggregationPublicInputsDto = RollupProofPublicInputsDto

data class RollupAggregationProofRequestDto(
  val proverVersion: String,
  val blockRange: BlockRangeDto,
  val rollupProofs: List<InlinedRollupProofDto>,
  val expectedRollupAggregationPublicInputs: RollupAggregationPublicInputsDto,
)

/** Response of a rollup-aggregation proof: the aggregated proof bytes plus the 14-field PI tuple (§2.4). */
data class RollupAggregationProofResponseDto(
  val proof: String,
  val publicInputs: RollupAggregationPublicInputsDto,
)

// ---------------------------------------------------------------------------------------------------------------------
// to/fromDomainObject helper functions
// ---------------------------------------------------------------------------------------------------------------------

internal fun L2ExecutionProofPublicInputsDto.toDomainObject(): L2ExecutionProofPublicInputs {
  return L2ExecutionProofPublicInputs(
    parentBlockHash = parentBlockHash.decodeHex(),
    endBlockHash = endBlockHash.decodeHex(),
    endBlockNumber = endBlockNumber.toULong(),
    endBlockTimestamp = endBlockTimestamp.toULong(),
    L2L1MessagesHash = L2L1MessagesHash.decodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.decodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toULong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    lastProcessedFtxNumber = lastProcessedFtxNumber.toULong(),
    filteredAddressesHash = filteredAddressesHash.decodeHex(),
    txFromsHash = txFromsHash.decodeHex(),
  )
}

internal fun L2ExecutionProofPublicInputs.fromDomainObject(): L2ExecutionProofPublicInputsDto {
  return L2ExecutionProofPublicInputsDto(
    parentBlockHash = parentBlockHash.encodeHex(),
    endBlockHash = endBlockHash.encodeHex(),
    endBlockNumber = endBlockNumber.toLong(),
    endBlockTimestamp = endBlockTimestamp.toLong(),
    L2L1MessagesHash = L2L1MessagesHash.encodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.encodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toLong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.encodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toLong(),
    dynamicChainConfigHash = dynamicChainConfigHash.encodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.encodeHex(),
    endFtxRollingHash = endFtxRollingHash.encodeHex(),
    lastProcessedFtxNumber = lastProcessedFtxNumber.toLong(),
    filteredAddressesHash = filteredAddressesHash.encodeHex(),
    txFromsHash = txFromsHash.encodeHex(),
  )
}

/**
 * Maps the RISC-V 14-field PI tuple DTO onto its domain twin. Shared by the rollup and rollup-aggregation response
 * mappers since both emit the same tuple (rollup_spec §2.4). Field names and types are identical, so it is a straight
 * field copy.
 */
internal fun RollupProofPublicInputsDto.toDomainObject(): RollupProofPublicInputs {
  return RollupProofPublicInputs(
    endBlockNumber = endBlockNumber.toULong(),
    endBlockTimestamp = Instant.fromEpochSeconds(endBlockTimestamp),
    L2L1BridgeTransactionTree = L2L1BridgeTransactionTree.decodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.decodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toULong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    lastProcessedFtxNumber = lastProcessedFtxNumber.toULong(),
    filteredAddressesHash = filteredAddressesHash.decodeHex(),
    parentShnarf = parentShnarf.decodeHex(),
    endShnarf = endShnarf.decodeHex(),
  )
}

internal fun RollupProofPublicInputs.fromDomainObject(): RollupProofPublicInputsDto {
  return RollupProofPublicInputsDto(
    endBlockNumber = endBlockNumber.toLong(),
    endBlockTimestamp = endBlockTimestamp.epochSeconds,
    L2L1BridgeTransactionTree = L2L1BridgeTransactionTree.encodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.encodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toLong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.encodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toLong(),
    dynamicChainConfigHash = dynamicChainConfigHash.encodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.encodeHex(),
    endFtxRollingHash = endFtxRollingHash.encodeHex(),
    lastProcessedFtxNumber = lastProcessedFtxNumber.toLong(),
    filteredAddressesHash = filteredAddressesHash.encodeHex(),
    parentShnarf = parentShnarf.encodeHex(),
    endShnarf = endShnarf.encodeHex(),
  )
}

internal fun L2ExecutionProofResponse.fromDomainObject(): InlinedL2ExecutionProofDto {
  return InlinedL2ExecutionProofDto(
    proof = proof.encodeHex(),
    publicInputs = publicInputs.fromDomainObject(),
    L2L1MsgList = L2L1MsgList.map { it.encodeHex() },
    froms = froms.map { it.encodeHex() },
    addrs = addrs.map { it.encodeHex() },
  )
}

internal fun RollupProofResponse.fromDomainObject(): InlinedRollupProofDto {
  return InlinedRollupProofDto(
    proof = proof.encodeHex(),
    publicInputs = publicInputs.fromDomainObject(),
    L2L1Roots = L2L1Roots.map { it.encodeHex() },
    filteredAddresses = filteredAddresses.map { it.encodeHex() },
  )
}
