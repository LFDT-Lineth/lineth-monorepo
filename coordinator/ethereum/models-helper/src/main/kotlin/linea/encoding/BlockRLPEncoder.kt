package linea.encoding

import lineth.domain.toBesu
import lineth.rlp.RLP

object BlockRLPEncoder : BlockEncoder {
  override fun encode(block: lineth.domain.Block): ByteArray = RLP.encodeBlock(block.toBesu())
}
