package net.consensys.zkevm.ethereum.coordination

import linea.domain.Batch
import linea.domain.Blob
import linea.domain.BlobRecord
import linea.domain.BlobSubmittedEvent
import linea.domain.BlocksConflation
import linea.domain.FinalizationSubmittedEvent
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.function.Supplier

abstract class MaxLongTracker<T>(initialValue: Long) : (T) -> SafeFuture<*>, Supplier<Number> {
  private val maxLongCache: MaxLongCache = MaxLongCache(initialValue)

  protected abstract fun convertToLong(trackable: T): Long

  override fun invoke(trackable: T): SafeFuture<*> {
    maxLongCache.accept(convertToLong(trackable))
    return SafeFuture.completedFuture(Unit)
  }

  override fun get(): Long {
    return maxLongCache.get()
  }
}

class HighestProvenBatchTracker(initialProvenBlockNumber: ULong) :
  MaxLongTracker<Batch>(initialProvenBlockNumber.toLong()) {
  override fun convertToLong(trackable: Batch): Long {
    return trackable.endBlockNumber.toLong()
  }
}

class HighestConflationTracker(initialProvenBlockNumber: ULong) :
  MaxLongTracker<BlocksConflation>(initialProvenBlockNumber.toLong()) {
  override fun convertToLong(trackable: BlocksConflation): Long {
    return trackable.blocks.last().number.toLong()
  }
}

class HighestProvenBlobTracker(initialProvenBlockNumber: ULong) :
  MaxLongTracker<BlobRecord>(initialProvenBlockNumber.toLong()) {
  override fun convertToLong(trackable: BlobRecord): Long {
    return trackable.endBlockNumber.toLong()
  }
}

class HighestULongTracker(initialProvenBlockNumber: ULong) :
  MaxLongTracker<ULong>(initialProvenBlockNumber.toLong()) {
  override fun convertToLong(trackable: ULong): Long {
    return trackable.toLong()
  }
}

class HighestUnprovenBlobTracker(initialProvenBlockNumber: ULong) :
  MaxLongTracker<Blob>(initialProvenBlockNumber.toLong()) {
  override fun convertToLong(trackable: Blob): Long {
    return trackable.endBlockNumber.toLong()
  }
}

class LatestBlobSubmittedBlockNumberTracker(initialLatestBlockNumber: ULong) :
  MaxLongTracker<BlobSubmittedEvent>(initialLatestBlockNumber.toLong()) {
  override fun convertToLong(trackable: BlobSubmittedEvent): Long {
    return trackable.blobs.last().endBlockNumber.toLong()
  }
}

class LatestFinalizationSubmittedBlockNumberTracker(initialLatestBlockNumber: ULong) :
  MaxLongTracker<FinalizationSubmittedEvent>(initialLatestBlockNumber.toLong()) {
  override fun convertToLong(trackable: FinalizationSubmittedEvent): Long {
    return trackable.aggregationProof.finalBlockNumber
  }
}
