package net.consensys.zkevm.ethereum.coordination.conflation

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.getOrThrow
import com.github.michaelbull.result.runCatching
import io.vertx.core.Vertx
import linea.domain.Block
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.domain.Batch
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.domain.ExecutionProofIndex
import net.consensys.zkevm.ethereum.coordination.proofcreation.BatchProofHandler
import net.consensys.zkevm.ethereum.coordination.proofcreation.ZkProofCreationCoordinator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedDeque
import kotlin.time.Duration

class ProofGeneratingConflationHandlerImpl(
  private val tracesProductionCoordinator: TracesConflationCoordinator,
  private val zkProofProductionCoordinator: ZkProofCreationCoordinator,
  private val batchProofHandler: BatchProofHandler,
  private val vertx: Vertx,
  private val config: Config,
  private val log: Logger = LogManager.getLogger(ProofGeneratingConflationHandlerImpl::class.java),
  metricsFacade: MetricsFacade,
) : ConflationHandler,
  VertxPeriodicPollingService(
    vertx = vertx,
    name = "ExecutionProofPollingService",
    pollingIntervalMs = config.executionProofPollingInterval.inWholeMilliseconds,
    log = log,
    timerSchedule = TimerSchedule.FIXED_DELAY,
  ) {

  data class Config(
    val conflationAndProofGenerationRetryBackoffDelay: Duration,
    val executionProofPollingInterval: Duration,
  )

  private val proofRequestsInProgress = ConcurrentLinkedDeque<ExecutionProofIndex>()

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "prover.pendingproofs",
      description = "Number of execution proof waiting responses",
      measurementSupplier = { proofRequestsInProgress.size },
    )
  }

  override fun action(): SafeFuture<*> {
    return if (proofRequestsInProgress.isNotEmpty()) {
      val proofIndex = proofRequestsInProgress.peekFirst()
      zkProofProductionCoordinator.isZkProofRequestProven(proofIndex).thenCompose { proven ->
        if (proven) {
          val batch = Batch(
            startBlockNumber = proofIndex.startBlockNumber,
            endBlockNumber = proofIndex.endBlockNumber,
          )
          log.info("execution proof generated: batch={}", batch)
          batchProofHandler.acceptNewBatch(batch).thenApply {
            proofRequestsInProgress.remove(proofIndex)
          }
        } else {
          SafeFuture.completedFuture(Unit)
        }
      }
    } else {
      SafeFuture.completedFuture(Unit)
    }
  }

  override fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*> {
    val blockIntervalString = conflation.conflationResult.intervalString()
    return runCatching {
      log.info(
        "new batch: batch={} trigger={} tracesCounters={}",
        blockIntervalString,
        conflation.conflationResult.conflationTrigger,
        conflation.conflationResult.tracesCounters,
      )
      AsyncRetryer.retry(
        vertx = vertx,
        backoffDelay = config.conflationAndProofGenerationRetryBackoffDelay,
        exceptionConsumer = {
          // log failure as warning, but keeps on retrying...
          log.warn(
            "conflation and proof creation flow failed batch={} will retry in backOff={} errorMessage={}",
            blockIntervalString,
            config.conflationAndProofGenerationRetryBackoffDelay,
            it.message,
          )
        },
      ) {
        conflationToProofCreation(conflation)
      }
    }.getOrElse { error -> SafeFuture.failedFuture<Unit>(error) }
      .whenException { th ->
        log.error(
          "traces conflation or proof request failed: batch={} errorMessage={}",
          blockIntervalString,
          th.message,
          th,
        )
      }
  }

  private fun conflationToProofCreation(conflation: BlocksConflation): SafeFuture<*> {
    val blockIntervalString = conflation.conflationResult.intervalString()
    return assertConsecutiveBlocksRange(conflation.blocks)
      .getOrThrow().let { blocksRange ->
        val batch = Batch(conflation.startBlockNumber, conflation.endBlockNumber)
        zkProofProductionCoordinator.isZkProofRequestProven(
          ExecutionProofIndex(
            startBlockNumber = batch.startBlockNumber,
            endBlockNumber = batch.endBlockNumber,
          ),
        ).thenCompose { responseAlreadyDone ->
          if (responseAlreadyDone) {
            log.info("skipping conflation and proof request: batch={} already proven", blockIntervalString)
            batchProofHandler.acceptNewBatch(batch)
          } else {
            tracesProductionCoordinator
              .conflateExecutionTraces(blocksRange)
              .whenException { th ->
                log.debug(
                  "traces conflation failed: batch={} errorMessage={}",
                  blockIntervalString,
                  th.message,
                  th,
                )
              }
              .thenCompose { blocksTracesConflated: BlocksTracesConflated ->
                log.debug(
                  "requesting execution proof: batch={} tracesFile={}",
                  blockIntervalString,
                  blocksTracesConflated.tracesResponse.tracesFileName,
                )
                zkProofProductionCoordinator
                  .createZkProofRequest(conflation, blocksTracesConflated)
                  .thenApply { proofIndex ->
                    log.info("execution proof request generated: proofIndex={}", proofIndex)
                    proofRequestsInProgress.addLast(proofIndex)
                  }
                  .whenException { th ->
                    log.debug(
                      "execution proof failure: batch={} errorMessage={}",
                      blockIntervalString,
                      th.message,
                      th,
                    )
                  }
              }
          }
        }
      }
  }
}

internal fun assertConsecutiveBlocksRange(blocks: List<Block>): Result<ULongRange, IllegalArgumentException> {
  if (blocks.isEmpty()) {
    return Err(IllegalArgumentException("Empty list of blocks"))
  }

  if (blocks.size == 1) {
    return Ok(blocks.first().number..blocks.last().number)
  }

  val sortedByNumber = blocks.sortedBy { it.number }
  val gapFound =
    sortedByNumber
      .zipWithNext { a, b -> b.number - a.number }
      .any { it != 1UL }

  if (gapFound) {
    return Err(IllegalArgumentException("Conflated blocks list has non consecutive blocks!"))
  }
  return Ok(sortedByNumber.first().number..sortedByNumber.last().number)
}
