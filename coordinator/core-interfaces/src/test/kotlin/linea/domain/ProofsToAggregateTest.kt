package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class ProofsToAggregateTest {
  private fun proofsToAggregate(
    parentCompressionProofIndex: CompressionProofIndex? = CompressionProofIndex(
      startBlockNumber = 1uL,
      endBlockNumber = 10uL,
      hash = ByteArray(32) { 1 },
      startBlockTimestamp = Instant.fromEpochSeconds(0),
    ),
  ): ProofsToAggregate =
    ProofsToAggregate(
      compressionProofIndexes = listOf(
        CompressionProofIndex(
          startBlockNumber = 1uL,
          endBlockNumber = 10uL,
          hash = ByteArray(32) { 5 },
          startBlockTimestamp = Instant.fromEpochSeconds(1),
        ),
      ),
      executionProofs = BlockIntervals(1uL, listOf(10uL)),
      invalidityProofs = emptyList(),
      parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(0),
      parentAggregationLastL1RollingHashMessageNumber = 0uL,
      parentAggregationLastL1RollingHash = ByteArray(32) { 6 },
      parentAggregationLastFtxNumber = 0uL,
      parentAggregationLastFtxRollingHash = ByteArray(32) { 7 },
      parentCompressionProofIndex = parentCompressionProofIndex,
      startBlockTimestamp = Instant.fromEpochSeconds(1),
    )

  @Test
  fun `equals returns true for equal parentCompressionProofIndex`() {
    assertThat(proofsToAggregate()).isEqualTo(proofsToAggregate())
  }

  @Test
  fun `equals returns false when parentCompressionProofIndex differs`() {
    val other = proofsToAggregate(
      parentCompressionProofIndex = CompressionProofIndex(
        startBlockNumber = 1uL,
        endBlockNumber = 10uL,
        hash = ByteArray(32) { 9 },
        startBlockTimestamp = Instant.fromEpochSeconds(0),
      ),
    )
    assertThat(proofsToAggregate()).isNotEqualTo(other)
  }

  @Test
  fun `equals returns false when parentCompressionProofIndex is null vs non-null`() {
    val other = proofsToAggregate(parentCompressionProofIndex = null)
    assertThat(proofsToAggregate()).isNotEqualTo(other)
  }

  @Test
  fun `equals returns true when parentCompressionProofIndex is null on both sides`() {
    assertThat(proofsToAggregate(parentCompressionProofIndex = null))
      .isEqualTo(proofsToAggregate(parentCompressionProofIndex = null))
  }

  @Test
  fun `hashCode is consistent with equals`() {
    assertThat(proofsToAggregate().hashCode()).isEqualTo(proofsToAggregate().hashCode())
  }

  @Test
  fun `toString includes parentCompressionProofIndex`() {
    val string = proofsToAggregate().toString()

    assertThat(string).contains("parentCompressionProofIndex=")
  }
}
