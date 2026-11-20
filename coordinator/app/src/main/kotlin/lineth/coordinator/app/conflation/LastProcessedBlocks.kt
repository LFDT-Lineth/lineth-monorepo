package lineth.coordinator.app.conflation

import linea.domain.BlockWithTxHashes

/** Where the conflation and aggregation pipelines resume. Open so the RISC-V pipeline can add its rollup stage. */
open class LastProcessedBlocks(
  val lastConflatedBlock: BlockWithTxHashes,
  val lastAggregatedBlock: BlockWithTxHashes,
)

/**
 * Where the RISC-V pipeline resumes: [lastConflatedBlock] ends the persisted batch run, [lastRollupEndBlockNumber] the
 * rollup row run, and [lastAggregatedBlock] is the last finalized block.
 */
class RiscvLastProcessedBlocks(
  lastConflatedBlock: BlockWithTxHashes,
  lastAggregatedBlock: BlockWithTxHashes,
  val lastRollupEndBlockNumber: ULong,
) : LastProcessedBlocks(lastConflatedBlock, lastAggregatedBlock)
