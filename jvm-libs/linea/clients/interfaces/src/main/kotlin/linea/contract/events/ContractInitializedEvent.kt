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
          version = ethLog.data.sliceOf32(sliceNumber = 0).toULongFromLast8Bytes().toUInt(),
        ),
        log = ethLog,
      )
    }
  }
}

/**
 * Event emitted by ProxyAdmin when the contract implementation address is upgraded.
 *
 * This event is used in Practice by contract client to know when the contract was upgraded
 * and use it's block number as earlier search block parameter.
 */
data class Upgraded(val implementationAddress: String) {
  companion object {
    val topic = "0xbc7cd75a20ee27fd9adebab32041f755214dbc6bffa90cc0225b39da2e5c2d3b"
  }
}
