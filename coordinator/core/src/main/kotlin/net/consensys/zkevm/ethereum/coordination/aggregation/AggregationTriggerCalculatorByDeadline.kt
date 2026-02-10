package net.consensys.zkevm.ethereum.coordination.aggregation

import io.vertx.core.Handler
import io.vertx.core.Vertx
import linea.LongRunningService
import net.consensys.zkevm.domain.BlobCounters
import net.consensys.zkevm.domain.BlobsToAggregate
import net.consensys.zkevm.ethereum.coordination.blockcreation.SafeBlockProvider
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.CompletableFuture
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

class AggregationTriggerCalculatorByDeadline(
  private val config: Config,
  private val clock: Clock = Clock.System,
  private val latestBlockProvider: SafeBlockProvider,
) : DeferredAggregationTriggerCalculator {
  data class Config(
    val aggregationDeadline: Duration,
    val noL2ActivityTimeout: Duration,
    val waitForNoL2ActivityToTriggerAggregation: Boolean,
  )

  data class InFlightAggregation(
    val aggregationStartTimeStamp: Instant,
    val blobsToAggregate: BlobsToAggregate,
  )

  private var inFlightAggregation: InFlightAggregation? = null
  private var aggregationTriggerHandler = AggregationTriggerHandler.NOOP_HANDLER
  private val log: Logger = LogManager.getLogger(this::class.java)

  private fun checkDeadlineTriggerCriteria(inFlightAggregation: InFlightAggregation?): SafeFuture<Boolean> {
    val now = clock.now()
    log.trace(
      "checking deadline: inflightAggregation={} timeElapsed={} deadline={}",
      inFlightAggregation,
      inFlightAggregation?.aggregationStartTimeStamp?.let { now.minus(it) } ?: 0.seconds,
      config.aggregationDeadline,
    )
    if (inFlightAggregation == null) {
      return SafeFuture.completedFuture(false)
    }

    val deadlineReached = now > inFlightAggregation.aggregationStartTimeStamp.plus(config.aggregationDeadline)

    if (!deadlineReached) {
      return SafeFuture.completedFuture(false)
    }

    if (!config.waitForNoL2ActivityToTriggerAggregation) {
      return SafeFuture.completedFuture(true)
    }

    // we need to check for NO L2 activity
    return latestBlockProvider.getLatestSafeBlockHeader().whenException { th ->
      log.warn(
        "SafeBlock request failed. Will retry aggregation deadline on next tick errorMessage={}",
        th.message,
        th,
      )
    }.thenApply {
      val noActivityOnL2 = clock.now().minus(config.noL2ActivityTimeout) > it.timestamp
      log.debug(
        "Aggregation deadline checking trigger criteria lastBlockNumber={} latestL2SafeBlock={} noActivityOnL2={}",
        inFlightAggregation.blobsToAggregate.endBlockNumber,
        it.number,
        noActivityOnL2,
      )
      if (it.number == inFlightAggregation.blobsToAggregate.endBlockNumber && noActivityOnL2) {
        true
      } else {
        false
      }
    }
  }

  @Synchronized
  fun checkAggregation(): SafeFuture<Unit> {
    val inFlightAggregationBeforeCheck = this.inFlightAggregation
    return checkDeadlineTriggerCriteria(inFlightAggregationBeforeCheck)
      .thenApply { deadlineTigger ->
        // inFlightAggregation can be updated while we were waiting for the latest safe block
        // trigger blob/proof limiting if criteria met
        if (deadlineTigger && this.inFlightAggregation == inFlightAggregationBeforeCheck) {
          log.info("aggregation deadline reached at block={}", inFlightAggregation!!.blobsToAggregate.endBlockNumber)
          aggregationTriggerHandler.onAggregationTrigger(
            AggregationTrigger(
              aggregationTriggerType = AggregationTriggerType.TIME_LIMIT,
              aggregation = inFlightAggregation!!.blobsToAggregate,
            ),
          )
        }
      }
  }

  @Synchronized
  override fun newBlob(blobCounters: BlobCounters) {
    if (inFlightAggregation == null) {
      inFlightAggregation =
        InFlightAggregation(
          aggregationStartTimeStamp = blobCounters.startBlockTimestamp,
          blobsToAggregate = BlobsToAggregate(blobCounters.startBlockNumber, blobCounters.endBlockNumber),
        )
    } else {
      inFlightAggregation =
        InFlightAggregation(
          aggregationStartTimeStamp = inFlightAggregation!!.aggregationStartTimeStamp,
          blobsToAggregate =
          BlobsToAggregate(
            inFlightAggregation!!.blobsToAggregate.startBlockNumber,
            blobCounters.endBlockNumber,
          ),
        )
    }
  }

  override fun onAggregationTrigger(aggregationTriggerHandler: AggregationTriggerHandler) {
    this.aggregationTriggerHandler = aggregationTriggerHandler
  }

  @Synchronized
  override fun reset() {
    log.trace("Reset on AggregationTriggerCalculatorByDeadline")
    inFlightAggregation = null
  }
}

class AggregationTriggerCalculatorByDeadlineRunner(
  private val vertx: Vertx,
  private val config: Config,
  private val aggregationTriggerByDeadline: AggregationTriggerCalculatorByDeadline,
) : DeferredAggregationTriggerCalculator by aggregationTriggerByDeadline, LongRunningService {
  data class Config(val deadlineCheckInterval: Duration)

  private val log: Logger = LogManager.getLogger(this::class.java)

  private var deadlineCheckerTimerId: Long? = null
  private lateinit var deadlineCheckerAction: Handler<Long>

  override fun start(): CompletableFuture<Unit> {
    if (deadlineCheckerTimerId == null) {
      deadlineCheckerAction =
        Handler<Long> {
          aggregationTriggerByDeadline.checkAggregation().whenComplete { _, error ->
            error?.let {
              log.error("Error in checking for aggregation deadline: errorMessage={}", error.message, error)
            }
            deadlineCheckerTimerId =
              vertx.setTimer(
                config.deadlineCheckInterval.inWholeMilliseconds,
                deadlineCheckerAction,
              )
          }
        }
      deadlineCheckerTimerId = vertx.setTimer(config.deadlineCheckInterval.inWholeMilliseconds, deadlineCheckerAction)
    }
    return SafeFuture.completedFuture(Unit)
  }

  override fun stop(): CompletableFuture<Unit> {
    if (deadlineCheckerTimerId != null) {
      vertx.cancelTimer(deadlineCheckerTimerId!!)
      deadlineCheckerAction = Handler<Long> {}
    }
    return SafeFuture.completedFuture(Unit)
  }
}
