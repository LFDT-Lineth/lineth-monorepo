package net.consensys.zkevm.ethereum.coordination.aggregation

import linea.domain.BlobAndBatchCounters
import tech.pegasys.teku.infrastructure.async.SafeFuture

fun interface ConsecutiveProvenBlobsProvider {
  fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>>
}
