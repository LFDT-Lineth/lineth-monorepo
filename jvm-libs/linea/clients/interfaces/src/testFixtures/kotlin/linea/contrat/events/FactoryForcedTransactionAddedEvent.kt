package linea.contrat.events

import linea.contract.events.ForcedTransactionAddedEvent
import linea.domain.EthLog
import linea.kotlin.decodeHex
import linea.kotlin.encodeHex
import linea.kotlin.padLeft
import linea.kotlin.toHexStringUInt256
import kotlin.random.Random

object FactoryForcedTransactionAddedEvent {
  fun createEthLog(
    l1BlockNumber: ULong = 100UL,
    logIndex: ULong = 0UL,
    transactionIndex: ULong = 0UL,
    contractAddress: String = Random.nextBytes(20).encodeHex(prefix = true),
    forcedTransactionNumber: ULong = 1UL,
    from: String = Random.nextBytes(20).encodeHex(prefix = true),
    blockNumberDeadline: ULong = 200UL,
    forcedTransactionRollingHash: ByteArray = Random.nextBytes(32),
    rlpEncodedSignedTransaction: ByteArray = (
      "0x02f86c0101840deadbef843b9aca0082520894deadbeefdeadbeefdeadbeef" +
        "deadbeefdeadbeef8080c080a0aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
      ).decodeHex(),
  ): EthLog {
    // ABI encoding for dynamic bytes:
    // - offset to where the data starts (0x60 = 96 bytes, after blockNumberDeadline,
    // forcedTransactionRollingHash, and offset)
    // - length of the bytes
    // - actual bytes data
    val offset = "0x0000000000000000000000000000000000000000000000000000000000000060" // 96 bytes
    val length = rlpEncodedSignedTransaction.size.toULong().toHexStringUInt256(hexPrefix = false)
    val paddedData = rlpEncodedSignedTransaction.encodeHex(prefix = false)
      .padEnd((rlpEncodedSignedTransaction.size + 31) / 32 * 64, '0') // Pad to 32-byte boundary

    return EthLog(
      removed = false,
      logIndex = logIndex,
      transactionIndex = transactionIndex,
      transactionHash = "0x2d408675b46835a04ba632ac437ca9b9ca41b834609b7453630fe594ba658b4c".decodeHex(),
      blockHash = l1BlockNumber.toHexStringUInt256().decodeHex(),
      blockNumber = l1BlockNumber,
      address = contractAddress.decodeHex(),
      data = (
        "0x" +
          blockNumberDeadline.toHexStringUInt256(hexPrefix = false) +
          forcedTransactionRollingHash.encodeHex(prefix = false) +
          offset.removePrefix("0x") +
          length +
          paddedData
        ).decodeHex(),
      topics = listOf(
        ForcedTransactionAddedEvent.topic.decodeHex(),
        forcedTransactionNumber.toHexStringUInt256().decodeHex(), // indexed forcedTransactionNumber
        from.decodeHex().padLeft(32), // indexed from address
      ),
    )
  }
}
