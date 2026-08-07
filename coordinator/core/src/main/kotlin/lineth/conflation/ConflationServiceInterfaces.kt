package lineth.conflation

import lineth.domain.Blob
import lineth.domain.Block
import lineth.domain.BlockCounters
import lineth.domain.BlocksConflation
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
