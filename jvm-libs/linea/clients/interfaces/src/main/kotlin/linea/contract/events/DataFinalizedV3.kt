package linea.contract.events

import linea.domain.BlockInterval
import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.encodeHex
import linea.kotlin.sliceOf32
import linea.kotlin.toULongFromLast8Bytes

data class DataFinalizedV3(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val shnarf: ByteArray,
  val parentStateRootHash: ByteArray,
  val finalStateRootHash: ByteArray,
) : BlockInterval {
  companion object {
    val topic = "0xa0262dc79e4ccb71ceac8574ae906311ae338aa4a2044fd4ec4b99fad5ab60cb"
    fun fromEthLog(ethLog: EthLog): EthLogEvent<DataFinalizedV3> {
      /**event DataFinalizedV3(
       uint256 indexed startBlockNumber,
       uint256 indexed endBlockNumber,
       bytes32 indexed shnarf,
       bytes32 parentStateRootHash,
       bytes32 finalStateRootHash
       );*/
      val dataBytes = ethLog.data
      return EthLogEvent(
        event = DataFinalizedV3(
          startBlockNumber = ethLog.topics[1].toULongFromLast8Bytes(),
          endBlockNumber = ethLog.topics[2].toULongFromLast8Bytes(),
          shnarf = ethLog.topics[3],
          parentStateRootHash = dataBytes.sliceOf32(sliceNumber = 0),
          finalStateRootHash = dataBytes.sliceOf32(sliceNumber = 1),
        ),
        log = ethLog,
      )
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as DataFinalizedV3

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!shnarf.contentEquals(other.shnarf)) return false
    if (!parentStateRootHash.contentEquals(other.parentStateRootHash)) return false
    if (!finalStateRootHash.contentEquals(other.finalStateRootHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + shnarf.contentHashCode()
    result = 31 * result + parentStateRootHash.contentHashCode()
    result = 31 * result + finalStateRootHash.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "DataFinalizedV3(" +
      "startBlockNumber=$startBlockNumber, " +
      "endBlockNumber=$endBlockNumber, " +
      "snarf=${shnarf.encodeHex()}, " +
      "parentStateRootHash=${parentStateRootHash.encodeHex()}, " +
      "finalStateRootHash=${finalStateRootHash.encodeHex()})"
  }
}
