package linea.clients

import linea.domain.BlockInterval
import linea.domain.ExecutionPayload
import linea.domain.ProofRequestMetaDataProvider
import linea.domain.StartBlockTimestampProvider
import linea.ethapi.ExecutionWitness
import linea.ethapi.SystemContracts
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import linea.kotlin.byteArrayListToHexString
import linea.kotlin.encodeHex
import kotlin.time.Instant

data class ExecutionInfo(
  val blockNumber: ULong,
  val executionPayload: ExecutionPayload,
  val executionWitness: ExecutionWitness,
  val executionRequests: List<ByteArray>,
  val forcedTransactions: List<ForcedTransaction>,
  val parentBeaconBlockRoot: ByteArray = ByteArray(32),
) {
  init {
    require(parentBeaconBlockRoot.size == 32) { "parentBeaconBlockRoot must be 32 bytes" }
  }
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ExecutionInfo

    if (blockNumber != other.blockNumber) return false
    if (executionPayload != other.executionPayload) return false
    if (executionWitness != other.executionWitness) return false
    if (!executionRequests.byteArrayListEquals(other.executionRequests)) return false
    if (forcedTransactions != other.forcedTransactions) return false
    if (!parentBeaconBlockRoot.contentEquals(other.parentBeaconBlockRoot)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blockNumber.hashCode()
    result = 31 * result + executionPayload.hashCode()
    result = 31 * result + executionWitness.hashCode()
    result = 31 * result + executionRequests.byteArrayListHashCode()
    result = 31 * result + forcedTransactions.hashCode()
    result = 31 * result + parentBeaconBlockRoot.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "ExecutionInfo(blockNumber=$blockNumber, " +
      "executionPayload=$executionPayload, " +
      "executionWitness=$executionWitness, " +
      "executionRequests=${executionRequests.byteArrayListToHexString()}, " +
      "forcedTransactions=$forcedTransactions, " +
      "parentBeaconBlockRoot=${parentBeaconBlockRoot.encodeHex()})"
  }
}

data class L2ExecutionProofRequestV1(
  val executions: List<ExecutionInfo>,
  val chainId: ULong,
  val coinbase: String,
  val systemContracts: SystemContracts,
  val parentFtxRollingHash: ByteArray,
  val parentFtxNumber: ULong,
) : BlockInterval, StartBlockTimestampProvider, ProofRequestMetaDataProvider {
  init {
    require(executions.isNotEmpty()) { "executions must not be empty" }
    require(
      executions.zipWithNext().all { (current, next) ->
        next.blockNumber == current.blockNumber + 1UL
      },
    ) {
      "executions must be sorted ascending and consecutive"
    }
  }

  override val startBlockNumber: ULong
    get() = executions.first().blockNumber
  override val endBlockNumber: ULong
    get() = executions.last().blockNumber
  override val startBlockTimestamp: Instant
    get() = Instant.fromEpochSeconds(executions.first().executionPayload.timestamp.toLong())
  override val endBlockTimestamp: Instant
    get() = Instant.fromEpochSeconds(executions.last().executionPayload.timestamp.toLong())
  override val transactionsCount: Long
    get() = executions.sumOf { it.executionPayload.transactions.size.toLong() }
  override val totalGasUsed: Long
    get() = executions.sumOf { it.executionPayload.gasUsed.toLong() }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L2ExecutionProofRequestV1

    if (executions != other.executions) return false
    if (chainId != other.chainId) return false
    if (coinbase != other.coinbase) return false
    if (systemContracts != other.systemContracts) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (parentFtxNumber != other.parentFtxNumber) return false

    return true
  }

  override fun hashCode(): Int {
    var result = executions.hashCode()
    result = 31 * result + chainId.hashCode()
    result = 31 * result + coinbase.hashCode()
    result = 31 * result + systemContracts.hashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + parentFtxNumber.hashCode()
    return result
  }

  override fun toString(): String {
    return "L2ExecutionProofRequestV1(executions=$executions, chainId=$chainId, coinbase=$coinbase, " +
      "systemContracts=$systemContracts, " +
      "parentFtxRollingHash=${parentFtxRollingHash.encodeHex()}, parentFtxNumber=$parentFtxNumber)"
  }
}

