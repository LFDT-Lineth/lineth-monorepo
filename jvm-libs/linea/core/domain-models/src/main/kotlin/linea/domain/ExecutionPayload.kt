package linea.domain

import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import java.math.BigInteger

data class Withdrawal(
  val index: ULong,
  val validatorIndex: ULong,
  val address: ByteArray,
  val amount: ULong,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as Withdrawal

    if (index != other.index) return false
    if (validatorIndex != other.validatorIndex) return false
    if (!address.contentEquals(other.address)) return false
    if (amount != other.amount) return false

    return true
  }

  override fun hashCode(): Int {
    var result = index.hashCode()
    result = 31 * result + validatorIndex.hashCode()
    result = 31 * result + address.contentHashCode()
    result = 31 * result + amount.hashCode()
    return result
  }
}

/**
 * Execution PayLoad V4 (Payload V3 + blockAccessList) for the Engine API and Beacon Block
 * Should retrieve by using eth_getBlockByNumber/eth_getBlockByHash or debug_getRawBlock
 */
data class ExecutionPayload(
  val parentHash: ByteArray,
  val feeRecipient: ByteArray,
  val stateRoot: ByteArray,
  val receiptsRoot: ByteArray,
  val logsBloom: ByteArray,
  val prevRandao: ByteArray,
  val blockNumber: ULong,
  val gasLimit: ULong,
  val gasUsed: ULong,
  val timestamp: ULong,
  val extraData: ByteArray,
  val baseFeePerGas: BigInteger,
  val blockHash: ByteArray,
  val transactions: List<ByteArray>,
  val withdrawals: List<Withdrawal>,
  val blobGasUsed: ULong,
  val excessBlobGas: ULong,
  val blockAccessList: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ExecutionPayload

    if (!parentHash.contentEquals(other.parentHash)) return false
    if (!feeRecipient.contentEquals(other.feeRecipient)) return false
    if (!stateRoot.contentEquals(other.stateRoot)) return false
    if (!receiptsRoot.contentEquals(other.receiptsRoot)) return false
    if (!logsBloom.contentEquals(other.logsBloom)) return false
    if (!prevRandao.contentEquals(other.prevRandao)) return false
    if (blockNumber != other.blockNumber) return false
    if (gasLimit != other.gasLimit) return false
    if (gasUsed != other.gasUsed) return false
    if (timestamp != other.timestamp) return false
    if (!extraData.contentEquals(other.extraData)) return false
    if (baseFeePerGas != other.baseFeePerGas) return false
    if (!blockHash.contentEquals(other.blockHash)) return false
    if (!transactions.byteArrayListEquals(other.transactions)) return false
    if (withdrawals != other.withdrawals) return false
    if (blobGasUsed != other.blobGasUsed) return false
    if (excessBlobGas != other.excessBlobGas) return false
    if (!blockAccessList.contentEquals(other.blockAccessList)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = parentHash.contentHashCode()
    result = 31 * result + feeRecipient.contentHashCode()
    result = 31 * result + stateRoot.contentHashCode()
    result = 31 * result + receiptsRoot.contentHashCode()
    result = 31 * result + logsBloom.contentHashCode()
    result = 31 * result + prevRandao.contentHashCode()
    result = 31 * result + blockNumber.hashCode()
    result = 31 * result + gasLimit.hashCode()
    result = 31 * result + gasUsed.hashCode()
    result = 31 * result + timestamp.hashCode()
    result = 31 * result + extraData.contentHashCode()
    result = 31 * result + baseFeePerGas.hashCode()
    result = 31 * result + blockHash.contentHashCode()
    result = 31 * result + transactions.byteArrayListHashCode()
    result = 31 * result + withdrawals.hashCode()
    result = 31 * result + blobGasUsed.hashCode()
    result = 31 * result + excessBlobGas.hashCode()
    result = 31 * result + blockAccessList.contentHashCode()
    return result
  }
}
