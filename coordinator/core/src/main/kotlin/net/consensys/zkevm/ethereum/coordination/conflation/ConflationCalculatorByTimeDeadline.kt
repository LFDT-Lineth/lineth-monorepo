package net.consensys.zkevm.ethereum.coordination.conflation

import kotlinx.datetime.Clock
import kotlinx.datetime.Instant
import linea.LongRunningService
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.domain.ConflationTrigger
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Timer
import java.util.concurrent.CompletableFuture
import kotlin.concurrent.timer
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

class DeadlineConflationCalculatorRunner(
  private val conflationDeadlineCheckInterval: Duration,
  private val delegate: ConflationCalculatorByTimeDeadline,
) : DeferredTriggerConflationCalculator by delegate, LongRunningService {
  private lateinit var timerInstance: Timer

  override fun start(): CompletableFuture<Unit> {
    if (!this::timerInstance.isInitialized) {
      timerInstance =
        timer(
          name = "conflation-deadline-checker",
          initialDelay = conflationDeadlineCheckInterval.inWholeMilliseconds,
          period = conflationDeadlineCheckInterval.inWholeMilliseconds,
        ) {
          delegate.checkConflationDeadline()
        }
    }
    return SafeFuture.completedFuture(Unit)
  }

  override fun stop(): CompletableFuture<Unit> {
    if (this::timerInstance.isInitialized) {
      timerInstance.cancel()
    }

    return SafeFuture.completedFuture(Unit)
  }
}

class ConflationCalculatorByTimeDeadline(
  private val config: Config,
  private var lastBlockNumber: ULong,
  private val clock: Clock = Clock.System,
  private val latestBlockProvider: SafeBlockProvider,
  val log: Logger = LogManager.getLogger(ConflationCalculatorByTimeDeadline::class.java),
) : DeferredTriggerConflationCalculator {
  data class Config(
    val conflationDeadline: Duration,
    // this is to avoid triggering conflation on the last block while new is being created
    // which is a false positive, network has activity.
    // it should be 2*blocktime
    val conflationDeadlineLastBlockConfirmationDelay: Duration,
  )

  override val id: String = ConflationTrigger.TIME_LIMIT.name
  private var startBlockTimestamp: Instant? = null
  private var conflationTriggerConsumer: ConflationTriggerConsumer = ConflationTriggerConsumer.noopConsumer

  @Synchronized
  fun checkConflationDeadline() {
    val now = clock.now()
    log.trace(
      "Checking conflation deadline: startBlockTime={} timeElapsed={} deadline={}",
      startBlockTimestamp,
      startBlockTimestamp?.let { now.minus(it) } ?: 0.seconds,
      config.conflationDeadline,
    )

    val deadlineReachedForFirstBlockInProgress =
      startBlockTimestamp != null && now > startBlockTimestamp!!.plus(config.conflationDeadline)

    if (!deadlineReachedForFirstBlockInProgress) {
      // deadline not reached yet. Can just return
      return
    }

    latestBlockProvider.getLatestSafeBlockHeader().thenPeek {
      // wait for 2+ block intervals, otherwise if the ticker happens during block creation we get a false positive.

      val noMoreBlocksInL2ChainToConflate =
        it.number == lastBlockNumber && now.minus(config.conflationDeadlineLastBlockConfirmationDelay) > it.timestamp

      if (noMoreBlocksInL2ChainToConflate) {
        log.trace("Conflation trigger: Deadline reached for block {}", lastBlockNumber)
        runCatching { conflationTriggerConsumer.handleConflationTrigger(ConflationTrigger.TIME_LIMIT) }
          .onFailure {
            log.error("Conflation consumer failed: errorMessage: {}", it.message, it)
          }
        reset()
      }
    }.whenException { th ->
      log.warn(
        "SafeBlock request failed. Will Retry conflation deadline on next tick errorMessage={}",
        th.message,
        th,
      )
    }
  }

  override fun checkOverflow(blockCounters: BlockCounters): ConflationCalculator.OverflowTrigger? = null

  @Synchronized
  override fun appendBlock(blockCounters: BlockCounters) {
    this.startBlockTimestamp ?: run {
      this.startBlockTimestamp = blockCounters.blockTimestamp
    }
    this.lastBlockNumber = blockCounters.blockNumber
  }

  @Synchronized
  override fun reset() {
    this.startBlockTimestamp = null
  }

  override fun copyCountersTo(counters: ConflationCounters) {
    // nothing to do here
  }

  @Synchronized
  override fun setConflationTriggerConsumer(conflationTriggerConsumer: ConflationTriggerConsumer) {
    this.conflationTriggerConsumer = conflationTriggerConsumer
  }
}