data class ForcedTransaction(
  val ftxNumber: ULong,
  val deadlineBlockNumber: ULong,
  val signedTxRlp: ByteArray,
  val acceptance: ForcedTransactionInclusionResult,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForcedTransaction

    if (ftxNumber != other.ftxNumber) return false
    if (deadlineBlockNumber != other.deadlineBlockNumber) return false
    if (!signedTxRlp.contentEquals(other.signedTxRlp)) return false
    if (acceptance != other.acceptance) return false

    return true
  }

  override fun hashCode(): Int {
    var result = ftxNumber.hashCode()
    result = 31 * result + deadlineBlockNumber.hashCode()
    result = 31 * result + signedTxRlp.contentHashCode()
    result = 31 * result + acceptance.hashCode()
    return result
  }

  override fun toString(): String {
    return "ForcedTransaction(ftxNumber=$ftxNumber, deadlineBlockNumber=$deadlineBlockNumber, " +
      "signedTxRlp=${signedTxRlp.encodeHex()}, acceptance=$acceptance)"
  }
}

/**
 * The PI tuple emitted by an l2-execution proof (rollup_spec §2.1, `L2ExecutionProofPublicInput`).
 *
 * Domain twin of `lineth.coordinator.clients.prover.L2ExecutionProofPublicInputsDto`. Kept here (rather than reusing
 * the DTO) because this module is depended upon by the prover-client modules, not the other way around. Where the DTO
 * uses `String` (hex) this uses `ByteArray`, and where the DTO uses `Long` this uses `ULong`.
 */
data class L2ExecutionProofPublicInputs(
  val parentBlockHash: ByteArray,
  val endBlockHash: ByteArray,
  val endBlockNumber: ULong,
  val endBlockTimestamp: Instant,
  val l2L1Messages: List<ByteArray>,
  val endL1L2BridgeRollingHash: ByteArray,
  val endL1L2BridgeRollingHashMessageNumber: ULong,
  val dynamicChainConfigHash: ByteArray,
  val parentFtxRollingHash: ByteArray,
  val parentFtxNumber: ULong,
  val endFtxRollingHash: ByteArray,
  val endFtxNumber: ULong,
  val filteredAddresses: List<ByteArray>,
  val txFromsHash: ByteArray,
  val blockCount: ULong,
  val l2MessagingBlocksOffsets: List<ULong>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L2ExecutionProofPublicInputs

    if (!parentBlockHash.contentEquals(other.parentBlockHash)) return false
    if (!endBlockHash.contentEquals(other.endBlockHash)) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (!l2L1Messages.byteArrayListEquals(other.l2L1Messages)) return false
    if (!endL1L2BridgeRollingHash.contentEquals(other.endL1L2BridgeRollingHash)) return false
    if (endL1L2BridgeRollingHashMessageNumber != other.endL1L2BridgeRollingHashMessageNumber) return false
    if (!dynamicChainConfigHash.contentEquals(other.dynamicChainConfigHash)) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (parentFtxNumber != other.parentFtxNumber) return false
    if (!endFtxRollingHash.contentEquals(other.endFtxRollingHash)) return false
    if (endFtxNumber != other.endFtxNumber) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false
    if (!txFromsHash.contentEquals(other.txFromsHash)) return false
    if (blockCount != other.blockCount) return false
    if (l2MessagingBlocksOffsets != other.l2MessagingBlocksOffsets) return false

    return true
  }

  override fun hashCode(): Int {
    var result = parentBlockHash.contentHashCode()
    result = 31 * result + endBlockHash.contentHashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + l2L1Messages.byteArrayListHashCode()
    result = 31 * result + endL1L2BridgeRollingHash.contentHashCode()
    result = 31 * result + endL1L2BridgeRollingHashMessageNumber.hashCode()
    result = 31 * result + dynamicChainConfigHash.contentHashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + parentFtxNumber.hashCode()
    result = 31 * result + endFtxRollingHash.contentHashCode()
    result = 31 * result + endFtxNumber.hashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    result = 31 * result + txFromsHash.contentHashCode()
    result = 31 * result + blockCount.hashCode()
    result = 31 * result + l2MessagingBlocksOffsets.hashCode()
    return result
  }
}

/**
 * Response of a l2-execution proof.
 *
 * Mirrors `lineth.coordinator.clients.prover.L2ExecutionProofResponseDto` field-for-field so that a proof
 * response — whether read from a JSON file or returned by a REST endpoint — deserializes into the DTO and maps
 * directly onto this domain type.
 */
data class L2ExecutionProofResponseV1(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val proof: ByteArray,
  val publicInputs: L2ExecutionProofPublicInputs,
  val programVk: ByteArray,
) : BlockInterval {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L2ExecutionProofResponseV1

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!proof.contentEquals(other.proof)) return false
    if (publicInputs != other.publicInputs) return false
    if (!programVk.contentEquals(other.programVk)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + proof.contentHashCode()
    result = 31 * result + publicInputs.hashCode()
    result = 31 * result + programVk.contentHashCode()
    return result
  }
}
