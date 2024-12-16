package net.consensys.zkevm.ethereum.coordination.conflation.upgrade

import linea.domain.createBlock
import net.consensys.linea.traces.TracesCountersV1
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.ConflationCalculationResult
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.conflation.ConflationHandler
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.mockito.kotlin.any
import org.mockito.kotlin.eq
import org.mockito.kotlin.mock
import org.mockito.kotlin.never
import org.mockito.kotlin.times
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture

@Suppress("DEPRECATION")
class SwitchAwareConflationHandlerTest {
  lateinit var switchAwareConflationHandler: SwitchAwareConflationHandler
  lateinit var oldHandler: ConflationHandler
  lateinit var newHandler: ConflationHandler
  lateinit var switchProvider: SwitchProvider
  private val switchVersion = SwitchProvider.ProtocolSwitches.DATA_COMPRESSION_PROOF_AGGREGATION
  private val switchBlock = 100UL

  private fun generateArbitraryConflation(startBlockNumber: ULong, blocksLong: UInt): BlocksConflation {
    val executionPayloads = (startBlockNumber..startBlockNumber + blocksLong)
      .map { createBlock(number = it) }

    val conflationCalculationResult = ConflationCalculationResult(
      startBlockNumber = executionPayloads.first().number.toULong(),
      endBlockNumber = executionPayloads.last().number.toULong(),
      conflationTrigger = ConflationTrigger.TRACES_LIMIT,
      tracesCounters = TracesCountersV1.EMPTY_TRACES_COUNT
    )

    return BlocksConflation(
      executionPayloads,
      conflationCalculationResult
    )
  }

  @BeforeEach
  fun initializeHandlers() {
    switchProvider = mock()
    oldHandler = mock()
    newHandler = mock()
    switchAwareConflationHandler = SwitchAwareConflationHandler(oldHandler, newHandler, switchProvider, switchVersion)
    whenever(oldHandler.handleConflatedBatch(any())).thenReturn(SafeFuture.completedFuture(Unit))
    whenever(newHandler.handleConflatedBatch(any())).thenReturn(SafeFuture.completedFuture(Unit))
  }

  @Test
  fun `old handler is called if switch block is unknown`() {
    whenever(switchProvider.getSwitch(eq(switchVersion))).thenReturn(SafeFuture.completedFuture(null))
    val conflation = generateArbitraryConflation(5UL, 5U)
    switchAwareConflationHandler.handleConflatedBatch(conflation)

    verify(newHandler, never()).handleConflatedBatch(any())
    verify(oldHandler, times(1)).handleConflatedBatch(eq(conflation))
  }

  @Test
  fun `new handler is called if conflation is past switch block`() {
    whenever(switchProvider.getSwitch(eq(switchVersion))).thenReturn(SafeFuture.completedFuture(switchBlock))
    val conflation = generateArbitraryConflation(105UL, 5U)
    switchAwareConflationHandler.handleConflatedBatch(conflation)

    verify(newHandler, times(1)).handleConflatedBatch(eq(conflation))
    verify(oldHandler, never()).handleConflatedBatch(any())
  }

  @Test
  fun `new handler isn't used if there was a switch for another version`() {
    whenever(switchProvider.getSwitch(eq(SwitchProvider.ProtocolSwitches.INITIAL_VERSION))).thenReturn(
      SafeFuture.completedFuture(switchBlock)
    )
    whenever(switchProvider.getSwitch(eq(switchVersion))).thenReturn(SafeFuture.completedFuture(null))
    val conflation = generateArbitraryConflation(105UL, 5U)
    switchAwareConflationHandler.handleConflatedBatch(conflation)

    verify(newHandler, never()).handleConflatedBatch(any())
    verify(oldHandler, times(1)).handleConflatedBatch(eq(conflation))
  }

  @Test
  fun `old handler is used if conflation is before switch block`() {
    whenever(switchProvider.getSwitch(eq(switchVersion))).thenReturn(SafeFuture.completedFuture(switchBlock))
    val conflation = generateArbitraryConflation(99UL, 1U)
    switchAwareConflationHandler.handleConflatedBatch(conflation)

    verify(newHandler, never()).handleConflatedBatch(any())
    verify(oldHandler, times(1)).handleConflatedBatch(eq(conflation))
  }

  @Test
  fun `new handler is used if conflation is exactly at the switch block`() {
    whenever(switchProvider.getSwitch(eq(switchVersion))).thenReturn(SafeFuture.completedFuture(switchBlock))
    val conflation = generateArbitraryConflation(switchBlock, 3U)
    switchAwareConflationHandler.handleConflatedBatch(conflation)

    verify(newHandler, times(1)).handleConflatedBatch(eq(conflation))
    verify(oldHandler, never()).handleConflatedBatch(any())
  }
}
