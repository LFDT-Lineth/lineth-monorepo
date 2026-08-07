package lineth.coordination.aggregation

import lineth.domain.BlobAndBatchCounters
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface ConsecutiveProvenBlobsProvider {
  fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>>
}
