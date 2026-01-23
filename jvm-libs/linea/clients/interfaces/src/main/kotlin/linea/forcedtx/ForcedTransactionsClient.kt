package linea.forcedtx

import linea.kotlin.encodeHex
import tech.pegasys.teku.infrastructure.async.SafeFuture

enum class ForcedTransactionInclusionResult {
  Included,
  BadNonce,
  BadBalance,
  BadPrecompile,
  TooManyLogs,
  FilteredAddresses,
  Phylax,
}

data class ForcedTransactionInclusionStatus(
  val ftxNumber: ULong,
  val blockNumber: ULong,
  val inclusionResult: ForcedTransactionInclusionResult,
  val ftxHash: ByteArray,
  val from: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForcedTransactionInclusionStatus

    if (ftxNumber != other.ftxNumber) return false
    if (blockNumber != other.blockNumber) return false
    if (inclusionResult != other.inclusionResult) return false
    if (!ftxHash.contentEquals(other.ftxHash)) return false
    if (!from.contentEquals(other.from)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blockNumber.hashCode()
    result = 31 * result + inclusionResult.hashCode()
    result = 31 * result + ftxHash.contentHashCode()
    result = 31 * result + ftxNumber.hashCode()
    result = 31 * result + from.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "ForcedTransactionInclusionStatus(ftxNumber=$ftxNumber, blockNumber=$blockNumber, " +
      "inclusionResult=$inclusionResult, ftxHash=${ftxHash.encodeHex()}, from=${from.encodeHex()})"
  }
}

data class ForcedTransactionRequest(
  val ftxNumber: ULong,
  val deadlineBlockNumber: ULong,
  val ftxRlp: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForcedTransactionRequest

    if (ftxNumber != other.ftxNumber) return false
    if (!ftxRlp.contentEquals(other.ftxRlp)) return false
    if (deadlineBlockNumber != other.deadlineBlockNumber) return false

    return true
  }

  override fun hashCode(): Int {
    var result = ftxNumber.hashCode()
    result = 31 * result + ftxRlp.contentHashCode()
    result = 31 * result + deadlineBlockNumber.hashCode()
    return result
  }

  override fun toString(): String {
    return "ForcedTransactionRequest(" +
      "ftxNumber=$ftxNumber, " +
      "deadlineBlockNumber=$deadlineBlockNumber, " +
      "ftxRlp=${ftxRlp.encodeHex()})"
  }
}

data class ForcedTransactionResponse(
  val ftxNumber: ULong,
  val ftxHash: ByteArray?,
  val ftxError: String?,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForcedTransactionResponse

    if (ftxNumber != other.ftxNumber) return false
    if (!ftxHash.contentEquals(other.ftxHash)) return false
    if (ftxError != other.ftxError) return false

    return true
  }

  override fun hashCode(): Int {
    var result = ftxNumber.hashCode()
    result = 31 * result + (ftxHash?.contentHashCode() ?: 0)
    result = 31 * result + (ftxError?.hashCode() ?: 0)
    return result
  }

  override fun toString(): String {
    return "ForcedTransactionResponse(ftxNumber=$ftxNumber, ftxHash=${ftxHash?.encodeHex()}, ftxError=$ftxError)"
  }
}

interface ForcedTransactionsClient {
  /**
   * Sends a list of ForcedTransactionRequest that contains:
   *  ftxNumber -  L1 submission order
   *  deadlineBlockNumber - the l2 deadline block number for inclusion
   *  ftxRlp - RLP encoded signed transactions, just like eth_sendRawTransaction.
   *
   * Sequencer must evaluate transactions by their ftxNumber
   *
   * @param transactions List of forced transaction requests containing RLP encoded transactions
   * @return SafeFuture containing list of responses with transaction hashes or errors
   */
  fun lineaSendForcedRawTransaction(
    transactions: List<ForcedTransactionRequest>,
  ): SafeFuture<List<ForcedTransactionResponse>>
  fun lineaFindForcedTransactionStatus(ftxNumber: ULong): SafeFuture<ForcedTransactionInclusionStatus?>
  fun lineaGetForcedTransactionStatus(ftxNumber: ULong): SafeFuture<ForcedTransactionInclusionStatus> =
    lineaFindForcedTransactionStatus(ftxNumber).thenApply {
      it ?: throw IllegalStateException("Forced transaction not found: ftxNumber=$ftxNumber")
    }
}
