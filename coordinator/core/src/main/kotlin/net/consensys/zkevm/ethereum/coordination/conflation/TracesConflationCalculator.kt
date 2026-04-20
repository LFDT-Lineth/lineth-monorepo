package net.consensys.zkevm.ethereum.coordination.conflation

import linea.domain.Blob
import linea.domain.Block
import linea.domain.BlockCounters
import linea.domain.BlocksConflation
import linea.domain.ConflationCalculationResult
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
