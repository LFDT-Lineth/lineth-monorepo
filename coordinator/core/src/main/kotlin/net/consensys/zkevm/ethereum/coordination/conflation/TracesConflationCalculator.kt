package net.consensys.zkevm.ethereum.coordination.conflation

import linea.domain.Block
import net.consensys.zkevm.domain.Blob
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.ConflationCalculationResult
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface BlobCreationHandler {
  fun handleBlob(blob: Blob): SafeFuture<*>
}

fun interface ConflationHandler {
  fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*>
}

interface TracesConflationCalculator {
  val lastBlockNumber: ULong

  fun newBlock(blockCounters: BlockCounters)

  fun onConflatedBatch(conflationConsumer: (ConflationCalculationResult) -> SafeFuture<*>)

  fun onBlobCreation(blobHandler: BlobCreationHandler)
}

interface ConflationService {
  fun newBlock(block: Block, blockCounters: BlockCounters)

  fun onConflatedBatch(consumer: ConflationHandler)
}
