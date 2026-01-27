package linea.contract.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.encodeHex
import linea.kotlin.sliceOf32

data class DataSubmittedV3(
  val parentShnarf: ByteArray,
  val shnarf: ByteArray,
  val finalStateRootHash: ByteArray,
) {
  companion object {
    val topic = "0x55f4c645c36aa5cd3f443d6be44d7a7a5df9d2100d7139dfc69d4289ee072319"
    fun fromEthLog(ethLog: EthLog): EthLogEvent<DataSubmittedV3> {
      // DataSubmittedV3(bytes32 parentShnarf, bytes32 indexed shnarf, bytes32 finalStateRootHash);
      return EthLogEvent(
        event = DataSubmittedV3(
          parentShnarf = ethLog.data.sliceOf32(0),
          shnarf = ethLog.topics[1],
          finalStateRootHash = ethLog.data.sliceOf32(1),
        ),
        log = ethLog,
      )
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as DataSubmittedV3

    if (!parentShnarf.contentEquals(other.parentShnarf)) return false
    if (!shnarf.contentEquals(other.shnarf)) return false
    if (!finalStateRootHash.contentEquals(other.finalStateRootHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = parentShnarf.contentHashCode()
    result = 31 * result + shnarf.contentHashCode()
    result = 31 * result + finalStateRootHash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "DataSubmittedV3(" +
      "parentShnarf=${parentShnarf.encodeHex()}," +
      " shnarf=${shnarf.encodeHex()}," +
      " finalStateRootHash=${finalStateRootHash.encodeHex()})"
  }
}
