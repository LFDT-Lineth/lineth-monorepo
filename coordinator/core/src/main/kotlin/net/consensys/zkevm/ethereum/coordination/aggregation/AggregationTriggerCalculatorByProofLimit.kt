package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate

class AggregationTriggerCalculatorByProofLimit(private val maxProofsPerAggregation: UInt) :
  SyncAggregationTriggerCalculator {

  data class InFlightAggregation(
    val proofsCount: UInt,
    val blobsToAggregate: BlobsToAggregate,
  )

  private var inFlightAggregation: InFlightAggregation? = null

  private fun countProofs(blobCounters: BlobCounters): UInt {
    return blobCounters.numberOfBatches + 1u
  }

  @Synchronized
  override fun checkAggregationTrigger(blobCounters: BlobCounters): AggregationTrigger? {
    val blobProofCount = countProofs(blobCounters)
    if (blobProofCount > maxProofsPerAggregation) {
      throw IllegalArgumentException(
        "Number of proofs in one blob exceed the aggregation proof limit. " +
          "blob = $blobCounters",
      )
    }

    return if (inFlightAggregation != null) {
      val newProofsCount = inFlightAggregation!!.proofsCount + blobProofCount
      if (newProofsCount > maxProofsPerAggregation) {
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.PROOF_LIMIT,
          aggregation = inFlightAggregation!!.blobsToAggregate,
        )
      } else if (newProofsCount == maxProofsPerAggregation) {
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.PROOF_LIMIT,
          aggregation = BlobsToAggregate(
            inFlightAggregation!!.blobsToAggregate.startBlockNumber,
            blobCounters.endBlockNumber,
          ),
        )
      } else {
        null
      }
    } else {
      if (blobProofCount == maxProofsPerAggregation) {
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.PROOF_LIMIT,
          aggregation = BlobsToAggregate(
            blobCounters.startBlockNumber,
            blobCounters.endBlockNumber,
          ),
        )
      } else {
        null
      }
    }
  }

  @Synchronized
  override fun newBlob(blobCounters: BlobCounters) {
    val blobProofCount = countProofs(blobCounters)
    if (blobProofCount > maxProofsPerAggregation) {
      throw IllegalArgumentException(
        "Number of proofs in one blob exceed the aggregation proof limit. " +
          "blob = $blobCounters",
      )
    }
    val newProofsCount = (inFlightAggregation?.proofsCount ?: 0u) + blobProofCount
    if (newProofsCount > maxProofsPerAggregation) {
      throw IllegalStateException("Proof count already overflowed, should have been reset before this new blob")
    } else {
      inFlightAggregation = InFlightAggregation(
        proofsCount = newProofsCount,
        blobsToAggregate = BlobsToAggregate(
          inFlightAggregation?.blobsToAggregate?.startBlockNumber ?: blobCounters.startBlockNumber,
          blobCounters.endBlockNumber,
        ),
      )
    }
  }

  @Synchronized
  override fun reset() {
    inFlightAggregation = null
  }
}
