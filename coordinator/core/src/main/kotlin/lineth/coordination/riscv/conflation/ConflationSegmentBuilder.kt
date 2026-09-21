package lineth.coordination.riscv.conflation

import linea.domain.Block

fun interface ConflationSegmentBuilder {
  // TODO("implement: [4-byte-len BE][lz4(truncated_conflation)]")
  fun buildSegment(blocks: List<Block>, chainId: ULong): ByteArray
}
