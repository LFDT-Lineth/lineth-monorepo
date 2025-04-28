package linea.anchoring.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.encodeHex
import linea.kotlin.sliceOf32
import linea.kotlin.toULongFromLast8Bytes
import java.math.BigInteger

/**
 * @notice Emitted when a message is sent.
 * @param _from The indexed sender address of the message (msg.sender).
 * @param _to The indexed intended recipient address of the message on the other layer.
 * @param _fee The fee being being paid to deliver the message to the recipient in Wei.
 * @param _value The value being sent to the recipient in Wei.
 * @param _nonce The unique message number.
 * @param _calldata The calldata being passed to the intended recipient when being called on claiming.
 * @param _messageHash The indexed hash of the message parameters.
 * @dev _calldata has the _ because calldata is a reserved word.
 * @dev We include the message hash to save hashing costs on the rollup.
 * @dev This event is used on both L1 and L2.
event MessageSent(
 address indexed _from,
 address indexed _to,
 uint256 _fee,
 uint256 _value,
 uint256 _nonce,
 bytes _calldata,
 bytes32 indexed _messageHash
);
 */
data class MessageSentEvent(
  val messageNumber: ULong, // Unique message number
  val from: ByteArray, // Address of the sender
  val to: ByteArray, // Address of the recipient
  val fee: BigInteger, // Fee paid in Wei
  val value: BigInteger, // Value sent in Wei
  val calldata: ByteArray, // Calldata passed to the recipient
  val messageHash: ByteArray // Hash of the message parameters
) : Comparable<MessageSentEvent> {
  companion object {
    const val topic = "0xe856c2b8bd4eb0027ce32eeaf595c21b0b6b4644b326e5b7bd80a1cf8db72e6c"

    fun fromEthLog(ethLog: EthLog): EthLogEvent<MessageSentEvent> {
      return EthLogEvent(
        event = MessageSentEvent(
          messageNumber = ethLog.data.sliceOf32(sliceNumber = 2).toULongFromLast8Bytes(),
          from = ethLog.topics[1].sliceArray(12..31),
          to = ethLog.topics[2].sliceArray(12..31),
          fee = BigInteger(ethLog.data.sliceOf32(sliceNumber = 0)),
          value = BigInteger(ethLog.data.sliceOf32(sliceNumber = 1)),
          calldata = ethLog.data.sliceArray(32 * 3..ethLog.data.size - 1),
          messageHash = ethLog.topics[3]
        ),
        log = ethLog
      )
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as MessageSentEvent

    if (!from.contentEquals(other.from)) return false
    if (!to.contentEquals(other.to)) return false
    if (fee != other.fee) return false
    if (value != other.value) return false
    if (messageNumber != other.messageNumber) return false
    if (!calldata.contentEquals(other.calldata)) return false
    if (!messageHash.contentEquals(other.messageHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = from.contentHashCode()
    result = 31 * result + to.contentHashCode()
    result = 31 * result + fee.hashCode()
    result = 31 * result + value.hashCode()
    result = 31 * result + messageNumber.hashCode()
    result = 31 * result + calldata.contentHashCode()
    result = 31 * result + messageHash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "MessageSentEvent(" +
      "messageNumber=$messageNumber, " +
      "from=${from.encodeHex()}, " +
      "to=${to.encodeHex()}, " +
      "fee=$fee, " +
      "value=$value, " +
      "calldata=${calldata.encodeHex()}, " +
      "messageHash=${messageHash.encodeHex()}" +
      ")"
  }

  override fun compareTo(other: MessageSentEvent): Int {
    return messageNumber.compareTo(other.messageNumber)
  }
}
