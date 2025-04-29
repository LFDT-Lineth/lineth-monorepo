package linea.contract.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.encodeHex
import linea.kotlin.toULongFromLast8Bytes

/**
 * @notice Emitted when a new message is sent and the rolling hash updated.
 * @param messageNumber The unique indexed message number for the message.
 * @param rollingHash The indexed rolling hash computed for the current message number.
 * @param messageHash The indexed hash of the message parameters.

event RollingHashUpdated(uint256 indexed messageNumber, bytes32 indexed rollingHash, bytes32 indexed messageHash);
 */
data class L1RollingHashUpdatedEvent(
  val messageNumber: ULong, // Unique indexed message number for the message
  val rollingHash: ByteArray, // Rolling hash computed for the current message number
  val messageHash: ByteArray // Hash of the message parameters
) {

  companion object {
    const val topic = "0xea3b023b4c8680d4b4824f0143132c95476359a2bb70a81d6c5a36f6918f6339"

    fun fromEthLog(ethLog: EthLog): EthLogEvent<L1RollingHashUpdatedEvent> {
      return EthLogEvent(
        event = L1RollingHashUpdatedEvent(
          messageNumber = ethLog.topics[1].toULongFromLast8Bytes(),
          rollingHash = ethLog.topics[2],
          messageHash = ethLog.topics[3]
        ),
        log = ethLog
      )
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L1RollingHashUpdatedEvent

    if (messageNumber != other.messageNumber) return false
    if (!rollingHash.contentEquals(other.rollingHash)) return false
    if (!messageHash.contentEquals(other.messageHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = messageNumber.hashCode()
    result = 31 * result + rollingHash.contentHashCode()
    result = 31 * result + messageHash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "L1RollingHashUpdatedEvent(" +
      "messageNumber=$messageNumber, " +
      "rollingHash=${rollingHash.encodeHex()}, " +
      "messageHash=${messageHash.encodeHex()})"
  }
}
