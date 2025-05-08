package linea.contrat.events

import linea.contract.events.L1RollingHashUpdatedEvent
import linea.contract.events.L2RollingHashUpdatedEvent
import linea.contract.events.MessageSentEvent
import linea.domain.EthLog
import linea.domain.EthLogEvent
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.kotlin.padLeft
import linea.kotlin.toHexStringUInt256
import kotlin.random.Random

fun createMessageSentEthLogV1(
  blockNumber: ULong = 100UL,
  logIndex: ULong = 0UL,
  transactionIndex: ULong = 0UL,
  contractAddress: String = Random.nextBytes(20).encodeHex(prefix = true),
  from: String = Random.nextBytes(20).encodeHex(prefix = true),
  to: String = Random.nextBytes(20).encodeHex(prefix = true),
  fee: ULong = 1_000UL,
  value: ULong = 2_000UL,
  messageNumber: ULong = 10_000UL,
  calldata: ByteArray = "deadbeef".decodeHex(),
  messageHash: ByteArray = Random.nextBytes(32)
): EthLog {
  return EthLog(
    removed = false,
    logIndex = logIndex,
    transactionIndex = transactionIndex,
    transactionHash = "0x2d408675b46835a04ba632ac437ca9b9ca41b834609b7453630fe594ba658b4c".decodeHex(),
    blockHash = "0x4d63489ac2faee706cca0f078f23973facc42a87dc75cfdf6fae5ac2d8c9b243".decodeHex(),
    blockNumber = blockNumber,
    address = contractAddress.decodeHex(),
    data = (
      "0x" +
        fee.toHexStringUInt256(hexPrefix = false) +
        value.toHexStringUInt256(hexPrefix = false) +
        messageNumber.toHexStringUInt256(hexPrefix = false) +
        calldata.encodeHex(prefix = false) // calldata
      ).decodeHex(),
    topics = listOf(
      MessageSentEvent.topic.decodeHex(),
      from.decodeHex().padLeft(32), // from
      to.decodeHex().padLeft(32), // to
      messageHash // messageHash
    )
  )
}

fun createL1RollingHashUpdatedEthLogV1(
  blockNumber: ULong,
  logIndex: ULong = 0UL,
  transactionIndex: ULong = 0UL,
  contractAddress: String,
  messageNumber: ULong = 10_000UL,
  rollingHash: ByteArray = Random.nextBytes(32),
  messageHash: ByteArray = Random.nextBytes(32),
  transactionHash: ByteArray = Random.nextBytes(32),
  blockHash: ByteArray = Random.nextBytes(32)
): EthLog {
  return EthLog(
    removed = false,
    logIndex = logIndex,
    transactionIndex = transactionIndex,
    transactionHash = transactionHash,
    blockHash = blockHash,
    blockNumber = blockNumber,
    address = contractAddress.decodeHex(),
    data = "0x".decodeHex(),
    topics = listOf(
      L1RollingHashUpdatedEvent.topic.decodeHex(), // topic is static
      messageNumber.toHexStringUInt256().decodeHex(),
      rollingHash,
      messageHash
    )
  )
}

data class L1MessageSentV1EthLogs(
  val messageSent: EthLogEvent<MessageSentEvent>,
  val l1RollingHashUpdated: EthLogEvent<L1RollingHashUpdatedEvent>
)

fun createL1MessageSentV1Logs(
  blockNumber: ULong = 100UL,
  logIndex: ULong = 0UL,
  transactionIndex: ULong = 0UL,
  contractAddress: String,
  from: String = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa01",
  to: String = "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa02",
  fee: ULong = 1_000UL,
  value: ULong = 2_000UL,
  messageNumber: ULong = 10_000UL,
  calldata: ByteArray = "deadbeef".decodeHex(),
  messageHash: ByteArray,
  rollingHash: ByteArray
): L1MessageSentV1EthLogs {
  val l1RollingHashUpdated = createL1RollingHashUpdatedEthLogV1(
    blockNumber = blockNumber,
    logIndex = logIndex,
    transactionIndex = transactionIndex,
    contractAddress = contractAddress,
    messageNumber = messageNumber,
    rollingHash = rollingHash,
    messageHash = messageHash
  )
  val messageSent = createMessageSentEthLogV1(
    blockNumber = blockNumber,
    logIndex = logIndex + 1UL, // MessageSent is emitted after L1RollingHashUpdated
    transactionIndex = transactionIndex,
    contractAddress = contractAddress,
    from = from,
    to = to,
    fee = fee,
    value = value,
    messageNumber = messageNumber,
    calldata = calldata,
    messageHash = messageHash
  )

  return L1MessageSentV1EthLogs(
    messageSent = MessageSentEvent.fromEthLog(messageSent),
    l1RollingHashUpdated = L1RollingHashUpdatedEvent.fromEthLog(l1RollingHashUpdated)
  )
}

fun createL2RollingHashUpdatedEthLogV1(
  blockNumber: ULong,
  logIndex: ULong = 0UL,
  transactionIndex: ULong = 0UL,
  contractAddress: String = Random.nextBytes(20).encodeHex(prefix = true),
  messageNumber: ULong = 10_000UL,
  rollingHash: ByteArray = Random.nextBytes(32),
  transactionHash: ByteArray = Random.nextBytes(32),
  blockHash: ByteArray = Random.nextBytes(32)
): EthLog {
  return EthLog(
    removed = false,
    logIndex = logIndex,
    transactionIndex = transactionIndex,
    transactionHash = transactionHash,
    blockHash = blockHash,
    blockNumber = blockNumber,
    address = contractAddress.decodeHex(),
    data = "0x".decodeHex(),
    topics = listOf(
      L2RollingHashUpdatedEvent.topic.decodeHex(), // topic is static
      messageNumber.toHexStringUInt256().decodeHex(),
      rollingHash
    )
  )
}
