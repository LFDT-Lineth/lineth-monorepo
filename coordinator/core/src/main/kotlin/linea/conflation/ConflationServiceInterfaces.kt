package linea.conflation

import linea.domain.Blob
import linea.domain.Block
import linea.domain.BlockCounters
import linea.domain.BlocksConflation
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface BlobCreationHandler {
  fun handleBlob(blob: Blob): SafeFuture<*>
}

fun interface ConflationHandler {
  fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*>
}

interface ConflationService {
  fun newBlock(block: Block, blockCounters: BlockCounters)

  fun onConflatedBatch(consumer: ConflationHandler)
}
