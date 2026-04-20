package linea.encoding

import linea.domain.toBesu
import linea.rlp.RLP

object BlockRLPEncoder : BlockEncoder {
  override fun encode(block: linea.domain.Block): ByteArray = RLP.encodeBlock(block.toBesu())
}
