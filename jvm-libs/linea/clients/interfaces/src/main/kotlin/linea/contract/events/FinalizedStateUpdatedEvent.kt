package linea.contract.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.sliceOf32
import linea.kotlin.toULongFromLast8Bytes
import kotlin.time.Instant

data class FinalizedStateUpdatedEvent(
  val blockNumber: ULong,
  val timestamp: Instant,
  val messageNumber: ULong,
  val forcedTransactionNumber: ULong,
) {
  companion object {
    const val topic = "0x32e016ccc5c33419c35caa94023fdeb75143da613fb2ac738ab736404c09fc5d"
    fun fromEthLog(ethLog: EthLog): EthLogEvent<FinalizedStateUpdatedEvent> {
      /**event FinalizedStateUpdated(
       uint256 indexed blockNumber,
       uint256 timestamp,
       uint256 messageNumber,
       uint256 forcedTransactionNumber
       );*/
      val dataBytes = ethLog.data
      return EthLogEvent(
        event = FinalizedStateUpdatedEvent(
          blockNumber = ethLog.topics[1].toULongFromLast8Bytes(),
          timestamp = Instant.fromEpochSeconds(dataBytes.sliceOf32(sliceNumber = 0).toULongFromLast8Bytes().toLong()),
          messageNumber = dataBytes.sliceOf32(sliceNumber = 1).toULongFromLast8Bytes(),
          forcedTransactionNumber = dataBytes.sliceOf32(sliceNumber = 2).toULongFromLast8Bytes(),
        ),
        log = ethLog,
      )
    }
  }
}
