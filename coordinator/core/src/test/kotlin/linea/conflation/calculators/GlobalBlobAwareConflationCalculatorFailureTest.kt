package linea.conflation.calculators

import io.micrometer.core.instrument.simple.SimpleMeterRegistry
import linea.coordination.blob.FakeBlobCompressor
import linea.domain.BlockCounters
import linea.domain.ConflationCalculationResult
import linea.domain.ConflationTrigger
import net.consensys.linea.metrics.micrometer.MicrometerMetricsFacade
import net.consensys.linea.traces.TracesCountersV2
import net.consensys.linea.traces.fakeTracesCountersV2
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Instant

class GlobalBlobAwareConflationCalculatorFailureTest {
  @Test
  fun `observes a failed blob handler without failing the completed conflation`() {
    val blobCalculator = ConflationTriggerCalculatorByDataCompressed(
      FakeBlobCompressor(dataLimit = 100, fakeCompressionRatio = 1.0),
    )
    val conflationCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber = 0UL,
      syncCalculators = listOf(blobCalculator),
      deferredTriggerConflationCalculators = emptyList(),
      emptyTracesCounters = TracesCountersV2.EMPTY_TRACES_COUNT,
    )
    val calculator = GlobalBlobAwareConflationCalculator(
      conflationCalculator = conflationCalculator,
      blobCalculator = blobCalculator,
      batchesLimit = 1U,
      metricsFacade = MicrometerMetricsFacade(SimpleMeterRegistry(), "linea"),
      aggregationTargetEndBlocks = mutableSetOf(),
    )
    val blockCounters = BlockCounters(
      blockNumber = 1UL,
      blockTimestamp = Instant.fromEpochSeconds(1),
      tracesCounters = fakeTracesCountersV2(1U),
      blockRLPEncoded = byteArrayOf(1),
      numOfTransactions = 1U,
      gasUsed = 1UL,
    )
    calculator.newBlock(blockCounters)
    calculator.onBlobCreation {
      SafeFuture.failedFuture<Unit>(IllegalStateException("blob persistence failed"))
    }

    val conflationFuture = calculator.handleBatchTrigger(
      ConflationCalculationResult(
        startBlockNumber = 1UL,
        endBlockNumber = 1UL,
        conflationTrigger = ConflationTrigger.TRACES_LIMIT,
        tracesCounters = blockCounters.tracesCounters,
      ),
    )

    assertThat(conflationFuture).isCompleted
    assertThat(conflationFuture).isNotCompletedExceptionally
  }
}
