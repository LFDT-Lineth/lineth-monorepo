package linea.coordinator.clients.prover.riscv

import linea.clients.BlobWitness
import linea.clients.ExecutionPayload
import linea.clients.ExecutionWitness
import linea.clients.ForcedTransaction
import linea.clients.L2ExecutionProofPublicInputs
import linea.clients.L2ExecutionProofResponseV1
import linea.clients.RollupProofPublicInputs
import linea.clients.RollupProofResponseV1
import linea.domain.CompressionProofIndex
import linea.domain.ExecutionProofIndex
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import java.math.BigInteger
import kotlin.time.Instant

/**
 * Request/response DTOs for the RISC-V provers.
 *
 * These mirror the JSON request fixtures under `rollup_spec/prover_inputs/` one-to-one, EXCLUDING the documentation
 * helper fields whose names start with an underscore (`_comment`, `_comment_*`). Field names are kept identical to
 * the JSON keys so they serialize without custom naming.
 *
 * The DTO <-> domain mapping lives in `RiscVProofDtos.kt`.
 */

/** The 15-field PI tuple emitted by a l2-execution proof (rollup_spec §2.1). */
data class L2ExecutionProofPublicInputsDto(
  val parentBlockHash: String,
  val endBlockHash: String,
  val endBlockNumber: Long,
  val endBlockTimestamp: Long,
  val l2L1MessagesHash: String,
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
  val l2L1BridgeTransactionTree: String,
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

data class MetaDataDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
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

// ExecutionPayLoadV4 (ExecutionPayloadV3 plus blockAccessList)
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
  val baseFeePerGas: BigInteger,
  val blockHash: String,
  val transactions: List<String>,
  val withdrawals: List<WithdrawalDto>,
  val blobGasUsed: Long,
  val excessBlobGas: Long,
  val blockAccessList: String,
)

data class NewPayloadRequestDto(
  val executionPayload: ExecutionPayloadDto,
  val versionedHashes: List<String>,
  val parentBeaconBlockRoot: String,
  val executionRequests: List<String>,
)

/** Static chain configuration (preimage inputs of `dynamicChainConfigHash`). */
data class ChainConfigDto(
  val l2MessageServiceAddress: String,
  val coinbase: String,
  val chainId: Long,
  val forkName: String,
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
  val parentLastProcessedFtxNumber: Long,
  val payloads: List<PayloadInputDto>,
  val chainConfig: ChainConfigDto,
)

data class L2ExecutionProofRequestDto(
  val guestProgramId: String,
  val proofRequest: L2ExecutionProofRequestParamsDto,
  val metadata: MetaDataDto,
)

