package net.consensys.zkevm.coordinator.app.conflationbacktesting

import linea.domain.BlockIntervals
import net.consensys.zkevm.domain.Blob
import net.consensys.zkevm.domain.BlobAndBatchCounters
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.CompressionProofIndex
import net.consensys.zkevm.ethereum.coordination.aggregation.ConsecutiveProvenBlobsProvider
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.ConcurrentSkipListMap

/**
 * In-memory implementation of [ConsecutiveProvenBlobsProvider] for conflation backtesting.
 *
 * Correlates two events that happen in sequence:
 *  1. [captureBlobExecutionProofs] — called at blob-creation time (before compression proof),
 *     while the original Blob.conflations are still available. Stores per-batch block
 *     boundaries keyed by blob block range.
 *  2. [acceptProvenBlobRecord] — called when the compression proof request is submitted.
 *     Looks up the stored execution proofs, builds [BlobAndBatchCounters], and marks the
 *     blob as ready for aggregation.
 *
 * [findConsecutiveProvenBlobs] returns the longest consecutive run of proven blobs starting
 * from the given fromBlockNumber, mirroring the behaviour of the Postgres-backed implementation.
 */
class InMemoryConsecutiveProvenBlobsProvider : ConsecutiveProvenBlobsProvider {

  // Temporary store: "startBlock-endBlock" → per-batch execution proof block intervals.
  // Populated by captureBlobExecutionProofs before the shnarf is known.
  private val pendingExecutionProofs = ConcurrentHashMap<String, BlockIntervals>()

  // Ordered by startBlockNumber so that consecutive-run scan is O(n).
  private val provenBlobs = ConcurrentSkipListMap<ULong, BlobAndBatchCounters>()

  /**
   * Must be called at blob-creation time (i.e. inside the onBlobCreation callback) so that
   * the per-batch conflation boundaries are captured before the blob is handed off to the
   * compression prover.
   */
  fun captureBlobExecutionProofs(
    blob: Blob,
  ) {
    val key = blobKey(blob.startBlockNumber, blob.endBlockNumber)
    pendingExecutionProofs[key] = BlockIntervals(
      startingBlockNumber = blob.conflations.first().startBlockNumber,
      upperBoundaries = blob.conflations.map { it.endBlockNumber },
    )
  }

  /**
   * Called by BlobCompressionProofRequestHandler when the compression proof request has been
   * submitted and the shnarf is known. Promotes the blob to the proven set.
   */
  fun acceptProvenBlobRecord(
    proofIndex: CompressionProofIndex,
    blobRecord: BlobRecord,
  ) {
    val key = blobKey(blobRecord.startBlockNumber, blobRecord.endBlockNumber)
    val executionProofs = pendingExecutionProofs.remove(key) ?: return
    provenBlobs[blobRecord.startBlockNumber] = BlobAndBatchCounters(
      blobCounters = BlobCounters(
        numberOfBatches = blobRecord.batchesCount,
        startBlockNumber = blobRecord.startBlockNumber,
        endBlockNumber = blobRecord.endBlockNumber,
        startBlockTimestamp = blobRecord.startBlockTime,
        endBlockTimestamp = blobRecord.endBlockTime,
        expectedShnarf = proofIndex.hash,
      ),
      executionProofs = executionProofs,
    )
  }

  override fun findConsecutiveProvenBlobs(fromBlockNumber: Long): SafeFuture<List<BlobAndBatchCounters>> {
    val result = mutableListOf<BlobAndBatchCounters>()
    var expectedNext = fromBlockNumber.toULong()
    for ((startBlock, blobAndBatchCounters) in provenBlobs.tailMap(expectedNext)) {
      if (startBlock != expectedNext) break
      result.add(blobAndBatchCounters)
      expectedNext = blobAndBatchCounters.blobCounters.endBlockNumber + 1uL
    }
    return SafeFuture.completedFuture(result)
  }

  private fun blobKey(startBlockNumber: ULong, endBlockNumber: ULong) =
    "$startBlockNumber-$endBlockNumber"
}
