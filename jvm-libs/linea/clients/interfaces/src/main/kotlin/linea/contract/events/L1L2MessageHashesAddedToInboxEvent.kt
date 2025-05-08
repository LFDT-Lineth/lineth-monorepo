package linea.contract.events

data class L1L2MessageHashesAddedToInboxEvent(
  val messageHashesRlpEncoded: ByteArray
) {
  companion object {
    const val topic = "0x9995fb3da0c2de4012f2b814b6fc29ce7507571dcb20b8d0bd38621a842df1eb"
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as L1L2MessageHashesAddedToInboxEvent

    return messageHashesRlpEncoded.contentEquals(other.messageHashesRlpEncoded)
  }

  override fun hashCode(): Int {
    return messageHashesRlpEncoded.contentHashCode()
  }
}
