package net.consensys.zkevm.ethereum.coordination.aggregation

import kotlinx.datetime.Instant
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertDoesNotThrow
import org.junit.jupiter.api.assertThrows
import kotlin.random.Random
import kotlin.random.nextULong

class AggregationTriggerCalculatorByProofLimitTest {

  private fun createBlob(startBlockNumber: ULong, endBlockNumber: ULong, batchesCount: UInt): BlobCounters {
    return BlobCounters(
      startBlockNumber = startBlockNumber,
      endBlockNumber = endBlockNumber,
      numberOfBatches = batchesCount,
      startBlockTimestamp = Instant.fromEpochMilliseconds((startBlockNumber * 100uL).toLong()),
      endBlockTimestamp = Instant.fromEpochMilliseconds((endBlockNumber * 100uL).toLong()),
      expectedShnarf = Random.nextBytes(32),
    )
  }

  @Test
  fun when_proof_limit_overflows_with_exact_proof_limit_include_current_blob_in_aggregation() {
    val aggregationTrigger = AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = 10u)

    val blobCounters = mutableListOf<BlobCounters>()
    var startBlockNumber = 1uL
    var endBlockNumber = startBlockNumber + Random.nextULong(10uL)
    val batchesCount = 1u
    for (i in 1..4) {
      val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
      startBlockNumber = endBlockNumber + 1uL
      endBlockNumber = startBlockNumber + Random.nextULong(10uL)

      blobCounters.add(blob)

      assertThat(aggregationTrigger.checkAggregationTrigger(blob)).isNull()
      aggregationTrigger.newBlob(blob)
    }
    val blob = createBlob(startBlockNumber, endBlockNumber, 1u)
    blobCounters.add(blob)

