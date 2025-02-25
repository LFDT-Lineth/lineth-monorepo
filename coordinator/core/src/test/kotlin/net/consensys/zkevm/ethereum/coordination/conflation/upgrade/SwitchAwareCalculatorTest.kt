package net.consensys.zkevm.ethereum.coordination.conflation.upgrade

import kotlinx.datetime.Instant
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.linea.traces.TracesCountersV1
import net.consensys.linea.traces.fakeTracesCountersV1
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationCalculatorByDataCompressed
import net.consensys.zkevm.ethereum.coordination.conflation.GlobalBlobAwareConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.GlobalBlockConflationCalculator
import net.consensys.zkevm.ethereum.coordination.conflation.TracesConflationCalculator
import org.assertj.core.api.Assertions
import org.junit.jupiter.api.Test
import org.mockito.Mockito
import org.mockito.Mockito.clearInvocations
import org.mockito.Mockito.mock
import org.mockito.Mockito.spy
import org.mockito.Mockito.times
import org.mockito.Mockito.verify
import org.mockito.Mockito.verifyNoMoreInteractions
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.never
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

@Suppress("DEPRECATION")
class SwitchAwareCalculatorTest {
  private var lastBlockNumber = 0UL
  private lateinit var switchCutoffCalculator: SwitchCutoffCalculator
  private lateinit var oldCalculator: GlobalBlockConflationCalculator
  private lateinit var newCalculator: GlobalBlobAwareConflationCalculator
  private lateinit var mockBlobCalculator: ConflationCalculatorByDataCompressed
  private lateinit var switchAwareCalculator: SwitchAwareCalculator
  private val switchBlockNumber = 5UL
  private val baseBlockCounters = BlockCounters(
    blockNumber = lastBlockNumber + 1UL,
    blockTimestamp = Instant.parse("2021-01-01T00:00:00.000Z"),
    tracesCounters = fakeTracesCountersV1(0U),
    blockRLPEncoded = ByteArray(0)
  )

  private fun newCalculatorProvider(lastBlockNumber: ULong): TracesConflationCalculator {
    newCalculator = GlobalBlobAwareConflationCalculator(
      conflationCalculator = GlobalBlockConflationCalculator(
        lastBlockNumber,
        listOf(mockBlobCalculator),
        emptyList(),
        TracesCountersV1.EMPTY_TRACES_COUNT
      ),
      blobCalculator = mockBlobCalculator,
      batchesLimit = 10U,
      metricsFacade = org.mockito.kotlin.mock<MetricsFacade>(defaultAnswer = Mockito.RETURNS_DEEP_STUBS)
    )
    return newCalculator
  }

