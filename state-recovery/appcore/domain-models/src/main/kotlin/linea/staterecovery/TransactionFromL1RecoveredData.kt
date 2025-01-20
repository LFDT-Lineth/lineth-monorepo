package linea.staterecovery

import net.consensys.encodeHex
import java.math.BigInteger

data class TransactionFromL1RecoveredData(
  val type: UByte,
  val nonce: ULong,
  val maxPriorityFeePerGas: BigInteger?,
  val maxFeePerGas: BigInteger?,
  val gasPrice: BigInteger?,
  val gasLimit: ULong,
  val from: ByteArray,
  val to: ByteArray?,
  val value: BigInteger,
  val data: ByteArray?,
  val accessList: List<AccessTuple>?
) {

  data class AccessTuple(
    val address: ByteArray,
    val storageKeys: List<ByteArray>
  ) {
    override fun equals(other: Any?): Boolean {
      if (this === other) return true
      if (javaClass != other?.javaClass) return false

      other as AccessTuple

      if (!address.contentEquals(other.address)) return false
      if (storageKeys != other.storageKeys) return false

      return true
    }

    override fun hashCode(): Int {
      var result = address.contentHashCode()
      result = 31 * result + storageKeys.hashCode()
      return result
    }

    override fun toString(): String {
      return "AccessTuple(address=${address.encodeHex()}, storageKeys=$storageKeys)"
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as TransactionFromL1RecoveredData

    if (type != other.type) return false
    if (nonce != other.nonce) return false
    if (maxPriorityFeePerGas != other.maxPriorityFeePerGas) return false
    if (maxFeePerGas != other.maxFeePerGas) return false
    if (gasPrice != other.gasPrice) return false
    if (gasLimit != other.gasLimit) return false
    if (!from.contentEquals(other.from)) return false
    if (to != null) {
      if (other.to == null) return false
      if (!to.contentEquals(other.to)) return false
    } else if (other.to != null) return false
    if (value != other.value) return false
    if (data != null) {
      if (other.data == null) return false
      if (!data.contentEquals(other.data)) return false
    } else if (other.data != null) return false
    if (accessList != other.accessList) return false

    return true
  }

  override fun hashCode(): Int {
    var result = type.hashCode()
    result = 31 * result + nonce.hashCode()
    result = 31 * result + (maxPriorityFeePerGas?.hashCode() ?: 0)
    result = 31 * result + (maxFeePerGas?.hashCode() ?: 0)
    result = 31 * result + (gasPrice?.hashCode() ?: 0)
    result = 31 * result + gasLimit.hashCode()
    result = 31 * result + from.contentHashCode()
    result = 31 * result + (to?.contentHashCode() ?: 0)
    result = 31 * result + value.hashCode()
    result = 31 * result + (data?.contentHashCode() ?: 0)
    result = 31 * result + (accessList?.hashCode() ?: 0)
    return result
  }

  override fun toString(): String {
    return "TransactionL1RecoveredData(" +
      "type=$type, " +
      "nonce=$nonce, " +
      "maxPriorityFeePerGas=$maxPriorityFeePerGas, " +
      "maxFeePerGas=$maxFeePerGas, " +
      "gasPrice=$gasPrice, " +
      "gasLimit=$gasLimit, " +
      "from=${from.encodeHex()}, " +
      "to=${to?.encodeHex()}, " +
      "value=$value, " +
      "data=${data?.encodeHex()}, " +
      "accessList=$accessList)"
  }
}
