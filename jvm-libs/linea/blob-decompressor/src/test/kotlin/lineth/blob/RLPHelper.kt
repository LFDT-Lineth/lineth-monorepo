package lineth.blob

import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.rlp.RLP

internal fun rlpEncode(list: List<ByteArray>): ByteArray {
  return RLP.encode { rlpWriter ->
    rlpWriter.startList()
    list.forEach { bytes ->
      rlpWriter.writeBytes(Bytes.wrap(bytes))
    }
    rlpWriter.endList()
  }.toArray()
}

internal fun rlpDecodeAsListOfBytes(rlpEncoded: ByteArray): List<ByteArray> {
  val decodedBytes = mutableListOf<ByteArray>()
  RLP.input(Bytes.wrap(rlpEncoded), true).also { rlpInput ->
    rlpInput.enterList()
    while (!rlpInput.isEndOfCurrentList) {
      decodedBytes.add(rlpInput.readBytes().toArray())
    }
    rlpInput.leaveList()
  }
  return decodedBytes
}