data class L2ExecutionProofResponseDto(
  val proverVersion: String,
  val startBlockNumber: Long,
  val proof: String,
  val publicInputs: L2ExecutionProofPublicInputsDto,
  val l2L1Messages: List<String>,
  val txFroms: List<String>,
  val filteredAddresses: List<String>,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkRollupProof.request.json (§2.2)
// ---------------------------------------------------------------------------------------------------------------------

data class BlobWitnessDto(
  val startBlockNumber: Long,
  val endBlockNumber: Long,
  val blobHash: String,
  val blobKzgProof: String,
  val blockRlps: List<String>,
)

/** An inlined l2-execution proof consumed by the rollup guest. */
data class L2ExecutionProofDto(
  val proof: String,
  val startBlockNumber: Long,
  val publicInputs: L2ExecutionProofPublicInputsDto,
  val l2L1Messages: List<String>,
  val txFroms: List<String>,
  val filteredAddresses: List<String>,
)

data class FileBasedRollupProofRequestParamsDto(
  val chainId: Long,
  val blobs: List<BlobWitnessDto>,
  val parentShnarf: String,
  val l2ExecutionProofs: List<L2ExecutionProofDto>,
)

data class RestfulRollupProofRequestParamsDto(
  val chainId: Long,
  val blobs: List<BlobWitnessDto>,
  val parentShnarf: String,
  val l2ExecutionProofIndexes: List<ExecutionProofIndex>,
)

data class FileBasedRollupProofRequestDto(
  val guestProgramId: String,
  val proofRequest: FileBasedRollupProofRequestParamsDto,
  val metadata: MetaDataDto,
)

data class RestfulRollupProofRequestDto(
  val guestProgramId: String,
  val proofRequest: RestfulRollupProofRequestParamsDto,
  val metadata: MetaDataDto,
)

data class RollupProofResponseDto(
  val proverVersion: String,
  val startBlockNumber: Long,
  val proof: String,
  val publicInputs: RollupProofPublicInputsDto,
  val l2L1Roots: List<String>,
  val filteredAddresses: List<String>,
)

// ---------------------------------------------------------------------------------------------------------------------
// getZkRollupAggregationProof.request.json (§2.3)
// ---------------------------------------------------------------------------------------------------------------------

/** An inlined rollup proof consumed by the rollup-aggregation guest. */
data class RollupProofDto(
  val proof: String,
  val startBlockNumber: Long,
  val publicInputs: RollupProofPublicInputsDto,
  val l2L1Roots: List<String>,
  val filteredAddresses: List<String>,
)

typealias RollupAggregationPublicInputsDto = RollupProofPublicInputsDto

data class FileBasedRollupAggregationProofRequestParamsDto(
  val rollupProofs: List<RollupProofDto>,
)

data class RestfulRollupAggregationProofRequestParamsDto(
  val rollupProofIndexes: List<CompressionProofIndex>,
)

data class FileBasedRollupAggregationProofRequestDto(
  val guestProgramId: String,
  val proofRequest: FileBasedRollupAggregationProofRequestParamsDto,
  val metadata: MetaDataDto,
)

data class RestfulRollupAggregationProofRequestDto(
  val guestProgramId: String,
  val proofRequest: RestfulRollupAggregationProofRequestParamsDto,
  val metadata: MetaDataDto,
)

/** Response of a rollup-aggregation proof: the aggregated proof bytes plus the 14-field PI tuple (§2.4). */
data class RollupAggregationProofResponseDto(
  val proverVersion: String,
  val startBlockNumber: Long,
  val proof: String,
  val publicInputs: RollupAggregationPublicInputsDto,
  val l2L1Roots: List<String>,
  val filteredAddresses: List<String>,
  val l2MessagingBlocksOffsets: List<Long>,
)

// ---------------------------------------------------------------------------------------------------------------------
// to/fromDomainObject helper functions between the RISC-V proof DTOs (RiscVProofDtos.kt) and their domain twins.
// ---------------------------------------------------------------------------------------------------------------------

internal fun L2ExecutionProofPublicInputsDto.toDomainObject(): L2ExecutionProofPublicInputs {
  return L2ExecutionProofPublicInputs(
    parentBlockHash = parentBlockHash.decodeHex(),
    endBlockHash = endBlockHash.decodeHex(),
    endBlockNumber = endBlockNumber.toULong(),
    endBlockTimestamp = endBlockTimestamp.toULong(),
    l2L1MessagesHash = l2L1MessagesHash.decodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.decodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toULong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    parentFtxNumber = lastProcessedFtxNumber.toULong(),
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
    l2L1MessagesHash = l2L1MessagesHash.encodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeRollingHash.encodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toLong(),
    endL1L2BridgeRollingHash = endL1L2BridgeRollingHash.encodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeRollingHashMessageNumber.toLong(),
    dynamicChainConfigHash = dynamicChainConfigHash.encodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.encodeHex(),
    endFtxRollingHash = endFtxRollingHash.encodeHex(),
    lastProcessedFtxNumber = parentFtxNumber.toLong(),
    filteredAddressesHash = filteredAddressesHash.encodeHex(),
    txFromsHash = txFromsHash.encodeHex(),
  )
}

internal fun ExecutionPayload.fromDomainObject(): ExecutionPayloadDto {
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
    baseFeePerGas = baseFeePerGas,
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
  )
}

