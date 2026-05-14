package linea.coordinator.app.conflationbacktesting

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class BacktestingProgressTrackerTest {

  private val startBlock: ULong = 50uL
  private val targetEndBlock: ULong = 100uL
  private val initialEndBlock: Long = startBlock.toLong() - 1L

  private fun newTracker(): BacktestingProgressTracker = BacktestingProgressTracker(
    startBlockNumber = startBlock,
    targetEndBlockNumber = targetEndBlock,
  )

  @Test
  fun `initial last-end-block values are startBlockNumber minus one for all flows`() {
    val tracker = newTracker()
    assertThat(tracker.isComplete()).isFalse()
    assertThat(tracker.lastExecutionRequestEndBlock()).isEqualTo(initialEndBlock)
    assertThat(tracker.lastCompressionRequestEndBlock()).isEqualTo(initialEndBlock)
    assertThat(tracker.lastAggregationRequestEndBlock()).isEqualTo(initialEndBlock)
  }

  @Test
  fun `is not complete when only execution flow reaches target`() {
    val tracker = newTracker()
    tracker.recordExecutionRequestEndBlock(targetEndBlock)
    assertThat(tracker.isComplete()).isFalse()
  }

  @Test
  fun `is not complete when only execution and compression flows reach target`() {
    val tracker = newTracker()
    tracker.recordExecutionRequestEndBlock(targetEndBlock)
    tracker.recordCompressionRequestEndBlock(targetEndBlock)
    assertThat(tracker.isComplete()).isFalse()
  }

  @Test
  fun `is not complete when only aggregation flow reaches target`() {
    // Reproduces the bug: aggregation can finish while execution still lags due to retrying
    // traces conflation, so completion must require all three flows.
    val tracker = newTracker()
    tracker.recordAggregationRequestEndBlock(targetEndBlock)
    assertThat(tracker.isComplete()).isFalse()
  }

  @Test
  fun `is not complete when only compression and aggregation flows reach target`() {
    val tracker = newTracker()
    tracker.recordCompressionRequestEndBlock(targetEndBlock)
    tracker.recordAggregationRequestEndBlock(targetEndBlock)
    assertThat(tracker.isComplete()).isFalse()
  }

  @Test
  fun `is complete only when all three flows reach target`() {
    val tracker = newTracker()
    tracker.recordExecutionRequestEndBlock(targetEndBlock)
    tracker.recordCompressionRequestEndBlock(targetEndBlock)
    tracker.recordAggregationRequestEndBlock(targetEndBlock)
    assertThat(tracker.isComplete()).isTrue()
  }

  @Test
  fun `is complete when flows surpass target`() {
    val tracker = newTracker()
    tracker.recordExecutionRequestEndBlock(targetEndBlock + 5uL)
    tracker.recordCompressionRequestEndBlock(targetEndBlock + 3uL)
    tracker.recordAggregationRequestEndBlock(targetEndBlock + 1uL)
    assertThat(tracker.isComplete()).isTrue()
  }

  @Test
  fun `out of order execution callbacks do not regress progress`() {
    val tracker = newTracker()
    tracker.recordExecutionRequestEndBlock(80uL)
    tracker.recordExecutionRequestEndBlock(60uL)
    assertThat(tracker.lastExecutionRequestEndBlock()).isEqualTo(80L)
  }

  @Test
  fun `out of order compression callbacks do not regress progress`() {
    val tracker = newTracker()
    tracker.recordCompressionRequestEndBlock(75uL)
    tracker.recordCompressionRequestEndBlock(60uL)
    assertThat(tracker.lastCompressionRequestEndBlock()).isEqualTo(75L)
  }

  @Test
  fun `out of order aggregation callbacks do not regress progress`() {
    val tracker = newTracker()
    tracker.recordAggregationRequestEndBlock(70uL)
    tracker.recordAggregationRequestEndBlock(60uL)
    assertThat(tracker.lastAggregationRequestEndBlock()).isEqualTo(70L)
  }

  @Test
  fun `lastExecutionRequestEndBlock returns highest recorded value`() {
    val tracker = newTracker()
    tracker.recordExecutionRequestEndBlock(60uL)
    tracker.recordExecutionRequestEndBlock(90uL)
    tracker.recordExecutionRequestEndBlock(70uL)
    assertThat(tracker.lastExecutionRequestEndBlock()).isEqualTo(90L)
  }

  @Test
  fun `is complete is independent of order in which flows reach target`() {
    val tracker = newTracker()
    tracker.recordAggregationRequestEndBlock(targetEndBlock)
    assertThat(tracker.isComplete()).isFalse()
    tracker.recordCompressionRequestEndBlock(targetEndBlock)
    assertThat(tracker.isComplete()).isFalse()
    tracker.recordExecutionRequestEndBlock(targetEndBlock)
    assertThat(tracker.isComplete()).isTrue()
  }

  @Test
  fun `lastExecutionRequestEndBlock baseline keeps BlockCreationMonitor fetch gap small at startup`() {
    // Regression guard: BlockCreationMonitor refuses to fetch when
    // (nextBlockToFetch - lastKnownProvenBlock) > blocksFetchLimit. With startBlockNumber=50,
    // initial nextBlockToFetch=startBlockNumber=50, so the initial gap must be 1.
    val tracker = newTracker()
    val initialGap = startBlock.toLong() - tracker.lastExecutionRequestEndBlock()
    assertThat(initialGap).isEqualTo(1L)
  }
}
