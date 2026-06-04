package linea.clients

import linea.domain.Block
import linea.domain.BlockInterval
import linea.domain.StartBlockTimestampProvider
import linea.forcedtx.ForcedTransactionInclusionResult
import linea.kotlin.byteArrayListEquals
import kotlin.time.Instant

data class L2ExecutionProofRequestV1(
  val blocks: List<Block>,
  val forcedTransactions: List<ForcedTransaction>,
  val l2L1MessagesHash: ByteArray,
  val parentL1L2BridgeRollingHash: ByteArray,
  val parentL1L2BridgeRollingHashMessageNumber: ULong,
  val endL1L2BridgeRollingHash: ByteArray,
  val endL1L2BridgeRollingHashMessageNumber: ULong,
  val dynamicChainConfigHash: ByteArray,
  val parentFtxRollingHash: ByteArray,
  val endFtxRollingHash: ByteArray,
  val lastProcessedFtxNumber: ULong,
  val filteredAddressesHash: ByteArray,
  val txFromsHash: ByteArray,
) : BlockInterval, StartBlockTimestampProvider {
  override val startBlockNumber: ULong
    get() = blocks.first().number
  override val endBlockNumber: ULong
    get() = blocks.last().number
  override val startBlockTimestamp: Instant
    get() = blocks.first().headerSummary.timestamp

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L2ExecutionProofRequestV1

    if (blocks != other.blocks) return false
    if (forcedTransactions != other.forcedTransactions) return false
    if (!l2L1MessagesHash.contentEquals(other.l2L1MessagesHash)) return false
    if (!parentL1L2BridgeRollingHash.contentEquals(other.parentL1L2BridgeRollingHash)) return false
    if (parentL1L2BridgeRollingHashMessageNumber != other.parentL1L2BridgeRollingHashMessageNumber) return false
    if (!endL1L2BridgeRollingHash.contentEquals(other.endL1L2BridgeRollingHash)) return false
    if (endL1L2BridgeRollingHashMessageNumber != other.endL1L2BridgeRollingHashMessageNumber) return false
    if (!dynamicChainConfigHash.contentEquals(other.dynamicChainConfigHash)) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (!endFtxRollingHash.contentEquals(other.endFtxRollingHash)) return false
    if (lastProcessedFtxNumber != other.lastProcessedFtxNumber) return false
    if (!filteredAddressesHash.contentEquals(other.filteredAddressesHash)) return false
    if (!txFromsHash.contentEquals(other.txFromsHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blocks.hashCode()
    result = 31 * result + forcedTransactions.hashCode()
    result = 31 * result + l2L1MessagesHash.contentHashCode()
    result = 31 * result + parentL1L2BridgeRollingHash.contentHashCode()
    result = 31 * result + parentL1L2BridgeRollingHashMessageNumber.hashCode()
    result = 31 * result + endL1L2BridgeRollingHash.contentHashCode()
    result = 31 * result + endL1L2BridgeRollingHashMessageNumber.hashCode()
    result = 31 * result + dynamicChainConfigHash.contentHashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + endFtxRollingHash.contentHashCode()
    result = 31 * result + lastProcessedFtxNumber.hashCode()
    result = 31 * result + filteredAddressesHash.contentHashCode()
    result = 31 * result + txFromsHash.contentHashCode()
    return result
  }
}

data class ForcedTransaction(
  val ftxNumber: ULong,
  val blockNumber: ULong,
  val deadlineBlockNumber: ULong,
  val signedTxRlp: ByteArray,
  val acceptance: ForcedTransactionInclusionResult,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForcedTransaction

    if (ftxNumber != other.ftxNumber) return false
    if (blockNumber != other.blockNumber) return false
    if (deadlineBlockNumber != other.deadlineBlockNumber) return false
    if (!signedTxRlp.contentEquals(other.signedTxRlp)) return false
    if (acceptance != other.acceptance) return false

    return true
  }

  override fun hashCode(): Int {
    var result = ftxNumber.hashCode()
    result = 31 * result + blockNumber.hashCode()
    result = 31 * result + deadlineBlockNumber.hashCode()
    result = 31 * result + signedTxRlp.contentHashCode()
    result = 31 * result + acceptance.hashCode()
    return result
  }
}

