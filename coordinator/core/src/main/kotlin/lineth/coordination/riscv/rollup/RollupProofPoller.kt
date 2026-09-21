package lineth.coordination.riscv.rollup

import io.vertx.core.Vertx
import linea.clients.RollupProofResponseV1
import linea.clients.RollupProverClientV1
import linea.domain.BlobData
import linea.domain.BlockIntervalProofIndex
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import lineth.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedDeque
import kotlin.time.Duration
import kotlin.time.Instant

fun interface RollupProofHandler {
  fun acceptNewRollupProof(
    proof: RollupProofResponseV1,
    context: RollupProofPoller.ProofContext,
  ): SafeFuture<*>
}

class RollupProofPoller(
  private val rollupProverClient: RollupProverClientV1,
  private val rollupProofHandler: RollupProofHandler,
  private val vertx: Vertx,
  private val config: Config,
  private val log: Logger = LogManager.getLogger(RollupProofPoller::class.java),
  metricsFacade: MetricsFacade,
) : VertxPeriodicPollingService(
  vertx = vertx,
  name = "RollupProofPoller",
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log,
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {

  data class Config(
    val pollingInterval: Duration,
    val proofPollsPerTick: Int = 20,
  )

  data class ProofContext(
    val proofIndex: BlockIntervalProofIndex,
    val blobsData: List<BlobData>,
    val parentDataRollingHash: ByteArray,
    val dataRollingHash: ByteArray,
    val endOffset: Int,
    val startBlockTimestamp: Instant,
    val endBlockTimestamp: Instant,
    val totalBatchesCount: Int,
  )

  private val proofRequestsInProgress = ConcurrentLinkedDeque<ProofContext>()

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "prover.riscv.rollup.pendingproofs",
      description = "Number of rollup proof requests waiting for responses",
      measurementSupplier = { proofRequestsInProgress.size },
    )
  }

  @Synchronized
  fun addProofInProgress(
    proofIndex: BlockIntervalProofIndex,
    blobsData: List<BlobData>,
    parentDataRollingHash: ByteArray,
    dataRollingHash: ByteArray,
    endOffset: Int,
    startBlockTimestamp: Instant,
    endBlockTimestamp: Instant,
    totalBatchesCount: Int,
  ) {
    proofRequestsInProgress.add(
      ProofContext(
        proofIndex = proofIndex,
        blobsData = blobsData,
        parentDataRollingHash = parentDataRollingHash,
        dataRollingHash = dataRollingHash,
        endOffset = endOffset,
        startBlockTimestamp = startBlockTimestamp,
        endBlockTimestamp = endBlockTimestamp,
        totalBatchesCount = totalBatchesCount,
      ),
    )
  }

  private fun pollProofContext(context: ProofContext): SafeFuture<*> {
    return rollupProverClient.findProofResponse(context.proofIndex).thenCompose { proofResponse ->
      if (proofResponse != null) {
        log.info("rollup proof generated: blocks={}", context.proofIndex.intervalString())
        rollupProofHandler.acceptNewRollupProof(proofResponse, context).thenApply {
          proofRequestsInProgress.remove(context)
        }
      } else {
        SafeFuture.completedFuture(Unit)
      }
    }
  }

  override fun action(): SafeFuture<*> {
    if (proofRequestsInProgress.isEmpty()) return SafeFuture.completedFuture(Unit)
    val iterator = proofRequestsInProgress.iterator()
    val toBePoll = mutableListOf<ProofContext>()
    while (iterator.hasNext() && toBePoll.size < config.proofPollsPerTick) {
      toBePoll.add(iterator.next())
    }
    return SafeFuture.allOf(toBePoll.map { pollProofContext(it) }.stream())
  }
}
