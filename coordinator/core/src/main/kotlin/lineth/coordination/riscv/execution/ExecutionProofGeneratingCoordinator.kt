package lineth.coordination.riscv.execution

import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.runCatching
import io.vertx.core.Vertx
import linea.clients.L2ExecutionProofRequestV1
import linea.clients.L2ExecutionProofResponseV1
import linea.clients.L2ExecutionProverClientV1
import linea.domain.BlockIntervalProofIndex
import linea.domain.BlocksConflation
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import lineth.conflation.ConflationHandler
import lineth.metrics.LineaMetricsCategory
import net.consensys.linea.async.AsyncRetryer
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedDeque
import kotlin.time.Duration

fun interface L2ExecutionRequestBuilder {
  fun build(conflation: BlocksConflation): SafeFuture<L2ExecutionProofRequestV1>
}

fun interface L2ExecutionProofHandler {
  fun acceptNewL2ExecutionProof(proof: L2ExecutionProofResponseV1): SafeFuture<*>
}

class ExecutionProofGeneratingCoordinator(
  private val l2ExecutionProverClient: L2ExecutionProverClientV1,
  private val l2ExecutionRequestBuilder: L2ExecutionRequestBuilder,
  private val l2ExecutionProofHandler: L2ExecutionProofHandler,
  private val vertx: Vertx,
  private val config: Config,
  private val log: Logger = LogManager.getLogger(ExecutionProofGeneratingCoordinator::class.java),
  metricsFacade: MetricsFacade,
) : ConflationHandler,
  VertxPeriodicPollingService(
    vertx = vertx,
    name = "L2ExecutionProofPollingService",
    pollingIntervalMs = config.executionProofPollingInterval.inWholeMilliseconds,
    log = log,
    timerSchedule = TimerSchedule.FIXED_DELAY,
  ) {

  data class Config(
    val conflationAndProofGenerationRetryBackoffDelay: Duration,
    val executionProofPollingInterval: Duration,
    val executionProofPollsPerTick: Int = 200,
  )

  private val proofRequestsInProgress = ConcurrentLinkedDeque<BlockIntervalProofIndex>()

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BATCH,
      name = "prover.pendingproofs",
      description = "Number of l2-execution proof requests waiting for responses",
      measurementSupplier = { proofRequestsInProgress.size },
    )
  }

  private fun pollProofIndex(proofIndex: BlockIntervalProofIndex): SafeFuture<*> {
    return l2ExecutionProverClient.findProofResponse(proofIndex).thenCompose { proofResponse ->
      if (proofResponse != null) {
        log.info("l2-execution proof generated: blocks={}", proofIndex.intervalString())
        l2ExecutionProofHandler.acceptNewL2ExecutionProof(proofResponse).thenApply {
          proofRequestsInProgress.remove(proofIndex)
        }
      } else {
        SafeFuture.completedFuture(Unit)
      }
    }
  }

  override fun action(): SafeFuture<*> {
    if (proofRequestsInProgress.isEmpty()) {
      return SafeFuture.completedFuture(Unit)
    }
    val iterator = proofRequestsInProgress.iterator()
    val proofIndicesToPoll = mutableListOf<BlockIntervalProofIndex>()
    while (iterator.hasNext() && proofIndicesToPoll.size < config.executionProofPollsPerTick) {
      proofIndicesToPoll.add(iterator.next())
    }
    val proofsPollFutures = proofIndicesToPoll.map { pollProofIndex(it) }
    return SafeFuture.allOf(proofsPollFutures.stream())
  }

  override fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*> {
    val blockIntervalString = conflation.conflationResult.intervalString()
    return runCatching {
      log.info(
        "new batch: batch={} trigger={}",
        blockIntervalString,
        conflation.conflationResult.conflationTrigger,
      )
      AsyncRetryer.retry(
        vertx = vertx,
        backoffDelay = config.conflationAndProofGenerationRetryBackoffDelay,
        exceptionConsumer = {
          log.warn(
            "l2-execution proof creation flow failed batch={} will retry in backOff={} errorMessage={}",
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
          "l2-execution proof request failed: batch={} errorMessage={}",
          blockIntervalString,
          th.message,
          th,
        )
      }
  }

  private fun conflationToProofCreation(conflation: BlocksConflation): SafeFuture<*> {
    val blockIntervalString = conflation.conflationResult.intervalString()
    return l2ExecutionRequestBuilder.build(conflation)
      .whenException { th ->
        log.debug(
          "l2-execution request building failed: batch={} errorMessage={}",
          blockIntervalString,
          th.message,
          th,
        )
      }
      .thenCompose { proofRequest ->
        l2ExecutionProverClient.createProofRequest(proofRequest)
          .thenCompose { proofIndex ->
            l2ExecutionProverClient.findProofResponse(proofIndex)
              .thenCompose<Unit> { existingResponse ->
                if (existingResponse != null) {
                  log.info(
                    "batch={} already proven, skipping l2-execution proof response polling",
                    blockIntervalString,
                  )
                  l2ExecutionProofHandler.acceptNewL2ExecutionProof(existingResponse).thenApply { }
                } else {
                  log.info(
                    "l2-execution proof request generated: proofIndex={} batch={}",
                    proofIndex,
                    blockIntervalString,
                  )
                  proofRequestsInProgress.addLast(proofIndex)
                  SafeFuture.completedFuture(Unit)
                }
              }
          }
          .whenException { th ->
            log.debug(
              "l2-execution proof failure: batch={} errorMessage={}",
              blockIntervalString,
              th.message,
              th,
            )
          }
      }
  }
}
