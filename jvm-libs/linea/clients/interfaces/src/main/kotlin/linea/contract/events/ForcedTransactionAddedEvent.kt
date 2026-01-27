package linea.contract.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.encodeHex
import linea.kotlin.sliceOf32
import linea.kotlin.toULongFromLast8Bytes
import java.math.BigInteger

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
  companion object {
    const val topic = "0x8fbc8fbd65675eb32c567d4a559963c7d002c2be67b5b266fb13d85b4375fce5"

    fun fromEthLog(ethLog: EthLog): EthLogEvent<ForcedTransactionAddedEvent> {
      /**event ForcedTransactionAdded(
       uint256 indexed forcedTransactionNumber,
       address indexed from,
       uint256 blockNumberDeadline,
       bytes32 forcedTransactionRollingHash,
       bytes rlpEncodedSignedTransaction
       );*/
      val dataBytes = ethLog.data
      // ABI encoding: [blockNumberDeadline 32][forcedTransactionRollingHash 32][offset 32][length 32][data...]
      val offset = BigInteger(dataBytes.sliceOf32(sliceNumber = 2)).toInt()
      val length = BigInteger(dataBytes.sliceArray(offset until offset + 32)).toInt()

      return EthLogEvent(
        event = ForcedTransactionAddedEvent(
          forcedTransactionNumber = ethLog.topics[1].toULongFromLast8Bytes(),
          from = ethLog.topics[2].sliceArray(12..31), // Address is 20 bytes, padded to 32 in topics
          blockNumberDeadline = dataBytes.sliceOf32(sliceNumber = 0).toULongFromLast8Bytes(),
          forcedTransactionRollingHash = dataBytes.sliceOf32(sliceNumber = 1),
          rlpEncodedSignedTransaction = dataBytes.sliceArray(offset + 32 until offset + 32 + length),
        ),
        log = ethLog,
      )
    }
  }

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
