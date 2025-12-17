package net.consensys.zkevm.ethereum.submission

import linea.domain.BlockIntervals
import net.consensys.zkevm.domain.BlobRecord

fun interface BlobsGrouperForSubmission {
  fun chunkBlobs(blobsIntervals: List<BlobRecord>, aggregations: BlockIntervals): List<List<BlobRecord>>
}

class BlobsGrouperForSubmissionSwitcherByTargetBock(
  private val eip4844TargetBlobsPerTx: UInt = 9U,
) : BlobsGrouperForSubmission {
  override fun chunkBlobs(blobsIntervals: List<BlobRecord>, aggregations: BlockIntervals): List<List<BlobRecord>> {
    if (blobsIntervals.isEmpty()) {
      return emptyList()
    }

    val sortedBlobs = blobsIntervals.sortedBy { it.startBlockNumber }

    val blobsAggregations = if (aggregations.startingBlockNumber < blobsIntervals.first().startBlockNumber) {
      // drop aggregations that are before the first blob.
      // It's blobs where already submitted and we don't need to submit them again
      // this is caused due to different submission delays
      aggregations.toIntervalList()
        .dropWhile { it.endBlockNumber < blobsIntervals.first().startBlockNumber }
        .let { aggsIntervals ->
          BlockIntervals(aggsIntervals.first().startBlockNumber, aggsIntervals.map { it.endBlockNumber })
        }
    } else {
      aggregations
    }

    return chunkBlobs(sortedBlobs, blobsAggregations, targetChunkSize = eip4844TargetBlobsPerTx.toInt())
  }
}
