package linea.contract.events

import linea.kotlin.encodeHex

/**
 * @notice Emitted when a forced transaction is added.
 * @param forcedTransactionNumber The indexed forced transaction number.
 * @param from The recovered signer's from address.
 * @param blockNumberDeadline The maximum expected L2 block number processing will occur by.
 * @param forcedTransactionRollingHash The computed rolling Mimc based hash.
 * @param rlpEncodedSignedTransaction The RLP encoded type 02 transaction payload including signature.
 */
data class ForcedTransactionAddedEvent(
  val forcedTransactionNumber: ULong,
  val from: ByteArray,
  val blockNumberDeadline: ULong,
  val forcedTransactionRollingHash: ByteArray,
  val rlpEncodedSignedTransaction: ByteArray,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ForcedTransactionAddedEvent

    if (forcedTransactionNumber != other.forcedTransactionNumber) return false
    if (!from.contentEquals(other.from)) return false
    if (blockNumberDeadline != other.blockNumberDeadline) return false
    if (!forcedTransactionRollingHash.contentEquals(other.forcedTransactionRollingHash)) return false
    if (!rlpEncodedSignedTransaction.contentEquals(other.rlpEncodedSignedTransaction)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = forcedTransactionNumber.hashCode()
    result = 31 * result + from.contentHashCode()
    result = 31 * result + blockNumberDeadline.hashCode()
    result = 31 * result + forcedTransactionRollingHash.contentHashCode()
    result = 31 * result + rlpEncodedSignedTransaction.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "ForcedTransactionAdded(" +
      "forcedTransactionNumber=$forcedTransactionNumber, " +
      "blockNumberDeadline=$blockNumberDeadline, " +
      "from=${from.encodeHex()}, " +
      "forcedTransactionRollingHash=${forcedTransactionRollingHash.encodeHex()}, " +
      "rlpEncodedSignedTransaction=${rlpEncodedSignedTransaction.encodeHex()})"
  }
}
