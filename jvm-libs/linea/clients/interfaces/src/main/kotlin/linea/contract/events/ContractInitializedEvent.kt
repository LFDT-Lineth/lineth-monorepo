package linea.contract.events

import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.sliceOf32
import linea.kotlin.toULongFromLast8Bytes

data class ContractInitializedEvent(val version: UInt) {
  companion object {
    val topic = "0x7f26b83ff96e1f2b6a682f133852f6798a09c465da95921460cefb3847402498"

    fun fromEthLog(ethLog: EthLog): EthLogEvent<ContractInitializedEvent> {
      return EthLogEvent(
        event = ContractInitializedEvent(
          version = ethLog.data.sliceOf32(sliceNumber = 0).toULongFromLast8Bytes().toUInt()
        ),
        log = ethLog
      )
    }
  }
}