internal fun ExecutionWitness.fromDomainObject(): ExecutionWitnessDto {
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

internal fun ForcedTransaction.fromDomainObject(): ForcedTransactionDto {
  return ForcedTransactionDto(
    number = ftxNumber.toLong(),
    deadline = deadlineBlockNumber.toLong(),
    signedTxRlp = signedTxRlp.encodeHex(),
    acceptance = mapFtxInclusionResultToAcceptance(acceptance),
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
    l2L1BridgeTransactionTree = l2L1BridgeTransactionTree.decodeHex(),
    parentL1L2BridgeMessageRollingHash = parentL1L2BridgeRollingHash.decodeHex(),
    parentL1L2BridgeMessageNumber = parentL1L2BridgeRollingHashMessageNumber.toULong(),
    endL1L2BridgeMessageRollingHash = endL1L2BridgeRollingHash.decodeHex(),
    endL1L2BridgeMessageNumber = endL1L2BridgeRollingHashMessageNumber.toULong(),
    dynamicChainConfigHash = dynamicChainConfigHash.decodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.decodeHex(),
    endFtxRollingHash = endFtxRollingHash.decodeHex(),
    parentFtxNumber = lastProcessedFtxNumber.toULong(),
    filteredAddressesHash = filteredAddressesHash.decodeHex(),
    parentShnarf = parentShnarf.decodeHex(),
    endShnarf = endShnarf.decodeHex(),
  )
}

internal fun RollupProofPublicInputs.fromDomainObject(): RollupProofPublicInputsDto {
  return RollupProofPublicInputsDto(
    endBlockNumber = endBlockNumber.toLong(),
    endBlockTimestamp = endBlockTimestamp.epochSeconds,
    l2L1BridgeTransactionTree = l2L1BridgeTransactionTree.encodeHex(),
    parentL1L2BridgeRollingHash = parentL1L2BridgeMessageRollingHash.encodeHex(),
    parentL1L2BridgeRollingHashMessageNumber = parentL1L2BridgeMessageNumber.toLong(),
    endL1L2BridgeRollingHash = endL1L2BridgeMessageRollingHash.encodeHex(),
    endL1L2BridgeRollingHashMessageNumber = endL1L2BridgeMessageNumber.toLong(),
    dynamicChainConfigHash = dynamicChainConfigHash.encodeHex(),
    parentFtxRollingHash = parentFtxRollingHash.encodeHex(),
    endFtxRollingHash = endFtxRollingHash.encodeHex(),
    lastProcessedFtxNumber = parentFtxNumber.toLong(),
    filteredAddressesHash = filteredAddressesHash.encodeHex(),
    parentShnarf = parentShnarf.encodeHex(),
    endShnarf = endShnarf.encodeHex(),
  )
}

internal fun BlobWitness.fromDomainObject(): BlobWitnessDto {
  return BlobWitnessDto(
    startBlockNumber = startBlockNumber.toLong(),
    endBlockNumber = endBlockNumber.toLong(),
    blobHash = blobHash.encodeHex(),
    blobKzgProof = blobKzgProof.encodeHex(),
    blockRlps = blockRlps.map { blockRlp ->
      blockRlp.encodeHex()
    },
  )
}

internal fun L2ExecutionProofResponseV1.fromDomainObject(): L2ExecutionProofDto {
  return L2ExecutionProofDto(
    proof = proof.encodeHex(),
    startBlockNumber = startBlockNumber.toLong(),
    publicInputs = publicInputs.fromDomainObject(),
    l2L1Messages = l2L1Messages.map { it.encodeHex() },
    txFroms = txFroms.map { it.encodeHex() },
    filteredAddresses = filteredAddresses.map { it.encodeHex() },
  )
}

internal fun RollupProofResponseV1.fromDomainObject(): RollupProofDto {
  return RollupProofDto(
    proof = proof.encodeHex(),
    startBlockNumber = startBlockNumber.toLong(),
    publicInputs = publicInputs.fromDomainObject(),
    l2L1Roots = l2L1Roots.map { it.encodeHex() },
    filteredAddresses = filteredAddresses.map { it.encodeHex() },
  )
}
