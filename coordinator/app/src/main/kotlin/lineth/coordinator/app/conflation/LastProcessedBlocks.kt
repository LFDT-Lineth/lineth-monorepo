package lineth.coordinator.app.conflation

import lineth.domain.BlockWithTxHashes

data class LastProcessedBlocks(
  val lastConflatedBlock: BlockWithTxHashes,
  val lastAggregatedBlock: BlockWithTxHashes,
)
