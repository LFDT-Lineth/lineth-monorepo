package lineth.coordination.aggregation

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Instant

class AggregationL2StateEqualityTest {
  @Test
  fun `equality includes forced transaction state`() {
    val state = AggregationL2State(
      parentAggregationLastBlockTimestamp = Instant.fromEpochSeconds(10),
      parentAggregationLastL1RollingHashMessageNumber = 1UL,
      parentAggregationLastL1RollingHash = byteArrayOf(2),
      parentAggregationLastFtxNumber = 3UL,
      parentAggregationLastFtxRollingHash = byteArrayOf(4),
    )
    val equalState = state.copy(
      parentAggregationLastL1RollingHash = state.parentAggregationLastL1RollingHash.copyOf(),
      parentAggregationLastFtxRollingHash = state.parentAggregationLastFtxRollingHash.copyOf(),
    )

    assertThat(state).isEqualTo(equalState)
    assertThat(state.hashCode()).isEqualTo(equalState.hashCode())
    assertThat(state).isNotEqualTo(state.copy(parentAggregationLastFtxNumber = 4UL))
    assertThat(state).isNotEqualTo(state.copy(parentAggregationLastFtxRollingHash = byteArrayOf(5)))
  }
}
