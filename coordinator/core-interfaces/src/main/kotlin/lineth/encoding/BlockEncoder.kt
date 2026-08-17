package lineth.encoding

import linea.domain.Block

fun interface BlockEncoder {
  fun encode(block: Block): ByteArray
}
