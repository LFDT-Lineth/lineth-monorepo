package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.BlobAndBatchCounters
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface ConsecutiveProvenBlobsProvider {
  fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>>
}