  private fun initializeCalculators(lastBlockNumber: ULong) {
    switchCutoffCalculator = spy(SwitchCutoffCalculator(switchBlockNumber))
    oldCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber,
      listOf(switchCutoffCalculator),
      emptyList(),
      TracesCountersV1.EMPTY_TRACES_COUNT
    )
    mockBlobCalculator = mock()
    switchAwareCalculator = SwitchAwareCalculator(oldCalculator, ::newCalculatorProvider, switchBlockNumber)
  }

  @Test
  fun `if switch block number is unknown, blocks are processed by old calculator`() {
    initializeCalculators(lastBlockNumber)
    val block = baseBlockCounters
    switchAwareCalculator.newBlock(
      block
    )
    verify(switchCutoffCalculator, times(1)).appendBlock(eq(block))
    verify(mockBlobCalculator, never()).appendBlock(any())
  }

  @Test
  fun `if new block less than switch block number, blocks are processed by old calculator`() {
    initializeCalculators(lastBlockNumber)
    val block = baseBlockCounters
    switchAwareCalculator.newBlock(
      block
    )
    verify(switchCutoffCalculator, times(1)).appendBlock(eq(block))
    verify(mockBlobCalculator, never()).appendBlock(any())
  }

  @Test
  fun `if new block is switch block number, blocks are received by both calculators`() {
    val blockNumberBeforeSwitch = switchBlockNumber - 1UL
    initializeCalculators(switchBlockNumber - 2UL)
    val block = baseBlockCounters.copy(
      blockNumber = blockNumberBeforeSwitch
    )
    val cutoffBLock =
      block.copy(blockNumber = switchBlockNumber)
    switchAwareCalculator.newBlock(
      block
    )
    switchAwareCalculator.newBlock(
      block.copy(blockNumber = switchBlockNumber)
    )
    verify(switchCutoffCalculator, times(1)).appendBlock(eq(block))
    verify(mockBlobCalculator, times(1)).appendBlock(eq(cutoffBLock))
  }

  @Test
  fun `if new block is after switch block number, blocks are received only by new calculator`() {
    initializeCalculators(switchBlockNumber + 1UL)
    val switchBLockPlus2 =
      baseBlockCounters.copy(
        blockNumber = switchBlockNumber + 2UL
      )
    val switchBlockPlus3 =
      baseBlockCounters.copy(
        blockNumber = switchBlockNumber + 3UL
      )
    switchAwareCalculator.newBlock(
      switchBLockPlus2
    )
    switchAwareCalculator.newBlock(
      switchBlockPlus3
    )
    verify(switchCutoffCalculator, never()).appendBlock(any())
    verify(mockBlobCalculator, times(1)).appendBlock(eq(switchBLockPlus2))
    verify(mockBlobCalculator, times(1)).appendBlock(eq(switchBlockPlus3))
  }

  @Test
  fun `handlers are assigned properly to new calculator when it's created`() {
    initializeCalculators(switchBlockNumber - 2UL)

    var conflationHandlerWasCalled = false
    var blobHandlerWasCalled = false
    switchAwareCalculator.onConflatedBatch {
      conflationHandlerWasCalled = true
      SafeFuture.completedFuture(Unit)
    }
    whenever(mockBlobCalculator.getCompressedData()).thenReturn(ByteArray(0))
    switchAwareCalculator.onBlobCreation {
      blobHandlerWasCalled = true
      SafeFuture.completedFuture(Unit)
    }

    val block = baseBlockCounters.copy(
      switchBlockNumber - 1UL
    )
    switchAwareCalculator.newBlock(
      block
    )
    switchAwareCalculator.newBlock(
      block.copy(blockNumber = switchBlockNumber)
    )

    oldCalculator.handleConflationTrigger(mock())
    newCalculator.handleBatchTrigger(
      ConflationCalculationResult(
        startBlockNumber = switchBlockNumber,
        endBlockNumber = switchBlockNumber,
        conflationTrigger = ConflationTrigger.DATA_LIMIT,
        tracesCounters = TracesCountersV1.EMPTY_TRACES_COUNT
      )
    )

    Assertions.assertThat(conflationHandlerWasCalled).isTrue()
    Assertions.assertThat(blobHandlerWasCalled).isTrue()
  }

  @Test
  fun `if new calculator creation fails, error is being propagated`() {
    switchCutoffCalculator = spy(SwitchCutoffCalculator(switchBlockNumber))
    oldCalculator = GlobalBlockConflationCalculator(
      lastBlockNumber,
      listOf(switchCutoffCalculator),
      emptyList(),
      TracesCountersV1.EMPTY_TRACES_COUNT
    )
    val expectedException = RuntimeException("New calculator creation fails!")
    switchAwareCalculator = SwitchAwareCalculator(
      oldCalculator,
      newCalculatorProvider = { _: ULong -> throw expectedException },
      switchBlockNumber = switchBlockNumber
    )

    Assertions.assertThatThrownBy {
      switchAwareCalculator.newBlock(baseBlockCounters)
    }.isEqualTo(expectedException)
  }

  @Test
  fun `consecutive blocks are accepted and conflations are handled correctly`() {
    val initialBlockNumber = switchBlockNumber - 3UL
    initializeCalculators(initialBlockNumber)

    val block1 =
      baseBlockCounters.copy(
        blockNumber = switchBlockNumber - 2UL
      )
    switchAwareCalculator.newBlock(block1)
    Assertions.assertThat(switchAwareCalculator.lastBlockNumber).isEqualTo(block1.blockNumber)
    verify(switchCutoffCalculator, times(1)).appendBlock(any())
    verify(mockBlobCalculator, never()).appendBlock(any())

    val block2 =
      baseBlockCounters.copy(
        blockNumber = switchBlockNumber - 1UL
      )
    switchAwareCalculator.newBlock(block2)
    Assertions.assertThat(switchAwareCalculator.lastBlockNumber).isEqualTo(block2.blockNumber)
    verify(switchCutoffCalculator, times(2)).appendBlock(any())
    verify(mockBlobCalculator, never()).appendBlock(any())

    val cutoffBlock =
      baseBlockCounters.copy(
        blockNumber = switchBlockNumber
      )
    switchAwareCalculator.newBlock(
      cutoffBlock
    )
    Assertions.assertThat(switchAwareCalculator.lastBlockNumber).isEqualTo(cutoffBlock.blockNumber)
    verify(switchCutoffCalculator, times(3)).appendBlock(any())
    verify(mockBlobCalculator, times(1)).appendBlock(any())
    clearInvocations(switchCutoffCalculator)

    val blockAfterCutoff1 =
      baseBlockCounters.copy(
        blockNumber = switchBlockNumber + 1UL
      )
    switchAwareCalculator.newBlock(
      blockAfterCutoff1
    )
    Assertions.assertThat(switchAwareCalculator.lastBlockNumber).isEqualTo(blockAfterCutoff1.blockNumber)

    verifyNoMoreInteractions(switchCutoffCalculator)
    verify(mockBlobCalculator, times(2)).appendBlock(any())

    val blockAfterCutoff2 =
      baseBlockCounters.copy(
        blockNumber = switchBlockNumber + 2UL
      )
    switchAwareCalculator.newBlock(blockAfterCutoff2)
    verifyNoMoreInteractions(switchCutoffCalculator)
    Assertions.assertThat(switchAwareCalculator.lastBlockNumber)
      .isEqualTo(blockAfterCutoff2.blockNumber)
    verify(mockBlobCalculator, times(3)).appendBlock(any())
  }
}
