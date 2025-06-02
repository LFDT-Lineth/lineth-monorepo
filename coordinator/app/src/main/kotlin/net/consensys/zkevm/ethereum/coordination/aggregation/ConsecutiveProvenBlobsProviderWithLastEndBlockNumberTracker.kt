package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.persistence.AggregationsRepository
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.atomic.AtomicReference
import java.util.function.Supplier

class ConsecutiveProvenBlobsProviderWithLastEndBlockNumberTracker(
  private val repository: AggregationsRepository,
  initialBlockNumber: ULong,
) : ConsecutiveProvenBlobsProvider, Supplier<Number> {
  private val cache = AtomicReference(initialBlockNumber)

  override fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>> {
    val consecutiveProvenBlobs = repository.findConsecutiveProvenBlobs(fromBlockNumber)

    consecutiveProvenBlobs.thenPeek {
      if (it.isNotEmpty()) {
        cache.set(it.last().blobCounters.endBlockNumber)
      }
    }
    return consecutiveProvenBlobs
  }

  override fun get(): Number {
    return cache.get().toLong()
  }
}
