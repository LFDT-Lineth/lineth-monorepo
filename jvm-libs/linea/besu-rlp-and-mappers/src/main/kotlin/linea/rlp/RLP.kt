package linea.rlp

import org.apache.tuweni.bytes.Bytes
import org.hyperledger.besu.ethereum.core.Block
import org.hyperledger.besu.ethereum.mainnet.MainnetBlockHeaderFunctions
import org.hyperledger.besu.ethereum.rlp.BytesValueRLPOutput
import org.hyperledger.besu.ethereum.rlp.RLP

object RLP {
  fun encodeBlock(besuBlock: org.hyperledger.besu.ethereum.core.Block): ByteArray {
    return besuBlock.toRlp().toArray()
  }

  fun decodeBlockWithMainnetFunctions(block: ByteArray): org.hyperledger.besu.ethereum.core.Block {
    return Block.readFrom(
      RLP.input(Bytes.wrap(block)),
      MainnetBlockHeaderFunctions()
    )
  }

  fun encodeList(list: List<ByteArray>): ByteArray {
    val encoder = BytesValueRLPOutput()
    encoder.startList()
    list.forEach {
      encoder.writeBytes(Bytes.wrap(it))
    }
    encoder.endList()
    return encoder.encoded().toArray()
  }

  fun decodeList(
    bytes: ByteArray
  ): List<ByteArray> {
    val items = mutableListOf<ByteArray>()
    val rlpInput = RLP.input(Bytes.wrap(bytes), false)
    rlpInput.enterList()
    while (!rlpInput.isEndOfCurrentList) {
      items.add(rlpInput.readBytes().toArray())
    }
    rlpInput.leaveList()
    return items
  }
}
