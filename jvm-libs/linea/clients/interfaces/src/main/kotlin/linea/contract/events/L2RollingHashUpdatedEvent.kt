package linea.contract.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.toULongFromLast8Bytes

/**
 * @notice Emitted after all messages are anchored on L2 and the latest message index and rolling hash stored.
 * @param messageNumber The indexed unique L1 computed indexed message number for the message.
 * @param rollingHash The indexed L1 rolling hash computed for the current message number.
 * @dev NB: This event is used to provide data to the rollup. The last messageNumber and rollingHash,
 * emitted in a rollup will be used in the public input for validating the L1->L2 messaging state transition.
event RollingHashUpdated(uint256 indexed messageNumber, bytes32 indexed rollingHash);
 */

data class L2RollingHashUpdatedEvent(
  val messageNumber: ULong, // Unique L1 computed indexed message number for the message
  val rollingHash: ByteArray // L1 rolling hash computed for the current message number
) {

  companion object {
    const val topic = "0x99b65a4301b38c09fb6a5f27052d73e8372bbe8f6779d678bfe8a41b66cce7ac"

    fun fromEthLog(ethLog: EthLog): EthLogEvent<L2RollingHashUpdatedEvent> {
      return EthLogEvent(
        event = L2RollingHashUpdatedEvent(
          messageNumber = ethLog.topics[1].toULongFromLast8Bytes(),
          rollingHash = ethLog.topics[2]
        ),
        log = ethLog
      )
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L2RollingHashUpdatedEvent

    if (messageNumber != other.messageNumber) return false
    if (!rollingHash.contentEquals(other.rollingHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = messageNumber.hashCode()
    result = 31 * result + rollingHash.contentHashCode()
    return result
  }
}
