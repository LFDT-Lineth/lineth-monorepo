package linea.anchoring

import linea.kotlin.encodeHex

data class MessageNumberAndRollingHash(
  val messageNumber: ULong,
  val rollingHash: ByteArray
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as MessageNumberAndRollingHash

    if (messageNumber != other.messageNumber) return false
    if (!rollingHash.contentEquals(other.rollingHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = messageNumber.hashCode()
    result = 31 * result + rollingHash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "MessageNumberAndRollingHash(messageNumber=$messageNumber, rollingHash=${rollingHash.encodeHex()})"
  }
}
