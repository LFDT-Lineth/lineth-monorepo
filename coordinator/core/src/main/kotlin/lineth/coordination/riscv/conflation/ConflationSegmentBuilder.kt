package lineth.coordination.riscv.conflation

import linea.domain.Block

object ConflationSegmentBuilder {
  fun buildSegment(blocks: List<Block>, chainId: ULong): ByteArray =
    TODO("implement: [4-byte-len BE][lz4(truncated_conflation)]")
}
