package net.consensys.zkevm.ethereum.coordination.blockcreation

import linea.domain.Block
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class BlockCreated(
  val block: Block
)

fun interface BlockCreationListener {
  fun acceptBlock(blockEvent: BlockCreated): SafeFuture<Unit>
}
