package linea.encoding

import lineth.domain.Block

fun interface BlockEncoder {
  fun encode(block: Block): ByteArray
}