    val overflowTrigger = aggregationTrigger.checkAggregationTrigger(blob)
    assertThat(overflowTrigger)
      .isEqualTo(
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.PROOF_LIMIT,
          aggregation = BlobsToAggregate(blobCounters.first().startBlockNumber, blobCounters.last().endBlockNumber),
        ),
      )
  }

  @Test
  fun when_proof_limit_overflows_above_proof_limit_do_not_include_current_blob_in_aggregation() {
    val aggregationTrigger = AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = 8u)

    val blobCounters = mutableListOf<BlobCounters>()
    var startBlockNumber = 1uL
    var endBlockNumber = startBlockNumber + Random.nextULong(10uL)
    val batchesCount = 2u
    for (i in 1..2) {
      val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
      startBlockNumber = endBlockNumber + 1uL
      endBlockNumber = startBlockNumber + Random.nextULong(10uL)

      blobCounters.add(blob)

      assertThat(aggregationTrigger.checkAggregationTrigger(blob)).isNull()
      aggregationTrigger.newBlob(blob)
    }
    val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
    blobCounters.add(blob)

    val overflowTrigger = aggregationTrigger.checkAggregationTrigger(blob)
    assertThat(overflowTrigger)
      .isEqualTo(
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.PROOF_LIMIT,
          aggregation = BlobsToAggregate(
            blobCounters.first().startBlockNumber,
            blobCounters[blobCounters.size - 2].endBlockNumber, // Last blob [size-1] should not be included
          ),
        ),
      )
  }

  @Test
  fun when_reset_then_calculator_should_rest_proof_count() {
    val aggregationTrigger = AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = 10u)

    val blobCounters = mutableListOf<BlobCounters>()
    var startBlockNumber = 1uL
    var endBlockNumber = startBlockNumber + Random.nextULong(10uL)
    val batchesCount = 1u
    for (i in 1..4) {
      val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
      startBlockNumber = endBlockNumber + 1uL
      endBlockNumber = startBlockNumber + Random.nextULong(10uL)

      blobCounters.add(blob)

      assertThat(aggregationTrigger.checkAggregationTrigger(blob)).isNull()
      aggregationTrigger.newBlob(blob)
    }

    aggregationTrigger.reset()
    val blobCountersAfterReset = mutableListOf<BlobCounters>()

    for (i in 1..4) {
      val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
      startBlockNumber = endBlockNumber + 1uL
      endBlockNumber = startBlockNumber + Random.nextULong(10uL)

      blobCountersAfterReset.add(blob)

      assertThat(aggregationTrigger.checkAggregationTrigger(blob)).isNull()
      aggregationTrigger.newBlob(blob)
    }
    val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
    blobCountersAfterReset.add(blob)

    assertThat(aggregationTrigger.checkAggregationTrigger(blob))
      .isEqualTo(
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.PROOF_LIMIT,
          aggregation = BlobsToAggregate(
            blobCountersAfterReset.first().startBlockNumber,
            blobCountersAfterReset.last().endBlockNumber,
          ),
        ),
      )
  }

  @Test
  fun when_blob_proofs_equal_proof_limit_and_blob_is_not_included_in_current_aggregation() {
    val proofLimit = 10u
    val blobCounters = mutableListOf<BlobCounters>()
    var startBlockNumber = 1uL
    var endBlockNumber = startBlockNumber + Random.nextULong(10uL)
    val batchesCount = 2u // Blob proofs = 2 + 1
    val aggregationTriggerCalculator = AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = proofLimit)
    for (i in 1..3) {
      val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
      startBlockNumber = endBlockNumber + 1uL
      endBlockNumber = startBlockNumber + Random.nextULong(10uL)

      blobCounters.add(blob)
      assertThat(aggregationTriggerCalculator.checkAggregationTrigger(blob)).isNull()
      aggregationTriggerCalculator.newBlob(blob)
    }
    // Proof count should be 9

    // Check aggregation trigger with this new blob with proofs = proof limit
    // It will trigger aggregation but not be included in the aggregation

    // Blob proofs = (proofLimit - 1) + 1
    var blob = createBlob(startBlockNumber, endBlockNumber, proofLimit - 1u)

    startBlockNumber = endBlockNumber + 1uL
    endBlockNumber = startBlockNumber + Random.nextULong(10uL)
    blobCounters.add(blob)
    val aggregationTrigger1 = aggregationTriggerCalculator.checkAggregationTrigger(blob)
    assertThat(aggregationTrigger1).isNotNull()
    assertThat(aggregationTrigger1!!.aggregationTriggerType).isEqualTo(AggregationTriggerType.PROOF_LIMIT)
    assertThat(aggregationTrigger1.aggregation).isEqualTo(
      BlobsToAggregate(
        blobCounters.first().startBlockNumber,
        blobCounters[blobCounters.size - 2].endBlockNumber,
      ),
    )

    // Calculator should be reset after aggregation trigger
    aggregationTriggerCalculator.reset()
    // Proof Count should be zero after reset
    // Since that blob was not included in the aggregation it should be added to the calculator
    assertDoesNotThrow { aggregationTriggerCalculator.newBlob(blob) }
    // Now proofCount = proofLimit

    // The next blob should trigger an aggregation
    // Blob proofs = (proofLimit - 1) + 1
    blob = createBlob(startBlockNumber, endBlockNumber, proofLimit - 1u)
    blobCounters.add(blob)
    val aggregationTrigger2 = aggregationTriggerCalculator.checkAggregationTrigger(blob)
    assertThat(aggregationTrigger2).isNotNull()
    assertThat(aggregationTrigger2!!.aggregationTriggerType).isEqualTo(AggregationTriggerType.PROOF_LIMIT)
    assertThat(aggregationTrigger2.aggregation).isEqualTo(
      BlobsToAggregate(
        aggregationTrigger1.aggregation.endBlockNumber + 1u,
        blobCounters[blobCounters.size - 2].endBlockNumber,
      ),
    )

    // Calculator should be reset after aggregation trigger
    aggregationTriggerCalculator.reset()
    // Proof Count should be zero after reset
    // Since the blob was not included in the aggregation it should be added to the calculator
    assertDoesNotThrow { aggregationTriggerCalculator.newBlob(blob) }
  }

  @Test
  fun when_blob_proofs_equal_proof_limit_and_blob_is_included_in_current_aggregation() {
    val proofLimit = 10u
    val aggregationTriggerCalculator = AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = proofLimit)

    val startBlockNumber = 1uL
    val endBlockNumber = startBlockNumber + Random.nextULong(10uL)
    val batchesCount = proofLimit - 1u // Blob proofs = (proofLimit - 1) + 1

    val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)

    // Proof count should be 0
    // This blob will trigger aggregation and will be included
    val aggregationTrigger = aggregationTriggerCalculator.checkAggregationTrigger(blob)
    assertThat(aggregationTrigger).isNotNull()
    assertThat(aggregationTrigger!!.aggregationTriggerType).isEqualTo(AggregationTriggerType.PROOF_LIMIT)
    assertThat(aggregationTrigger.aggregation).isEqualTo(BlobsToAggregate(blob.startBlockNumber, blob.endBlockNumber))

    // Calculator should be reset after aggregation trigger
    aggregationTriggerCalculator.reset()
    // Proof Count should be zero
  }

  @Test
  fun when_after_trigger_new_blob_is_called_without_reset_then_throw_exception() {
    val aggregationTrigger = AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = 10u)

    var startBlockNumber = 1uL
    var endBlockNumber = startBlockNumber + Random.nextULong(10uL)
    val batchesCount = 2u
    for (i in 1..3) {
      val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
      startBlockNumber = endBlockNumber + 1uL
      endBlockNumber = startBlockNumber + Random.nextULong(10uL)

      assertThat(aggregationTrigger.checkAggregationTrigger(blob)).isNull()
      aggregationTrigger.newBlob(blob)
    }
    // Proof count should be 9, add a new blob without checking for aggregation and reset will cause overflow
    val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)
    val exception = assertThrows<IllegalStateException> { aggregationTrigger.newBlob(blob) }
    assertThat(exception.message)
      .isEqualTo("Proof count already overflowed, should have been reset before this new blob")
  }

  @Test
  fun when_one_blob_exceeds_proof_limit_then_throw_exception() {
    val proofLimit = 10u
    val aggregationTrigger = AggregationTriggerCalculatorByProofLimit(maxProofsPerAggregation = proofLimit)

    val startBlockNumber = 1uL
    val endBlockNumber = startBlockNumber + Random.nextULong(10uL)
    val batchesCount = proofLimit
    val blob = createBlob(startBlockNumber, endBlockNumber, batchesCount)

    val exception1 = assertThrows<IllegalArgumentException> {
      aggregationTrigger.checkAggregationTrigger(blob)
    }
    assertThat(exception1.message)
      .contains("Number of proofs in one blob exceed the aggregation proof limit")

    val exception2 = assertThrows<IllegalArgumentException> { aggregationTrigger.newBlob(blob) }
    assertThat(exception2.message)
      .contains("Number of proofs in one blob exceed the aggregation proof limit")
  }
}
