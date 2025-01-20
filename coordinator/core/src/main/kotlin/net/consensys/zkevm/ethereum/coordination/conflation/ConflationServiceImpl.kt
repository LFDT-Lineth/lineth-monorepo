package net.consensys.zkevm.ethereum.coordination.conflation

import linea.domain.Block
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.ConflationCalculationResult
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.PriorityBlockingQueue
import java.util.concurrent.TimeUnit

class ConflationServiceImpl(
  private val calculator: TracesConflationCalculator,
  metricsFacade: MetricsFacade
) :
  ConflationService {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private var listener: ConflationHandler = ConflationHandler { SafeFuture.completedFuture<Unit>(null) }
  private val blocksInProgress: MutableList<Block> = mutableListOf()

  data class PayloadAndBlockCounters(
    val block: Block,
    val blockCounters: BlockCounters
  ) : Comparable<PayloadAndBlockCounters> {
    override fun compareTo(other: PayloadAndBlockCounters): Int {
      return this.block.number.compareTo(other.block.number)
    }
  }

  internal val blocksToConflate = PriorityBlockingQueue<PayloadAndBlockCounters>()

  private val blocksCounter = metricsFacade.createCounter(
    category = LineaMetricsCategory.CONFLATION,
    name = "blocks.imported",
    description = "New blocks arriving to conflation service counter"
  )

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.CONFLATION,
      name = "inprogress.blocks",
      description = "Number of blocks in progress of conflation",
      measurementSupplier = { blocksInProgress.size }
    )
    metricsFacade.createGauge(
      category = LineaMetricsCategory.CONFLATION,
      name = "queue.size",
      description = "Number of blocks in conflation queue",
      measurementSupplier = { blocksToConflate.size }
    )
    calculator.onConflatedBatch(this::handleConflation)
  }

  @Synchronized
  internal fun handleConflation(conflation: ConflationCalculationResult): SafeFuture<*> {
    log.debug(
      "new conflation: batch={} trigger={} tracesCounters={} blocksNumbers={}",
      conflation.intervalString(),
      conflation.conflationTrigger,
      conflation.tracesCounters,
      conflation.blocksRange.joinToString(",", "[", "]") { it.toString() }
    )
    val blocksToConflate =
      blocksInProgress
        .filter { it.number in conflation.blocksRange }
        .sortedBy { it.number }
    blocksInProgress.removeAll(blocksToConflate)

    return listener.handleConflatedBatch(BlocksConflation(blocksToConflate, conflation))
      .whenException { th ->
        log.error(
          "Conflation listener failed: batch={} errorMessage={}",
          conflation.intervalString(),
          th.message,
          th
        )
      }
  }

  @Synchronized
  override fun newBlock(block: Block, blockCounters: BlockCounters) {
    require(block.number == blockCounters.blockNumber) {
      "block=${block.number} does not match blockCounters.blockNumber=${blockCounters.blockNumber}"
    }
    blocksCounter.increment()
    log.trace(
      "newBlock={} calculatorLastBlockNumber={} blocksToConflateSize={} blocksInProgressSize={}",
      block.number,
      calculator.lastBlockNumber,
      blocksToConflate.size,
      blocksInProgress.size
    )
    blocksToConflate.add(PayloadAndBlockCounters(block, blockCounters))
    blocksInProgress.add(block)
    log.trace("block {} added to conflation queue", block.number)
    sendBlocksInOrderToTracesCounter()
  }

  private fun sendBlocksInOrderToTracesCounter() {
    var nextBlockNumberToConflate = calculator.lastBlockNumber + 1u
    var nextAvailableBlock = blocksToConflate.peek()

    while (nextAvailableBlock?.block?.number == nextBlockNumberToConflate) {
      nextAvailableBlock = blocksToConflate.poll(100, TimeUnit.MILLISECONDS)
      log.trace(
        "block {} removed from conflation queue and sent to calculator",
        nextAvailableBlock?.block?.number
      )
      calculator.newBlock(nextAvailableBlock.blockCounters)
      nextBlockNumberToConflate = nextAvailableBlock.block.number + 1u
      nextAvailableBlock = blocksToConflate.peek()
    }
  }

  override fun onConflatedBatch(consumer: ConflationHandler) {
    this.listener = consumer
  }
}
