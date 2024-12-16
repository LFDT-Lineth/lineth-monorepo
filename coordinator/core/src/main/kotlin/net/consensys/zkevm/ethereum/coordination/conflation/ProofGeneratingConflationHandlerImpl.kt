package net.consensys.zkevm.ethereum.coordination.conflation

import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.runCatching
import io.vertx.core.Vertx
import net.consensys.linea.async.AsyncRetryer
import net.consensys.zkevm.domain.BlocksConflation
import net.consensys.zkevm.ethereum.coordination.proofcreation.BatchProofHandler
import net.consensys.zkevm.ethereum.coordination.proofcreation.ZkProofCreationCoordinator
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class ProofGeneratingConflationHandlerImpl(
  private val tracesProductionCoordinator: TracesConflationCoordinator,
  private val zkProofProductionCoordinator: ZkProofCreationCoordinator,
  private val batchProofHandler: BatchProofHandler,
  private val vertx: Vertx,
  private val config: Config
) : ConflationHandler {
  private val log: Logger = LogManager.getLogger(this::class.java)

  data class Config(
    val conflationAndProofGenerationRetryInterval: Duration
  )

  override fun handleConflatedBatch(conflation: BlocksConflation): SafeFuture<*> {
    val blockIntervalString = conflation.conflationResult.intervalString()
    return runCatching {
      log.info(
        "new batch: batch={} trigger={} tracesCounters={}",
        blockIntervalString,
        conflation.conflationResult.conflationTrigger,
        conflation.conflationResult.tracesCounters
      )
      AsyncRetryer.retry(
        vertx = vertx,
        backoffDelay = config.conflationAndProofGenerationRetryInterval,
        exceptionConsumer = {
          log.error("Conflation and proof creation flow failed!", it)
        }
      ) {
        conflationToProofCreation(conflation)
      }
    }.getOrElse { error -> SafeFuture.failedFuture<Unit>(error) }
      .whenException { th ->
        log.error(
          "Traces conflation or proof failed: batch={} errorMessage={}",
          blockIntervalString,
          th.message,
          th
        )
      }
  }

  private fun conflationToProofCreation(conflation: BlocksConflation): SafeFuture<*> {
    val blockNumbersAndHash = conflation.blocks.map { it.numberAndHash }
    val blockIntervalString = conflation.conflationResult.intervalString()
    return tracesProductionCoordinator
      .conflateExecutionTraces(blockNumbersAndHash)
      .whenException { th ->
        log.error(
          "Traces conflation failed: batch={} errorMessage={}",
          conflation.conflationResult.intervalString(),
          th.message,
          th
        )
      }
      .thenCompose { blocksTracesConflated: BlocksTracesConflated ->
        log.debug(
          "requesting execution proof: batch={} tracesFile={}",
          blockIntervalString,
          blocksTracesConflated.tracesResponse.tracesFileName
        )
        zkProofProductionCoordinator
          .createZkProof(conflation, blocksTracesConflated)
          .thenPeek {
            log.info("execution proof generated: batch={}", blockIntervalString)
          }
          .whenException { th ->
            log.error(
              "execution proof failure: batch={} errorMessage={}",
              blockIntervalString,
              th.message,
              th
            )
          }
      }
      .thenCompose { batchProofHandler.acceptNewBatch(it) }
  }
}
