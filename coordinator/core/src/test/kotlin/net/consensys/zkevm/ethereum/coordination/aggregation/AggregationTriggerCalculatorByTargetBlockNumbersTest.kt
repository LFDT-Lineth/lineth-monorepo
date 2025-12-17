package net.consensys.zkevm.ethereum.coordination.aggregation

import net.consensys.zkevm.domain.BlobsToAggregate
import net.consensys.zkevm.domain.blobCounters
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.mock
import org.mockito.kotlin.verify

class AggregationTriggerCalculatorByTargetBlockNumbersTest {
  private lateinit var log: Logger
  private lateinit var calculator: AggregationTriggerCalculatorByTargetBlockNumbers

  @BeforeEach
  fun beforeEach() {
    log = mock<Logger>()
    calculator =
      AggregationTriggerCalculatorByTargetBlockNumbers(
        targetEndBlockNumbers = listOf(10uL, 20uL, 30uL),
        log = log,
      )
  }

  @Test
  fun `when endBlockNumbers is empty then returns null`() {
    val calculator = AggregationTriggerCalculatorByTargetBlockNumbers(targetEndBlockNumbers = emptyList())
    assertThat(calculator.checkAggregationTrigger(blob = blobCounters(startBlockNumber = 1uL, endBlockNumber = 10uL)))
      .isNull()
  }

  @Test
  fun `when blob endBlockNumber does not match target aggregation end block number then returns null`() {
    val blob = blobCounters(startBlockNumber = 15uL, endBlockNumber = 25uL)
    assertThat(calculator.checkAggregationTrigger(blob = blobCounters(startBlockNumber = 15uL, endBlockNumber = 25uL)))
      .isNull()
    verify(log).warn("blob={} overlaps target aggregation with endBlockNumber={}", blob.intervalString(), 20uL)
  }

  @Test
  fun `when blob startBlockNumber matches target aggregation end block number then returns null`() {
    val blob = blobCounters(startBlockNumber = 20uL, endBlockNumber = 25uL)
    assertThat(calculator.checkAggregationTrigger(blob = blobCounters(startBlockNumber = 20uL, endBlockNumber = 25uL)))
      .isNull()
    verify(log).warn("blob={} overlaps target aggregation with endBlockNumber={}", blob.intervalString(), 20uL)
  }

  @Test
  fun `when blob overlaps aggregation end block number then returns null and logs warning`() {
    val blob = blobCounters(startBlockNumber = 15uL, endBlockNumber = 25uL)
    assertThat(calculator.checkAggregationTrigger(blob = blob))
      .isNull()
    verify(log).warn("blob={} overlaps target aggregation with endBlockNumber={}", blob.intervalString(), 20uL)
  }

  @Test
  fun `when blob endBlockNumber matches target aggregation endBlockNumber then returns AggregationTrigger`() {
    assertThat(calculator.checkAggregationTrigger(blob = blobCounters(startBlockNumber = 15uL, endBlockNumber = 20uL)))
      .isEqualTo(
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.TARGET_BLOCK_NUMBER,
          aggregation = BlobsToAggregate(15uL, 20uL),
        ),
      )
  }

  @Test
  fun `when blob start and endBlockNumber matches target aggregation endBlockNumber then returns AggregationTrigger`() {
    assertThat(calculator.checkAggregationTrigger(blob = blobCounters(startBlockNumber = 20uL, endBlockNumber = 20uL)))
      .isEqualTo(
        AggregationTrigger(
          aggregationTriggerType = AggregationTriggerType.TARGET_BLOCK_NUMBER,
          aggregation = BlobsToAggregate(20uL, 20uL),
        ),
      )
  }
}
