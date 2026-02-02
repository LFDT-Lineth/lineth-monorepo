package linea.contrat.events

import linea.contract.events.FinalizedStateUpdatedEvent
import linea.domain.EthLog
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.kotlin.toHexStringUInt256
import kotlin.random.Random

object FactoryFinalizedStateUpdatedEvent {
  fun createEthLog(
    blockNumber: ULong = 100UL,
    logIndex: ULong = 0UL,
    transactionIndex: ULong = 0UL,
    contractAddress: String = Random.nextBytes(20).encodeHex(prefix = true),
    l2FinalizedBlockNumber: ULong = 100UL,
    timestamp: ULong = 1000000UL,
    messageNumber: ULong = 50UL,
    forcedTransactionNumber: ULong = 10UL,
  ): EthLog {
    return EthLog(
      removed = false,
      logIndex = logIndex,
      transactionIndex = transactionIndex,
      transactionHash = "0x2d408675b46835a04ba632ac437ca9b9ca41b834609b7453630fe594ba658b4c".decodeHex(),
      blockHash = blockNumber.toHexStringUInt256().decodeHex(),
      blockNumber = blockNumber,
      address = contractAddress.decodeHex(),
      data = (
        "0x" +
          timestamp.toHexStringUInt256(hexPrefix = false) +
          messageNumber.toHexStringUInt256(hexPrefix = false) +
          forcedTransactionNumber.toHexStringUInt256(hexPrefix = false)
        ).decodeHex(),
      topics = listOf(
        FinalizedStateUpdatedEvent.topic.decodeHex(),
        l2FinalizedBlockNumber.toHexStringUInt256().decodeHex(), // indexed blockNumber
      ),
    )
  }
}
