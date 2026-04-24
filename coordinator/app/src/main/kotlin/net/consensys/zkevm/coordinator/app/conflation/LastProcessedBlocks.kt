package net.consensys.zkevm.coordinator.app.conflation

import linea.domain.BlockWithTxHashes

data class LastProcessedBlocks(
  val lastConflatedBlock: BlockWithTxHashes,
  val lastAggregatedBlock: BlockWithTxHashes,
)
