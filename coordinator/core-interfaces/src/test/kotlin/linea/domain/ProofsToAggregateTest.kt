package linea.domain

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class ProofsToAggregateTest {
  private fun proofsToAggregate(
    grandparentShnarf: ByteArray = ByteArray(32) { 1 },
    parentShnarfSnarkHash: ByteArray = ByteArray(32) { 2 },
    parentShnarfX: ByteArray = ByteArray(32) { 3 },
    parentShnarfY: ByteArray = ByteArray(32) { 4 },
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
      grandparentShnarf = grandparentShnarf,
      parentShnarfSnarkHash = parentShnarfSnarkHash,
      parentShnarfX = parentShnarfX,
      parentShnarfY = parentShnarfY,
      startBlockTimestamp = Instant.fromEpochSeconds(1),
    )

  @Test
  fun `equals returns true for equal shnarf preimage fields`() {
    assertThat(proofsToAggregate()).isEqualTo(proofsToAggregate())
  }

  @Test
  fun `equals returns false when grandparentShnarf differs`() {
    val other = proofsToAggregate(grandparentShnarf = ByteArray(32) { 9 })
    assertThat(proofsToAggregate()).isNotEqualTo(other)
  }

  @Test
  fun `equals returns false when parentShnarfSnarkHash differs`() {
    val other = proofsToAggregate(parentShnarfSnarkHash = ByteArray(32) { 9 })
    assertThat(proofsToAggregate()).isNotEqualTo(other)
  }

  @Test
  fun `equals returns false when parentShnarfX differs`() {
    val other = proofsToAggregate(parentShnarfX = ByteArray(32) { 9 })
    assertThat(proofsToAggregate()).isNotEqualTo(other)
  }

  @Test
  fun `equals returns false when parentShnarfY differs`() {
    val other = proofsToAggregate(parentShnarfY = ByteArray(32) { 9 })
    assertThat(proofsToAggregate()).isNotEqualTo(other)
  }

  @Test
  fun `hashCode is consistent with equals`() {
    assertThat(proofsToAggregate().hashCode()).isEqualTo(proofsToAggregate().hashCode())
  }

  @Test
  fun `toString includes hex-encoded shnarf preimage fields`() {
    val proof = proofsToAggregate(
      grandparentShnarf = ByteArray(32) { 1 },
      parentShnarfSnarkHash = ByteArray(32) { 2 },
      parentShnarfX = ByteArray(32) { 3 },
      parentShnarfY = ByteArray(32) { 4 },
    )
    val string = proof.toString()

    assertThat(string).contains("grandparentShnarf=")
    assertThat(string).contains("parentShnarfSnarkHash=")
    assertThat(string).contains("parentShnarfX=")
    assertThat(string).contains("parentShnarfY=")
  }
}