/**
 * The 15-field PI tuple emitted by an l2-execution proof (rollup_spec §2.1).
 *
 * Domain twin of `linea.coordinator.clients.prover.riscv.ExecutionPublicInputsDto`. Kept here (rather than reusing
 * the DTO) because this module is depended upon by the prover-client modules, not the other way around. Field names
 * and types are identical to the DTO so the DTO -> domain mapping is a straight field copy.
 */
data class L2ExecutionProofPublicInputs(
  val parentBlockHash: ByteArray,
  val endBlockHash: ByteArray,
  val endBlockNumber: ULong,
  val endBlockTimestamp: ULong,
  val L2L1MessagesHash: ByteArray,
  val parentL1L2BridgeRollingHash: ByteArray,
  val parentL1L2BridgeRollingHashMessageNumber: ULong,
  val endL1L2BridgeRollingHash: ByteArray,
  val endL1L2BridgeRollingHashMessageNumber: ULong,
  val dynamicChainConfigHash: ByteArray,
  val parentFtxRollingHash: ByteArray,
  val endFtxRollingHash: ByteArray,
  val lastProcessedFtxNumber: ULong,
  val filteredAddressesHash: ByteArray,
  val txFromsHash: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L2ExecutionProofPublicInputs

    if (!parentBlockHash.contentEquals(other.parentBlockHash)) return false
    if (!endBlockHash.contentEquals(other.endBlockHash)) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (!L2L1MessagesHash.contentEquals(other.L2L1MessagesHash)) return false
    if (!parentL1L2BridgeRollingHash.contentEquals(other.parentL1L2BridgeRollingHash)) return false
    if (parentL1L2BridgeRollingHashMessageNumber != other.parentL1L2BridgeRollingHashMessageNumber) return false
    if (!endL1L2BridgeRollingHash.contentEquals(other.endL1L2BridgeRollingHash)) return false
    if (endL1L2BridgeRollingHashMessageNumber != other.endL1L2BridgeRollingHashMessageNumber) return false
    if (!dynamicChainConfigHash.contentEquals(other.dynamicChainConfigHash)) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (!endFtxRollingHash.contentEquals(other.endFtxRollingHash)) return false
    if (lastProcessedFtxNumber != other.lastProcessedFtxNumber) return false
    if (!filteredAddressesHash.contentEquals(other.filteredAddressesHash)) return false
    if (!txFromsHash.contentEquals(other.txFromsHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = parentBlockHash.contentHashCode()
    result = 31 * result + endBlockHash.contentHashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + L2L1MessagesHash.contentHashCode()
    result = 31 * result + parentL1L2BridgeRollingHash.contentHashCode()
    result = 31 * result + parentL1L2BridgeRollingHashMessageNumber.hashCode()
    result = 31 * result + endL1L2BridgeRollingHash.contentHashCode()
    result = 31 * result + endL1L2BridgeRollingHashMessageNumber.hashCode()
    result = 31 * result + dynamicChainConfigHash.contentHashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + endFtxRollingHash.contentHashCode()
    result = 31 * result + lastProcessedFtxNumber.hashCode()
    result = 31 * result + filteredAddressesHash.contentHashCode()
    result = 31 * result + txFromsHash.contentHashCode()
    return result
  }
}

/**
 * Response of an l2-execution proof.
 *
 * Mirrors `linea.coordinator.clients.prover.riscv.L2ExecutionProofResponseDto` field-for-field so that a proof
 * response — whether read from a JSON file or returned by a REST endpoint — deserializes into the DTO and maps
 * directly onto this domain type.
 */
data class L2ExecutionProofResponse(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val proof: ByteArray,
  val publicInputs: L2ExecutionProofPublicInputs,
  val L2L1MsgList: List<ByteArray>,
  val froms: List<ByteArray>,
  val addrs: List<ByteArray>,
) : BlockInterval {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L2ExecutionProofResponse

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!proof.contentEquals(other.proof)) return false
    if (publicInputs != other.publicInputs) return false
    if (!L2L1MsgList.byteArrayListEquals(other.L2L1MsgList)) return false
    if (!froms.byteArrayListEquals(other.froms)) return false
    if (!addrs.byteArrayListEquals(other.addrs)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + proof.contentHashCode()
    result = 31 * result + publicInputs.hashCode()
    result = 31 * result + L2L1MsgList.hashCode()
    result = 31 * result + froms.hashCode()
    result = 31 * result + addrs.hashCode()
    return result
  }
}
